package llm

import "testing"

func TestStripArtifacts(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no artifacts", "Hello world", "Hello world"},
		{"gemma tool call", "text<|tool_call>call:thought:{}<tool_call|>more", "textmore"},
		{"gemma tool call alt", "text<|tool_call>call:thought:{}<|tool_call|>more", "textmore"},
		{"trailing tool call", "text<|tool_call|>", "text"},
		{"think tags (via nlk)", "before<think>internal thought</think>after", "beforeafter"},
		{"thinking tags (via nlk)", "before<thinking>deep thought</thinking>after", "beforeafter"},
		{"unclosed think (via nlk)", "before<think>internal", "before"},
		{"mixed", "<think>thought</think>Hello<|tool_call>x<tool_call|> world", "Hello world"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripArtifacts(tt.input)
			if got != tt.want {
				t.Errorf("StripArtifacts(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
