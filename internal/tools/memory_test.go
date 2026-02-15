package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"goagent/internal/memory"
)

// mockProvider is a minimal in-memory Provider for testing.
type mockProvider struct {
	entries []memory.SearchResult
}

func (m *mockProvider) Load() error { return nil }

func (m *mockProvider) Save(text string) error {
	m.entries = append(m.entries, memory.SearchResult{
		Text:      text,
		Score:     1.0,
		Timestamp: "2026-02-15T00:00:00Z",
	})
	return nil
}

func (m *mockProvider) Search(query string, topK int) ([]memory.SearchResult, error) {
	var results []memory.SearchResult
	for _, e := range m.entries {
		if strings.Contains(strings.ToLower(e.Text), strings.ToLower(query)) {
			results = append(results, e)
		}
	}
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func (m *mockProvider) Count() int { return len(m.entries) }

func TestMemorySave(t *testing.T) {
	provider := &mockProvider{}

	params := json.RawMessage(`{"text":"User prefers Go"}`)
	result, err := MemorySave(provider, params)
	if err != nil {
		t.Fatalf("MemorySave() error: %v", err)
	}

	m := result.(map[string]any)
	if m["status"] != "saved" {
		t.Errorf("status = %v, want 'saved'", m["status"])
	}
	if m["count"] != 1 {
		t.Errorf("count = %v, want 1", m["count"])
	}

	if provider.Count() != 1 {
		t.Fatalf("provider count = %d, want 1", provider.Count())
	}
}

func TestMemorySave_EmptyText(t *testing.T) {
	provider := &mockProvider{}

	params := json.RawMessage(`{"text":""}`)
	_, err := MemorySave(provider, params)
	if err == nil {
		t.Fatal("MemorySave() should error on empty text")
	}
}

func TestMemoryRecall(t *testing.T) {
	provider := &mockProvider{}
	provider.Save("User prefers Go for backend")
	provider.Save("User lives in Berlin")

	params := json.RawMessage(`{"query":"Go","top_k":5}`)
	result, err := MemoryRecall(provider, params)
	if err != nil {
		t.Fatalf("MemoryRecall() error: %v", err)
	}

	m := result.(map[string]any)
	results := m["results"].([]memory.SearchResult)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'Go', got %d", len(results))
	}
	if !strings.Contains(results[0].Text, "Go") {
		t.Errorf("result should contain 'Go', got: %q", results[0].Text)
	}
}

func TestMemoryRecall_EmptyQuery(t *testing.T) {
	provider := &mockProvider{}

	params := json.RawMessage(`{"query":""}`)
	_, err := MemoryRecall(provider, params)
	if err == nil {
		t.Fatal("MemoryRecall() should error on empty query")
	}
}

func TestMemoryRecall_DefaultTopK(t *testing.T) {
	provider := &mockProvider{}
	for i := 0; i < 10; i++ {
		provider.Save("fact about Go")
	}

	// No top_k specified — should default to 5.
	params := json.RawMessage(`{"query":"Go"}`)
	result, err := MemoryRecall(provider, params)
	if err != nil {
		t.Fatalf("MemoryRecall() error: %v", err)
	}

	m := result.(map[string]any)
	results := m["results"].([]memory.SearchResult)
	if len(results) > 5 {
		t.Errorf("default top_k should cap at 5, got %d", len(results))
	}
}

func TestMemoryToolDefs(t *testing.T) {
	if len(MemoryToolDefs) != 2 {
		t.Fatalf("expected 2 memory tool defs, got %d", len(MemoryToolDefs))
	}

	names := map[string]bool{}
	for _, def := range MemoryToolDefs {
		if def.Type != "function" {
			t.Errorf("Type = %q, want 'function'", def.Type)
		}
		names[def.Function.Name] = true
	}

	if !names["memory_save"] {
		t.Error("missing memory_save tool def")
	}
	if !names["memory_recall"] {
		t.Error("missing memory_recall tool def")
	}
}
