package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewClient() *Client {
	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "deepseek-chat"
	}

	return &Client{
		baseURL: baseURL,
		apiKey:  os.Getenv("LLM_API_KEY"),
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) Chat(systemPrompt string, messages []Message) (string, error) {
	msgs := make([]Message, 0, len(messages)+1)
	msgs = append(msgs, Message{Role: "system", Content: systemPrompt})
	msgs = append(msgs, messages...)

	body, err := json.Marshal(chatRequest{
		Model:    c.model,
		Messages: msgs,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm returned %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("llm error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

type streamChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type streamDelta struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// ChatStream sends a streaming chat completion request and calls onChunk for
// each token received. It returns the fully assembled response when done.
func (c *Client) ChatStream(systemPrompt string, messages []Message, onChunk func(chunk string)) (string, error) {
	msgs := make([]Message, 0, len(messages)+1)
	msgs = append(msgs, Message{Role: "system", Content: systemPrompt})
	msgs = append(msgs, messages...)

	body, err := json.Marshal(streamChatRequest{
		Model:    c.model,
		Messages: msgs,
		Stream:   true,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Use a client without timeout — streaming responses can take a while.
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm returned %d: %s", resp.StatusCode, string(respBody))
	}

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var delta streamDelta
		if err := json.Unmarshal([]byte(payload), &delta); err != nil {
			continue
		}
		if len(delta.Choices) > 0 && delta.Choices[0].Delta.Content != "" {
			chunk := delta.Choices[0].Delta.Content
			full.WriteString(chunk)
			if onChunk != nil {
				onChunk(chunk)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("stream read error: %w", err)
	}

	return full.String(), nil
}
