package tools

import (
	"encoding/json"
	"fmt"

	"goagent/internal/llm"
	"goagent/internal/memory"
)

// MemoryToolDefs returns the LLM-facing tool definitions for memory.save and memory.recall.
// These are registered separately from builtins because they need a per-request Provider.
var MemoryToolDefs = []llm.ToolDef{
	{
		Type: "function",
		Function: llm.ToolSchema{
			Name:        "memory_save",
			Description: "Save an important fact, user preference, or insight for future recall. Use this when the user shares something worth remembering across conversations.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text": map[string]any{
						"type":        "string",
						"description": "The fact or preference to remember",
					},
				},
				"required": []string{"text"},
			},
		},
	},
	{
		Type: "function",
		Function: llm.ToolSchema{
			Name:        "memory_recall",
			Description: "Search stored memories for facts relevant to a topic. Use this to check what you already know about the user or a subject.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query to find relevant memories",
					},
					"top_k": map[string]any{
						"type":        "integer",
						"description": "Number of results to return (default: 5)",
					},
				},
				"required": []string{"query"},
			},
		},
	},
}

// MemorySave handles the memory_save tool call.
func MemorySave(provider memory.Provider, params json.RawMessage) (any, error) {
	var req struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}
	if req.Text == "" {
		return nil, fmt.Errorf("text is required")
	}

	if err := provider.Save(req.Text); err != nil {
		return nil, err
	}

	return map[string]any{
		"status": "saved",
		"count":  provider.Count(),
	}, nil
}

// MemoryRecall handles the memory_recall tool call.
func MemoryRecall(provider memory.Provider, params json.RawMessage) (any, error) {
	var req struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if req.TopK == 0 {
		req.TopK = 5
	}

	results, err := provider.Search(req.Query, req.TopK)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"results": results,
		"count":   len(results),
	}, nil
}
