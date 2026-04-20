package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/nlink-jp/data-agent/internal/casemgr"
	"github.com/nlink-jp/data-agent/internal/config"
	"github.com/nlink-jp/data-agent/internal/dbengine"
	"github.com/nlink-jp/data-agent/internal/job"
	"github.com/nlink-jp/data-agent/internal/llm"
	"github.com/nlink-jp/data-agent/internal/logger"
	"github.com/nlink-jp/data-agent/internal/report"
	"github.com/nlink-jp/data-agent/internal/session"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct holds the application state and Wails bindings.
type App struct {
	ctx     context.Context
	cfg     *config.Config
	cases   *casemgr.Manager
	jobs    *job.Manager
	log     *logger.Logger
	backend llm.Backend
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	dataDir := config.DefaultDataDir()

	// Load config
	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		fmt.Printf("Warning: config load failed: %v\n", err)
		cfg = config.DefaultConfig()
	}
	a.cfg = cfg

	// Initialize logger
	logEmitter := func(entry logger.Entry) {
		wailsRuntime.EventsEmit(ctx, "log:entry", entry)
	}
	a.log, err = logger.New(filepath.Join(dataDir, "logs"), logEmitter)
	if err != nil {
		fmt.Printf("Warning: logger init failed: %v\n", err)
	}

	// Initialize case manager
	a.cases, err = casemgr.NewManager(dataDir)
	if err != nil {
		a.log.Error("case manager init failed", logger.F("error", err.Error()))
	}

	// Initialize job manager
	a.jobs = job.NewManager(
		func(jobID string, progress float64) {
			wailsRuntime.EventsEmit(ctx, "job:progress", map[string]any{"id": jobID, "progress": progress})
		},
		func(jobID string, err error) {
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			wailsRuntime.EventsEmit(ctx, "job:complete", map[string]any{"id": jobID, "error": errStr})
		},
	)

	// Initialize LLM backend
	a.backend, err = llm.NewBackend(cfg)
	if err != nil {
		a.log.Warn("LLM backend init failed, will retry on first use", logger.F("error", err.Error()))
	} else {
		a.log.Info("LLM backend initialized", logger.F("backend", a.backend.Name()))
	}

	a.log.Info("data-agent started", logger.F("data_dir", dataDir))
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	if a.log != nil {
		a.log.Info("data-agent shutting down")
		a.log.Close()
	}
}

// --- Case Management ---

func (a *App) CreateCase(name string) (*casemgr.Case, error) {
	c, err := a.cases.Create(name)
	if err != nil {
		return nil, err
	}
	a.log.Info("case created", logger.F("id", c.ID), logger.F("name", name))
	return c, nil
}

func (a *App) ListCases() ([]casemgr.Case, error) {
	return a.cases.List()
}

func (a *App) OpenCase(id string) error {
	if err := a.cases.Open(id); err != nil {
		return err
	}
	a.log.Info("case opened", logger.F("id", id))
	wailsRuntime.EventsEmit(a.ctx, "case:updated", map[string]any{"id": id, "status": "open"})
	return nil
}

func (a *App) CloseCase(id string) error {
	if err := a.cases.Close(id); err != nil {
		return err
	}
	a.log.Info("case closed", logger.F("id", id))
	wailsRuntime.EventsEmit(a.ctx, "case:updated", map[string]any{"id": id, "status": "closed"})
	return nil
}

func (a *App) DeleteCase(id string) error {
	return a.cases.Delete(id)
}

// --- Data Management ---

func (a *App) ImportData(caseID, path, tableName string) error {
	engine, err := a.cases.Engine(caseID)
	if err != nil {
		return err
	}
	if err := engine.Import(path, tableName); err != nil {
		return err
	}
	a.log.Info("data imported", logger.F("case", caseID), logger.F("table", tableName))
	return nil
}

func (a *App) RemoveTable(caseID, tableName string) error {
	engine, err := a.cases.Engine(caseID)
	if err != nil {
		return err
	}
	return engine.RemoveTable(tableName)
}

func (a *App) GetTables(caseID string) ([]*dbengine.TableMeta, error) {
	engine, err := a.cases.Engine(caseID)
	if err != nil {
		return nil, err
	}
	return engine.Tables(), nil
}

// --- Session Management ---

func (a *App) CreateSession(caseID string) (*session.Session, error) {
	sess := session.New(caseID)
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
	if err := sess.Save(sessionsDir); err != nil {
		return nil, err
	}
	a.log.Info("session created", logger.F("session", sess.ID), logger.F("case", caseID))
	return sess, nil
}

func (a *App) ListSessions(caseID string) ([]session.Session, error) {
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
	return session.ListSessions(sessionsDir)
}

func (a *App) GetSession(caseID, sessionID string) (*session.Session, error) {
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
	return session.Load(sessionsDir, sessionID)
}

// --- Analysis (within Session) ---

func (a *App) SendMessage(caseID, sessionID, content string) error {
	sess, err := a.GetSession(caseID, sessionID)
	if err != nil {
		return err
	}

	sess.AddMessage("user", content)

	if a.backend == nil {
		return fmt.Errorf("LLM backend not initialized")
	}

	// Build prompt based on phase
	engine, err := a.cases.Engine(caseID)
	if err != nil {
		return err
	}
	schemaCtx := engine.SchemaContext()

	var systemPrompt string
	switch sess.Phase {
	case session.PhasePlanning:
		systemPrompt = buildPlanningPrompt(schemaCtx)
	case session.PhaseReview:
		systemPrompt = buildReviewPrompt()
	default:
		systemPrompt = buildPlanningPrompt(schemaCtx)
	}

	// Build messages for LLM
	var messages []llm.Message
	for _, msg := range sess.Chat {
		messages = append(messages, llm.Message{Role: msg.Role, Content: msg.Content})
	}

	// Stream response
	var fullResponse string
	err = a.backend.ChatStream(a.ctx, &llm.ChatRequest{
		SystemPrompt: systemPrompt,
		Messages:     messages,
	}, func(token string, done bool) {
		if !done {
			fullResponse += token
			wailsRuntime.EventsEmit(a.ctx, "chat:stream", map[string]any{
				"session": sessionID,
				"token":   token,
			})
		} else {
			wailsRuntime.EventsEmit(a.ctx, "chat:complete", map[string]any{
				"session": sessionID,
				"content": fullResponse,
			})
		}
	})
	if err != nil {
		return fmt.Errorf("LLM error: %w", err)
	}

	sess.AddMessage("assistant", fullResponse)

	// Save session
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
	return sess.Save(sessionsDir)
}

func (a *App) ApprovePlan(caseID, sessionID string) error {
	sess, err := a.GetSession(caseID, sessionID)
	if err != nil {
		return err
	}
	if err := sess.ApprovePlan(); err != nil {
		return err
	}
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
	if err := sess.Save(sessionsDir); err != nil {
		return err
	}
	a.log.Info("plan approved", logger.F("session", sessionID))
	wailsRuntime.EventsEmit(a.ctx, "session:phase", map[string]any{"session": sessionID, "phase": "execution"})
	return nil
}

func (a *App) ExecuteSQL(caseID, sessionID, sql string) (*dbengine.QueryResult, error) {
	engine, err := a.cases.Engine(caseID)
	if err != nil {
		return nil, err
	}
	result, err := engine.Execute(sql)
	if err != nil {
		return result, err
	}

	// Record in exec log if we have a session
	if sessionID != "" {
		sess, serr := a.GetSession(caseID, sessionID)
		if serr == nil {
			sess.RecordExec(session.ExecEntry{
				StepID: "adhoc",
				Type:   session.StepTypeSQL,
				SQL:    sql,
				Result: &session.StepResult{Summary: fmt.Sprintf("%d rows returned", result.RowCount)},
			})
			sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
			sess.Save(sessionsDir)
		}
	}

	return result, nil
}

func (a *App) RequestAdditionalAnalysis(caseID, sessionID string) error {
	sess, err := a.GetSession(caseID, sessionID)
	if err != nil {
		return err
	}
	if err := sess.RequestAdditionalAnalysis(); err != nil {
		return err
	}
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
	if err := sess.Save(sessionsDir); err != nil {
		return err
	}
	wailsRuntime.EventsEmit(a.ctx, "session:phase", map[string]any{"session": sessionID, "phase": "planning"})
	return nil
}

func (a *App) FinalizeSession(caseID, sessionID string) (*report.Report, error) {
	sess, err := a.GetSession(caseID, sessionID)
	if err != nil {
		return nil, err
	}

	r, err := report.GenerateFromSession(sess)
	if err != nil {
		return nil, err
	}

	reportsDir := filepath.Join(a.cases.CaseDir(caseID), "reports")
	if err := r.SaveToCase(reportsDir); err != nil {
		return nil, err
	}

	sess.Finalize()
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
	sess.Save(sessionsDir)

	a.log.Info("session finalized", logger.F("session", sessionID), logger.F("report", r.ID))
	wailsRuntime.EventsEmit(a.ctx, "session:phase", map[string]any{"session": sessionID, "phase": "done"})
	return r, nil
}

// --- Jobs ---

func (a *App) CancelJob(jobID string) error {
	return a.jobs.Cancel(jobID)
}

func (a *App) GetJobStatus(jobID string) (*job.Job, error) {
	return a.jobs.Get(jobID)
}

// --- Reports ---

func (a *App) ListReports(caseID string) ([]report.Report, error) {
	reportsDir := filepath.Join(a.cases.CaseDir(caseID), "reports")
	return report.ListReports(reportsDir)
}

func (a *App) ExportReport(caseID, reportID, dest string) error {
	reportsDir := filepath.Join(a.cases.CaseDir(caseID), "reports")
	reports, err := report.ListReports(reportsDir)
	if err != nil {
		return err
	}
	for _, r := range reports {
		if r.ID == reportID {
			return r.ExportFile(dest)
		}
	}
	return fmt.Errorf("report %s not found", reportID)
}

// --- Config ---

func (a *App) GetConfig() *config.Config {
	return a.cfg
}

func (a *App) SaveConfig(cfg *config.Config) error {
	if err := config.Save(cfg, config.DefaultConfigPath()); err != nil {
		return err
	}
	a.cfg = cfg

	// Reinitialize LLM backend
	backend, err := llm.NewBackend(cfg)
	if err != nil {
		a.log.Warn("LLM backend reinit failed", logger.F("error", err.Error()))
		return nil // Don't fail config save for LLM init failure
	}
	a.backend = backend
	a.log.Info("config saved, LLM backend reinitialized", logger.F("backend", backend.Name()))
	return nil
}

// --- Helpers ---

func buildPlanningPrompt(schemaCtx string) string {
	return `You are a data analysis planner.
Collaborate with the user to build a structured investigation plan.

## Database Schema
` + schemaCtx + `

## Step Types
- sql: Execute a SQL query
- interpret: LLM interprets previous step results
- aggregate: LLM synthesizes multiple results

Respond in the user's language. When the plan is ready, output structured JSON.`
}

func buildReviewPrompt() string {
	return `You are a data analysis reviewer.
Synthesize the analysis results and provide:
1. Key findings summary
2. Areas needing additional analysis
3. Conclusions

Respond in the user's language.`
}
