package memory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

// KeywordEntry is a memory stored without embedding vectors.
type KeywordEntry struct {
	Text      string `json:"text"`
	Timestamp string `json:"ts"`
}

// KeywordStore is a zero-cost memory backend using TF-IDF keyword scoring.
// No embedding API calls — all scoring runs in-process.
type KeywordStore struct {
	path    string
	entries []KeywordEntry
	// idf caches inverse document frequency across all entries.
	idf map[string]float64
}

// SetPath sets the file path for the keyword store (used by tests with fixtures).
func (k *KeywordStore) SetPath(path string) { k.path = path }

// NewKeywordStore creates a keyword-based memory store for the given agent under a user's home.
func NewKeywordStore(dataDir, userID, agentName string) *KeywordStore {
	return &KeywordStore{
		path: filepath.Join(dataDir, "home", userID, "agents", agentName, "memory", "memories.jsonl"),
	}
}

// Load reads all memories from disk.
func (k *KeywordStore) Load() error {
	k.entries = nil
	k.idf = nil

	f, err := os.Open(k.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open memories: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry KeywordEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		k.entries = append(k.entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	k.buildIDF()
	return nil
}

// Save stores a text as a new memory (no API call).
func (k *KeywordStore) Save(text string) error {
	entry := KeywordEntry{
		Text:      text,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if err := os.MkdirAll(filepath.Dir(k.path), 0755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}

	f, err := os.OpenFile(k.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

	k.entries = append(k.entries, entry)
	k.buildIDF()
	return nil
}

// Search finds the top-K memories most relevant to the query using TF-IDF cosine similarity.
func (k *KeywordStore) Search(query string, topK int) ([]SearchResult, error) {
	if len(k.entries) == 0 {
		return nil, nil
	}

	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return nil, nil
	}

	queryTFIDF := k.tfidfVector(queryTerms)

	type scored struct {
		entry KeywordEntry
		score float64
	}

	results := make([]scored, 0, len(k.entries))
	for _, e := range k.entries {
		docTerms := tokenize(e.Text)
		docTFIDF := k.tfidfVector(docTerms)
		score := cosineSim(queryTFIDF, docTFIDF)
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
	return out, nil
}

// Count returns the number of stored memories.
func (k *KeywordStore) Count() int {
	return len(k.entries)
}

// buildIDF computes inverse document frequency for all terms across all entries.
func (k *KeywordStore) buildIDF() {
	n := float64(len(k.entries))
	if n == 0 {
		k.idf = nil
		return
	}

	// Count how many documents contain each term.
	docFreq := make(map[string]int)
	for _, e := range k.entries {
		seen := make(map[string]bool)
		for _, term := range tokenize(e.Text) {
			if !seen[term] {
				docFreq[term]++
				seen[term] = true
			}
		}
	}

	k.idf = make(map[string]float64, len(docFreq))
	for term, df := range docFreq {
		k.idf[term] = math.Log(1 + n/float64(df))
	}
}

// tfidfVector returns a sparse TF-IDF vector for the given terms.
func (k *KeywordStore) tfidfVector(terms []string) map[string]float64 {
	// Term frequency.
	tf := make(map[string]int, len(terms))
	for _, t := range terms {
		tf[t]++
	}

	vec := make(map[string]float64, len(tf))
	n := float64(len(terms))
	for term, count := range tf {
		idf := k.idf[term]
		if idf == 0 {
			// Unknown term (not in corpus) — give it a high IDF.
			idf = math.Log(1 + float64(len(k.entries)))
		}
		vec[term] = (float64(count) / n) * idf
	}
	return vec
}

// cosineSim computes cosine similarity between two sparse vectors.
func cosineSim(a, b map[string]float64) float64 {
	var dot, normA, normB float64

	for term, va := range a {
		if vb, ok := b[term]; ok {
			dot += va * vb
		}
		normA += va * va
	}
	for _, vb := range b {
		normB += vb * vb
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// tokenize splits text into lowercase terms, stripping punctuation.
func tokenize(text string) []string {
	lower := strings.ToLower(text)
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	// Filter stop words.
	out := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) > 1 && !stopWords[w] {
			out = append(out, w)
		}
	}
	return out
}

var stopWords = map[string]bool{
	"the": true, "is": true, "at": true, "in": true, "on": true,
	"to": true, "of": true, "and": true, "or": true, "for": true,
	"an": true, "it": true, "by": true, "as": true, "be": true,
	"was": true, "are": true, "were": true, "been": true, "has": true,
	"had": true, "have": true, "do": true, "does": true, "did": true,
	"will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true,
	"this": true, "that": true, "these": true, "those": true,
	"am": true, "with": true, "from": true, "not": true,
	"but": true, "what": true, "which": true, "who": true,
	"when": true, "where": true, "how": true, "all": true,
	"each": true, "every": true, "both": true, "few": true,
	"more": true, "most": true, "other": true, "some": true,
	"such": true, "no": true, "nor": true, "too": true,
	"very": true, "just": true, "about": true, "above": true,
	"after": true, "again": true, "also": true, "any": true,
}
