# GoAgent

A clean, high-performance, git-friendly agent gateway written in Go — inspired by OpenClaw.

## Quick Start

```bash
# 1. Copy and edit the environment file
cp .env.example .env
# Edit .env with your token and API keys

# 2. Build
make build

# 3. Run the gateway
./bin/goagent-gw

# 4. In another terminal, use the CLI
./bin/goagent-cli --token=your-token ping
./bin/goagent-cli --token=your-token search "golang concurrency"
```

## Configuration

GoAgent loads configuration from a `.env` file in the project root. Existing environment variables always take precedence.

| Variable | Description | Required |
|----------|-------------|----------|
| `GOAGENT_TOKEN` | Authentication token for WebSocket connections | No (auth disabled if unset) |
| `BRAVE_SEARCH_API_KEY` | Brave Search API key | No |
| `SEARCH_BACKEND` | `brave` or `duckduckgo` (auto-detects if unset) | No |
| `LLM_BASE_URL` | OpenAI-compatible API base URL (default: `https://api.deepseek.com`) | No |
| `LLM_API_KEY` | API key for the LLM provider | Yes (for cloud providers) |
| `LLM_MODEL` | Model name to use (default: `deepseek-chat`) | No |

## Agent Chat

Create an agent and chat with it via any OpenAI-compatible LLM (DeepSeek, Ollama, OpenAI, etc.):

```bash
# Create an agent
./bin/goagent-cli --token=your-token create advisor "You are a financial advisor who gives clear, practical advice."

# Chat with it (tokens stream to your terminal in real time)
./bin/goagent-cli --token=your-token chat advisor "What are good index funds for beginners?"

# Conversation history is maintained automatically
./bin/goagent-cli --token=your-token chat advisor "How much should I invest monthly?"
```

The `chat` command streams tokens in real time over WebSocket — you see each word as it arrives from the LLM instead of waiting for the full response.

For local models via Ollama, set `LLM_BASE_URL=http://localhost:11434/v1` and `LLM_MODEL=deepseek-r1:7b` in your `.env` file.

## Web Search

Two backends are supported:

- **Brave Search** — requires `BRAVE_SEARCH_API_KEY`
- **DuckDuckGo** — free, no API key needed (default fallback)

Set `SEARCH_BACKEND` to force a specific backend, or leave it unset for auto-detection (Brave if key exists, otherwise DuckDuckGo).

## OHLCV Market Data

Fetch OHLCV (Open, High, Low, Close, Volume) candlestick data from Yahoo Finance. No API key required — works for stocks, ETFs, and crypto.

```bash
# Default: 1 month of daily bars (JSON)
./bin/goagent-cli --token=your-token ohlcv AAPL

# 1 year of daily bars
./bin/goagent-cli --token=your-token ohlcv AAPL 1y

# 3 months of daily bars for Bitcoin
./bin/goagent-cli --token=your-token ohlcv BTC-USD 3mo 1d

# CSV output (pipe-friendly)
./bin/goagent-cli --token=your-token ohlcv SPY 5d --csv
```

| Param | Default | Values |
|-------|---------|--------|
| `symbol` | *(required)* | Ticker: `AAPL`, `BTC-USD`, `SPY` |
| `range` | `1mo` | `1d`, `5d`, `1mo`, `3mo`, `6mo`, `1y`, `2y`, `5y`, `10y`, `ytd`, `max` |
| `interval` | `1d` | `1d`, `1wk`, `1mo` |

The `--csv` flag outputs plain CSV instead of JSON — useful for piping to other tools or spreadsheets.

## Skills (OpenClaw-Compatible)

GoAgent can load skills from `~/.goagent/skills/` — including skills from OpenClaw's ClawHub marketplace. A skill is a directory with a `SKILL.md` file containing YAML frontmatter and markdown instructions that get injected into agent system prompts.

```bash
# List installed skills and their status
./bin/goagent-cli --token=your-token skills

# Reload skills after adding new ones (no restart needed)
./bin/goagent-cli --token=your-token skill-refresh

# Run a command via the generic exec tool (used by skills)
./bin/goagent-cli --token=your-token exec "gh pr list"
```

### Creating a Skill

```bash
mkdir -p ~/.goagent/skills/my-skill
cat > ~/.goagent/skills/my-skill/SKILL.md << 'EOF'
---
name: my-skill
description: Does something useful
metadata: {"openclaw":{"requires":{"bins":["curl"],"env":["MY_API_KEY"]}}}
---

# My Skill

When the user asks to do the thing, run:
`exec curl -s https://api.example.com/data`
EOF
```

Skills are automatically checked for requirements (binaries and env vars) and only activated when all dependencies are satisfied.

### OpenClaw Compatibility

Most ClawHub skills work unmodified — GoAgent parses the same `SKILL.md` frontmatter format and injects skill instructions into agent prompts. Skills that shell out to CLI tools (`gh`, `curl`, `ffmpeg`, etc.) via `exec` work identically.

## Session Logging

All WebSocket messages are automatically logged as JSONL files in `~/.goagent/sessions/`. Each line contains a timestamp, direction (`in`/`out`), and the raw message data:

```json
{"ts":"2026-02-12T10:30:00.000Z","dir":"in","data":{"jsonrpc":"2.0","method":"ping","id":1}}
{"ts":"2026-02-12T10:30:00.001Z","dir":"out","data":{"jsonrpc":"2.0","id":1,"result":"pong"}}
```

Session logs are auto-committed by the git-native persistence layer.

## Project Structure

```
cmd/gateway/       WebSocket server entry point
cmd/cli/           CLI client (REPL + one-shot)
internal/agent/    Agent prompt loading and history management
internal/auth/     Token-based authentication
internal/config/   .env file loader
internal/llm/      OpenAI-compatible LLM client
internal/logging/  JSONL session logger
internal/persistence/  File I/O and git auto-commit
internal/protocol/ JSON-RPC 2.0 types
internal/server/   HTTP/WebSocket server and request dispatch
internal/session/  WebSocket session management
internal/skills/   Skill loader (OpenClaw-compatible SKILL.md parser)
internal/tools/    Agent tools (web search, OHLCV, exec)
```

## Building

```bash
make build    # Compiles bin/goagent-gw and bin/goagent-cli
make clean    # Removes build artifacts
```
