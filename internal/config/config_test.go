package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.LLM.Backend != "local" {
		t.Errorf("default backend = %q, want %q", cfg.LLM.Backend, "local")
	}
	if cfg.VertexAI.Region != "us-central1" {
		t.Errorf("default region = %q, want %q", cfg.VertexAI.Region, "us-central1")
	}
	if cfg.Analysis.ContextLimit != 131072 {
		t.Errorf("default context_limit = %d, want %d", cfg.Analysis.ContextLimit, 131072)
	}
	if cfg.Tuning.CJKTokenRatio != 2.0 {
		t.Errorf("default cjk_token_ratio = %f, want %f", cfg.Tuning.CJKTokenRatio, 2.0)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[llm]
backend = "vertex_ai"

[vertex_ai]
project = "test-project"
region = "asia-northeast1"
model = "gemini-2.5-pro"

[local_llm]
endpoint = "http://localhost:8080/v1"
model = "custom-model"
api_key = "test-key"

[analysis]
context_limit = 65536
max_findings = 50
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.LLM.Backend != "vertex_ai" {
		t.Errorf("backend = %q, want %q", cfg.LLM.Backend, "vertex_ai")
	}
	if cfg.VertexAI.Project != "test-project" {
		t.Errorf("project = %q, want %q", cfg.VertexAI.Project, "test-project")
	}
	if cfg.VertexAI.Region != "asia-northeast1" {
		t.Errorf("region = %q, want %q", cfg.VertexAI.Region, "asia-northeast1")
	}
	if cfg.LocalLLM.APIKey != "test-key" {
		t.Errorf("api_key = %q, want %q", cfg.LocalLLM.APIKey, "test-key")
	}
	if cfg.Analysis.ContextLimit != 65536 {
		t.Errorf("context_limit = %d, want %d", cfg.Analysis.ContextLimit, 65536)
	}
	// Defaults preserved for unset fields
	if cfg.Container.Runtime != "podman" {
		t.Errorf("container runtime = %q, want default %q", cfg.Container.Runtime, "podman")
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	// Should return defaults
	if cfg.LLM.Backend != "local" {
		t.Errorf("backend = %q, want default %q", cfg.LLM.Backend, "local")
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("DATA_AGENT_LLM_BACKEND", "vertex_ai")
	t.Setenv("DATA_AGENT_VERTEX_PROJECT", "env-project")
	t.Setenv("DATA_AGENT_CONTEXT_LIMIT", "32768")
	t.Setenv("DATA_AGENT_LOCAL_API_KEY", "env-key")

	cfg, err := Load("/nonexistent/config.toml")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.LLM.Backend != "vertex_ai" {
		t.Errorf("backend = %q, want %q", cfg.LLM.Backend, "vertex_ai")
	}
	if cfg.VertexAI.Project != "env-project" {
		t.Errorf("project = %q, want %q", cfg.VertexAI.Project, "env-project")
	}
	if cfg.Analysis.ContextLimit != 32768 {
		t.Errorf("context_limit = %d, want %d", cfg.Analysis.ContextLimit, 32768)
	}
	if cfg.LocalLLM.APIKey != "env-key" {
		t.Errorf("api_key = %q, want %q", cfg.LocalLLM.APIKey, "env-key")
	}
}

func TestValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Analysis.ContextLimit = 100 // too low
	cfg.Analysis.MaxRecordsPerWindow = 1
	cfg.Tuning.CharsPerToken = 0

	validate(cfg)

	if cfg.Analysis.ContextLimit != 131072 {
		t.Errorf("context_limit = %d, want reset to %d", cfg.Analysis.ContextLimit, 131072)
	}
	if cfg.Analysis.MaxRecordsPerWindow != 200 {
		t.Errorf("max_records = %d, want reset to %d", cfg.Analysis.MaxRecordsPerWindow, 200)
	}
	if cfg.Tuning.CharsPerToken != 4 {
		t.Errorf("chars_per_token = %d, want reset to %d", cfg.Tuning.CharsPerToken, 4)
	}
}

func TestJSONSerialization(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.Backend = "local"
	cfg.LocalLLM.Endpoint = "http://localhost:1234/v1"
	cfg.LocalLLM.Model = "gemma-4-12b"
	cfg.LocalLLM.APIKey = "test-key"
	cfg.VertexAI.Project = "my-project"
	cfg.Analysis.ContextLimit = 65536
	cfg.Window.Width = 1400

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Verify snake_case keys (matching frontend expectations)
	jsonStr := string(data)
	checks := []struct {
		key  string
		want string
	}{
		{`"llm"`, "top-level llm key"},
		{`"backend":"local"`, "llm.backend"},
		{`"local_llm"`, "top-level local_llm key"},
		{`"endpoint":"http://localhost:1234/v1"`, "local_llm.endpoint"},
		{`"api_key":"test-key"`, "local_llm.api_key"},
		{`"vertex_ai"`, "top-level vertex_ai key"},
		{`"project":"my-project"`, "vertex_ai.project"},
		{`"context_limit":65536`, "analysis.context_limit"},
		{`"max_records_per_window"`, "analysis.max_records_per_window"},
		{`"overlap_ratio"`, "analysis.overlap_ratio"},
		{`"window"`, "top-level window key"},
		{`"width":1400`, "window.width"},
	}
	for _, c := range checks {
		if !contains(jsonStr, c.key) {
			t.Errorf("JSON missing %s: expected key %q in %s", c.want, c.key, truncJSON(jsonStr))
		}
	}

	// Round-trip: unmarshal back
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.LocalLLM.Endpoint != "http://localhost:1234/v1" {
		t.Errorf("endpoint = %q after JSON round-trip", loaded.LocalLLM.Endpoint)
	}
	if loaded.Analysis.ContextLimit != 65536 {
		t.Errorf("context_limit = %d after JSON round-trip", loaded.Analysis.ContextLimit)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && searchStr(s, substr))
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func truncJSON(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.toml")

	cfg := DefaultConfig()
	cfg.LLM.Backend = "vertex_ai"
	cfg.VertexAI.Project = "round-trip"

	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.LLM.Backend != "vertex_ai" {
		t.Errorf("backend = %q, want %q", loaded.LLM.Backend, "vertex_ai")
	}
	if loaded.VertexAI.Project != "round-trip" {
		t.Errorf("project = %q, want %q", loaded.VertexAI.Project, "round-trip")
	}
}
