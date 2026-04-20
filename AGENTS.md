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
│   ├── analysis/            # DuckDB engine, SQL, sliding window
│   ├── client/              # LLM client (Vertex AI + local)
│   ├── config/              # config.toml management
│   └── container/           # Podman/Docker execution
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
- LLM interface is backend-agnostic: Vertex AI (ADC auth) or local LLM (OpenAI-compatible API with optional API key)
- Config lives at `~/Library/Application Support/data-agent/config.toml`

## Series

util-series (umbrella: nlink-jp/util-series)
