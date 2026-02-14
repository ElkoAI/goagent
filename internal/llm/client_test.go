package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockLLMServer returns an httptest.Server that simulates an OpenAI-compatible
// /v1/chat/completions endpoint. If stream is requested it responds with SSE
// events; otherwise it responds with a normal JSON body.
func mockLLMServer(response string) *httptest.Server {
	chunks := splitIntoChunks(response, 5) // ~5-char chunks

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", 404)
			return
		}

		var req struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", 400)
			return
		}

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(200)
			flusher, _ := w.(http.Flusher)

			for _, chunk := range chunks {
				event := fmt.Sprintf(`{"choices":[{"delta":{"content":"%s"}}]}`, chunk)
				fmt.Fprintf(w, "data: %s\n\n", event)
				if flusher != nil {
					flusher.Flush()
				}
			}
			fmt.Fprint(w, "data: [DONE]\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			resp := fmt.Sprintf(`{"choices":[{"message":{"content":"%s"}}]}`, response)
			w.Write([]byte(resp))
		}
	}))
}

func splitIntoChunks(s string, size int) []string {
	var chunks []string
	for len(s) > 0 {
		end := size
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[:end])
		s = s[end:]
	}
	return chunks
}

func TestChat(t *testing.T) {
	srv := mockLLMServer("Hello from the mock LLM!")
	defer srv.Close()

	client := &Client{
		baseURL:    srv.URL,
		apiKey:     "test-key",
		model:      "test-model",
		httpClient: http.DefaultClient,
	}

	resp, err := client.Chat("You are helpful.", []Message{
		{Role: "user", Content: "Hi"},
	})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if resp != "Hello from the mock LLM!" {
		t.Fatalf("Chat() = %q, want %q", resp, "Hello from the mock LLM!")
	}
}

func TestChatStream(t *testing.T) {
	want := "Hello from the streaming mock!"
	srv := mockLLMServer(want)
	defer srv.Close()

	client := &Client{
		baseURL:    srv.URL,
		apiKey:     "test-key",
		model:      "test-model",
		httpClient: http.DefaultClient,
	}

	var chunks []string
	resp, err := client.ChatStream("You are helpful.", []Message{
		{Role: "user", Content: "Hi"},
	}, func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("ChatStream() error: %v", err)
	}
	if resp != want {
		t.Fatalf("ChatStream() full response = %q, want %q", resp, want)
	}
	if len(chunks) == 0 {
		t.Fatal("ChatStream() delivered zero chunks")
	}

	// Reassemble chunks and verify they equal the full response.
	var assembled string
	for _, c := range chunks {
		assembled += c
	}
	if assembled != want {
		t.Fatalf("reassembled chunks = %q, want %q", assembled, want)
	}
}

func TestChatStream_NoCallback(t *testing.T) {
	want := "Works without callback"
	srv := mockLLMServer(want)
	defer srv.Close()

	client := &Client{
		baseURL:    srv.URL,
		apiKey:     "test-key",
		model:      "test-model",
		httpClient: http.DefaultClient,
	}

	resp, err := client.ChatStream("system", []Message{
		{Role: "user", Content: "test"},
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream(nil callback) error: %v", err)
	}
	if resp != want {
		t.Fatalf("ChatStream(nil callback) = %q, want %q", resp, want)
	}
}

func TestChat_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := &Client{
		baseURL:    srv.URL,
		apiKey:     "test-key",
		model:      "test-model",
		httpClient: http.DefaultClient,
	}

	_, err := client.Chat("system", []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("Chat() should return error on 500")
	}
}

func TestChatStream_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte("rate limited"))
	}))
	defer srv.Close()

	client := &Client{
		baseURL:    srv.URL,
		apiKey:     "test-key",
		model:      "test-model",
		httpClient: http.DefaultClient,
	}

	_, err := client.ChatStream("system", []Message{{Role: "user", Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("ChatStream() should return error on 429")
	}
}

// mockToolCallServer simulates an LLM that returns a tool call on the first
// request, then returns a text response on the second request (after receiving
// the tool result).
func mockToolCallServer() *httptest.Server {
	callCount := 0

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", 404)
			return
		}

		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			// First call: return a tool call.
			w.Write([]byte(`{
				"choices": [{
					"message": {
						"content": "",
						"tool_calls": [{
							"id": "call_001",
							"type": "function",
							"function": {
								"name": "web_search",
								"arguments": "{\"query\": \"Go programming\"}"
							}
						}]
					},
					"finish_reason": "tool_calls"
				}]
			}`))
		} else {
			// Second call (after tool result): return text.
			w.Write([]byte(`{
				"choices": [{
					"message": {
						"content": "Based on the search results, Go is great!"
					},
					"finish_reason": "stop"
				}]
			}`))
		}
	}))
}

func TestChatWithTools_TextResponse(t *testing.T) {
	srv := mockLLMServer("Direct text answer")
	defer srv.Close()

	client := &Client{
		baseURL:    srv.URL,
		apiKey:     "test-key",
		model:      "test-model",
		httpClient: http.DefaultClient,
	}

	result, err := client.ChatWithTools([]Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hi"},
	}, []ToolDef{{
		Type: "function",
		Function: ToolSchema{
			Name:        "web_search",
			Description: "Search the web",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}})
	if err != nil {
		t.Fatalf("ChatWithTools() error: %v", err)
	}
	if result.Content != "Direct text answer" {
		t.Errorf("Content = %q, want 'Direct text answer'", result.Content)
	}
	if len(result.ToolCalls) != 0 {
		t.Errorf("ToolCalls should be empty, got %d", len(result.ToolCalls))
	}
}

func TestChatWithTools_ToolCallResponse(t *testing.T) {
	srv := mockToolCallServer()
	defer srv.Close()

	client := &Client{
		baseURL:    srv.URL,
		apiKey:     "test-key",
		model:      "test-model",
		httpClient: http.DefaultClient,
	}

	// First call: should return a tool call.
	result, err := client.ChatWithTools([]Message{
		{Role: "user", Content: "Search for Go programming"},
	}, nil)
	if err != nil {
		t.Fatalf("ChatWithTools() round 1 error: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}

	tc := result.ToolCalls[0]
	if tc.ID != "call_001" {
		t.Errorf("ToolCall.ID = %q", tc.ID)
	}
	if tc.Function.Name != "web_search" {
		t.Errorf("ToolCall.Function.Name = %q", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"query": "Go programming"}` {
		t.Errorf("ToolCall.Function.Arguments = %q", tc.Function.Arguments)
	}

	// Second call: feed tool result back, should get text.
	result, err = client.ChatWithTools([]Message{
		{Role: "user", Content: "Search for Go programming"},
		{Role: "assistant", ToolCalls: result.ToolCalls},
		{Role: "tool", ToolCallID: tc.ID, Content: `{"results": [{"title": "Go", "url": "https://go.dev"}]}`},
	}, nil)
	if err != nil {
		t.Fatalf("ChatWithTools() round 2 error: %v", err)
	}
	if result.Content != "Based on the search results, Go is great!" {
		t.Errorf("Content = %q", result.Content)
	}
	if len(result.ToolCalls) != 0 {
		t.Errorf("round 2 should have no tool calls, got %d", len(result.ToolCalls))
	}
}
