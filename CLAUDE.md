# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make build              # Build bin/goagent-gw and bin/goagent-cli
make test               # Run unit tests (mock only, no API key needed)
make test-integration   # Run all tests including LLM integration (needs .env with LLM_API_KEY)
make run-gw             # Build and start the gateway server
make clean              # Remove build artifacts
```

All `go` commands require `GOTOOLCHAIN=local` prefix (enforced in Makefile). To run a single test:

```bash
GOTOOLCHAIN=local go test ./internal/llm/ -run TestChatCompletion -v
```

Integration tests are prefixed with `TestI` — the `make test` target excludes them via `-run 'Test[^I]'`.

## Architecture

GoAgent is a multi-user WebSocket gateway that routes JSON-RPC 2.0 requests to tools and LLM-powered agents. Two binaries: `goagent-gw` (server) and `goagent-cli` (client).

```mermaid
flowchart TD
    CLI[goagent-cli] -->|WebSocket JSON-RPC 2.0| GW[goagent-gw]
    GW --> AUTH[Auth — token → userID]
    GW --> SESS[Session — user-scoped]
    GW --> DISPATCH[Server Dispatcher]

    DISPATCH --> TOOLS[Tool Registry]
    DISPATCH --> AGENT[Agent Manager]
    DISPATCH --> SKILLS[Skill Loader]

    TOOLS --> WS[web_search]
    TOOLS --> OH[ohlcv]
    TOOLS --> EX[exec]

    AGENT --> LLM[LLM Client]
    AGENT --> HIST[Chat History JSONL]
    AGENT --> MEM[Memory Provider]
    MEM --> KW[Keyword TF-IDF — zero cost]
    MEM --> VEC[Vector Embeddings — opt-in]

    DISPATCH --> PERSIST[Persistence]
    PERSIST --> GIT[Git Auto-Commit]
```

**Agent chat loop** (`server.go:dispatch`): Assembles system prompt from USER.md + SOUL.md + AGENT.md intent + skills + memories, sends to an OpenAI-compatible LLM, executes tool calls via the tool registry (max 10 rounds), and streams responses. See `MEMORY.md` for the memory/context engineering design.

**Key packages:**
- `internal/server` — Central dispatcher; routes all JSON-RPC methods. This is the main integration point.
- `internal/auth` — Multi-user auth via `users.json` token→userID mapping. Falls back to `"default"` user when no auth configured.
- `internal/llm` — OpenAI-compatible HTTP client with streaming support. Works with DeepSeek, Ollama, OpenAI, Groq, etc.
- `internal/tools` — Tool registry mapping tool names to handler functions + LLM schemas. Tools: `web_search`, `ohlcv`, `exec`.
- `internal/agent` — Loads USER.md (global), SOUL.md (per-agent), AGENT.md intent, and manages JSONL conversation history.
- `internal/memory` — Plug-and-play memory via `Provider` interface. Default: zero-cost TF-IDF keyword scorer. Opt-in: vector embeddings when `EMBED_*` env vars are set.
- `internal/skills` — Parses OpenClaw-compatible `SKILL.md` files (YAML frontmatter + markdown). Skills inject instructions into agent prompts. Auto-checks binary/env requirements.
- `internal/persistence` — Atomic file writes via temp+rename, plus debounced git auto-commit (2s delay).
- `internal/session` — Wraps WebSocket connections with user identity and JSONL message logging.

## Multi-User Data Layout

```
~/.goagent/
├── users.json                              ← token → userID mapping
├── skills/                                 ← global, shared by all users
│   └── {skillname}/SKILL.md
├── home/
│   └── {userID}/                           ← user's home directory
│       ├── USER.md                         ← preferences, style — shared by ALL user's agents
│       ├── sessions/
│       │   └── sess-{timestamp}.jsonl
│       └── agents/
│           └── {agentName}/
│               ├── SOUL.md                 ← agent personality, tone
│               ├── {AGENTNAME}.md          ← intent + status
│               └── memory/
│                   ├── chat.jsonl          ← conversation history
│                   └── memories.jsonl      ← long-term memory
```

All state is plain-text (JSON, JSONL, Markdown) designed to be git-tracked. When no `users.json` exists, all connections map to user `"default"`.

## System Prompt Assembly Order

```mermaid
flowchart LR
    A[USER.md] --> F[System Prompt]
    B[SOUL.md] --> F
    C[AGENT.md Intent] --> F
    D[Skills] --> F
    E[Memories] --> F
    F --> G[+ Chat History + User Message → LLM]
```

1. **USER.md** — who the user is (global preferences, OS, communication style)
2. **SOUL.md** — who the agent is (personality, tone, behavioral guidelines)
3. **AGENT.md # Intent** — what the agent does (task/domain focus)
4. **Skills** — what tools it has
5. **Memories** — what it remembers (keyword or vector retrieval)

## Environment Variables

Configured via `.env` file in project root (see `.env.example`). Existing env vars take precedence.

- `LLM_BASE_URL` — Full API prefix for the LLM provider (default: `https://api.deepseek.com/v1`)
- `LLM_API_KEY` — Required for cloud LLM providers
- `LLM_MODEL` — Model name (default: `deepseek-chat`)
- `EMBED_BASE_URL` / `EMBED_API_KEY` / `EMBED_MODEL` — Embedding config (falls back to LLM_* values). When unset, memory uses zero-cost keyword scoring instead.
- `GOAGENT_TOKEN` — WebSocket auth token (auth disabled if unset)
- `BRAVE_SEARCH_API_KEY` / `SEARCH_BACKEND` — Web search configuration

## Conventions

- Single external dependency: `nhooyr.io/websocket`. No frameworks.
- Standard library `testing` package only — no test frameworks.
- All server methods are dispatched through a single `switch` in `internal/server/server.go`.
- Tool handlers accept `json.RawMessage` params and return `(any, error)`.
- The `exec` tool has a safety blocklist for dangerous shell patterns (rm -rf /, mkfs, etc.).
- All data paths flow through user home directories: `home/{userID}/agents/{name}/...`
- Memory backends implement the `Provider` interface (`Load`, `Search`, `Save`, `Count`).
