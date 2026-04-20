# CLAUDE.md — data-agent

**Organization rules (mandatory): https://github.com/nlink-jp/.github/blob/main/CONVENTIONS.md**

## Overview

Data analysis desktop GUI tool. Go + Wails v2 + React. Per-case DuckDB with instantiated DB access layer. Dual LLM backend (Vertex AI + local LLM).

## Build

- Always `make build` (outputs to `dist/data-agent.app`)
- Development: `make dev`
- Tests: `make test`
- Build tag: `no_duckdb_arrow` is required for DuckDB + Wails compatibility

## Architecture

- **main.go** — Entry point, Wails app initialization
- **app.go** — App struct, Wails bindings
- **internal/casemgr/** — Case management, DB lifecycle
- **internal/analysis/** — DuckDB engine, SQL generation, sliding window
- **internal/client/** — LLM client (Vertex AI + OpenAI-compatible)
- **internal/config/** — config.toml management
- **internal/container/** — Podman/Docker execution
- **frontend/src/** — React frontend

## Key Design Decisions

- **Per-case DB isolation** — Each case has its own DuckDB file. No shared DB.
- **DB instance lifecycle** — Open on case open, destroy on case close.
- **LLM loose coupling** — Backend-agnostic interface for easy switching.
- **Token/context management** — Built into core design from Phase 1.
- **No general chat** — Data analysis only (language processing for analysis is in scope).

## Series

util-series (umbrella: nlink-jp/util-series)
