package llm

import (
	"regexp"
	"strings"
)

// toolCallRe matches Gemma-style tool call blocks.
var toolCallRe = regexp.MustCompile(`<\|?tool_call\|?>[\s\S]*?<\|?tool_call\|?>`)

// thinkRe matches thinking tag blocks.
var thinkRe = regexp.MustCompile(`<think>[\s\S]*?</think>`)

// StripArtifacts removes raw model artifacts from LLM responses.
func StripArtifacts(s string) string {
	// Remove matched pairs
	s = toolCallRe.ReplaceAllString(s, "")
	s = thinkRe.ReplaceAllString(s, "")

	// Remove unclosed tags (truncate from opening tag)
	if idx := strings.Index(s, "<think>"); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.Index(s, "<|tool_call>"); idx >= 0 {
		s = s[:idx]
	}

	// Clean up any remaining standalone tags
	s = strings.ReplaceAll(s, "<|tool_call|>", "")
	s = strings.ReplaceAll(s, "<tool_call|>", "")
	s = strings.ReplaceAll(s, "<|tool_call>", "")

	return strings.TrimSpace(s)
}
