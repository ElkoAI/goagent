package memory

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goagent/internal/llm"
)

// mockEmbedServer returns a deterministic embedding based on the input text.
// "finance" → [1, 0, 0], "weather" → [0, 1, 0], anything else → [0, 0, 1].
// This lets us verify cosine similarity in tests.
func mockEmbedServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			http.Error(w, "not found", 404)
			return
		}

		var req struct {
			Input string `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		var embedding []float64
		lower := strings.ToLower(req.Input)
		switch {
		case strings.Contains(lower, "finance") || strings.Contains(lower, "stock") || strings.Contains(lower, "invest"):
			embedding = []float64{1, 0, 0}
		case strings.Contains(lower, "weather") || strings.Contains(lower, "rain") || strings.Contains(lower, "temperature"):
			embedding = []float64{0, 1, 0}
		default:
			embedding = []float64{0, 0, 1}
		}

		w.Header().Set("Content-Type", "application/json")
		resp := fmt.Sprintf(`{"data":[{"embedding":[%f,%f,%f],"index":0}]}`,
			embedding[0], embedding[1], embedding[2])
		w.Write([]byte(resp))
	}))
}

func newTestStore(t *testing.T) (*Store, *httptest.Server) {
	t.Helper()

	srv := mockEmbedServer()
	embedClient := &llm.EmbedClient{}
	// Use reflection-free approach: create a client pointing at our mock.
	*embedClient = *createTestEmbedClient(srv.URL)

	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "home", "test-user", "agents", "test-agent", "memory")
	os.MkdirAll(agentDir, 0755)

	store := &Store{
		path:  filepath.Join(agentDir, "memories.jsonl"),
		embed: embedClient,
	}

	return store, srv
}

// createTestEmbedClient creates an EmbedClient pointing at a test server.
func createTestEmbedClient(baseURL string) *llm.EmbedClient {
	// We can't set fields directly since they're unexported.
	// Use the env var approach with a test override.
	os.Setenv("EMBED_BASE_URL", baseURL)
	os.Setenv("EMBED_API_KEY", "test-key")
	os.Setenv("EMBED_MODEL", "test-model")
	defer os.Unsetenv("EMBED_BASE_URL")
	defer os.Unsetenv("EMBED_API_KEY")
	defer os.Unsetenv("EMBED_MODEL")
	return llm.NewEmbedClient()
}

func TestStoreSaveAndCount(t *testing.T) {
	store, srv := newTestStore(t)
	defer srv.Close()

	if store.Count() != 0 {
		t.Fatalf("empty store count = %d", store.Count())
	}

	if err := store.Save("AAPL stock is at 230"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if store.Count() != 1 {
		t.Fatalf("count after save = %d, want 1", store.Count())
	}

	if err := store.Save("It will rain tomorrow"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if store.Count() != 2 {
		t.Fatalf("count after second save = %d, want 2", store.Count())
	}
}

func TestStoreLoadFromDisk(t *testing.T) {
	store, srv := newTestStore(t)
	defer srv.Close()

	store.Save("Stock market is up")
	store.Save("Rain expected Friday")

	// Create a new store pointing at the same file.
	store2 := &Store{path: store.path, embed: store.embed}
	if err := store2.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if store2.Count() != 2 {
		t.Fatalf("loaded count = %d, want 2", store2.Count())
	}
}

func TestStoreSearch(t *testing.T) {
	store, srv := newTestStore(t)
	defer srv.Close()

	// Save memories in different domains.
	store.Save("AAPL stock price is 230 dollars")
	store.Save("The weather is sunny today")
	store.Save("Investing in index funds is smart")

	// Search for finance-related query.
	results, err := store.Search("stock market performance", 2)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	// The top result should be finance-related (cosine similarity = 1.0 for same vector).
	if results[0].Score < 0.9 {
		t.Errorf("top result score = %f, expected near 1.0", results[0].Score)
	}

	// The finance memories should rank higher than the weather memory.
	if strings.Contains(results[0].Text, "weather") {
		t.Errorf("top result should be finance-related, got: %q", results[0].Text)
	}
}

func TestStoreSearchEmpty(t *testing.T) {
	store, srv := newTestStore(t)
	defer srv.Close()

	results, err := store.Search("anything", 5)
	if err != nil {
		t.Fatalf("Search() on empty store error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("empty store should return 0 results, got %d", len(results))
	}
}

func TestSearchByVector(t *testing.T) {
	store, srv := newTestStore(t)
	defer srv.Close()

	store.Save("Stock analysis for AAPL")
	store.Save("Temperature forecast")

	// Search with a finance vector directly (no API call needed).
	results := store.SearchByVector([]float64{1, 0, 0}, 1)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !strings.Contains(results[0].Text, "Stock") {
		t.Errorf("expected finance result, got: %q", results[0].Text)
	}
}

func TestBuildContext(t *testing.T) {
	results := []SearchResult{
		{Text: "User likes Python", Score: 0.95},
		{Text: "User lives in Berlin", Score: 0.85},
		{Text: "Unrelated fact", Score: 0.3},
	}

	ctx := BuildContext(results, 0.5)
	if !strings.Contains(ctx, "User likes Python") {
		t.Error("context should include high-score result")
	}
	if !strings.Contains(ctx, "User lives in Berlin") {
		t.Error("context should include medium-score result")
	}
	if strings.Contains(ctx, "Unrelated fact") {
		t.Error("context should exclude low-score result")
	}
}

func TestBuildContext_Empty(t *testing.T) {
	ctx := BuildContext(nil, 0.5)
	if ctx != "" {
		t.Errorf("empty results should produce empty context, got: %q", ctx)
	}
}

func TestCosineSimilarity(t *testing.T) {
	// Identical vectors → 1.0
	score := llm.CosineSimilarity([]float64{1, 0, 0}, []float64{1, 0, 0})
	if math.Abs(score-1.0) > 0.001 {
		t.Errorf("identical vectors: score = %f, want 1.0", score)
	}

	// Orthogonal vectors → 0.0
	score = llm.CosineSimilarity([]float64{1, 0, 0}, []float64{0, 1, 0})
	if math.Abs(score) > 0.001 {
		t.Errorf("orthogonal vectors: score = %f, want 0.0", score)
	}

	// Opposite vectors → -1.0
	score = llm.CosineSimilarity([]float64{1, 0, 0}, []float64{-1, 0, 0})
	if math.Abs(score+1.0) > 0.001 {
		t.Errorf("opposite vectors: score = %f, want -1.0", score)
	}

	// Empty/mismatched → 0.0
	score = llm.CosineSimilarity([]float64{}, []float64{})
	if score != 0 {
		t.Errorf("empty vectors: score = %f, want 0.0", score)
	}
	score = llm.CosineSimilarity([]float64{1, 0}, []float64{1, 0, 0})
	if score != 0 {
		t.Errorf("mismatched length: score = %f, want 0.0", score)
	}
}
