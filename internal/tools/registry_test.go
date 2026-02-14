package tools

import (
	"strings"
	"testing"
)

func TestRegistryToolDefs(t *testing.T) {
	r := NewRegistry()
	defs := r.ToolDefs()

	if len(defs) < 3 {
		t.Fatalf("expected at least 3 tool defs (web_search, ohlcv, exec), got %d", len(defs))
	}

	// Check that all defs have required fields.
	for _, d := range defs {
		if d.Type != "function" {
			t.Errorf("ToolDef.Type = %q, want function", d.Type)
		}
		if d.Function.Name == "" {
			t.Error("ToolDef.Function.Name is empty")
		}
		if d.Function.Description == "" {
			t.Errorf("ToolDef %q has empty description", d.Function.Name)
		}
		if d.Function.Parameters == nil {
			t.Errorf("ToolDef %q has nil parameters", d.Function.Name)
		}
	}
}

func TestRegistryCall(t *testing.T) {
	r := NewRegistry()

	// Call web_search with a query — it will fail (no network in test),
	// but the error should be returned as JSON, not as a Go error.
	result, err := r.Call("web_search", `{"query":"test"}`)
	if err != nil {
		t.Fatalf("Call() returned Go error (should wrap in JSON): %v", err)
	}
	// DuckDuckGo might succeed or fail, but we should get valid JSON back.
	if result == "" {
		t.Error("Call() returned empty result")
	}
}

func TestRegistryCall_UnknownTool(t *testing.T) {
	r := NewRegistry()

	_, err := r.Call("nonexistent_tool", `{}`)
	if err == nil {
		t.Fatal("Call() should error on unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("error = %q, want 'unknown tool'", err.Error())
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()

	entry, ok := r.Get("ohlcv")
	if !ok {
		t.Fatal("Get(ohlcv) should return true")
	}
	if entry.Def.Function.Name != "ohlcv" {
		t.Errorf("Name = %q", entry.Def.Function.Name)
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("Get(nonexistent) should return false")
	}
}
