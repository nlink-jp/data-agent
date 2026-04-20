package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/nlink-jp/data-agent/internal/analysis"
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

// App holds the application state and provides Wails bindings.
// Business logic lives in internal/ packages; this layer only bridges
// the frontend (React) and backend (Go) via Wails bindings and events.
type App struct {
	ctx     context.Context
	cfg     *config.Config
	cases   *casemgr.Manager
	jobs    *job.Manager
	log     *logger.Logger
	backend llm.Backend
}

func NewApp() *App { return &App{} }

// ── Lifecycle ─────────────────────────────────────────────

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	dataDir := config.DefaultDataDir()

	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		fmt.Printf("Warning: config load failed: %v\n", err)
		cfg = config.DefaultConfig()
	}
	a.cfg = cfg

	logEmitter := func(entry logger.Entry) {
		wailsRuntime.EventsEmit(ctx, "log:entry", entry)
	}
	a.log, err = logger.New(filepath.Join(dataDir, "logs"), logEmitter)
	if err != nil {
		fmt.Printf("Warning: logger init failed: %v\n", err)
	} else {
		a.log.SetLevel(logger.LevelDebug)
	}

	a.cases, err = casemgr.NewManager(dataDir)
	if err != nil {
		a.log.Error("case manager init failed", logger.F("error", err.Error()))
	} else {
		if count := a.cases.ResetGhostStatus(); count > 0 {
			a.log.Info("reset ghost status", logger.F("count", fmt.Sprintf("%d", count)))
		}
	}

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

	a.backend, err = llm.NewBackend(cfg)
	if err != nil {
		a.log.Warn("LLM backend init failed, will retry on first use", logger.F("error", err.Error()))
	} else {
		a.log.Info("LLM backend initialized", logger.F("backend", a.backend.Name()))
	}

	a.log.Info("data-agent started", logger.F("data_dir", dataDir))
}

func (a *App) shutdown(ctx context.Context) {
	if a.log != nil {
		a.log.Info("data-agent shutting down")
		a.log.Close()
	}
}

// ── Case Management ───────────────────────────────────────

func (a *App) CreateCase(name string) (*casemgr.CaseInfo, error) {
	c, err := a.cases.Create(name)
	if err != nil {
		return nil, err
	}
	a.log.Info("case created", logger.F("id", c.ID), logger.F("name", name))
	return c, nil
}

func (a *App) ListCases() ([]casemgr.CaseInfo, error) { return a.cases.List() }

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

func (a *App) DeleteCase(id string) error { return a.cases.Delete(id) }

// ── Data Management ────────────────────────────────��──────

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

// ── Session Management ─────────────────────────────���──────

func (a *App) CreateSession(caseID string) (*session.Session, error) {
	sess := session.New(caseID)
	if err := sess.Save(a.sessionsDir(caseID)); err != nil {
		return nil, err
	}
	a.log.Info("session created", logger.F("session", sess.ID), logger.F("case", caseID))
	return sess, nil
}

func (a *App) ListSessions(caseID string) ([]session.Session, error) {
	return session.ListSessions(a.sessionsDir(caseID))
}

func (a *App) GetSession(caseID, sessionID string) (*session.Session, error) {
	return session.Load(a.sessionsDir(caseID), sessionID)
}

func (a *App) ReopenSession(caseID, sessionID string) error {
	sess, err := a.GetSession(caseID, sessionID)
	if err != nil {
		return err
	}
	if err := sess.Reopen(); err != nil {
		return err
	}
	sess.AddMessage("system", "Session reopened for additional analysis.")
	if err := sess.Save(a.sessionsDir(caseID)); err != nil {
		return err
	}
	a.log.Info("session reopened", logger.F("session", sessionID))
	wailsRuntime.EventsEmit(a.ctx, "session:phase", map[string]any{"session": sessionID, "phase": "planning"})
	if sess.Plan != nil {
		wailsRuntime.EventsEmit(a.ctx, "session:plan_detected", map[string]any{
			"session": sessionID, "objective": sess.Plan.Objective, "perspectives": len(sess.Plan.Perspectives),
		})
	}
	return nil
}

func (a *App) DeleteSession(caseID, sessionID string) error {
	reportsDir := a.reportsDir(caseID)
	reports, err := report.ListReports(reportsDir)
	if err != nil {
		a.log.Warn("list reports for cascade delete", logger.F("error", err.Error()))
	}
	for _, r := range reports {
		if r.SessionID == sessionID {
			if err := report.DeleteReport(reportsDir, r.ID); err != nil {
				a.log.Warn("cascade delete report failed", logger.F("report", r.ID), logger.F("error", err.Error()))
			} else {
				a.log.Info("cascade deleted report", logger.F("report", r.ID))
			}
		}
	}
	if err := session.DeleteSession(a.sessionsDir(caseID), sessionID); err != nil {
		return err
	}
	a.log.Info("session deleted", logger.F("session", sessionID))
	return nil
}

func (a *App) RenameSession(caseID, sessionID, newTitle string) error {
	sess, err := a.GetSession(caseID, sessionID)
	if err != nil {
		return err
	}
	if sess.Plan == nil {
		sess.Plan = &session.Plan{}
	}
	sess.Plan.Objective = newTitle
	return sess.Save(a.sessionsDir(caseID))
}

// ── Analysis ────────────────────────────────���─────────────

func (a *App) SendMessage(caseID, sessionID, content string) error {
	sess, err := a.GetSession(caseID, sessionID)
	if err != nil {
		return err
	}
	sess.AddMessage("user", content)

	if a.backend == nil {
		return fmt.Errorf("LLM backend not initialized")
	}

	engine, err := a.cases.Engine(caseID)
	if err != nil {
		return err
	}

	systemPrompt := analysis.BuildPlanningSystemPrompt(engine.SchemaContext())

	// Build messages for LLM (exclude non-chat roles)
	var messages []llm.Message
	for _, msg := range sess.Chat {
		switch msg.Role {
		case "user", "assistant", "system":
			messages = append(messages, llm.Message{Role: msg.Role, Content: msg.Content})
		}
	}

	// Stream response (with size limit to prevent memory exhaustion)
	const maxResponseBytes = 10 * 1024 * 1024 // 10MB
	var fullResponse string
	err = a.backend.ChatStream(a.ctx, &llm.ChatRequest{
		SystemPrompt: systemPrompt,
		Messages:     messages,
	}, func(token string, done bool) {
		if !done {
			if len(fullResponse)+len(token) > maxResponseBytes {
				return // silently stop accumulating if response is too large
			}
			fullResponse += token
			wailsRuntime.EventsEmit(a.ctx, "chat:stream", map[string]any{"session": sessionID, "token": token})
		}
	})
	if err != nil {
		return fmt.Errorf("LLM error: %w", err)
	}

	fullResponse = llm.StripArtifacts(fullResponse)
	sess.AddMessage("assistant", fullResponse)

	// Plan extraction
	if sess.Phase == session.PhasePlanning {
		if plan, _ := session.ExtractPlanJSON(fullResponse); plan != nil {
			sess.SetPlan(plan)
			a.log.Info("plan detected", logger.F("objective", truncateRunes(plan.Objective, 60)))
		}
	}

	// Save then emit events
	if err := sess.Save(a.sessionsDir(caseID)); err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	if sess.Plan != nil && sess.Phase == session.PhasePlanning {
		wailsRuntime.EventsEmit(a.ctx, "session:plan_detected", map[string]any{
			"session": sessionID, "objective": sess.Plan.Objective, "perspectives": len(sess.Plan.Perspectives),
		})
	}
	wailsRuntime.EventsEmit(a.ctx, "chat:complete", map[string]any{"session": sessionID, "content": fullResponse})
	return nil
}

func (a *App) ApprovePlan(caseID, sessionID string) error {
	sess, err := a.GetSession(caseID, sessionID)
	if err != nil {
		return err
	}
	if err := sess.ApprovePlan(); err != nil {
		return err
	}
	if err := sess.Save(a.sessionsDir(caseID)); err != nil {
		return err
	}
	a.log.Info("plan approved, starting execution", logger.F("session", sessionID))
	wailsRuntime.EventsEmit(a.ctx, "session:phase", map[string]any{"session": sessionID, "phase": "execution"})

	go a.runExecution(caseID, sessionID)
	return nil
}

func (a *App) ExecuteSQL(caseID, sessionID, sql string) (*dbengine.QueryResult, error) {
	engine, err := a.cases.Engine(caseID)
	if err != nil {
		a.log.Error("SQL execution failed: engine unavailable", logger.F("case", caseID), logger.F("error", err.Error()))
		return nil, err
	}

	a.log.Info("executing SQL", logger.F("sql", truncateRunes(sql, 100)))
	result, err := engine.Execute(sql)
	if err != nil {
		a.log.Warn("SQL error", logger.F("error", err.Error()))
		return result, err
	}
	a.log.Info("SQL completed", logger.F("rows", fmt.Sprintf("%d", result.RowCount)), logger.F("duration", result.Duration.String()))

	if sessionID != "" {
		if sess, serr := a.GetSession(caseID, sessionID); serr == nil {
			sess.RecordExec(session.ExecEntry{
				StepID: "adhoc", Type: session.StepTypeSQL, SQL: sql,
				Result: &session.StepResult{Summary: fmt.Sprintf("%d rows returned", result.RowCount)},
			})
			sess.Save(a.sessionsDir(caseID))
		}
	}
	return result, nil
}

// ── Execution (delegates to session.Executor) ─────────────

func (a *App) runExecution(caseID, sessionID string) {
	sess, err := a.GetSession(caseID, sessionID)
	if err != nil {
		a.log.Error("load session for execution", logger.F("error", err.Error()))
		return
	}
	engine, err := a.cases.Engine(caseID)
	if err != nil {
		a.log.Error("engine unavailable for execution", logger.F("error", err.Error()))
		return
	}

	executor := &session.Executor{
		Engine:  engine,
		Backend: a.backend,
		Config: session.ExecutorConfig{
			MaxSQLRetries:       2,
			MaxRecordsPerWindow: a.cfg.Analysis.MaxRecordsPerWindow,
			OverlapRatio:        a.cfg.Analysis.OverlapRatio,
			MaxFindings:         a.cfg.Analysis.MaxFindings,
			ContextLimit:        a.cfg.Analysis.ContextLimit,
		},
		Events: &wailsEventSink{ctx: a.ctx, log: a.log},
	}

	saveFunc := func() error { return sess.Save(a.sessionsDir(caseID)) }

	if err := executor.RunPlan(a.ctx, sess, saveFunc); err != nil {
		a.log.Error("plan execution failed", logger.F("error", err.Error()))
		return
	}

	a.log.Info("execution complete, generating report", logger.F("session", sessionID))

	saveReport := func(reviewContent string) (string, string, error) {
		r, err := report.GenerateFromSession(sess)
		if err != nil {
			return "", "", err
		}
		if err := r.SaveToCase(a.reportsDir(caseID)); err != nil {
			return "", "", err
		}
		return r.ID, r.Title, nil
	}

	if err := executor.GenerateReviewAndFinalize(a.ctx, sess, saveFunc, saveReport); err != nil {
		a.log.Error("finalize failed", logger.F("error", err.Error()))
	}
}

// ── Jobs ──────────────────────────────────────────────────

func (a *App) CancelJob(jobID string) error        { return a.jobs.Cancel(jobID) }
func (a *App) GetJobStatus(jobID string) (*job.Job, error) { return a.jobs.Get(jobID) }

// ── Reports ───────────────────────────────────────────────

func (a *App) ListReports(caseID string) ([]report.Report, error) {
	return report.ListReports(a.reportsDir(caseID))
}

func (a *App) ExportReport(caseID, reportID, dest string) error {
	reports, err := report.ListReports(a.reportsDir(caseID))
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

func (a *App) DeleteReport(caseID, reportID string) error {
	if err := report.DeleteReport(a.reportsDir(caseID), reportID); err != nil {
		return err
	}
	a.log.Info("report deleted", logger.F("report", reportID))
	return nil
}

func (a *App) RenameReport(caseID, reportID, newTitle string) error {
	return report.RenameReport(a.reportsDir(caseID), reportID, newTitle)
}

// ── Config ────────────────────────────────────────────────

func (a *App) GetConfig() *config.Config { return a.cfg }

func (a *App) SaveConfig(cfg *config.Config) error {
	if err := config.Save(cfg, config.DefaultConfigPath()); err != nil {
		return err
	}
	a.cfg = cfg
	backend, err := llm.NewBackend(cfg)
	if err != nil {
		a.log.Warn("LLM backend reinit failed", logger.F("error", err.Error()))
		return nil
	}
	a.backend = backend
	a.log.Info("config saved, LLM backend reinitialized", logger.F("backend", backend.Name()))
	return nil
}

// ── Path helpers ─────────────────────────────��────────────

func (a *App) sessionsDir(caseID string) string {
	return filepath.Join(a.cases.CaseDir(caseID), "sessions")
}

func (a *App) reportsDir(caseID string) string {
	return filepath.Join(a.cases.CaseDir(caseID), "reports")
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}

// ── Wails EventSink implementation ────────────────────────

// wailsEventSink bridges session.EventSink to Wails runtime events.
type wailsEventSink struct {
	ctx context.Context
	log *logger.Logger
}

func (w *wailsEventSink) OnStepStart(sessionID, stepID string, stepType session.StepType, description string) {
	wailsRuntime.EventsEmit(w.ctx, "chat:step_progress", map[string]any{
		"session": sessionID, "event": "start", "id": stepID, "type": string(stepType), "description": description,
	})
}

func (w *wailsEventSink) OnStepDone(sessionID, stepID string, stepType session.StepType, description, summary string) {
	wailsRuntime.EventsEmit(w.ctx, "chat:step_progress", map[string]any{
		"session": sessionID, "event": "done", "id": stepID, "type": string(stepType), "description": description, "summary": summary,
	})
}

func (w *wailsEventSink) OnStepFailed(sessionID, stepID string, stepType session.StepType, description, errMsg string) {
	wailsRuntime.EventsEmit(w.ctx, "chat:step_progress", map[string]any{
		"session": sessionID, "event": "failed", "id": stepID, "type": string(stepType), "description": description, "error": errMsg,
	})
}

func (w *wailsEventSink) OnStepSkipped(sessionID, stepID, description string) {
	wailsRuntime.EventsEmit(w.ctx, "chat:step_progress", map[string]any{
		"session": sessionID, "event": "skipped", "id": stepID, "description": description,
	})
}

func (w *wailsEventSink) OnStepInfo(sessionID, stepID, message string) {
	wailsRuntime.EventsEmit(w.ctx, "chat:step_progress", map[string]any{
		"session": sessionID, "event": "info", "id": stepID, "description": message,
	})
}

func (w *wailsEventSink) OnPhaseChange(sessionID string, phase session.Phase) {
	wailsRuntime.EventsEmit(w.ctx, "session:phase", map[string]any{"session": sessionID, "phase": string(phase)})
}

func (w *wailsEventSink) OnReportStart(sessionID, title string) {
	wailsRuntime.EventsEmit(w.ctx, "chat:report_start", map[string]any{"session": sessionID, "title": title})
}

func (w *wailsEventSink) OnStream(sessionID, token string) {
	wailsRuntime.EventsEmit(w.ctx, "chat:stream", map[string]any{"session": sessionID, "token": token})
}

func (w *wailsEventSink) OnComplete(sessionID string) {
	wailsRuntime.EventsEmit(w.ctx, "chat:complete", map[string]any{"session": sessionID, "content": ""})
}

func (w *wailsEventSink) OnReportReady(sessionID, reportID, title string) {
	wailsRuntime.EventsEmit(w.ctx, "session:report_ready", map[string]any{
		"session": sessionID, "report_id": reportID, "title": title,
	})
}

func (w *wailsEventSink) OnLog(level, msg string, fields map[string]string) {
	var lf []logger.Field
	for k, v := range fields {
		lf = append(lf, logger.F(k, v))
	}
	switch level {
	case "error":
		w.log.Error(msg, lf...)
	case "warn":
		w.log.Warn(msg, lf...)
	case "debug":
		w.log.Debug(msg, lf...)
	default:
		w.log.Info(msg, lf...)
	}
}
