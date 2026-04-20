package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

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
	} else {
		a.log.SetLevel(logger.LevelDebug)
	}

	// Initialize case manager
	a.cases, err = casemgr.NewManager(dataDir)
	if err != nil {
		a.log.Error("case manager init failed", logger.F("error", err.Error()))
	} else {
		// Reset ghost "open" status from unclean shutdown
		a.cases.ResetGhostStatus()
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

func (a *App) CreateCase(name string) (*casemgr.CaseInfo, error) {
	c, err := a.cases.Create(name)
	if err != nil {
		return nil, err
	}
	a.log.Info("case created", logger.F("id", c.ID), logger.F("name", name))
	return c, nil
}

func (a *App) ListCases() ([]casemgr.CaseInfo, error) {
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

func (a *App) ReopenSession(caseID, sessionID string) error {
	sess, err := a.GetSession(caseID, sessionID)
	if err != nil {
		return err
	}
	if err := sess.Reopen(); err != nil {
		return err
	}
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
	if err := sess.Save(sessionsDir); err != nil {
		return err
	}
	a.log.Info("session reopened", logger.F("session", sessionID))
	wailsRuntime.EventsEmit(a.ctx, "session:phase", map[string]any{"session": sessionID, "phase": "planning"})
	return nil
}

func (a *App) DeleteSession(caseID, sessionID string) error {
	// Delete associated reports first
	reportsDir := filepath.Join(a.cases.CaseDir(caseID), "reports")
	reports, _ := report.ListReports(reportsDir)
	for _, r := range reports {
		if r.SessionID == sessionID {
			report.DeleteReport(reportsDir, r.ID)
			a.log.Info("cascade deleted report", logger.F("report", r.ID), logger.F("session", sessionID))
		}
	}

	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
	if err := session.DeleteSession(sessionsDir, sessionID); err != nil {
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
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
	return sess.Save(sessionsDir)
}

func (a *App) DeleteReport(caseID, reportID string) error {
	reportsDir := filepath.Join(a.cases.CaseDir(caseID), "reports")
	if err := report.DeleteReport(reportsDir, reportID); err != nil {
		return err
	}
	a.log.Info("report deleted", logger.F("report", reportID))
	return nil
}

func (a *App) RenameReport(caseID, reportID, newTitle string) error {
	reportsDir := filepath.Join(a.cases.CaseDir(caseID), "reports")
	return report.RenameReport(reportsDir, reportID, newTitle)
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
	systemPrompt = buildPlanningPrompt(schemaCtx)

	// Build messages for LLM
	var messages []llm.Message
	for _, msg := range sess.Chat {
		messages = append(messages, llm.Message{Role: msg.Role, Content: msg.Content})
	}

	// Always stream for responsiveness. Plan JSON extraction happens after completion.
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
		}
	})
	if err != nil {
		return fmt.Errorf("LLM error: %w", err)
	}

	sess.AddMessage("assistant", fullResponse)

	// In planning phase, try to extract a structured plan from the response
	if sess.Phase == session.PhasePlanning {
		a.log.Debug("attempting plan extraction",
			logger.F("response_len", fmt.Sprintf("%d", len(fullResponse))),
			logger.F("has_json_fence", fmt.Sprintf("%v", strings.Contains(fullResponse, "```json"))),
			logger.F("has_objective", fmt.Sprintf("%v", strings.Contains(fullResponse, "objective"))),
		)
		plan, _ := session.ExtractPlanJSON(fullResponse)
		if plan != nil {
			sess.SetPlan(plan)
			a.log.Info("plan detected",
				logger.F("objective", truncate(plan.Objective, 60)),
				logger.F("perspectives", fmt.Sprintf("%d", len(plan.Perspectives))),
			)
		} else {
			a.log.Debug("no plan extracted from response")
		}
	}

	// Save session FIRST, then emit events (frontend loads from disk)
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
	if err := sess.Save(sessionsDir); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	// Now emit events — session is persisted, loadSession() will see latest state
	if sess.Plan != nil && sess.Phase == session.PhasePlanning {
		wailsRuntime.EventsEmit(a.ctx, "session:plan_detected", map[string]any{
			"session":      sessionID,
			"objective":    sess.Plan.Objective,
			"perspectives": len(sess.Plan.Perspectives),
		})
	}
	wailsRuntime.EventsEmit(a.ctx, "chat:complete", map[string]any{
		"session": sessionID,
		"content": fullResponse,
	})

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
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")
	if err := sess.Save(sessionsDir); err != nil {
		return err
	}
	a.log.Info("plan approved, starting execution", logger.F("session", sessionID))
	wailsRuntime.EventsEmit(a.ctx, "session:phase", map[string]any{"session": sessionID, "phase": "execution"})

	// Execute plan steps in background
	go a.executePlan(caseID, sessionID)
	return nil
}

// executePlan runs all plan steps sequentially.
func (a *App) executePlan(caseID, sessionID string) {
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")

	sess, err := a.GetSession(caseID, sessionID)
	if err != nil {
		a.log.Error("load session for execution", logger.F("error", err.Error()))
		return
	}

	engine, err := a.cases.Engine(caseID)
	if err != nil {
		a.log.Error("engine not available for execution", logger.F("error", err.Error()))
		return
	}

	totalSteps := 0
	for _, p := range sess.Plan.Perspectives {
		totalSteps += len(p.Steps)
	}
	completedSteps := 0

	for pi := range sess.Plan.Perspectives {
		p := &sess.Plan.Perspectives[pi]
		for si := range p.Steps {
			step := &p.Steps[si]

			// Check dependencies met
			if !a.dependenciesMet(sess, step) {
				step.Status = session.StepSkipped
				a.log.Warn("step skipped (unmet deps)", logger.F("step", step.ID))
				completedSteps++
				continue
			}

			step.Status = session.StepRunning
			wailsRuntime.EventsEmit(a.ctx, "session:step", map[string]any{
				"session": sessionID, "step": step.ID, "status": "running",
			})
			a.log.Info("executing step", logger.F("step", step.ID), logger.F("type", string(step.Type)))

			// Notify chat of step start
			a.emitStepEvent(sessionID, map[string]any{
				"event": "start", "id": step.ID, "type": string(step.Type), "description": step.Description,
			})

			var stepErr error
			switch step.Type {
			case session.StepTypeSQL:
				stepErr = a.executeSQLStep(engine, sess, step)
			case session.StepTypeInterpret, session.StepTypeAggregate:
				stepErr = a.executeLLMStep(engine, sess, step)
			default:
				step.Status = session.StepSkipped
				a.log.Warn("unsupported step type", logger.F("type", string(step.Type)))
			}

			if stepErr != nil {
				step.Status = session.StepFailed
				step.Error = &session.StepError{Message: stepErr.Error(), Severity: session.ErrorMinor}
				a.log.Warn("step failed", logger.F("step", step.ID), logger.F("error", stepErr.Error()))

				// For SQL errors, try retry with feedback (up to 2 times)
				if step.Type == session.StepTypeSQL && step.RetryCount < 2 {
					step.RetryCount++
					a.log.Info("retrying SQL step", logger.F("step", step.ID), logger.F("retry", fmt.Sprintf("%d", step.RetryCount)))
					retryErr := a.retrySQLWithFeedback(engine, sess, step)
					if retryErr == nil {
						stepErr = nil
					}
				}
			}

			if stepErr == nil && step.Status == session.StepRunning {
				step.Status = session.StepDone
			}

			completedSteps++
			progress := float64(completedSteps) / float64(totalSteps)
			wailsRuntime.EventsEmit(a.ctx, "session:step", map[string]any{
				"session": sessionID, "step": step.ID, "status": string(step.Status),
				"progress": progress,
			})

			// Notify chat of step result
			switch step.Status {
			case session.StepDone:
				summary := ""
				if step.Result != nil {
					summary = step.Result.Summary
				}
				a.emitStepEvent(sessionID, map[string]any{
					"event": "done", "id": step.ID, "type": string(step.Type),
					"description": step.Description, "summary": summary,
				})
			case session.StepFailed:
				errMsg := ""
				if step.Error != nil {
					errMsg = step.Error.Message
				}
				a.emitStepEvent(sessionID, map[string]any{
					"event": "failed", "id": step.ID, "type": string(step.Type),
					"description": step.Description, "error": errMsg,
				})
			case session.StepSkipped:
				a.emitStepEvent(sessionID, map[string]any{
					"event": "skipped", "id": step.ID, "description": step.Description,
				})
			}

			// Save after each step
			sess.Save(sessionsDir)
		}
	}

	// Generate review summary and report, then finalize
	a.log.Info("execution complete, generating report", logger.F("session", sessionID))
	a.generateAndFinalize(caseID, sessionID)
}

func (a *App) dependenciesMet(sess *session.Session, step *session.Step) bool {
	for _, depID := range step.DependsOn {
		dep, _ := sess.FindStep(depID)
		if dep == nil || dep.Status != session.StepDone {
			return false
		}
	}
	return true
}

func (a *App) executeSQLStep(engine *dbengine.Engine, sess *session.Session, step *session.Step) error {
	if step.SQL == "" {
		return fmt.Errorf("no SQL provided for step %s", step.ID)
	}

	result, err := engine.Execute(step.SQL)
	if err != nil {
		sess.RecordExec(session.ExecEntry{
			StepID: step.ID, Type: step.Type, SQL: step.SQL, Error: err.Error(),
		})
		return err
	}

	// Build summary from result
	summary := fmt.Sprintf("%d rows returned. Columns: %s", result.RowCount, strings.Join(result.Columns, ", "))
	if result.RowCount > 0 && result.RowCount <= 20 {
		dataBytes, _ := json.MarshalIndent(result.Rows, "", "  ")
		summary += "\n\n```json\n" + string(dataBytes) + "\n```"
	}

	step.Result = &session.StepResult{Summary: summary}
	sess.RecordExec(session.ExecEntry{
		StepID: step.ID, Type: step.Type, SQL: step.SQL,
		Result: &session.StepResult{Summary: summary}, Duration: result.Duration,
	})

	a.log.Info("SQL step done", logger.F("step", step.ID), logger.F("rows", fmt.Sprintf("%d", result.RowCount)))
	return nil
}

func (a *App) executeLLMStep(engine *dbengine.Engine, sess *session.Session, step *session.Step) error {
	if a.backend == nil {
		return fmt.Errorf("LLM backend not initialized")
	}

	// Gather dependency results
	var depResults strings.Builder
	for _, depID := range step.DependsOn {
		dep, _ := sess.FindStep(depID)
		if dep != nil && dep.Result != nil {
			fmt.Fprintf(&depResults, "## Step %s: %s\n%s\n\n", dep.ID, dep.Description, dep.Result.Summary)
		}
	}

	schemaCtx := engine.SchemaContext()
	systemPrompt := fmt.Sprintf(`You are a data analysis assistant.
Interpret the analysis results below in the context of the investigation plan.

## Database Schema
%s

## Investigation Objective
%s

Respond concisely in the user's language.`, schemaCtx, sess.Plan.Objective)

	userPrompt := fmt.Sprintf("## Step: %s\n%s\n\n## Previous Step Results:\n%s",
		step.ID, step.Description, depResults.String())

	resp, err := a.backend.Chat(a.ctx, &llm.ChatRequest{
		SystemPrompt: systemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: userPrompt}},
	})
	if err != nil {
		sess.RecordExec(session.ExecEntry{
			StepID: step.ID, Type: step.Type, Error: err.Error(),
		})
		return err
	}

	step.Result = &session.StepResult{Summary: resp.Content}
	sess.RecordExec(session.ExecEntry{
		StepID: step.ID, Type: step.Type, Result: &session.StepResult{Summary: resp.Content},
	})

	a.log.Info("LLM step done", logger.F("step", step.ID), logger.F("tokens", fmt.Sprintf("%d", resp.Usage.TotalTokens)))
	return nil
}

func (a *App) retrySQLWithFeedback(engine *dbengine.Engine, sess *session.Session, step *session.Step) error {
	if a.backend == nil {
		return fmt.Errorf("LLM backend not initialized")
	}

	schemaCtx := engine.SchemaContext()
	prompt := fmt.Sprintf(`The following SQL query failed. Please fix it.

## Schema
%s

## Original SQL
%s

## Error
%s

Respond with ONLY the corrected SQL query, no explanation.`, schemaCtx, step.SQL, step.Error.Message)

	resp, err := a.backend.Chat(a.ctx, &llm.ChatRequest{
		SystemPrompt: "You are a SQL expert. Output only valid SQL.",
		Messages:     []llm.Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return err
	}

	// Extract SQL from response (strip markdown code fences if present)
	newSQL := strings.TrimSpace(resp.Content)
	newSQL = strings.TrimPrefix(newSQL, "```sql\n")
	newSQL = strings.TrimPrefix(newSQL, "```\n")
	newSQL = strings.TrimSuffix(newSQL, "\n```")
	newSQL = strings.TrimSuffix(newSQL, "```")
	newSQL = strings.TrimSpace(newSQL)

	a.log.Info("SQL retry with corrected query", logger.F("step", step.ID), logger.F("sql", truncate(newSQL, 80)))
	step.SQL = newSQL
	step.Error = nil
	return a.executeSQLStep(engine, sess, step)
}

func (a *App) generateAndFinalize(caseID, sessionID string) {
	sess, err := a.GetSession(caseID, sessionID)
	if err != nil {
		return
	}
	sessionsDir := filepath.Join(a.cases.CaseDir(caseID), "sessions")

	// Generate LLM review summary
	if a.backend != nil {
		var analysisCtx strings.Builder
		fmt.Fprintf(&analysisCtx, "## Investigation Objective\n%s\n\n", sess.Plan.Objective)
		for _, p := range sess.Plan.Perspectives {
			fmt.Fprintf(&analysisCtx, "### %s: %s\n", p.ID, p.Description)
			for _, s := range p.Steps {
				fmt.Fprintf(&analysisCtx, "#### %s [%s]: %s\n", s.ID, s.Type, s.Description)
				if s.Status == session.StepDone && s.Result != nil {
					fmt.Fprintf(&analysisCtx, "%s\n", s.Result.Summary)
				} else if s.Status == session.StepFailed && s.Error != nil {
					fmt.Fprintf(&analysisCtx, "Failed: %s\n", s.Error.Message)
				} else if s.Status == session.StepSkipped {
					analysisCtx.WriteString("Skipped\n")
				}
				analysisCtx.WriteString("\n")
			}
		}

		a.log.Info("generating review summary via LLM")
		sess.AddMessage("report_header", "Analysis Review Report")
		sess.Save(sessionsDir)
		wailsRuntime.EventsEmit(a.ctx, "chat:report_start", map[string]any{
			"session": sessionID, "title": "Analysis Review Report",
		})

		var fullReview string
		a.backend.ChatStream(a.ctx, &llm.ChatRequest{
			SystemPrompt: `You are a data analysis reviewer. Based on the analysis results below, provide:
1. A concise executive summary of key findings
2. Notable patterns or concerns identified
3. Recommended next steps or areas for deeper analysis

Write in the user's language. Be concise and actionable. Use markdown formatting.`,
			Messages: []llm.Message{{Role: "user", Content: analysisCtx.String()}},
		}, func(token string, done bool) {
			if !done {
				fullReview += token
				wailsRuntime.EventsEmit(a.ctx, "chat:stream", map[string]any{
					"session": sessionID, "token": token,
				})
			}
		})

		if fullReview != "" {
			sess.AddMessage("assistant", fullReview)
		}
	}

	// Generate and save report
	r, err := report.GenerateFromSession(sess)
	if err != nil {
		a.log.Error("report generation failed", logger.F("error", err.Error()))
		sess.Save(sessionsDir)
		return
	}
	reportsDir := filepath.Join(a.cases.CaseDir(caseID), "reports")
	r.SaveToCase(reportsDir)

	// Finalize session
	sess.Finalize()
	sess.AddMessage("report_link", r.ID+"|"+r.Title)
	sess.Save(sessionsDir)

	a.log.Info("session finalized with report", logger.F("session", sessionID), logger.F("report", r.ID))
	wailsRuntime.EventsEmit(a.ctx, "chat:complete", map[string]any{"session": sessionID, "content": ""})
	wailsRuntime.EventsEmit(a.ctx, "session:phase", map[string]any{"session": sessionID, "phase": "done"})
	wailsRuntime.EventsEmit(a.ctx, "session:report_ready", map[string]any{
		"session": sessionID, "report_id": r.ID, "title": r.Title,
	})
}

func (a *App) ExecuteSQL(caseID, sessionID, sql string) (*dbengine.QueryResult, error) {
	engine, err := a.cases.Engine(caseID)
	if err != nil {
		a.log.Error("SQL execution failed: engine not available", logger.F("case", caseID), logger.F("error", err.Error()))
		return nil, err
	}

	a.log.Info("executing SQL", logger.F("sql", truncate(sql, 100)))
	result, err := engine.Execute(sql)
	if err != nil {
		a.log.Warn("SQL error", logger.F("sql", truncate(sql, 80)), logger.F("error", err.Error()))
		return result, err
	}
	a.log.Info("SQL completed", logger.F("rows", fmt.Sprintf("%d", result.RowCount)), logger.F("duration", result.Duration.String()))

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

func (a *App) emitStepEvent(sessionID string, data map[string]any) {
	data["session"] = sessionID
	wailsRuntime.EventsEmit(a.ctx, "chat:step_progress", data)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}

func buildPlanningPrompt(schemaCtx string) string {
	return `You are a data analysis planner.
Collaborate with the user to build a structured investigation plan.

## Database Schema
` + schemaCtx + `

## Step Types
- sql: Execute a SQL query (you MUST provide the actual SQL in the "sql" field)
- interpret: LLM interprets the result of previous steps
- aggregate: LLM synthesizes results from multiple steps

## Rules
- During discussion, respond in natural language in the user's language
- When the plan is sufficiently developed and the user is ready, output the plan as a JSON code block
- All SQL must be read-only (SELECT only)
- Each sql step MUST include a valid SQL query in the "sql" field
- Use depends_on to express step dependencies

## Required JSON Schema
When outputting the plan, use EXACTLY this structure in a ` + "```json" + ` code block:
{
  "objective": "What we are investigating",
  "perspectives": [
    {
      "id": "P1",
      "description": "Analysis angle description",
      "steps": [
        {"id": "P1-S1", "type": "sql", "description": "What this query does", "sql": "SELECT ...", "depends_on": []},
        {"id": "P1-S2", "type": "interpret", "description": "Interpret the results", "depends_on": ["P1-S1"]}
      ]
    }
  ]
}

Only output the JSON when you and the user have agreed on the analysis plan.`
}

