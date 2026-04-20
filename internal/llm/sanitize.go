package llm

import (
	"regexp"
	"strings"

	"github.com/nlink-jp/nlk/strip"
)

// toolCallRe matches Gemma-style tool call blocks.
var toolCallRe = regexp.MustCompile(`<\|?tool_call\|?>[\s\S]*?<\|?tool_call\|?>`)

// StripArtifacts removes raw model artifacts from LLM responses.
// Uses nlk/strip.ThinkTags for thinking tags (comprehensive coverage)
// and custom regex for tool call tags.
func StripArtifacts(s string) string {
	// nlk/strip handles: <think>, <thinking>, <reasoning>, <reflection>,
	// <|channel>thought (Gemma 4), case-insensitive, unclosed tags
	s = strip.ThinkTags(s)

	// Tool call tags (Gemma-style) — not covered by nlk
	s = toolCallRe.ReplaceAllString(s, "")
	if idx := strings.Index(s, "<|tool_call>"); idx >= 0 {
		s = s[:idx] // unclosed tool call: truncate
	}
	s = strings.ReplaceAll(s, "<|tool_call|>", "")
	s = strings.ReplaceAll(s, "<tool_call|>", "")
	s = strings.ReplaceAll(s, "<|tool_call>", "")

	return strings.TrimSpace(s)
}
