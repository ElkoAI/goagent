# GoAgent Course

A structured tutorial that follows the project's commit history. Each module maps to a real commit so you can check out that point in history and study the code.

---

## Module 1: Project Scaffold + Gateway Basics

**Commit**: `1ff3d05`

**Learning Objectives**:
- Set up a Go project with `cmd/` and `internal/` layout
- Build a WebSocket server with JSON-RPC 2.0 dispatch
- Implement token-based authentication
- Create a CLI client with REPL and one-shot modes
- Add JSONL session logging and git-native persistence

**Key Files**:
- `cmd/gateway/main.go` — server entry point, graceful shutdown
- `cmd/cli/main.go` — CLI client with WebSocket connection
- `internal/server/server.go` — HTTP server, WebSocket handler, request dispatch
- `internal/protocol/protocol.go` — JSON-RPC 2.0 message types
- `internal/session/session.go` — session lifecycle and message logging
- `internal/auth/auth.go` — token validation
- `internal/persistence/persistence.go` — atomic file writes
- `internal/persistence/git.go` — auto-commit with debounce
- `internal/logging/jsonl.go` — append-only session logs

---

## Module 2: Environment Config, JSONL Logging, Multi-Backend Search

**Commit**: `1ff3d05` (landed together with Module 1)

**Learning Objectives**:
- Load `.env` files without external dependencies
- Implement a web search tool dispatcher with pluggable backends
- Integrate Brave Search and DuckDuckGo APIs
- Create an agent filesystem layout (`agent.create` RPC)

**Key Files**:
- `internal/config/dotenv.go` — `.env` parser with quote stripping
- `internal/tools/websearch.go` — search dispatcher and Brave backend
- `internal/tools/duckduckgo.go` — DuckDuckGo instant answer API
- `.env.example` — configuration template

---

## Module 3: LLM Integration + Agent Chat

**Commit**: *(this commit)*

**Learning Objectives**:
- Build an OpenAI-compatible LLM client that works with any provider (DeepSeek, Ollama, etc.)
- Implement conversation history as append-only JSONL
- Wire an `agent.chat` RPC that loads system prompts, maintains context, and calls the LLM
- Add a CLI `chat` command for interactive agent conversations

**Key Files**:
- `internal/llm/client.go` — HTTP client for `/v1/chat/completions`
- `internal/agent/agent.go` — system prompt loading, JSONL history management
- `internal/server/server.go` — `agent.chat` dispatch case
- `cmd/cli/main.go` — `chat` command routing

---

## Module 4: Streaming Responses

**Commit**: *(this commit)*

**Learning Objectives**:
- Add SSE-based streaming to the OpenAI-compatible LLM client (`ChatStream`)
- Send incremental token deltas over WebSocket using JSON-RPC with `delta`/`done` fields
- Implement a streaming read loop in the CLI that prints tokens as they arrive
- Keep the non-streaming `Chat()` path for backwards compatibility

**Key Files**:
- `internal/llm/client.go` — `ChatStream` method: SSE line parsing, `onChunk` callback
- `internal/server/server.go` — streaming branch in `agent.chat` dispatch
- `cmd/cli/main.go` — `readStream()` loop, `stream: true` param for chat

---

## Module 5: Tool Use — OHLCV Market Data

**Commit**: *(this commit)*

**Learning Objectives**:
- Build a stateless tool that fetches real-world financial data (Yahoo Finance OHLCV)
- Handle session-based API auth with crumb+cookie (cached via `sync.Once`)
- Parse deeply nested JSON responses into clean domain structs
- Wire a new `tool.ohlcv` JSON-RPC method through the server dispatch
- Add a CLI command with positional args and a `--csv` output flag
- Write mock tests using `httptest` to avoid network dependencies

**Key Files**:
- `internal/tools/ohlcv.go` — Yahoo Finance client: crumb auth, chart API parsing, CSV output
- `internal/tools/ohlcv_test.go` — mock test with canned JSON, validation tests, integration test
- `internal/server/server.go` — `tool.ohlcv` dispatch case
- `cmd/cli/main.go` — `ohlcv` command with `--csv` flag

---

## Module 6: Skills — OpenClaw-Compatible Skill Loader

**Commit**: *(this commit)*

**Learning Objectives**:
- Build a skill discovery system that scans directories for `SKILL.md` files
- Parse YAML frontmatter with inline JSON metadata (matching OpenClaw's format)
- Validate skill requirements (binaries via `exec.LookPath`, env vars via `os.Getenv`)
- Inject skill instructions into agent system prompts at runtime
- Implement a generic `tool.exec` for skill-backed command execution
- Understand how OpenClaw ClawHub skills work and why ~80% are compatible without modification

**Key Files**:
- `internal/skills/loader.go` — skill discovery, frontmatter parser, requirement checker, prompt builder
- `internal/skills/loader_test.go` — 12 tests covering parsing, loading, requirements, prompt building
- `internal/tools/exec.go` — generic command execution tool with safety validation
- `internal/server/server.go` — `skill.list`, `skill.refresh`, `tool.exec` dispatch + prompt injection in `agent.chat`
- `cmd/cli/main.go` — `skills`, `skill-refresh`, `exec` CLI commands

---

## Module 7: Agent Tool-Calling Loop

**Commit**: *(this commit)*

**Learning Objectives**:
- Extend the LLM client with OpenAI-compatible tool-call types (`ToolCall`, `ToolDef`, `ChatWithTools`)
- Build a tool registry that maps tool names to JSON Schemas and handler functions
- Implement the tool-calling loop: LLM → tool call → execute → feed result → repeat
- Handle multi-round tool calls with a max iteration safety limit
- Notify the client of in-progress tool calls via WebSocket messages
- Understand the OpenAI tool-calling wire format (`tool_calls`, `finish_reason`, role `"tool"`)

**Key Files**:
- `internal/llm/client.go` — `ToolCall`, `ToolDef`, `ChatResult` types, `ChatWithTools` method
- `internal/llm/client_test.go` — mock tool-call server (returns tool call → text), `TestChatWithTools_*`
- `internal/tools/registry.go` — tool registry: schema definitions, `Call()` dispatch, `ToolDefs()` for LLM
- `internal/tools/registry_test.go` — registry tests
- `internal/server/server.go` — tool-calling loop in `agent.chat`, `maxToolRounds` limit

---

## Module 8: Memory + Retrieval *(planned)*

**Learning Objectives**:
- Build long-term memory beyond chat history
- Implement embedding-based retrieval for relevant context
- Design a memory compaction strategy for large conversation histories
