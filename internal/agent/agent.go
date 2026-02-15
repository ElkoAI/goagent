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

// agentDir returns the base directory for an agent under a user's home.
func agentDir(dataDir, userID, agentName string) string {
	return filepath.Join(dataDir, "home", userID, "agents", agentName)
}

// LoadSystemPrompt reads the agent's markdown file and extracts the Intent section as the system prompt.
func LoadSystemPrompt(dataDir, userID, agentName string) (string, error) {
	mdName := strings.ToUpper(agentName) + ".md"
	mdPath := filepath.Join(agentDir(dataDir, userID, agentName), mdName)

	data, err := os.ReadFile(mdPath)
	if err != nil {
		return "", fmt.Errorf("read agent file %s: %w", mdPath, err)
	}

	return extractSection(string(data), "# Intent"), nil
}

// LoadUserProfile reads the global USER.md for a user. Returns "" if not found.
func LoadUserProfile(dataDir, userID string) (string, error) {
	profilePath := filepath.Join(dataDir, "home", userID, "USER.md")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read user profile: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// LoadSoul reads the per-agent SOUL.md. Returns "" if not found.
func LoadSoul(dataDir, userID, agentName string) (string, error) {
	soulPath := filepath.Join(agentDir(dataDir, userID, agentName), "SOUL.md")
	data, err := os.ReadFile(soulPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read agent soul: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// EnsureUserHome creates the user's home directory and a starter USER.md if missing.
func EnsureUserHome(dataDir, userID string) error {
	homeDir := filepath.Join(dataDir, "home", userID)
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		return err
	}

	profilePath := filepath.Join(homeDir, "USER.md")
	if _, err := os.Stat(profilePath); err == nil {
		return nil // already exists
	}

	starter := "# User Profile\n\n" +
		"<!-- This file is shared by all your agents. -->\n" +
		"<!-- Add preferences, context, and communication style here. -->\n"
	return os.WriteFile(profilePath, []byte(starter), 0644)
}

// extractSection pulls the content under a given heading (e.g. "# Intent").
func extractSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	var collecting bool
	var result []string

	for _, line := range lines {
		if strings.HasPrefix(line, heading) {
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
func LoadHistory(dataDir, userID, agentName string) ([]llm.Message, error) {
	histPath := filepath.Join(agentDir(dataDir, userID, agentName), "memory", "chat.jsonl")

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
func AppendHistory(dataDir, userID, agentName string, msgs ...llm.Message) error {
	histPath := filepath.Join(agentDir(dataDir, userID, agentName), "memory", "chat.jsonl")

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
