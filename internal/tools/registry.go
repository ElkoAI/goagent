package tools

import (
	"encoding/json"
	"fmt"

	"goagent/internal/llm"
)

// ToolEntry pairs a tool's LLM-facing schema with its server-side handler.
type ToolEntry struct {
	Def     llm.ToolDef
	Handler func(params json.RawMessage) (any, error)
}

// Registry holds all registered tools.
type Registry struct {
	entries map[string]ToolEntry
}

// NewRegistry creates a registry pre-loaded with all built-in tools.
func NewRegistry() *Registry {
	r := &Registry{entries: make(map[string]ToolEntry)}
	r.registerBuiltins()
	return r
}

// ToolDefs returns the OpenAI-compatible tool definitions for the LLM.
func (r *Registry) ToolDefs() []llm.ToolDef {
	defs := make([]llm.ToolDef, 0, len(r.entries))
	for _, e := range r.entries {
		defs = append(defs, e.Def)
	}
	return defs
}

// Call dispatches a tool call by name and returns the result as a JSON string.
func (r *Registry) Call(name string, argsJSON string) (string, error) {
	entry, ok := r.entries[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}

	result, err := entry.Handler(json.RawMessage(argsJSON))
	if err != nil {
		// Return the error as a JSON string so the LLM can see what went wrong.
		errResult, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errResult), nil
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal tool result: %w", err)
	}
	return string(out), nil
}

// Register adds a tool to the registry.
func (r *Registry) Register(name string, entry ToolEntry) {
	r.entries[name] = entry
}

// Get returns a tool entry by name.
func (r *Registry) Get(name string) (ToolEntry, bool) {
	e, ok := r.entries[name]
	return e, ok
}

func (r *Registry) registerBuiltins() {
	r.Register("web_search", ToolEntry{
		Def: llm.ToolDef{
			Type: "function",
			Function: llm.ToolSchema{
				Name:        "web_search",
				Description: "Search the web for information. Returns titles, URLs, and snippets.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "The search query",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		Handler: WebSearch,
	})

	r.Register("ohlcv", ToolEntry{
		Def: llm.ToolDef{
			Type: "function",
			Function: llm.ToolSchema{
				Name:        "ohlcv",
				Description: "Fetch OHLCV (Open, High, Low, Close, Volume) candlestick data from Yahoo Finance. Works for stocks (AAPL), ETFs (SPY), and crypto (BTC-USD).",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"symbol": map[string]any{
							"type":        "string",
							"description": "Ticker symbol (e.g. AAPL, BTC-USD, SPY)",
						},
						"range": map[string]any{
							"type":        "string",
							"description": "Time range: 1d, 5d, 1mo, 3mo, 6mo, 1y, 2y, 5y, 10y, ytd, max",
							"default":     "1mo",
						},
						"interval": map[string]any{
							"type":        "string",
							"description": "Bar interval: 1d, 1wk, 1mo",
							"default":     "1d",
						},
					},
					"required": []string{"symbol"},
				},
			},
		},
		Handler: OHLCV,
	})

	r.Register("exec", ToolEntry{
		Def: llm.ToolDef{
			Type: "function",
			Function: llm.ToolSchema{
				Name:        "exec",
				Description: "Execute a shell command and return its output. Use for running CLI tools like gh, curl, jq, etc.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "The shell command to run",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		Handler: Exec,
	})
}
