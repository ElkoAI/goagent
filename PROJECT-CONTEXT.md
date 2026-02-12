# PROJECT-CONTEXT.md

## Project Name
GoAgent

## One-line description
A clean, high-performance, git-friendly agent gateway written in Go—inspired by OpenClaw.

## Core Philosophy
- **Git-Native State**: All important state (memory, sessions, nodes) is plain text (JSON/Markdown/JSONL) and git-trackable.
- **Protocol**: Uses a simple WebSocket JSON-RPC 2.0 API.
- **Environment**: Linux-first. Leverages standard Unix tools (`rg`, `jq`, `grep`) for debugging.
- **Resource Usage**: Local-first, low footprint, suitable for always-on background processes.

## Technology Stack
- **Language**: Go 1.26+
- **Communication**: `nhooyr.io/websocket`
- **AI Brain**: DeepSeek-R1 / DeepSeek-V3
- **Persistence**: File-based (JSON, JSONL, Markdown)

## Linux Environment Defaults
- **Data Directory**: `~/.goagent/`
- **Persistence Pattern**: Atomic writes via `os.Rename` to ensure file integrity.
- **Observability**: Designed to be piped into standard Linux utilities.

## Configuration
- **`.env` support**: Place a `.env` file in the project root with `KEY=VALUE` pairs. Existing environment variables take precedence. See `.env.example` for available keys.
- **Env vars**: `GOAGENT_TOKEN`, `BRAVE_SEARCH_API_KEY`, `SEARCH_BACKEND`

## Key Directories
- `cmd/gateway/`: The central WebSocket server.
- `cmd/cli/`: Client-side interaction tool (REPL + one-shot mode).
- `internal/config/`: `.env` file loader.
- `internal/logging/`: JSONL session logger.
- `internal/persistence/`: Handles git-friendly file I/O and auto-commit.
- `internal/session/`: WebSocket session management with message logging.
- `internal/tools/`: Extension point for agent skills (web search: Brave + DuckDuckGo).

## Session Logging
All WebSocket messages are persisted as JSONL files in `~/.goagent/sessions/<session-id>.jsonl`. Each line is `{"ts":"...","dir":"in|out","data":{...}}`. Files are auto-committed by the persistence layer.

## Web Search
Configurable via `SEARCH_BACKEND` env var:
- `brave` — requires `BRAVE_SEARCH_API_KEY`
- `duckduckgo` — no API key required (default fallback)
- Auto-detection: uses Brave if API key is set, otherwise DuckDuckGo.

Last updated: February 2026.
