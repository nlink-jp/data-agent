package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
)

// Config holds all application configuration.
type Config struct {
	LLM       LLMConfig       `toml:"llm"`
	VertexAI  VertexAIConfig  `toml:"vertex_ai"`
	LocalLLM  LocalLLMConfig  `toml:"local_llm"`
	Analysis  AnalysisConfig  `toml:"analysis"`
	Container ContainerConfig `toml:"container"`
	Tuning    TuningConfig    `toml:"tuning"`
	Window    WindowConfig    `toml:"window"`
}

// WindowConfig holds window position and size.
type WindowConfig struct {
	X      int `toml:"x"`
	Y      int `toml:"y"`
	Width  int `toml:"width"`
	Height int `toml:"height"`
}

// LLMConfig selects the active LLM backend.
type LLMConfig struct {
	Backend string `toml:"backend"` // "vertex_ai" or "local"
}

// VertexAIConfig holds Vertex AI (Gemini) settings.
type VertexAIConfig struct {
	Project string `toml:"project"`
	Region  string `toml:"region"`
	Model   string `toml:"model"`
}

// LocalLLMConfig holds OpenAI-compatible local LLM settings.
type LocalLLMConfig struct {
	Endpoint string `toml:"endpoint"`
	Model    string `toml:"model"`
	APIKey   string `toml:"api_key"`
}

// AnalysisConfig holds analysis engine settings.
type AnalysisConfig struct {
	ContextLimit       int     `toml:"context_limit"`
	OverlapRatio       float64 `toml:"overlap_ratio"`
	MaxFindings        int     `toml:"max_findings"`
	MaxRecordsPerWindow int    `toml:"max_records_per_window"`
}

// ContainerConfig holds container runtime settings.
type ContainerConfig struct {
	Runtime string `toml:"runtime"` // "podman" or "docker"
	Image   string `toml:"image"`
}

// TuningConfig holds token estimation tuning parameters.
type TuningConfig struct {
	CJKTokenRatio  float64 `toml:"cjk_token_ratio"`
	ASCIITokenRatio float64 `toml:"ascii_token_ratio"`
	CharsPerToken  int     `toml:"chars_per_token"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Backend: "local",
		},
		VertexAI: VertexAIConfig{
			Region: "us-central1",
			Model:  "gemini-2.5-flash",
		},
		LocalLLM: LocalLLMConfig{
			Endpoint: "http://localhost:1234/v1",
			Model:    "gemma-4-12b",
		},
		Analysis: AnalysisConfig{
			ContextLimit:        131072,
			OverlapRatio:        0.1,
			MaxFindings:         100,
			MaxRecordsPerWindow: 200,
		},
		Container: ContainerConfig{
			Runtime: "podman",
			Image:   "python:3.12-slim",
		},
		Tuning: TuningConfig{
			CJKTokenRatio:   2.0,
			ASCIITokenRatio: 1.3,
			CharsPerToken:   4,
		},
		Window: WindowConfig{
			Width:  1280,
			Height: 800,
		},
	}
}

// Load reads config from the given path, falling back to defaults.
// Environment variables (DATA_AGENT_*) override file values.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if _, err := os.Stat(path); err == nil {
		if _, err := toml.DecodeFile(path, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	}

	applyEnvOverrides(cfg)
	validate(cfg)
	return cfg, nil
}

// Save writes the config to the given path, creating directories as needed.
func Save(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

// DefaultConfigPath returns the platform-specific config file path.
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "data-agent", "config.toml")
}

// DefaultDataDir returns the platform-specific data directory.
func DefaultDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "data-agent")
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("DATA_AGENT_LLM_BACKEND"); v != "" {
		cfg.LLM.Backend = v
	}
	if v := os.Getenv("DATA_AGENT_VERTEX_PROJECT"); v != "" {
		cfg.VertexAI.Project = v
	}
	if v := os.Getenv("DATA_AGENT_VERTEX_REGION"); v != "" {
		cfg.VertexAI.Region = v
	}
	if v := os.Getenv("DATA_AGENT_VERTEX_MODEL"); v != "" {
		cfg.VertexAI.Model = v
	}
	if v := os.Getenv("DATA_AGENT_LOCAL_ENDPOINT"); v != "" {
		cfg.LocalLLM.Endpoint = v
	}
	if v := os.Getenv("DATA_AGENT_LOCAL_MODEL"); v != "" {
		cfg.LocalLLM.Model = v
	}
	if v := os.Getenv("DATA_AGENT_LOCAL_API_KEY"); v != "" {
		cfg.LocalLLM.APIKey = v
	}
	if v := os.Getenv("DATA_AGENT_CONTEXT_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Analysis.ContextLimit = n
		}
	}
	if v := os.Getenv("DATA_AGENT_CONTAINER_RUNTIME"); v != "" {
		cfg.Container.Runtime = v
	}
}

func validate(cfg *Config) {
	if cfg.Analysis.ContextLimit < 8192 {
		cfg.Analysis.ContextLimit = 131072
	}
	if cfg.Analysis.MaxRecordsPerWindow < 10 {
		cfg.Analysis.MaxRecordsPerWindow = 200
	}
	if cfg.Analysis.MaxFindings < 10 {
		cfg.Analysis.MaxFindings = 100
	}
	if cfg.Tuning.CharsPerToken < 1 {
		cfg.Tuning.CharsPerToken = 4
	}
}
