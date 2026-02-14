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
