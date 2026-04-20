# data-agent Architecture Design

> Status: Draft
> Date: 2026-04-20

## Overview

data-agent is a desktop GUI tool specialized for interactive data analysis. Built with Go + Wails v2 + React, it manages independent DuckDB instances per case. The LLM backend supports both Vertex AI (Gemini) and local LLM (OpenAI-compatible API) through a loosely coupled interface.

## Design Principles

1. **Case Isolation** — Each case has its own DB file. Concurrent access issues are structurally eliminated.
2. **LLM Loose Coupling** — Backend-agnostic interface. Switching requires only configuration changes.
3. **Plan-Driven Analysis** — Planning→Execution→Review loop. LLM structures the plan, code executes it.
4. **Token Budget Management** — Dynamic context allocation. Reflects lessons from data-analyzer.
5. **Safety** — Read-only SQL constraints, prompt injection defense, container sandboxing.
6. **Transparency** — Log window provides always-visible operation status. Current phase is always displayed.

## Package Structure

```
internal/
├── casemgr/      Case management & DB lifecycle
├── dbengine/     DuckDB operations, data import, SQL execution
├── llm/          LLM client interface & backend implementations
├── session/      Analysis session, phase management, investigation plan
├── analysis/     SQL generation, execution, sliding window analysis
├── job/          Job management & background execution
├── report/       Report generation & export (includes plan + execution log)
├── config/       config.toml management
├── container/    Podman/Docker execution (Phase 2)
└── logger/       Structured logging & event emission
```

## 1. Case Management (`internal/casemgr/`)

### Storage Layout

```
~/Library/Application Support/data-agent/
├── config.toml
├── cases/
│   └── {case-id}/
│       ├── meta.json              Case metadata
│       ├── data.duckdb            Analysis data
│       ├── sessions/              Analysis sessions
│       │   └── {session-id}/
│       │       ├── session.json   Session state & phase
│       │       ├── plan.json      Investigation plan (versioned)
│       │       ├── chat.json      Conversation log
│       │       ├── execlog.json   Execution record (SQL, results, decisions)
│       │       ├── findings.json  Findings
│       │       └── checkpoints/   Job checkpoints
│       └── reports/               Generated reports
└── logs/
    └── data-agent.log
```

### Lifecycle

Opening a case creates a DBEngine instance; closing destroys it. Background jobs increment a reference count, preventing premature close. **Rejected alternative:** Central DB with case-based table separation — DuckDB's single-writer constraint makes concurrent access difficult.

## 2. DB Engine (`internal/dbengine/`)

DuckDB file open/close, data import (JSON/JSONL/CSV/TSV/SQLite via DuckDB built-in readers), table metadata management, SQL execution with read-only enforcement (inheriting shell-agent's `IsReadOnlySQL()` pattern).

## 3. LLM Interface (`internal/llm/`)

### Interface Design

```go
type Backend interface {
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req *ChatRequest, cb StreamCallback) error
    EstimateTokens(text string) int
    Name() string
}
```

- **VertexAIBackend:** Uses `google.golang.org/genai` SDK with ADC authentication
- **LocalLLMBackend:** Raw HTTP client for OpenAI-compatible `/v1/chat/completions`

Both support streaming. Dual token estimation (word-based + char-based, takes max). Exponential backoff retry with pre-flight health checks.

## 4. Analysis Session (`internal/session/`)

Data analysis is not a single query execution but a Planning→Execution→Review loop. The `session` package manages this entire loop.

### Design Decisions

- **Separation of LLM and execution roles** — LLM generates a structured analysis plan; code executes it sequentially. Using logical code execution instead of LLM tool calling ensures reproducibility and control.
- **Plan as an auditable artifact** — Including the investigation plan and execution log in reports ensures analysis credibility.
- **Phase transitions are automatic but visible** — The system transitions phases automatically, but the current phase is always displayed in the UI.

### Phase State Machine

```
Planning ──(user approves)──→ Execution ──(all steps done)──→ Review
   ↑                            ↑    |                         |
   |                            |    ↓                         |
   |                       (dynamic step add)                  |
   └──────────(additional analysis)────────────────────────────┘
```

### Investigation Plan

A declarative pipeline that the LLM outputs as structured JSON and code executes.

```go
type Plan struct {
    Objective    string
    Perspectives []Perspective
    Version      int            // Incremented on each revision
    History      []PlanRevision // Change history (what changed and why)
}

type Step struct {
    ID          string
    Type        StepType       // sql, interpret, aggregate, container
    Description string
    SQL         string         // For Type=sql
    DependsOn   []string       // Dependent step IDs
    Status      StepStatus     // planned, running, done, failed, skipped, revised
    Result      *StepResult
    Error       *StepError
    RetryCount  int
}
```

### Step Types

| Type | Executor | Purpose |
|------|----------|---------|
| `sql` | Code | SQL execution, result collection |
| `interpret` | LLM | Interpret previous step results |
| `aggregate` | LLM | Synthesize results from multiple steps |
| `container` | Code | Python code execution (Phase 2) |

### Planning Phase

User and LLM collaborate through dialogue to build up the investigation plan structurally. The LLM responds in natural language during discussion and outputs structured plan JSON when the discussion is sufficient. Phase transitions to Execution when the user approves the plan.

### Execution Phase

Code executes plan steps sequentially. LLM is involved in interpret/aggregate steps and error recovery.

### Error Handling Strategy

Three-tier response based on error severity:

| Level | Situation | Response | Phase Transition |
|-------|-----------|----------|------------------|
| **Minor** | SQL syntax error, type mismatch | LLM regenerates SQL with schema+error feedback (max 3 retries) | None |
| **Moderate** | Column missing, empty data | Modify or skip step, notify user | None |
| **Critical** | Perspective premise collapsed | Trace dependency graph to identify impact scope → replan | Execution → Planning |

### Dependency Graph Analysis

When a step fails critically, all dependent steps are recursively identified and marked as skipped. LLM is asked to re-evaluate the plan, and the user confirms before re-planning.

### Execution Log

All executions are recorded to ensure report credibility:

```go
type ExecEntry struct {
    StepID      string
    Type        StepType
    SQL         string         // Executed SQL
    Result      *StepResult    // Result summary
    Error       string         // Error (if any)
    Decision    string         // Decision made on error
    Duration    time.Duration
    Timestamp   time.Time
    PlanVersion int            // Plan version at execution time
}
```

### Review Phase

After all steps complete, LLM synthesizes findings and presents to user. User decides: additional analysis (back to Planning) or finalize (generate report).

### Ad-hoc `/sql` Mode

Direct SQL execution available regardless of phase. Results are recorded in ExecLog but not treated as plan steps.

## 5. Analysis Engine (`internal/analysis/`)

SQL generation (including plan step SQL construction), SQL execution and result collection, sliding window analysis (following data-analyzer pattern), citation verification (3-layer: index validation, relevance check, forced original replacement).

### Context Budget Management

Dynamic allocation (128K default):
- System prompt: 2K (fixed)
- Schema context: variable
- Investigation plan context: variable
- Conversation history: max 20K (oldest compressed)
- Step result context: max 30K
- Response buffer: 5K (fixed)

## 6. Job Management (`internal/job/`)

Foreground (chat SQL with streaming) and background (sliding window, long-running analysis). Background jobs increment case DB reference count. Atomic checkpointing (write-to-temp-then-rename).

## 7. Report Generation (`internal/report/`)

Reports integrate investigation plan, execution log, and findings to ensure analysis reproducibility and credibility.

### Report Structure

```markdown
# Analysis Report: {Title}

## 1. Investigation Plan
- Objective, perspectives, steps (with plan version)
- Plan revision history (reasons and changes)

## 2. Execution Record
- Step execution result summaries
- Error handling and decisions
- Ad-hoc SQL execution records

## 3. Findings
- Findings list (by severity)
- Citation data (original record references)

## 4. Conclusion
- LLM-synthesized analysis

## 5. Metadata
- LLM backend used, token usage, analysis duration
```

## 8. Config Management (`internal/config/`)

BurntSushi/toml + environment variable override (Vertex AI config.toml unified pattern). No CLI flags (GUI app).

## 9. Logger (`internal/logger/`)

File logging + event emission for log window. Structured JSON format. `EventsEmit("log:entry", entry)` for frontend display.

## 10. Frontend Architecture

### Component Structure

```
App
├── CaseListView              Case list & management
├── AnalysisView              Main analysis screen
│   ├── PhaseIndicator        Current phase (Planning/Execution/Review)
│   ├── ChatPanel             Chat + result display
│   │   ├── MessageList
│   │   ├── ResultTable
│   │   ├── ResultChart       (Phase 3)
│   │   └── ChatInput
│   ├── SidePanel
│   │   ├── PlanView          Investigation plan & step status
│   │   ├── TableList         Schema browser
│   │   ├── JobList           Job status
│   │   └── ReportList
│   └── LogPanel              Log window (bottom)
└── SettingsView
```

### Wails Events

| Event | Direction | Purpose |
|-------|-----------|---------|
| `chat:stream` | Go→React | LLM streaming tokens |
| `chat:complete` | Go→React | LLM response complete |
| `session:phase` | Go→React | Phase transition |
| `session:step` | Go→React | Step status update |
| `session:replan_required` | Go→React | Replan request (with error info) |
| `job:progress` | Go→React | Job progress |
| `job:complete` | Go→React | Job completion |
| `log:entry` | Go→React | Log entry |
| `case:updated` | Go→React | Case state change |

## 11. Data Flow

### Full Analysis Session Flow

```
[User creates session] → Phase: Planning
    ↓
[Planning loop]
    ├── User input → LLM dialogue (plan building prompt)
    ├── LLM response (natural language or structured plan JSON)
    └── User approves → Phase: Execution
    ↓
[Execution]
    ├── For each perspective → for each step:
    │   ├── sql       → DBEngine.Execute → record result
    │   ├── interpret → LLM call → record interpretation
    │   ├── aggregate → LLM call → record synthesis
    │   └── error     → handleError (3-tier)
    │       ├── Minor    → SQL retry with feedback
    │       ├── Moderate → modify/skip step
    │       └── Critical → identify impact → Phase: Planning
    └── All steps done → Phase: Review
    ↓
[Review]
    ├── LLM: synthesize findings + suggest additional analysis
    ├── User decides:
    │   ├── Additional analysis → Phase: Planning
    │   └── Finalize → generate report
    ↓
[Report.GenerateFromSession]
    └── Plan + execution log + findings → Markdown → save to case
```

## Dependency Graph

```
app.go (Wails bindings)
  ├── casemgr
  │   └── dbengine
  ├── session
  │   ├── analysis
  │   │   ├── llm (Backend interface)
  │   │   │   ├── vertexai (genai SDK)
  │   │   │   └── local (HTTP client)
  │   │   └── dbengine
  │   └── report
  ├── job
  │   └── session
  ├── config
  └── logger
```

No circular dependencies. `dbengine` is LLM-unaware. `analysis` bridges `dbengine` and `llm`. `session` orchestrates `analysis` and `report`. `job` manages background execution of `session` but `session` doesn't know about `job`.

## Security Considerations

1. **SQL injection prevention:** `IsReadOnlySQL()` + `sanitizeIdentifier()`
2. **Prompt injection prevention:** `nlk/guard` nonce-tag wrapping
3. **Container isolation:** Podman/Docker sandbox with network/filesystem restrictions
4. **Credentials:** API keys in config.toml protected by file permissions, never committed
5. **LLM output validation:** JSON schema validation + citation verification (semantic consistency checks)
