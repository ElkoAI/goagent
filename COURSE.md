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

## Module 5: Tool Use *(planned)*

**Learning Objectives**:
- Define a tool schema that agents can invoke
- Implement tool-call parsing from LLM responses
- Execute tools server-side and feed results back into the conversation

---

## Module 6: Memory + Retrieval *(planned)*

**Learning Objectives**:
- Build long-term memory beyond chat history
- Implement embedding-based retrieval for relevant context
- Design a memory compaction strategy for large conversation histories
