# data-agent

Data analysis desktop GUI tool with interactive chat interface.

Ingest JSON/JSONL/CSV/TSV/SQLite data into per-case DuckDB databases and explore them through natural language queries or direct SQL. Supports table display, graph visualization, Markdown report generation, and container-based Python analysis.

## Features

- **Case-based data management** — isolate datasets per investigation/project
- **Natural language analysis** — describe what you want to find; the LLM generates and runs SQL
- **Direct SQL mode** — switch to raw SQL with `/sql`
- **Dual LLM backend** — Vertex AI (Gemini) and local LLM (OpenAI-compatible API)
- **Container execution** — run Python analysis code in Podman/Docker sandbox
- **Multiple output formats** — table, graph (bar/line/pie), Markdown report

## Install

Download a pre-built binary from the [releases page](https://github.com/nlink-jp/data-agent/releases).

## Build

```sh
make build    # build macOS app → dist/data-agent.app
make dev      # development mode with hot reload
make test     # run tests
```

Requires: Go 1.26+, Node.js, [Wails v2](https://wails.io/)

## Configuration

Settings are stored in `~/Library/Application Support/data-agent/config.toml`.

```toml
[vertex_ai]
project = "your-project-id"
region = "us-central1"
model = "gemini-2.5-flash"

[local_llm]
endpoint = "http://localhost:1234/v1"
model = "google/gemma-4-26b-a4b"
api_key = ""

[container]
runtime = "podman"
```

## License

MIT — see [LICENSE](LICENSE).
