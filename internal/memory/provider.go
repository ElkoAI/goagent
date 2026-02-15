package memory

import (
	"os"

	"goagent/internal/llm"
)

// Provider is the plug-and-play interface for memory backends.
// Implementations: KeywordStore (zero cost, default) and Store (vector embeddings).
type Provider interface {
	Load() error
	Search(query string, topK int) ([]SearchResult, error)
	Save(text string) error
	Count() int
}

// NewProvider auto-selects the best available memory backend.
// Uses vector embeddings if EMBED_BASE_URL or EMBED_API_KEY is set,
// otherwise falls back to the zero-cost keyword scorer.
func NewProvider(dataDir, userID, agentName string, embed *llm.EmbedClient) Provider {
	if os.Getenv("EMBED_BASE_URL") != "" || os.Getenv("EMBED_API_KEY") != "" {
		return NewStore(dataDir, userID, agentName, embed)
	}
	return NewKeywordStore(dataDir, userID, agentName)
}
