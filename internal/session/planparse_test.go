package session

import (
	"testing"
)

func TestExtractPlanJSON_CodeFence(t *testing.T) {
	text := `分析計画を作成しました。

` + "```json\n" + `{
  "objective": "Analyze incident trends",
  "perspectives": [
    {
      "id": "P1",
      "description": "Category distribution",
      "steps": [
        {"id": "S1", "type": "sql", "description": "Count by category", "sql": "SELECT category, COUNT(*) FROM incidents GROUP BY category", "depends_on": []},
        {"id": "S2", "type": "interpret", "description": "Interpret distribution", "depends_on": ["S1"]}
      ]
    }
  ]
}
` + "```\n" + `
このプランで進めますか？`

	plan, remaining := ExtractPlanJSON(text)
	if plan == nil {
		t.Fatal("expected plan to be extracted")
	}
	if plan.Objective != "Analyze incident trends" {
		t.Errorf("objective = %q", plan.Objective)
	}
	if len(plan.Perspectives) != 1 {
		t.Fatalf("perspectives = %d, want 1", len(plan.Perspectives))
	}
	if len(plan.Perspectives[0].Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(plan.Perspectives[0].Steps))
	}
	if plan.Perspectives[0].Steps[0].Type != StepTypeSQL {
		t.Errorf("step type = %q, want sql", plan.Perspectives[0].Steps[0].Type)
	}
	if plan.Perspectives[0].Steps[1].Type != StepTypeInterpret {
		t.Errorf("step type = %q, want interpret", plan.Perspectives[0].Steps[1].Type)
	}
	if plan.Perspectives[0].Steps[0].SQL == "" {
		t.Error("SQL should not be empty for sql step")
	}
	if len(plan.Perspectives[0].Steps[1].DependsOn) != 1 {
		t.Error("S2 should depend on S1")
	}

	if remaining == "" {
		t.Error("remaining text should not be empty")
	}
	if remaining == text {
		t.Error("remaining should differ from original")
	}
}

func TestExtractPlanJSON_InlineJSON(t *testing.T) {
	text := `計画は以下の通りです：
{"objective": "Test", "perspectives": [{"id": "P1", "description": "Test", "steps": [{"id": "S1", "type": "sql", "description": "Count", "sql": "SELECT 1"}]}]}
よろしいですか？`

	plan, _ := ExtractPlanJSON(text)
	if plan == nil {
		t.Fatal("expected plan from inline JSON")
	}
	if plan.Objective != "Test" {
		t.Errorf("objective = %q", plan.Objective)
	}
}

func TestExtractPlanJSON_NoJSON(t *testing.T) {
	text := "Let me think about how to analyze this data. What aspects are you most interested in?"

	plan, remaining := ExtractPlanJSON(text)
	if plan != nil {
		t.Error("expected nil plan for text without JSON")
	}
	if remaining != text {
		t.Error("remaining should equal original when no JSON found")
	}
}

func TestExtractPlanJSON_InvalidJSON(t *testing.T) {
	text := "```json\n{invalid json}\n```"

	plan, _ := ExtractPlanJSON(text)
	if plan != nil {
		t.Error("expected nil plan for invalid JSON")
	}
}

func TestExtractPlanJSON_NonPlanJSON(t *testing.T) {
	text := `{"key": "value", "count": 42}`

	plan, _ := ExtractPlanJSON(text)
	if plan != nil {
		t.Error("expected nil for JSON without objective/perspectives")
	}
}

func TestExtractPlanJSON_StepTypes(t *testing.T) {
	text := `{"objective": "Test types", "perspectives": [{"id": "P1", "description": "Test", "steps": [
		{"id": "S1", "type": "sql", "description": "SQL step"},
		{"id": "S2", "type": "interpret", "description": "Interpret step", "depends_on": ["S1"]},
		{"id": "S3", "type": "aggregate", "description": "Aggregate step", "depends_on": ["S1", "S2"]},
		{"id": "S4", "type": "container", "description": "Container step"}
	]}]}`

	plan, _ := ExtractPlanJSON(text)
	if plan == nil {
		t.Fatal("expected plan")
	}

	steps := plan.Perspectives[0].Steps
	expected := []StepType{StepTypeSQL, StepTypeInterpret, StepTypeAggregate, StepTypeContainer}
	for i, e := range expected {
		if steps[i].Type != e {
			t.Errorf("step[%d].Type = %q, want %q", i, steps[i].Type, e)
		}
	}
}
