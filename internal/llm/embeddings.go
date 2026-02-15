package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
)

// EmbedClient calls an OpenAI-compatible /v1/embeddings endpoint.
type EmbedClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewEmbedClient creates an embedding client.
// Uses EMBED_* env vars with fallback to LLM_* vars.
func NewEmbedClient() *EmbedClient {
	baseURL := os.Getenv("EMBED_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("LLM_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := os.Getenv("EMBED_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("LLM_API_KEY")
	}

	model := os.Getenv("EMBED_MODEL")
	if model == "" {
		model = "text-embedding-3-small"
	}

	return &EmbedClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		httpClient: http.DefaultClient,
	}
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Embed returns the embedding vector for the given text.
func (c *EmbedClient) Embed(text string) ([]float64, error) {
	body, err := json.Marshal(embedRequest{
		Model: c.model,
		Input: text,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embed response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var embedResp embedResponse
	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		return nil, fmt.Errorf("parse embed response: %w", err)
	}

	if embedResp.Error != nil {
		return nil, fmt.Errorf("embed error: %s", embedResp.Error.Message)
	}

	if len(embedResp.Data) == 0 {
		return nil, fmt.Errorf("embed returned no data")
	}

	return embedResp.Data[0].Embedding, nil
}

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
