# data-agent Architecture Design

> Status: Draft
> Date: 2026-04-20

## Overview

data-agent is a desktop GUI tool specialized for interactive data analysis. Built with Go + Wails v2 + React, it manages independent DuckDB instances per case. The LLM backend supports both Vertex AI (Gemini) and local LLM (OpenAI-compatible API) through a loosely coupled interface.

## Design Principles

1. **Case Isolation** — Each case has its own DB file. Concurrent access issues are structurally eliminated.
2. **LLM Loose Coupling** — Backend-agnostic interface. Switching requires only configuration changes.
3. **Token Budget Management** — Dynamic context allocation. Reflects lessons from data-analyzer.
4. **Safety** — Read-only SQL constraints, prompt injection defense, container sandboxing.
5. **Transparency** — Log window provides always-visible operation status.

## Package Structure

```
internal/
├── casemgr/      Case management & DB lifecycle
├── dbengine/     DuckDB operations, data import, SQL execution
├── llm/          LLM client interface & backend implementations
├── analysis/     NL→SQL conversion, sliding window analysis
├── job/          Job management & background execution
├── report/       Report generation & export
├── config/       config.toml management
├── container/    Podman/Docker execution (Phase 2)
└── logger/       Structured logging & event emission
```

## 1. Case Management (`internal/casemgr/`)

### Data Model

```go
type Case struct {
    ID        string    // UUID
    Name      string    // User-defined name
    CreatedAt time.Time
    UpdatedAt time.Time
    Status    Status    // open, closed
}
```

### Storage Layout

```
~/Library/Application Support/data-agent/
├── config.toml
├── cases/
│   └── {case-id}/
│       ├── meta.json          Case metadata
│       ├── data.duckdb        Analysis data
│       ├── chat.json          Chat history
│       ├── reports/           Generated reports
│       └── jobs/              Job checkpoints
└── logs/
    └── data-agent.log
```

### Lifecycle

```
Create → Open → [Analysis] → Close → (Re-open) → Delete
                    ↓
              Background Job
              (blocks Close)
```

**Design decision:** Opening a case creates a DBEngine instance; closing destroys it. Background jobs increment a reference count, preventing premature close. **Rejected alternative:** Central DB with case-based table separation — DuckDB's single-writer constraint makes concurrent access difficult.

### CaseManager Interface

```go
type CaseManager struct {
    baseDir string
    cases   map[string]*openCase
    mu      sync.RWMutex
}

type openCase struct {
    meta   Case
    engine *dbengine.Engine
    jobs   map[string]*job.Job
    refCnt int32 // reference count from background jobs
}
```

## 2. DB Engine (`internal/dbengine/`)

### Responsibilities

- DuckDB file open/close
- Data import (JSON/JSONL/CSV/TSV/SQLite)
- Table metadata management
- SQL execution with read-only enforcement

### SQL Safety

Inherits shell-agent's `IsReadOnlySQL()` pattern:
- Prefix check (SELECT/EXPLAIN/DESCRIBE/SHOW/WITH)
- Dangerous keyword scan (after stripping literals/comments)
- Multi-statement rejection

### Data Import

Leverages DuckDB's built-in readers: `read_json_auto()`, `read_csv_auto()`, SQLite scanner extension.

## 3. LLM Interface (`internal/llm/`)

### Interface Design

```go
type Backend interface {
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req *ChatRequest, cb StreamCallback) error
    EstimateTokens(text string) int
    Name() string
}

type StreamCallback func(token string, done bool)
```

### Backend Implementations

- **VertexAIBackend:** Uses `google.golang.org/genai` SDK with ADC authentication
- **LocalLLMBackend:** Raw HTTP client for OpenAI-compatible `/v1/chat/completions`

Both support streaming. Factory function creates the appropriate backend from config.

### Token Estimation

Dual-method approach (from data-analyzer): `max(word-based, char-based)`. Word-based alone underestimates JSON punctuation by 4-5x.

### Resilience

Exponential backoff retry (max 10 attempts, 2s-120s). Pre-flight health checks for local LLM. Model crash detection and recovery polling.

## 4. Analysis Engine (`internal/analysis/`)

### Natural Language → SQL

Builds prompt with schema context + chat history + guard-tagged user input. LLM generates SQL, which is validated via `IsReadOnlySQL()` before execution.

### Context Budget Management

Dynamic allocation (128K default):
- System prompt: 2K (fixed)
- Schema context: variable (depends on table count)
- Chat history: max 20K (oldest compressed)
- Query result context: max 30K
- Response buffer: 5K (fixed)
- Remainder: user prompt + data

### Sliding Window Analysis

Follows data-analyzer pattern: running summary + findings accumulation across windows. Citation verification (3-layer: index validation, relevance check, forced original replacement). Atomic checkpointing for crash recovery.

## 5. Job Management (`internal/job/`)

- **Foreground:** Chat SQL execution with streaming response
- **Background:** Sliding window analysis, container execution. Increments case DB reference count.
- Completion: Wails EventsEmit notification, result saved as case report

Atomic checkpoint pattern (write-to-temp-then-rename) from data-analyzer.

## 6. Config Management (`internal/config/`)

Uses BurntSushi/toml + environment variable override (Vertex AI config.toml unified pattern). No CLI flags (GUI app).

## 7. Frontend Architecture

### Component Structure

```
App
├── CaseListView          Case list & management
├── AnalysisView          Main analysis screen
│   ├── ChatPanel         Chat + result display
│   │   ├── MessageList
│   │   ├── ResultTable
│   │   ├── ResultChart   (Phase 3)
│   │   └── ChatInput
│   ├── SidePanel
│   │   ├── TableList     Schema browser
│   │   ├── JobList       Job status
│   │   └── ReportList
│   └── LogPanel          Log window (bottom)
└── SettingsView
```

### Wails Events

| Event | Direction | Purpose |
|-------|-----------|---------|
| `chat:stream` | Go→React | LLM streaming tokens |
| `chat:complete` | Go→React | LLM response complete |
| `job:progress` | Go→React | Job progress update |
| `job:complete` | Go→React | Job completion |
| `log:entry` | Go→React | Log entry |
| `case:updated` | Go→React | Case state change |

## 8. Dependency Graph

```
app.go (Wails bindings)
  ├── casemgr
  │   └── dbengine
  ├── analysis
  │   ├── llm (Backend interface)
  │   │   ├── vertexai (genai SDK)
  │   │   └── local (HTTP client)
  │   └── dbengine
  ├── job
  │   └── analysis
  ├── report
  ├── config
  └── logger
```

No circular dependencies. `dbengine` is LLM-unaware. `analysis` bridges `dbengine` and `llm`. `logger` is referenced from any package (always downward dependency).

## Security Considerations

1. **SQL injection prevention:** `IsReadOnlySQL()` + `sanitizeIdentifier()`
2. **Prompt injection prevention:** `nlk/guard` nonce-tag wrapping
3. **Container isolation:** Podman/Docker sandbox with network/filesystem restrictions
4. **Credentials:** API keys in config.toml protected by file permissions, never committed
5. **LLM output validation:** JSON schema validation + citation verification (semantic consistency checks)
