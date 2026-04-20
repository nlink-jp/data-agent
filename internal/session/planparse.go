package session

import (
	"encoding/json"
	"strings"

	"github.com/nlink-jp/nlk/jsonfix"
)

// rawPlan is the LLM output format for plan JSON.
type rawPlan struct {
	Objective    string           `json:"objective"`
	Perspectives []rawPerspective `json:"perspectives"`
}

type rawPerspective struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Steps       []rawStep `json:"steps"`
}

type rawStep struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	SQL         string   `json:"sql"`
	DependsOn   []string `json:"depends_on"`
}

// ExtractPlanJSON attempts to extract and parse a Plan from LLM response text.
// Uses nlk/jsonfix for robust JSON extraction (handles markdown fences,
// malformed JSON, trailing commas, etc.), then validates plan structure.
// Returns the parsed Plan and the remaining text (non-JSON parts).
// Returns nil if no valid plan JSON is found.
func ExtractPlanJSON(text string) (*Plan, string) {
	// Use nlk/jsonfix to extract JSON from LLM output
	extracted, err := jsonfix.Extract(text)
	if err != nil {
		return nil, text
	}

	if plan := tryParsePlan(extracted); plan != nil {
		// Build remaining text by removing the JSON portion
		remaining := removeJSONBlock(text)
		return plan, remaining
	}

	return nil, text
}

func tryParsePlan(jsonStr string) *Plan {
	var raw rawPlan
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil
	}

	// Must have either objective or perspectives to be a valid plan
	if raw.Objective == "" && len(raw.Perspectives) == 0 {
		return nil
	}

	plan := &Plan{
		Objective:    raw.Objective,
		Perspectives: make([]Perspective, len(raw.Perspectives)),
	}

	for i, rp := range raw.Perspectives {
		p := Perspective{
			ID:          rp.ID,
			Description: rp.Description,
			Steps:       make([]Step, len(rp.Steps)),
			Status:      PerspectiveActive,
		}
		for j, rs := range rp.Steps {
			stepType := StepTypeSQL
			switch rs.Type {
			case "sql":
				stepType = StepTypeSQL
			case "sliding_window":
				stepType = StepTypeSlidingWindow
			case "interpret":
				stepType = StepTypeInterpret
			case "aggregate":
				stepType = StepTypeAggregate
			case "container":
				stepType = StepTypeContainer
			}
			p.Steps[j] = Step{
				ID:          rs.ID,
				Type:        stepType,
				Description: rs.Description,
				SQL:         rs.SQL,
				DependsOn:   rs.DependsOn,
				Status:      StepPlanned,
			}
		}
		plan.Perspectives[i] = p
	}

	return plan
}

// removeJSONBlock removes markdown JSON code blocks or bare JSON objects from text.
func removeJSONBlock(text string) string {
	// Try to remove ```json ... ``` block
	if idx := strings.Index(text, "```json"); idx >= 0 {
		end := strings.Index(text[idx+7:], "```")
		if end >= 0 {
			return strings.TrimSpace(text[:idx] + text[idx+7+end+3:])
		}
	}
	// Try to remove ```...``` block containing JSON
	if idx := strings.Index(text, "```\n{"); idx >= 0 {
		end := strings.Index(text[idx+4:], "```")
		if end >= 0 {
			return strings.TrimSpace(text[:idx] + text[idx+4+end+3:])
		}
	}
	return text
}
