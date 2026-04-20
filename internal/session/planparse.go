package session

import (
	"encoding/json"
	"strings"
)

// rawPlan is the LLM output format for plan JSON.
type rawPlan struct {
	Objective    string            `json:"objective"`
	Perspectives []rawPerspective  `json:"perspectives"`
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
// Returns the parsed Plan and the remaining text (non-JSON parts).
// Returns nil if no valid plan JSON is found.
func ExtractPlanJSON(text string) (*Plan, string) {
	// Try to find JSON block in markdown code fence
	jsonStr, remaining := extractJSONFromCodeFence(text)
	if jsonStr != "" {
		if plan := tryParsePlan(jsonStr); plan != nil {
			return plan, remaining
		}
	}

	// Try to find a top-level JSON object with "objective" or "perspectives"
	jsonStr, remaining = extractJSONObject(text)
	if jsonStr != "" {
		if plan := tryParsePlan(jsonStr); plan != nil {
			return plan, remaining
		}
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

// extractJSONFromCodeFence extracts JSON from ```json ... ``` blocks.
func extractJSONFromCodeFence(text string) (string, string) {
	// Look for ```json or ``` followed by {
	markers := []string{"```json\n", "```json\r\n", "```\n{"}
	for _, marker := range markers {
		start := strings.Index(text, marker)
		if start == -1 {
			continue
		}

		jsonStart := start + len(marker)
		if marker == "```\n{" {
			jsonStart = start + 4 // after "```\n", include the "{"
		}

		end := strings.Index(text[jsonStart:], "```")
		if end == -1 {
			continue
		}

		jsonStr := strings.TrimSpace(text[jsonStart : jsonStart+end])
		remaining := strings.TrimSpace(text[:start] + text[jsonStart+end+3:])
		return jsonStr, remaining
	}
	return "", text
}

// extractJSONObject finds the outermost { ... } that looks like a plan.
func extractJSONObject(text string) (string, string) {
	start := strings.Index(text, "{")
	if start == -1 {
		return "", text
	}

	// Find matching closing brace
	depth := 0
	inString := false
	for i := start; i < len(text); i++ {
		if inString {
			if text[i] == '\\' {
				i++ // skip escaped char
				continue
			}
			if text[i] == '"' {
				inString = false
			}
			continue
		}
		switch text[i] {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				jsonStr := text[start : i+1]
				// Quick check: does it look like a plan?
				if strings.Contains(jsonStr, "objective") || strings.Contains(jsonStr, "perspectives") {
					remaining := strings.TrimSpace(text[:start] + text[i+1:])
					return jsonStr, remaining
				}
				return "", text
			}
		}
	}
	return "", text
}
