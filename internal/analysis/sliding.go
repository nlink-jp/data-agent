package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nlink-jp/data-agent/internal/llm"
	"github.com/nlink-jp/nlk/jsonfix"
)

// SlidingWindowConfig holds configuration for sliding window analysis.
type SlidingWindowConfig struct {
	MaxRecordsPerWindow int
	OverlapRatio        float64
	MaxFindings         int
	ContextLimit        int
}

// DefaultSlidingWindowConfig returns sensible defaults.
func DefaultSlidingWindowConfig() SlidingWindowConfig {
	return SlidingWindowConfig{
		MaxRecordsPerWindow: 200,
		OverlapRatio:        0.1,
		MaxFindings:         100,
		ContextLimit:        131072,
	}
}

// WindowResult holds the accumulated result of sliding window analysis.
type WindowResult struct {
	Summary     string    `json:"summary"`
	Findings    []Finding `json:"findings"`
	Windows     int       `json:"windows"`
	TotalRecords int     `json:"total_records"`
}

// Finding represents a discovery from analysis.
type Finding struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	RecordRefs  []int  `json:"record_refs,omitempty"`
	WindowIndex int    `json:"window_index"`
}

// ProgressCallback reports window processing progress.
type ProgressCallback func(windowIndex, totalWindows int)

// RunSlidingWindow processes records through overlapping windows with LLM analysis.
func RunSlidingWindow(
	ctx context.Context,
	backend llm.Backend,
	records []map[string]any,
	perspective string,
	cfg SlidingWindowConfig,
	onProgress ProgressCallback,
) (*WindowResult, error) {
	if len(records) == 0 {
		return &WindowResult{Summary: "No records to analyze"}, nil
	}

	windowSize := cfg.MaxRecordsPerWindow
	overlap := int(float64(windowSize) * cfg.OverlapRatio)
	step := windowSize - overlap
	if step < 1 {
		step = 1
	}

	totalWindows := (len(records)-1)/step + 1

	var summary string
	var allFindings []Finding

	for windowIdx := 0; windowIdx < totalWindows; windowIdx++ {
		start := windowIdx * step
		end := start + windowSize
		if end > len(records) {
			end = len(records)
		}
		windowRecords := records[start:end]

		if onProgress != nil {
			onProgress(windowIdx, totalWindows)
		}

		// Build window prompt
		systemPrompt := fmt.Sprintf(`You are a data analyst. Analyze the data records below from this perspective: %s

## Instructions
1. Update the running summary incorporating new observations from this window
2. Identify new findings with record references
3. Classify findings by severity: high, medium, low, info

Output ONLY valid JSON:
{
  "summary": "Updated running summary",
  "new_findings": [
    {"id": "F-NNN", "description": "What was found", "severity": "high|medium|low|info", "record_refs": [0, 1]}
  ]
}`, perspective)

		var userPrompt strings.Builder
		if summary != "" {
			fmt.Fprintf(&userPrompt, "## Previous Summary\n%s\n\n", summary)
		}
		if len(allFindings) > 0 {
			findingsJSON, _ := json.Marshal(allFindings)
			trimmed := TruncateToTokenBudget(string(findingsJSON), 20000)
			fmt.Fprintf(&userPrompt, "## Current Findings (%d)\n%s\n\n", len(allFindings), trimmed)
		}

		fmt.Fprintf(&userPrompt, "## Window %d/%d (Records %d-%d of %d)\n", windowIdx+1, totalWindows, start, end-1, len(records))
		for i, rec := range windowRecords {
			recJSON, _ := json.Marshal(rec)
			fmt.Fprintf(&userPrompt, "[Record #%d] %s\n", start+i, string(recJSON))
		}

		resp, err := backend.Chat(ctx, &llm.ChatRequest{
			SystemPrompt: systemPrompt,
			Messages:     []llm.Message{{Role: "user", Content: userPrompt.String()}},
			ResponseJSON: true,
		})
		if err != nil {
			return nil, fmt.Errorf("window %d: %w", windowIdx, err)
		}

		// Parse response using nlk/jsonfix for robust extraction
		var windowResp struct {
			Summary     string    `json:"summary"`
			NewFindings []Finding `json:"new_findings"`
		}
		extracted, extractErr := jsonfix.Extract(resp.Content)
		if extractErr == nil {
			json.Unmarshal([]byte(extracted), &windowResp)
		}

		if windowResp.Summary != "" {
			summary = windowResp.Summary
		}

		for i := range windowResp.NewFindings {
			windowResp.NewFindings[i].WindowIndex = windowIdx
			if windowResp.NewFindings[i].ID == "" {
				windowResp.NewFindings[i].ID = fmt.Sprintf("F-%d-%d", windowIdx, i+1)
			}
		}
		allFindings = append(allFindings, windowResp.NewFindings...)

		// Evict low-priority findings if over limit
		if len(allFindings) > cfg.MaxFindings {
			allFindings = evictFindings(allFindings, cfg.MaxFindings)
		}
	}

	return &WindowResult{
		Summary:      summary,
		Findings:     allFindings,
		Windows:      totalWindows,
		TotalRecords: len(records),
	}, nil
}

// evictFindings keeps high/medium severity and drops oldest low/info.
func evictFindings(findings []Finding, max int) []Finding {
	var important, minor []Finding
	for _, f := range findings {
		if f.Severity == "high" || f.Severity == "critical" || f.Severity == "medium" {
			important = append(important, f)
		} else {
			minor = append(minor, f)
		}
	}
	remaining := max - len(important)
	if remaining < 0 {
		return important[:max]
	}
	if remaining < len(minor) {
		minor = minor[len(minor)-remaining:]
	}
	return append(important, minor...)
}
