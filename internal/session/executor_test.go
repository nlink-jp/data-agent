package session

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/nlink-jp/data-agent/internal/dbengine"
	"github.com/nlink-jp/data-agent/internal/llm"
)

// mockEventSink records events for assertions.
type mockEventSink struct {
	mu     sync.Mutex
	events []mockEvent
}

type mockEvent struct {
	kind string
	data map[string]string
}

func (m *mockEventSink) record(kind string, kv ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data := make(map[string]string)
	for i := 0; i+1 < len(kv); i += 2 {
		data[kv[i]] = kv[i+1]
	}
	m.events = append(m.events, mockEvent{kind: kind, data: data})
}

func (m *mockEventSink) findAll(kind string) []mockEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []mockEvent
	for _, e := range m.events {
		if e.kind == kind {
			result = append(result, e)
		}
	}
	return result
}

func (m *mockEventSink) OnStepStart(sid, stepID string, st StepType, desc string) {
	m.record("step_start", "step", stepID, "type", string(st))
}
func (m *mockEventSink) OnStepDone(sid, stepID string, st StepType, desc, summary string) {
	m.record("step_done", "step", stepID)
}
func (m *mockEventSink) OnStepFailed(sid, stepID string, st StepType, desc, err string) {
	m.record("step_failed", "step", stepID, "error", err)
}
func (m *mockEventSink) OnStepSkipped(sid, stepID, desc string) {
	m.record("step_skipped", "step", stepID)
}
func (m *mockEventSink) OnStepInfo(sid, stepID, msg string) {
	m.record("step_info", "step", stepID, "msg", msg)
}
func (m *mockEventSink) OnPhaseChange(sid string, phase Phase) {
	m.record("phase", "phase", string(phase))
}
func (m *mockEventSink) OnReportStart(sid, title string)             { m.record("report_start") }
func (m *mockEventSink) OnStream(sid, token string)                  { m.record("stream") }
func (m *mockEventSink) OnComplete(sid string)                       { m.record("complete") }
func (m *mockEventSink) OnReportReady(sid, rid, title string)        { m.record("report_ready", "id", rid) }
func (m *mockEventSink) OnLog(level, msg string, fields map[string]string) {}

// mockBackend is a test LLM backend.
type mockBackend struct {
	chatFunc func(req *llm.ChatRequest) (*llm.ChatResponse, error)
}

func (m *mockBackend) Name() string                  { return "mock" }
func (m *mockBackend) EstimateTokens(text string) int { return len(text) / 4 }
func (m *mockBackend) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.chatFunc != nil {
		return m.chatFunc(req)
	}
	return &llm.ChatResponse{Content: "mock response", Usage: &llm.Usage{TotalTokens: 10}}, nil
}
func (m *mockBackend) ChatStream(ctx context.Context, req *llm.ChatRequest, cb llm.StreamCallback) error {
	resp, err := m.Chat(ctx, req)
	if err != nil {
		return err
	}
	cb(resp.Content, false)
	cb("", true)
	return nil
}

func setupTestEngine(t *testing.T) *dbengine.Engine {
	t.Helper()
	dir := t.TempDir()
	engine, err := dbengine.Open(dir + "/test.duckdb")
	if err != nil {
		t.Fatal(err)
	}
	// Create test table
	t.Cleanup(func() { engine.Close() })
	return engine
}

func TestExecutor_RunPlan_SQLSteps(t *testing.T) {
	engine := setupTestEngine(t)

	// Import test data via raw SQL (use engine's db directly)
	// We need to use Import, so create a temp CSV
	csvPath := t.TempDir() + "/data.csv"
	writeFile(t, csvPath, "name,value\nalice,10\nbob,20\ncharlie,30\n")
	if err := engine.Import(csvPath, "test_data"); err != nil {
		t.Fatal(err)
	}

	events := &mockEventSink{}
	executor := &Executor{
		Engine:  engine,
		Backend: &mockBackend{},
		Config:  DefaultExecutorConfig(),
		Events:  events,
	}

	sess := New("case-1")
	sess.SetPlan(&Plan{
		Objective: "Test SQL execution",
		Perspectives: []Perspective{{
			ID:          "P1",
			Description: "Count records",
			Steps: []Step{
				{ID: "S1", Type: StepTypeSQL, Description: "Count all", SQL: "SELECT COUNT(*) AS cnt FROM test_data"},
				{ID: "S2", Type: StepTypeSQL, Description: "Sum values", SQL: "SELECT SUM(value) AS total FROM test_data", DependsOn: []string{"S1"}},
			},
		}},
	})
	sess.ApprovePlan()

	saveCount := 0
	err := executor.RunPlan(context.Background(), sess, func() error {
		saveCount++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Both steps should be done
	s1, _ := sess.FindStep("S1")
	s2, _ := sess.FindStep("S2")
	if s1.Status != StepDone {
		t.Errorf("S1 status = %q, want done", s1.Status)
	}
	if s2.Status != StepDone {
		t.Errorf("S2 status = %q, want done", s2.Status)
	}

	// Results should contain data
	if s1.Result == nil || !strings.Contains(s1.Result.Summary, "1 rows") {
		t.Errorf("S1 result unexpected: %v", s1.Result)
	}

	// Save should be called after each step
	if saveCount != 2 {
		t.Errorf("save called %d times, want 2", saveCount)
	}

	// Events should be emitted
	starts := events.findAll("step_start")
	dones := events.findAll("step_done")
	if len(starts) != 2 {
		t.Errorf("step_start events = %d, want 2", len(starts))
	}
	if len(dones) != 2 {
		t.Errorf("step_done events = %d, want 2", len(dones))
	}

	// Exec log should have entries
	if len(sess.ExecLog) != 2 {
		t.Errorf("exec_log = %d, want 2", len(sess.ExecLog))
	}
}

func TestExecutor_RunPlan_DependencySkip(t *testing.T) {
	engine := setupTestEngine(t)
	events := &mockEventSink{}
	executor := &Executor{
		Engine: engine, Backend: &mockBackend{}, Config: DefaultExecutorConfig(), Events: events,
	}

	sess := New("case-1")
	sess.SetPlan(&Plan{
		Objective: "Test dependency skip",
		Perspectives: []Perspective{{
			ID: "P1",
			Steps: []Step{
				{ID: "S1", Type: StepTypeSQL, Description: "Bad SQL", SQL: "SELECT * FROM nonexistent_table"},
				{ID: "S2", Type: StepTypeInterpret, Description: "Depends on S1", DependsOn: []string{"S1"}},
			},
		}},
	})
	sess.ApprovePlan()

	executor.RunPlan(context.Background(), sess, func() error { return nil })

	s1, _ := sess.FindStep("S1")
	s2, _ := sess.FindStep("S2")

	if s1.Status != StepFailed {
		t.Errorf("S1 status = %q, want failed", s1.Status)
	}
	if s2.Status != StepSkipped {
		t.Errorf("S2 status = %q, want skipped (dependency S1 failed)", s2.Status)
	}
}

func TestExecutor_RunPlan_LLMInterpretStep(t *testing.T) {
	engine := setupTestEngine(t)
	csvPath := t.TempDir() + "/data.csv"
	writeFile(t, csvPath, "x\n1\n2\n3\n")
	engine.Import(csvPath, "nums")

	events := &mockEventSink{}
	executor := &Executor{
		Engine: engine,
		Backend: &mockBackend{
			chatFunc: func(req *llm.ChatRequest) (*llm.ChatResponse, error) {
				return &llm.ChatResponse{Content: "The data shows 3 values with a mean of 2.", Usage: &llm.Usage{TotalTokens: 20}}, nil
			},
		},
		Config: DefaultExecutorConfig(),
		Events: events,
	}

	sess := New("case-1")
	sess.SetPlan(&Plan{
		Objective: "Test interpret step",
		Perspectives: []Perspective{{
			ID: "P1",
			Steps: []Step{
				{ID: "S1", Type: StepTypeSQL, Description: "Get data", SQL: "SELECT * FROM nums"},
				{ID: "S2", Type: StepTypeInterpret, Description: "Interpret data", DependsOn: []string{"S1"}},
			},
		}},
	})
	sess.ApprovePlan()

	executor.RunPlan(context.Background(), sess, func() error { return nil })

	s2, _ := sess.FindStep("S2")
	if s2.Status != StepDone {
		t.Errorf("S2 status = %q, want done", s2.Status)
	}
	if s2.Result == nil || !strings.Contains(s2.Result.Summary, "mean of 2") {
		t.Errorf("S2 result unexpected: %v", s2.Result)
	}
}

func TestExecutor_RunPlan_SQLRetry(t *testing.T) {
	engine := setupTestEngine(t)
	csvPath := t.TempDir() + "/data.csv"
	writeFile(t, csvPath, "val\n42\n")
	engine.Import(csvPath, "items")

	events := &mockEventSink{}
	callCount := 0
	executor := &Executor{
		Engine: engine,
		Backend: &mockBackend{
			chatFunc: func(req *llm.ChatRequest) (*llm.ChatResponse, error) {
				callCount++
				// Return corrected SQL
				return &llm.ChatResponse{Content: "SELECT val FROM items", Usage: &llm.Usage{}}, nil
			},
		},
		Config: DefaultExecutorConfig(),
		Events: events,
	}

	sess := New("case-1")
	sess.SetPlan(&Plan{
		Objective: "Test SQL retry",
		Perspectives: []Perspective{{
			ID: "P1",
			Steps: []Step{
				{ID: "S1", Type: StepTypeSQL, Description: "Bad column", SQL: "SELECT nonexistent FROM items"},
			},
		}},
	})
	sess.ApprovePlan()

	executor.RunPlan(context.Background(), sess, func() error { return nil })

	s1, _ := sess.FindStep("S1")
	// After retry with corrected SQL, should be done
	if s1.Status != StepDone {
		t.Errorf("S1 status = %q, want done (after retry)", s1.Status)
	}
	if callCount == 0 {
		t.Error("expected LLM to be called for SQL fix")
	}
}

func TestExecutor_RunPlan_ContextCancellation(t *testing.T) {
	engine := setupTestEngine(t)
	events := &mockEventSink{}
	executor := &Executor{
		Engine: engine, Backend: &mockBackend{}, Config: DefaultExecutorConfig(), Events: events,
	}

	sess := New("case-1")
	sess.SetPlan(&Plan{
		Objective: "Test cancellation",
		Perspectives: []Perspective{{
			ID: "P1",
			Steps: []Step{
				{ID: "S1", Type: StepTypeSQL, Description: "Query", SQL: "SELECT 1"},
			},
		}},
	})
	sess.ApprovePlan()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := executor.RunPlan(ctx, sess, func() error { return nil })
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestExecutor_GenerateReviewAndFinalize(t *testing.T) {
	engine := setupTestEngine(t)
	events := &mockEventSink{}
	executor := &Executor{
		Engine: engine,
		Backend: &mockBackend{
			chatFunc: func(req *llm.ChatRequest) (*llm.ChatResponse, error) {
				return &llm.ChatResponse{Content: "## Summary\nKey finding: test data analysis complete.", Usage: &llm.Usage{}}, nil
			},
		},
		Config: DefaultExecutorConfig(),
		Events: events,
	}

	sess := New("case-1")
	sess.SetPlan(&Plan{
		Objective: "Test finalization",
		Perspectives: []Perspective{{
			ID: "P1", Description: "Test",
			Steps: []Step{
				{ID: "S1", Type: StepTypeSQL, Description: "Query", SQL: "SELECT 1",
					Status: StepDone, Result: &StepResult{Summary: "1 row"}},
			},
		}},
	})
	sess.ApprovePlan() // Planning → Execution

	saveCount := 0
	err := executor.GenerateReviewAndFinalize(context.Background(), sess,
		func() error { saveCount++; return nil },
		func(content string) (string, string, error) {
			return "report-123", "Test Report", nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if sess.Phase != PhaseDone {
		t.Errorf("phase = %q, want done", sess.Phase)
	}

	// Should have report_header, assistant review, report_link messages
	hasReportHeader := false
	hasReportLink := false
	hasAssistant := false
	for _, m := range sess.Chat {
		switch m.Role {
		case "report_header":
			hasReportHeader = true
		case "report_link":
			hasReportLink = true
			if !strings.Contains(m.Content, "report-123") {
				t.Errorf("report_link content = %q, want to contain report-123", m.Content)
			}
		case "assistant":
			hasAssistant = true
		}
	}
	if !hasReportHeader {
		t.Error("missing report_header message")
	}
	if !hasReportLink {
		t.Error("missing report_link message")
	}
	if !hasAssistant {
		t.Error("missing assistant review message")
	}

	// Events
	readyEvents := events.findAll("report_ready")
	if len(readyEvents) != 1 {
		t.Errorf("report_ready events = %d, want 1", len(readyEvents))
	}
	phaseEvents := events.findAll("phase")
	found := false
	for _, e := range phaseEvents {
		if e.data["phase"] == "done" {
			found = true
		}
	}
	if !found {
		t.Error("missing phase=done event")
	}
}

func TestSessionPhaseTransitions_FullLifecycle(t *testing.T) {
	// Planning → (set plan) → (approve) → Execution → Done → (reopen) → Planning
	sess := New("case-1")

	if sess.Phase != PhasePlanning {
		t.Fatalf("initial phase = %q, want planning", sess.Phase)
	}

	// Cannot approve without plan
	if err := sess.ApprovePlan(); err == nil {
		t.Error("should error: no plan")
	}

	// Set plan
	sess.SetPlan(&Plan{
		Objective: "Test lifecycle",
		Perspectives: []Perspective{{
			ID: "P1", Steps: []Step{{ID: "S1", Type: StepTypeSQL}},
		}},
	})
	if sess.Plan.Version != 1 {
		t.Errorf("plan version = %d, want 1", sess.Plan.Version)
	}

	// Approve → Execution
	if err := sess.ApprovePlan(); err != nil {
		t.Fatal(err)
	}
	if sess.Phase != PhaseExecution {
		t.Errorf("phase = %q, want execution", sess.Phase)
	}

	// Cannot approve again
	if err := sess.ApprovePlan(); err == nil {
		t.Error("should error: already in execution")
	}

	// Finalize → Done
	if err := sess.Finalize(); err != nil {
		t.Fatal(err)
	}
	if sess.Phase != PhaseDone {
		t.Errorf("phase = %q, want done", sess.Phase)
	}

	// Cannot finalize again
	if err := sess.Finalize(); err == nil {
		t.Error("should error: already done")
	}

	// Reopen → Planning (plan cleared)
	if err := sess.Reopen(); err != nil {
		t.Fatal(err)
	}
	if sess.Phase != PhasePlanning {
		t.Errorf("phase = %q, want planning", sess.Phase)
	}
	if sess.Plan != nil {
		t.Error("plan should be nil after reopen")
	}

	// Must set new plan before approve
	sess.SetPlan(&Plan{
		Objective: "New analysis after reopen",
		Perspectives: []Perspective{{
			ID: "P2", Steps: []Step{{ID: "S3", Type: StepTypeSQL}},
		}},
	})
	if err := sess.ApprovePlan(); err != nil {
		t.Fatal(err)
	}
	if sess.Phase != PhaseExecution {
		t.Errorf("phase = %q, want execution", sess.Phase)
	}
}

func TestSessionPhaseTransitions_ForceReplan(t *testing.T) {
	sess := New("case-1")
	sess.SetPlan(&Plan{Objective: "Test replan"})
	sess.ApprovePlan()

	if sess.Phase != PhaseExecution {
		t.Fatalf("phase = %q, want execution", sess.Phase)
	}

	sess.ForceReplan("column not found")

	if sess.Phase != PhasePlanning {
		t.Errorf("phase = %q, want planning", sess.Phase)
	}
	if sess.Plan.Version != 2 {
		t.Errorf("plan version = %d, want 2", sess.Plan.Version)
	}
	if len(sess.Plan.History) != 1 {
		t.Errorf("history = %d, want 1", len(sess.Plan.History))
	}
	if sess.Plan.History[0].Reason != "column not found" {
		t.Errorf("replan reason = %q", sess.Plan.History[0].Reason)
	}
}

func TestSessionPhaseTransitions_ForceReplanNilPlan(t *testing.T) {
	sess := New("case-1")
	// ForceReplan with no plan should not panic
	sess.ForceReplan("some error")
	if sess.Phase != PhasePlanning {
		t.Errorf("phase = %q, want planning", sess.Phase)
	}
}

func TestSessionPhaseTransitions_InvalidTransitions(t *testing.T) {
	sess := New("case-1")

	// Cannot finalize from planning
	if err := sess.Finalize(); err == nil {
		t.Error("should error: cannot finalize from planning")
	}

	// Cannot reopen from planning
	if err := sess.Reopen(); err == nil {
		t.Error("should error: cannot reopen from planning")
	}

	sess.SetPlan(&Plan{Objective: "Test"})
	sess.ApprovePlan()

	// Cannot reopen from execution
	if err := sess.Reopen(); err == nil {
		t.Error("should error: cannot reopen from execution")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
