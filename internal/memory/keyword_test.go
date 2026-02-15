package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestKeywordStore(t *testing.T) *KeywordStore {
	t.Helper()
	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "home", "test-user", "agents", "test-agent", "memory")
	os.MkdirAll(agentDir, 0755)
	return &KeywordStore{
		path: filepath.Join(agentDir, "memories.jsonl"),
	}
}

func TestKeywordStoreSaveAndCount(t *testing.T) {
	store := newTestKeywordStore(t)

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

func TestKeywordStoreLoadFromDisk(t *testing.T) {
	store := newTestKeywordStore(t)

	store.Save("Stock market is up")
	store.Save("Rain expected Friday")

	store2 := &KeywordStore{path: store.path}
	if err := store2.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if store2.Count() != 2 {
		t.Fatalf("loaded count = %d, want 2", store2.Count())
	}
}

func TestKeywordStoreSearch(t *testing.T) {
	store := newTestKeywordStore(t)

	store.Save("AAPL stock price is 230 dollars")
	store.Save("The weather forecast says sunny today")
	store.Save("Investing in index funds is smart for long term growth")

	results, err := store.Search("stock market performance", 2)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	// The top result should be stock-related since "stock" appears in both query and memory.
	if !strings.Contains(strings.ToLower(results[0].Text), "stock") {
		t.Errorf("top result should contain 'stock', got: %q", results[0].Text)
	}

	if results[0].Score <= 0 {
		t.Errorf("top result score should be positive, got: %f", results[0].Score)
	}
}

func TestKeywordStoreSearchEmpty(t *testing.T) {
	store := newTestKeywordStore(t)

	results, err := store.Search("anything", 5)
	if err != nil {
		t.Fatalf("Search() on empty store error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("empty store should return 0 results, got %d", len(results))
	}
}

func TestKeywordStoreSearchRelevance(t *testing.T) {
	store := newTestKeywordStore(t)

	store.Save("User prefers low-cost index funds over individual stocks")
	store.Save("User enjoys hiking in the mountains on weekends")
	store.Save("User asked about Bitcoin price prediction")

	results, err := store.Search("index funds investment strategy", 3)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	// The index funds memory should score highest.
	if !strings.Contains(results[0].Text, "index funds") {
		t.Errorf("top result should be about index funds, got: %q", results[0].Text)
	}

	// The hiking memory should score lowest (no keyword overlap).
	if results[0].Score <= results[2].Score {
		t.Errorf("relevant result should score higher than irrelevant")
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("The quick brown fox jumps over the lazy dog!")
	// "the" is a stop word, should be excluded
	for _, tok := range tokens {
		if tok == "the" {
			t.Error("stop word 'the' should be filtered")
		}
	}
	if len(tokens) == 0 {
		t.Error("tokenize should return some tokens")
	}
}

func TestKeywordBuildContextIntegration(t *testing.T) {
	store := newTestKeywordStore(t)

	store.Save("User prefers Python for data science")
	store.Save("User lives in Berlin, Germany")
	store.Save("Random unrelated memory about cooking pasta")

	results, _ := store.Search("Python programming", 3)
	ctx := BuildContext(results, 0.1)

	if !strings.Contains(ctx, "Python") {
		t.Error("context should include Python memory")
	}
}
