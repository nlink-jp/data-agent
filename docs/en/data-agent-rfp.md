# RFP: data-agent

> Generated: 2026-04-20
> Status: Draft

## 1. Problem Statement

shell-agent attempted to combine general-purpose chat with data analysis capabilities, which led to increased implementation complexity. data-agent is a new tool built on this lesson, specializing exclusively in data analysis as a desktop GUI application.

It ingests diverse data formats (JSON/JSONL/CSV/TSV/SQLite) into per-case independent DuckDB/SQLite databases, providing interactive data exploration, analysis, and visualization. The DB access layer is instantiated per case and exposed via API, avoiding concurrent access issues.

It supports both single-shot SQL aggregation and sliding window analysis, with table/graph display and Markdown report output. Multi-job parallel analysis is supported, and the LLM layer is loosely coupled to support both Vertex AI and local LLM backends.

Target users are the developer and close colleagues. Large-scale deployment is not planned.

## 2. Functional Specification

### Commands / API Surface

`data-agent` launches the GUI (no CLI subcommands).

**Case Management:**
- Create, list, open, close, delete, export
- Add/remove data from the case management screen
- Each case manages an independent DuckDB/SQLite file
- DB access layer instantiated per case with API exposure
- "Open" starts the instance, "Close" destroys it (behavior during background job execution requires further design)

**Analysis Operations:**
- Natural language instructions in chat → LLM generates and executes SQL
- `/sql` command switches to direct SQL mode
- Sliding window analysis
- Container-based Python execution (for analyses beyond SQL)

**Display:**
- Table view
- Graph display (bar, line, pie)
- Markdown report generation
- Log window (always-visible system log panel for operation transparency)

### Input / Output

**Data Input:**
- JSON, JSONL, CSV, TSV, SQLite
- Add/remove via case management screen

**Analysis Output:**
- Table view (in-GUI)
- Graph display (referencing jviz: bar, line, pie charts)
- Markdown reports

**Report Destinations:**
- Save as case data
- File export
- Clipboard export

**LLM Communication:**
- Vertex AI: Gemini API (streaming)
- Local LLM: OpenAI-compatible `/v1/chat/completions` (streaming)

### Configuration

Centralized via `config.toml`.

**Settings:**
- Vertex AI (project ID, region, model name)
- Local LLM (endpoint URL, model name, API key)
- Container runtime (Podman/Docker selection, image name)
- Analysis settings (token budget, context window size)

**Storage location:** `~/Library/Application Support/data-agent/` (macOS)

### External Dependencies

| Dependency | Type | Required |
|------------|------|----------|
| Vertex AI (Gemini) | Cloud API | No (one of two LLM options) |
| Local LLM (OpenAI-compatible API) | Local service | No (one of two LLM options) |
| DuckDB | Embedded DB | Yes |
| Podman / Docker | Container runtime | No (for Python analysis execution) |

## 3. Design Decisions

**Language & Framework:**
- Go + Wails v2 + React — inheriting shell-agent's technology stack. SwiftUI launcher is unnecessary (unused in shell-agent as well).

**Codebase:**
- Completely new design. No code reuse from shell-agent; starting from detailed architecture review.

**Relationship to Existing Tools:**
- `shell-agent` (util-series): Technology stack reference (Go + Wails v2 + React). Lesson learned from complexity caused by combining general chat with analysis.
- `data-analyzer` (util-series): Sliding window analysis methodology reference.
- `jviz` (util-series): Table/graph visualization reference.

**LLM Interface:**
- Dual support for Vertex AI and local LLM. Loosely coupled design for easy backend switching.
- Token management and context management incorporated into design from the initial phase.

**Streaming Response:**
- LLM responses displayed via streaming to eliminate the feeling of UI freezing during processing.

**Log Window:**
- Implemented as an always-visible UI panel, not just log file storage. Ensures transparency of system operation status.

**Out of Scope:**
- General-purpose chat functionality (natural language processing required for data analysis is in scope)

## 4. Development Plan

### Phase 1: Core — Architecture & Analysis Foundation

- Architecture design (overall design of DB layer, LLM interface, job management)
- LLM interface foundation (token management, context management, streaming response, Vertex AI/local LLM dual support)
- Case management (create/list/open/close/delete)
- Data ingestion (JSON/JSONL/CSV/TSV/SQLite → per-case DuckDB)
- DB access layer instantiation and API exposure
- Basic chat UI + natural language → SQL generation/execution
- `/sql` direct SQL mode
- Table display
- config.toml configuration foundation
- Basic tests

**Independently reviewable**

### Phase 2: Features — Analysis Capabilities

- Container execution (Python analysis code execution via Podman/Docker)
- Markdown report generation/output (case save/file/clipboard)
- Sliding window analysis
- Multi-job/background analysis instantiation
- Log window

**Independently reviewable (per-feature review possible)**

### Phase 3: Release — Visualization, Documentation & Release

- Graph display (bar, line, pie)
- README.md / README.ja.md
- CHANGELOG.md
- AGENTS.md
- Release build and distribution

## 5. Required API Scopes / Permissions

- **Vertex AI:** ADC (Application Default Credentials), `roles/aiplatform.user` role
- **Local LLM:** OpenAI-compatible API with API key support (managed in config.toml)
- **Podman/Docker:** Local execution only, no special permissions required

## 6. Series Placement

Series: **util-series**
Reason: Same category as shell-agent, data-analyzer, and jviz — data processing and analysis tools. Placement in util-series is well-established.

## 7. External Platform Constraints

| Constraint | Details |
|------------|---------|
| DuckDB + Wails | `no_duckdb_arrow` build tag required (proven in shell-agent) |
| Vertex AI | API rate limits (429 errors on high-volume sequential requests). Addressed in token management design |
| Podman/Docker on macOS | Known Unix socket issues. However, data-agent only submits code and retrieves results, so impact is negligible |

---

## Discussion Log

1. **Tool naming:** `data-agent` — an agent tool specialized for data analysis
2. **Lesson from shell-agent:** Combining general chat with data analysis led to complexity. data-agent clarifies purpose and functionality by specializing in data analysis
3. **Per-case DB isolation:** Initially planned central DB with case-based segmentation, but changed to isolating the DB itself per case with instantiated DB access layer exposed via API. Naturally avoids concurrent access issues (DuckDB single-writer constraint)
4. **DB instance lifecycle:** "Open" starts, "Close" destroys. However, instances must persist during background job execution — this behavior requires further design
5. **LLM token/context management:** Since LLM interaction is a core feature, incorporated into design from Phase 1. Reflects lessons from token budget underestimation in data-analyzer and shell-agent
6. **Container execution phasing:** Higher priority than graph display (directly enables data analysis). Placed in Phase 2; graphs moved to Phase 3
7. **Log window:** Not just log storage but an always-visible UI panel. Eliminates freeze perception and ensures system operation transparency
8. **Podman macOS socket issue:** Was problematic for 1Password SSH agent forwarding in shell-agent/cclaude, but data-agent only submits code to containers and retrieves results — no impact
9. **Streaming response:** Stream LLM responses to eliminate UI stall perception during processing
10. **Series placement:** util-series (shell-agent already placed there; consistent with related tool group)
