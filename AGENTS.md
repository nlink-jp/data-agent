# AGENTS.md — data-agent

## Project Summary

Data analysis desktop GUI tool with interactive chat interface. Specializes in data exploration and analysis — not a general-purpose chat tool. Built with Go + Wails v2 + React.

## Build & Test

```sh
make build    # wails build → dist/data-agent.app
make dev      # wails dev with hot reload
make test     # go test -tags no_duckdb_arrow ./...
```

**Required build tag:** `-tags no_duckdb_arrow` (DuckDB + Wails compatibility)

## Key Directory Structure

```
data-agent/
├── main.go                  # Entry point
├── app.go                   # Wails app struct and bindings
├── internal/
│   ├── casemgr/             # Case management, DB lifecycle
│   ├── dbengine/            # DuckDB operations, SQL execution
│   ├── llm/                 # LLM client interface (Vertex AI + local)
│   ├── session/             # Analysis session, Planning→Execution→Review
│   ├── analysis/            # SQL generation, sliding window
│   ├── job/                 # Job management, background execution
│   ├── report/              # Report generation (plan + exec log)
│   ├── config/              # config.toml management
│   ├── container/           # Podman/Docker execution (Phase 2)
│   └── logger/              # Structured logging + log window
├── frontend/
│   └── src/                 # React frontend
├── docs/
│   ├── en/                  # English docs
│   └── ja/                  # Japanese docs (*.ja.md)
├── Makefile
└── wails.json
```

## Module Path

`github.com/nlink-jp/data-agent`

## Gotchas

- DuckDB + Wails requires `no_duckdb_arrow` build tag — without it, build fails
- Each case uses an independent DuckDB file — no shared central database
- Analysis follows Planning→Execution→Review loop with structured investigation plans
- LLM generates plans, code executes them — not LLM tool calling
- 3-tier error handling: Minor (SQL retry), Moderate (step modify/skip), Critical (replan)
- LLM interface is backend-agnostic: Vertex AI (ADC auth) or local LLM (OpenAI-compatible API with optional API key)
- Config lives at `~/Library/Application Support/data-agent/config.toml`

## Series

util-series (umbrella: nlink-jp/util-series)
