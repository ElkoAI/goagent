package memory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"goagent/internal/llm"
)

// Entry is a single memory with its embedding vector.
type Entry struct {
	Text      string    `json:"text"`
	Embedding []float64 `json:"embedding"`
	Timestamp string    `json:"ts"`
}

// SearchResult is a memory entry with its similarity score.
type SearchResult struct {
	Text      string  `json:"text"`
	Score     float64 `json:"score"`
	Timestamp string  `json:"ts"`
}

// Store manages memories for a single agent, backed by a JSONL file.
type Store struct {
	path    string // path to memories.jsonl
	embed   *llm.EmbedClient
	entries []Entry
}

// NewStore creates a memory store for the given agent under a user's home.
func NewStore(dataDir, userID, agentName string, embed *llm.EmbedClient) *Store {
	return &Store{
		path:  filepath.Join(dataDir, "home", userID, "agents", agentName, "memory", "memories.jsonl"),
		embed: embed,
	}
}

// Load reads all memories from disk into memory.
func (s *Store) Load() error {
	s.entries = nil

	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no memories yet
		}
		return fmt.Errorf("open memories: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Allow large lines (embeddings can be big).
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip malformed lines
		}
		s.entries = append(s.entries, entry)
	}

	return scanner.Err()
}

// Save stores a text as a new memory, embedding it first.
func (s *Store) Save(text string) error {
	embedding, err := s.embed.Embed(text)
	if err != nil {
		return fmt.Errorf("embed memory: %w", err)
	}

	entry := Entry{
		Text:      text,
		Embedding: embedding,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Append to file.
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open memories for append: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal memory: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write memory: %w", err)
	}

	// Add to in-memory cache.
	s.entries = append(s.entries, entry)
	return nil
}

// Search finds the top-K memories most similar to the query text.
func (s *Store) Search(query string, topK int) ([]SearchResult, error) {
	if len(s.entries) == 0 {
		return nil, nil
	}

	queryEmbed, err := s.embed.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	return s.SearchByVector(queryEmbed, topK), nil
}

// SearchByVector finds the top-K memories most similar to the given vector.
// Does not require an embedding API call (useful when you already have the vector).
func (s *Store) SearchByVector(queryEmbed []float64, topK int) []SearchResult {
	if len(s.entries) == 0 {
		return nil
	}

	type scored struct {
		entry Entry
		score float64
	}

	results := make([]scored, 0, len(s.entries))
	for _, e := range s.entries {
		score := llm.CosineSimilarity(queryEmbed, e.Embedding)
		results = append(results, scored{entry: e, score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if topK > len(results) {
		topK = len(results)
	}

	out := make([]SearchResult, topK)
	for i := 0; i < topK; i++ {
		out[i] = SearchResult{
			Text:      results[i].entry.Text,
			Score:     results[i].score,
			Timestamp: results[i].entry.Timestamp,
		}
	}
	return out
}

// BuildContext formats the top-K search results into a string suitable
// for injection into a system prompt.
func BuildContext(results []SearchResult, minScore float64) string {
	var relevant []SearchResult
	for _, r := range results {
		if r.Score >= minScore {
			relevant = append(relevant, r)
		}
	}

	if len(relevant) == 0 {
		return ""
	}

	s := "\n\n---\n# Relevant Memories\n"
	for _, r := range relevant {
		s += fmt.Sprintf("- %s\n", r.Text)
	}
	return s
}

// Count returns the number of stored memories.
func (s *Store) Count() int {
	return len(s.entries)
}
