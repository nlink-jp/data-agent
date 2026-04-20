package analysis

import "github.com/nlink-jp/data-agent/internal/llm"

// BudgetConfig holds token budget allocation parameters.
type BudgetConfig struct {
	ContextLimit    int // Total context window (default: 131072)
	SystemReserve   int // System prompt reserve (default: 2000)
	ResponseReserve int // Response buffer (default: 5000)
	MaxHistory      int // Max tokens for conversation history (default: 20000)
	MaxStepResults  int // Max tokens for step result context (default: 30000)
}

// DefaultBudgetConfig returns budget config with sensible defaults.
func DefaultBudgetConfig() BudgetConfig {
	return BudgetConfig{
		ContextLimit:    131072,
		SystemReserve:   2000,
		ResponseReserve: 5000,
		MaxHistory:      20000,
		MaxStepResults:  30000,
	}
}

// MemoryMap represents the dynamic token allocation for a prompt.
type MemoryMap struct {
	SystemPrompt int // Actual tokens used by system prompt
	Schema       int // Actual tokens used by schema context
	Plan         int // Actual tokens used by plan context
	History      int // Allocated for conversation history
	StepResults  int // Allocated for step result context
	Response     int // Reserved for response
	Available    int // Remaining budget for user prompt + data
}

// ComputeMemoryMap calculates dynamic token allocation based on current context.
func ComputeMemoryMap(cfg BudgetConfig, systemPrompt, schemaCtx, planCtx string) MemoryMap {
	systemTokens := llm.EstimateTokenCount(systemPrompt)
	schemaTokens := llm.EstimateTokenCount(schemaCtx)
	planTokens := llm.EstimateTokenCount(planCtx)

	used := systemTokens + schemaTokens + planTokens + cfg.ResponseReserve
	remaining := cfg.ContextLimit - used

	historyBudget := cfg.MaxHistory
	stepResultBudget := cfg.MaxStepResults

	// If budget is tight, reduce history and step results proportionally
	if remaining < historyBudget+stepResultBudget {
		total := historyBudget + stepResultBudget
		if remaining > 0 {
			historyBudget = remaining * historyBudget / total
			stepResultBudget = remaining * stepResultBudget / total
		} else {
			historyBudget = 0
			stepResultBudget = 0
		}
	}

	available := remaining - historyBudget - stepResultBudget
	if available < 0 {
		available = 0
	}

	return MemoryMap{
		SystemPrompt: systemTokens,
		Schema:       schemaTokens,
		Plan:         planTokens,
		History:      historyBudget,
		StepResults:  stepResultBudget,
		Response:     cfg.ResponseReserve,
		Available:    available,
	}
}

// TruncateToTokenBudget truncates text to fit within the given token budget.
func TruncateToTokenBudget(text string, budget int) string {
	if budget <= 0 {
		return ""
	}
	tokens := llm.EstimateTokenCount(text)
	if tokens <= budget {
		return text
	}

	// Binary search for the right length
	runes := []rune(text)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if llm.EstimateTokenCount(string(runes[:mid])) <= budget {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return ""
	}
	return string(runes[:lo]) + "\n[truncated]"
}
