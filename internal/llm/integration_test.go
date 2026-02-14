package llm

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Integration tests that hit a real LLM API.
// Skipped when LLM_API_KEY is not set.
// Run with: go test ./internal/llm/ -run Integration -v

func realClient(t *testing.T) *Client {
	t.Helper()
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set — skipping integration test")
	}

	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	baseURL = strings.TrimSuffix(baseURL, "/")

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "llama-3.1-8b-instant" // fast, high RPD limit
	}

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func TestIntegration_Chat(t *testing.T) {
	client := realClient(t)

	resp, err := client.Chat(
		"You are a test assistant. Reply with exactly: PONG",
		[]Message{{Role: "user", Content: "PING"}},
	)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if resp == "" {
		t.Fatal("Chat() returned empty response")
	}
	t.Logf("Chat response: %s", resp)

	if !strings.Contains(strings.ToUpper(resp), "PONG") {
		t.Logf("Warning: expected PONG in response, got: %s", resp)
	}
}

func TestIntegration_ChatStream(t *testing.T) {
	client := realClient(t)

	var chunks []string
	resp, err := client.ChatStream(
		"You are a test assistant. Reply with exactly: STREAMING WORKS",
		[]Message{{Role: "user", Content: "Test streaming"}},
		func(chunk string) {
			chunks = append(chunks, chunk)
		},
	)
	if err != nil {
		t.Fatalf("ChatStream() error: %v", err)
	}
	if resp == "" {
		t.Fatal("ChatStream() returned empty response")
	}
	if len(chunks) == 0 {
		t.Fatal("ChatStream() delivered zero chunks — streaming not working")
	}

	// Verify chunks reassemble to the full response.
	var assembled strings.Builder
	for _, c := range chunks {
		assembled.WriteString(c)
	}
	if assembled.String() != resp {
		t.Fatalf("reassembled chunks = %q, full response = %q — mismatch", assembled.String(), resp)
	}

	t.Logf("Streaming response (%d chunks): %s", len(chunks), resp)
}
