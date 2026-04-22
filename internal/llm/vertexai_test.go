package llm

import (
	"testing"

	"google.golang.org/genai"
)

func TestBuildContents_RoleMapping(t *testing.T) {
	b := &VertexAIBackend{}
	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
			{Role: "system", Content: "Session reopened"},
			{Role: "user", Content: "next question"},
		},
	}

	contents := b.buildContents(req)

	// "system" message must be skipped → 3 contents
	if len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(contents))
	}

	wantRoles := []genai.Role{"user", "model", "user"}
	wantTexts := []string{"hello", "hi there", "next question"}

	for i, c := range contents {
		if genai.Role(c.Role) != wantRoles[i] {
			t.Errorf("contents[%d].Role = %q, want %q", i, c.Role, wantRoles[i])
		}
		if len(c.Parts) == 0 {
			t.Fatalf("contents[%d] has no parts", i)
		}
		if c.Parts[0].Text != wantTexts[i] {
			t.Errorf("contents[%d].Parts[0].Text = %q, want %q", i, c.Parts[0].Text, wantTexts[i])
		}
	}
}

func TestBuildContents_EmptyMessages(t *testing.T) {
	b := &VertexAIBackend{}
	contents := b.buildContents(&ChatRequest{})
	if len(contents) != 0 {
		t.Errorf("expected 0 contents for empty messages, got %d", len(contents))
	}
}

func TestBuildContents_SystemOnly(t *testing.T) {
	b := &VertexAIBackend{}
	req := &ChatRequest{
		Messages: []Message{
			{Role: "system", Content: "ignored"},
		},
	}
	contents := b.buildContents(req)
	if len(contents) != 0 {
		t.Errorf("expected 0 contents when only system messages, got %d", len(contents))
	}
}
