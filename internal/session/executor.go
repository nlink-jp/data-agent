package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nlink-jp/data-agent/internal/analysis"
	"github.com/nlink-jp/data-agent/internal/dbengine"
	"github.com/nlink-jp/data-agent/internal/llm"
	"github.com/nlink-jp/nlk/backoff"
)

// EventSink receives execution events for the UI layer.
// This interface decouples the executor from Wails.
type EventSink interface {
	OnStepStart(sessionID, stepID string, stepType StepType, description string)
	OnStepDone(sessionID, stepID string, stepType StepType, description, summary string)
	OnStepFailed(sessionID, stepID string, stepType StepType, description, errMsg string)
	OnStepSkipped(sessionID, stepID, description string)
	OnStepInfo(sessionID, stepID, message string)
	OnPhaseChange(sessionID string, phase Phase)
	OnReportStart(sessionID, title string)
	OnStream(sessionID, token string)
	OnComplete(sessionID string)
	OnReportReady(sessionID, reportID, title string)
	OnLog(level, msg string, fields map[string]string)
}

// Executor runs plan steps and generates reports.
type Executor struct {
	Engine  *dbengine.Engine
	Backend llm.Backend
	Config  ExecutorConfig
	Events  EventSink
}

// ExecutorConfig holds execution parameters.
type ExecutorConfig struct {
	MaxSQLRetries       int
	MaxRecordsPerWindow int
	OverlapRatio        float64
	MaxFindings         int
	ContextLimit        int
}

// DefaultExecutorConfig returns sensible defaults.
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		MaxSQLRetries:       2,
		MaxRecordsPerWindow: 200,
		OverlapRatio:        0.1,
		MaxFindings:         100,
		ContextLimit:        131072,
	}
}

// RunPlan executes all steps in the plan sequentially.
// Modifies sess in-place and saves after each step.
func (e *Executor) RunPlan(ctx context.Context, sess *Session, saveFunc func() error) error {
	if sess.Plan == nil {
		return fmt.Errorf("no plan to execute")
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

			if ctx.Err() != nil {
				return ctx.Err()
			}

			if !e.dependenciesMet(sess, step) {
				step.Status = StepSkipped
				e.Events.OnStepSkipped(sess.ID, step.ID, step.Description)
				e.Events.OnLog("warn", "step skipped (unmet deps)", map[string]string{"step": step.ID})
				completedSteps++
				continue
			}

			step.Status = StepRunning
			e.Events.OnStepStart(sess.ID, step.ID, step.Type, step.Description)
			e.Events.OnLog("info", "executing step", map[string]string{"step": step.ID, "type": string(step.Type)})

			var stepErr error
			switch step.Type {
			case StepTypeSQL:
				stepErr = e.executeSQLStep(sess, step)
			case StepTypeInterpret, StepTypeAggregate:
				stepErr = e.executeLLMStep(ctx, sess, step)
			case StepTypeSlidingWindow:
				stepErr = e.executeSlidingWindowStep(ctx, sess, step)
			default:
				step.Status = StepSkipped
				e.Events.OnLog("warn", "unsupported step type", map[string]string{"type": string(step.Type)})
			}

			if stepErr != nil {
				step.Status = StepFailed
				step.Error = &StepError{Message: stepErr.Error(), Severity: ErrorMinor}
				e.Events.OnLog("warn", "step failed", map[string]string{"step": step.ID, "error": stepErr.Error()})

				if step.Type == StepTypeSQL && step.RetryCount < e.Config.MaxSQLRetries {
					step.RetryCount++
					// Exponential backoff before retry (nlk/backoff)
					bo := backoff.New(backoff.WithBase(2*time.Second), backoff.WithMax(30*time.Second))
					wait := bo.Duration(step.RetryCount - 1)
					e.Events.OnLog("info", "retrying SQL step", map[string]string{
						"step": step.ID, "retry": fmt.Sprintf("%d", step.RetryCount), "backoff": wait.String(),
					})
					time.Sleep(wait)
					if retryErr := e.retrySQLWithFeedback(ctx, sess, step); retryErr == nil {
						stepErr = nil
						step.Status = StepRunning // reset so it gets marked Done below
					}
				}
			}

			if stepErr == nil && step.Status == StepRunning {
				step.Status = StepDone
			}

			completedSteps++

			// Emit result
			switch step.Status {
			case StepDone:
				summary := ""
				if step.Result != nil {
					summary = step.Result.Summary
				}
				e.Events.OnStepDone(sess.ID, step.ID, step.Type, step.Description, summary)
			case StepFailed:
				errMsg := ""
				if step.Error != nil {
					errMsg = step.Error.Message
				}
				e.Events.OnStepFailed(sess.ID, step.ID, step.Type, step.Description, errMsg)
			case StepSkipped:
				e.Events.OnStepSkipped(sess.ID, step.ID, step.Description)
			}

			// Save after each step
			if err := saveFunc(); err != nil {
				e.Events.OnLog("error", "session save failed after step", map[string]string{"step": step.ID, "error": err.Error()})
			}
		}
	}

	return nil
}

// GenerateReviewAndFinalize generates the LLM review, creates a report, and finalizes.
func (e *Executor) GenerateReviewAndFinalize(ctx context.Context, sess *Session, saveFunc func() error, saveReport func(content string) (reportID, title string, err error)) error {
	// Build analysis context for LLM review
	var perspectives []analysis.PerspectiveSummary
	for _, p := range sess.Plan.Perspectives {
		ps := analysis.PerspectiveSummary{ID: p.ID, Description: p.Description}
		for _, s := range p.Steps {
			ss := analysis.StepSummary{ID: s.ID, Type: string(s.Type), Description: s.Description}
			if s.Status == StepDone && s.Result != nil {
				ss.Result = s.Result.Summary
			} else if s.Status == StepFailed && s.Error != nil {
				ss.Error = s.Error.Message
			}
			ps.Steps = append(ps.Steps, ss)
		}
		perspectives = append(perspectives, ps)
	}

	reviewCtx := analysis.BuildReviewUserPrompt(sess.Plan.Objective, perspectives)

	e.Events.OnLog("info", "generating review summary via LLM", nil)
	sess.AddMessage("report_header", "Analysis Review Report")
	saveFunc()
	e.Events.OnReportStart(sess.ID, "Analysis Review Report")

	// Stream the review
	var fullReview string
	if e.Backend != nil {
		err := e.Backend.ChatStream(ctx, &llm.ChatRequest{
			SystemPrompt: analysis.BuildReviewSummaryPrompt(),
			Messages:     []llm.Message{{Role: "user", Content: reviewCtx}},
		}, func(token string, done bool) {
			if !done {
				fullReview += token
				e.Events.OnStream(sess.ID, token)
			}
		})
		if err != nil {
			e.Events.OnLog("error", "review generation failed", map[string]string{"error": err.Error()})
		}
	}

	if fullReview != "" {
		fullReview = llm.StripArtifacts(fullReview)
		sess.AddMessage("assistant", fullReview)
	}

	// Save report
	reportID, reportTitle, err := saveReport(fullReview)
	if err != nil {
		e.Events.OnLog("error", "report save failed", map[string]string{"error": err.Error()})
		saveFunc()
		return err
	}

	// Finalize
	sess.Finalize()
	sess.AddMessage("report_link", reportID+"|"+reportTitle)
	if err := saveFunc(); err != nil {
		return err
	}

	e.Events.OnLog("info", "session finalized with report", map[string]string{"session": sess.ID, "report": reportID})
	e.Events.OnComplete(sess.ID)
	e.Events.OnPhaseChange(sess.ID, PhaseDone)
	e.Events.OnReportReady(sess.ID, reportID, reportTitle)

	return nil
}

func (e *Executor) dependenciesMet(sess *Session, step *Step) bool {
	for _, depID := range step.DependsOn {
		dep, _ := sess.FindStep(depID)
		if dep == nil || dep.Status != StepDone {
			return false
		}
	}
	return true
}

func (e *Executor) executeSQLStep(sess *Session, step *Step) error {
	if step.SQL == "" {
		return fmt.Errorf("no SQL provided for step %s", step.ID)
	}

	result, err := e.Engine.Execute(step.SQL)
	if err != nil {
		sess.RecordExec(ExecEntry{StepID: step.ID, Type: step.Type, SQL: step.SQL, Error: err.Error()})
		return err
	}

	summary := fmt.Sprintf("%d rows returned. Columns: %s", result.RowCount, strings.Join(result.Columns, ", "))
	if result.RowCount > 0 && result.RowCount <= 20 {
		dataBytes, _ := json.MarshalIndent(result.Rows, "", "  ")
		summary += "\n\n```json\n" + string(dataBytes) + "\n```"
	}

	step.Result = &StepResult{Summary: summary}
	sess.RecordExec(ExecEntry{
		StepID: step.ID, Type: step.Type, SQL: step.SQL,
		Result: &StepResult{Summary: summary}, Duration: result.Duration,
	})

	e.Events.OnLog("info", "SQL step done", map[string]string{"step": step.ID, "rows": fmt.Sprintf("%d", result.RowCount)})
	return nil
}

func (e *Executor) executeLLMStep(ctx context.Context, sess *Session, step *Step) error {
	if e.Backend == nil {
		return fmt.Errorf("LLM backend not initialized")
	}

	var depResults strings.Builder
	for _, depID := range step.DependsOn {
		dep, _ := sess.FindStep(depID)
		if dep != nil && dep.Result != nil {
			fmt.Fprintf(&depResults, "## Step %s: %s\n%s\n\n", dep.ID, dep.Description, dep.Result.Summary)
		}
	}

	schemaCtx := e.Engine.SchemaContext()
	systemPrompt := analysis.BuildInterpretSystemPrompt(schemaCtx, sess.Plan.Objective)
	userPrompt := analysis.BuildInterpretUserPrompt(step.ID, step.Description, depResults.String())

	resp, err := e.Backend.Chat(ctx, &llm.ChatRequest{
		SystemPrompt: systemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: userPrompt}},
	})
	if err != nil {
		sess.RecordExec(ExecEntry{StepID: step.ID, Type: step.Type, Error: err.Error()})
		return err
	}

	content := llm.StripArtifacts(resp.Content)
	step.Result = &StepResult{Summary: content}
	sess.RecordExec(ExecEntry{
		StepID: step.ID, Type: step.Type, Result: &StepResult{Summary: content},
	})

	tokens := 0
	if resp.Usage != nil {
		tokens = resp.Usage.TotalTokens
	}
	e.Events.OnLog("info", "LLM step done", map[string]string{"step": step.ID, "tokens": fmt.Sprintf("%d", tokens)})
	return nil
}

func (e *Executor) executeSlidingWindowStep(ctx context.Context, sess *Session, step *Step) error {
	if e.Backend == nil {
		return fmt.Errorf("LLM backend not initialized")
	}
	if step.SQL == "" {
		return fmt.Errorf("no SQL for sliding_window step %s", step.ID)
	}

	result, err := e.Engine.Execute(step.SQL)
	if err != nil {
		sess.RecordExec(ExecEntry{StepID: step.ID, Type: step.Type, SQL: step.SQL, Error: err.Error()})
		return err
	}

	e.Events.OnLog("info", "sliding window: fetched data", map[string]string{"step": step.ID, "rows": fmt.Sprintf("%d", result.RowCount)})
	e.Events.OnStepInfo(sess.ID, step.ID, fmt.Sprintf("Fetched %d records, starting sliding window analysis...", result.RowCount))

	// Build rich perspective
	var perspDesc string
	for _, p := range sess.Plan.Perspectives {
		for _, s := range p.Steps {
			if s.ID == step.ID {
				perspDesc = p.Description
				break
			}
		}
	}
	var priorFindings strings.Builder
	for _, depID := range step.DependsOn {
		dep, _ := sess.FindStep(depID)
		if dep != nil && dep.Result != nil {
			fmt.Fprintf(&priorFindings, "## %s\n%s\n", dep.ID, dep.Result.Summary)
		}
	}

	perspective := analysis.BuildSlidingWindowPerspective(
		sess.Plan.Objective, perspDesc, step.Description, priorFindings.String())

	cfg := analysis.SlidingWindowConfig{
		MaxRecordsPerWindow: e.Config.MaxRecordsPerWindow,
		OverlapRatio:        e.Config.OverlapRatio,
		MaxFindings:         e.Config.MaxFindings,
		ContextLimit:        e.Config.ContextLimit,
	}

	windowResult, err := analysis.RunSlidingWindow(ctx, e.Backend, result.Rows, perspective, cfg,
		func(windowIdx, totalWindows int) {
			e.Events.OnStepInfo(sess.ID, step.ID, fmt.Sprintf("Processing window %d/%d...", windowIdx+1, totalWindows))
			e.Events.OnLog("info", "sliding window progress", map[string]string{
				"step": step.ID, "window": fmt.Sprintf("%d/%d", windowIdx+1, totalWindows),
			})
		},
	)
	if err != nil {
		sess.RecordExec(ExecEntry{StepID: step.ID, Type: step.Type, SQL: step.SQL, Error: err.Error()})
		return err
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Sliding window analysis: %d records in %d windows\n\n", windowResult.TotalRecords, windowResult.Windows)
	fmt.Fprintf(&sb, "**Summary:** %s\n\n", windowResult.Summary)
	if len(windowResult.Findings) > 0 {
		sb.WriteString("**Findings:**\n")
		for _, f := range windowResult.Findings {
			fmt.Fprintf(&sb, "- [%s] %s (severity: %s)\n", f.ID, f.Description, f.Severity)
		}
	}
	summary := sb.String()

	step.Result = &StepResult{Summary: summary}
	sess.RecordExec(ExecEntry{
		StepID: step.ID, Type: step.Type, SQL: step.SQL,
		Result: &StepResult{Summary: summary},
	})

	for _, f := range windowResult.Findings {
		sess.AddFinding(Finding{ID: f.ID, Description: f.Description, Severity: f.Severity, StepID: step.ID})
	}

	e.Events.OnLog("info", "sliding window done", map[string]string{
		"step": step.ID, "windows": fmt.Sprintf("%d", windowResult.Windows), "findings": fmt.Sprintf("%d", len(windowResult.Findings)),
	})
	return nil
}

func (e *Executor) retrySQLWithFeedback(ctx context.Context, sess *Session, step *Step) error {
	if e.Backend == nil {
		return fmt.Errorf("LLM backend not initialized")
	}

	schemaCtx := e.Engine.SchemaContext()
	prompt := analysis.BuildSQLFixPrompt(schemaCtx, step.SQL, step.Error.Message)

	resp, err := e.Backend.Chat(ctx, &llm.ChatRequest{
		SystemPrompt: "You are a SQL expert. Output only valid SQL.",
		Messages:     []llm.Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return err
	}

	newSQL := llm.StripArtifacts(resp.Content)
	newSQL = strings.TrimPrefix(newSQL, "```sql\n")
	newSQL = strings.TrimPrefix(newSQL, "```\n")
	newSQL = strings.TrimSuffix(newSQL, "\n```")
	newSQL = strings.TrimSuffix(newSQL, "```")
	newSQL = strings.TrimSpace(newSQL)

	e.Events.OnLog("info", "SQL retry with corrected query", map[string]string{"step": step.ID})
	step.SQL = newSQL
	step.Error = nil
	return e.executeSQLStep(sess, step)
}
