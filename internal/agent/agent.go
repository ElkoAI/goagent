package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goagent/internal/llm"
)

// LoadSystemPrompt reads the agent's markdown file and extracts the Intent section as the system prompt.
func LoadSystemPrompt(dataDir, agentName string) (string, error) {
	mdName := strings.ToUpper(agentName) + ".md"
	mdPath := filepath.Join(dataDir, "agents", agentName, mdName)

	data, err := os.ReadFile(mdPath)
	if err != nil {
		return "", fmt.Errorf("read agent file %s: %w", mdPath, err)
	}

	return extractIntent(string(data)), nil
}

// extractIntent pulls the content under the first "# Intent" heading.
func extractIntent(content string) string {
	lines := strings.Split(content, "\n")
	var collecting bool
	var result []string

	for _, line := range lines {
		if strings.HasPrefix(line, "# Intent") {
			collecting = true
			continue
		}
		if collecting && strings.HasPrefix(line, "# ") {
			break
		}
		if collecting {
			result = append(result, line)
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

// LoadHistory reads the agent's chat.jsonl file and returns the message history.
func LoadHistory(dataDir, agentName string) ([]llm.Message, error) {
	histPath := filepath.Join(dataDir, "agents", agentName, "memory", "chat.jsonl")

	f, err := os.Open(histPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open history: %w", err)
	}
	defer f.Close()

	var messages []llm.Message
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var msg llm.Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	return messages, scanner.Err()
}

// AppendHistory appends messages as JSONL lines to the agent's chat.jsonl file.
func AppendHistory(dataDir, agentName string, msgs ...llm.Message) error {
	histPath := filepath.Join(dataDir, "agents", agentName, "memory", "chat.jsonl")

	f, err := os.OpenFile(histPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open history for append: %w", err)
	}
	defer f.Close()

	for _, msg := range msgs {
		data, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("marshal message: %w", err)
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("write message: %w", err)
		}
	}

	return nil
}
