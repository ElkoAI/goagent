package memory

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"goagent/internal/llm"
)

// mockExtractionServer returns an LLM server that responds with a JSON array of facts
// encoded as a string in the content field (matching real LLM behavior).
func mockExtractionServer(facts []string) *httptest.Server {
	factsJSON, _ := json.Marshal(facts)
	return mockExtractionServerRaw(string(factsJSON))
}

// mockExtractionServerRaw returns an LLM server that responds with raw content.
func mockExtractionServerRaw(content string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", 404)
			return
		}

		escaped, _ := json.Marshal(content)
		w.Header().Set("Content-Type", "application/json")
		resp := fmt.Sprintf(`{"choices":[{"message":{"content":%s}}]}`, string(escaped))
		w.Write([]byte(resp))
	}))
}

func newTestClient(url string) *llm.Client {
	os.Setenv("LLM_BASE_URL", url)
	os.Setenv("LLM_API_KEY", "test-key")
	os.Setenv("LLM_MODEL", "test-model")
	defer os.Unsetenv("LLM_BASE_URL")
	defer os.Unsetenv("LLM_API_KEY")
	defer os.Unsetenv("LLM_MODEL")
	return llm.NewClient()
}

func TestExtractAndSave(t *testing.T) {
	facts := []string{
		"User prefers Go over Python",
		"User runs Ubuntu 24.04",
	}
	srv := mockExtractionServer(facts)
	defer srv.Close()

	client := newTestClient(srv.URL)

	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "home", "test-user", "agents", "test-agent", "memory")
	os.MkdirAll(agentDir, 0755)

	store := &KeywordStore{
		path: filepath.Join(agentDir, "memories.jsonl"),
	}

	ExtractAndSave(client, store, "I use Go on Ubuntu 24.04", "Great choice!")

	if store.Count() != 2 {
		t.Fatalf("expected 2 saved facts, got %d", store.Count())
	}

	// Verify the saved content.
	results, err := store.Search("Go programming", 5)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results after extraction")
	}
}

func TestExtractAndSave_EmptyArray(t *testing.T) {
	srv := mockExtractionServer([]string{})
	defer srv.Close()

	client := newTestClient(srv.URL)

	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "home", "test-user", "agents", "test-agent", "memory")
	os.MkdirAll(agentDir, 0755)

	store := &KeywordStore{
		path: filepath.Join(agentDir, "memories.jsonl"),
	}

	ExtractAndSave(client, store, "Hello", "Hi there!")

	if store.Count() != 0 {
		t.Fatalf("expected 0 saved facts for trivial exchange, got %d", store.Count())
	}
}

func TestExtractAndSave_EmptyInput(t *testing.T) {
	// Should be a no-op when input is empty.
	srv := mockExtractionServer([]string{"should not be called"})
	defer srv.Close()

	client := newTestClient(srv.URL)

	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "home", "test-user", "agents", "test-agent", "memory")
	os.MkdirAll(agentDir, 0755)

	store := &KeywordStore{
		path: filepath.Join(agentDir, "memories.jsonl"),
	}

	ExtractAndSave(client, store, "", "response")
	ExtractAndSave(client, store, "message", "")

	if store.Count() != 0 {
		t.Fatalf("expected 0 saves for empty input, got %d", store.Count())
	}
}

func TestExtractAndSave_MarkdownFences(t *testing.T) {
	// Some LLMs wrap JSON in markdown code fences.
	raw := "```json\n[\"User likes minimal dependencies\"]\n```"
	srv := mockExtractionServerRaw(raw)
	defer srv.Close()

	client := newTestClient(srv.URL)

	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "home", "test-user", "agents", "test-agent", "memory")
	os.MkdirAll(agentDir, 0755)

	store := &KeywordStore{
		path: filepath.Join(agentDir, "memories.jsonl"),
	}

	ExtractAndSave(client, store, "I hate heavy frameworks", "Got it!")

	if store.Count() != 1 {
		t.Fatalf("expected 1 saved fact (markdown-fenced), got %d", store.Count())
	}
}
