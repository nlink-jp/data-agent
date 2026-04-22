# Changelog

All notable changes to this project will be documented in this file.

## [0.1.0] - 2026-04-22

### Added

- Case-based data management with per-case DuckDB isolation
- Natural language analysis — LLM generates structured investigation plans and SQL
- Planning → Execution → Review loop with 3-tier error handling
- Sliding window analysis for large datasets
- Dual LLM backend: Vertex AI (Gemini) and local LLM (OpenAI-compatible API)
- Executive summary in reports from LLM review
- Color theme support (dark, light, warm, midnight)
- Window state persistence and settings UI
- Container execution support (Podman/Docker) for Python analysis
- Markdown report generation with plan, execution log, and findings
- Integration with nlk (guard, jsonfix, backoff, strip)

### Fixed

- Vertex AI role error on session reopen (system role in contents)
- Sliding window output language follows perspective language
- Session reopen clears plan instead of resetting step statuses
