package analysis

import (
	"strings"
	"testing"
)

func TestWrapGuard(t *testing.T) {
	wrapped := WrapGuard("test data")
	if !strings.Contains(wrapped, "test data") {
		t.Error("wrapped data should contain original content")
	}
	if !strings.HasPrefix(wrapped, "<data_") {
		t.Error("wrapped data should start with <data_ tag")
	}
	if !strings.HasSuffix(strings.TrimSpace(wrapped), ">") {
		t.Error("wrapped data should end with closing tag")
	}

	// Each call should produce different tags (nonce)
	wrapped2 := WrapGuard("test data")
	if wrapped == wrapped2 {
		t.Error("guard tags should use unique nonces")
	}
}

func TestBuildPlanningSystemPrompt(t *testing.T) {
	schema := "Table: users (100 rows)\nColumns:\n  - id: INTEGER\n  - name: VARCHAR\n"
	prompt := BuildPlanningSystemPrompt(schema)

	if !strings.Contains(prompt, "planner") {
		t.Error("planning prompt should mention planner role")
	}
	if !strings.Contains(prompt, "users") {
		t.Error("planning prompt should contain schema")
	}
	if !strings.Contains(prompt, "objective") {
		t.Error("planning prompt should mention objective")
	}
	if !strings.Contains(prompt, "sql") {
		t.Error("planning prompt should mention sql step type")
	}
}

func TestBuildInterpretSystemPrompt(t *testing.T) {
	prompt := BuildInterpretSystemPrompt("schema here", "plan summary here")
	if !strings.Contains(prompt, "schema here") {
		t.Error("interpret prompt should contain schema")
	}
	if !strings.Contains(prompt, "plan summary here") {
		t.Error("interpret prompt should contain plan summary")
	}
}

func TestBuildReviewSystemPrompt(t *testing.T) {
	prompt := BuildReviewSystemPrompt()
	if !strings.Contains(prompt, "reviewer") {
		t.Error("review prompt should mention reviewer role")
	}
}

func TestComputeMemoryMap(t *testing.T) {
	cfg := DefaultBudgetConfig()
	cfg.ContextLimit = 10000

	mm := ComputeMemoryMap(cfg, "system prompt", "schema", "plan")
	if mm.SystemPrompt <= 0 {
		t.Error("system prompt tokens should be positive")
	}
	if mm.Response != cfg.ResponseReserve {
		t.Errorf("response = %d, want %d", mm.Response, cfg.ResponseReserve)
	}
	total := mm.SystemPrompt + mm.Schema + mm.Plan + mm.History + mm.StepResults + mm.Response + mm.Available
	if total != cfg.ContextLimit {
		t.Errorf("total allocation = %d, want %d (context limit)", total, cfg.ContextLimit)
	}
}

func TestComputeMemoryMapTightBudget(t *testing.T) {
	cfg := BudgetConfig{
		ContextLimit:    1000,
		SystemReserve:   100,
		ResponseReserve: 200,
		MaxHistory:      5000,
		MaxStepResults:  5000,
	}

	// With a very small context limit, history and step results should be reduced
	mm := ComputeMemoryMap(cfg, "sys", "schema", "plan")
	if mm.History > cfg.MaxHistory {
		t.Errorf("history = %d, should not exceed max %d", mm.History, cfg.MaxHistory)
	}
	if mm.StepResults > cfg.MaxStepResults {
		t.Errorf("step_results = %d, should not exceed max %d", mm.StepResults, cfg.MaxStepResults)
	}
}

func TestTruncateToTokenBudget(t *testing.T) {
	text := strings.Repeat("word ", 1000)

	truncated := TruncateToTokenBudget(text, 100)
	if len(truncated) >= len(text) {
		t.Error("truncated text should be shorter than original")
	}
	if !strings.HasSuffix(truncated, "[truncated]") {
		t.Error("truncated text should end with [truncated] marker")
	}

	// Small budget
	tiny := TruncateToTokenBudget(text, 0)
	if tiny != "" {
		t.Errorf("zero budget should return empty, got %q", tiny)
	}

	// Large budget (no truncation needed)
	short := "hello"
	result := TruncateToTokenBudget(short, 1000)
	if result != short {
		t.Errorf("no truncation needed: got %q, want %q", result, short)
	}
}
