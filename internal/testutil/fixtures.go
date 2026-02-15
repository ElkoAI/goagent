// Package testutil provides shared test fixtures for GoAgent.
// All fixtures use t.TempDir() and clean up automatically.
package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Fixture holds paths and metadata for a complete test environment.
type Fixture struct {
	DataDir string // root data directory (temp)
}

// User represents a test user with their home directory populated.
type User struct {
	ID      string
	HomeDir string
}

// Agent represents a test agent under a user.
type Agent struct {
	Name     string
	AgentDir string
}

// NewFixture creates a fresh test environment with users.json and test users.
// It sets up two users:
//   - "john" (token: "tok-john") — has USER.md with real preferences, two agents
//   - "alice" (token: "tok-alice") — has USER.md, one agent
//
// Call this once per test function that needs a full environment.
func NewFixture(t *testing.T) *Fixture {
	t.Helper()
	dataDir := t.TempDir()
	f := &Fixture{DataDir: dataDir}

	// Write users.json
	users := map[string]any{
		"users": map[string]any{
			"tok-john":  map[string]string{"id": "john"},
			"tok-alice": map[string]string{"id": "alice"},
		},
	}
	writeJSON(t, filepath.Join(dataDir, "users.json"), users)

	// --- John ---
	john := f.ensureUser(t, "john")
	writeFile(t, filepath.Join(john.HomeDir, "USER.md"),
		"# User Profile\n\n"+
			"- Prefers Go over Python\n"+
			"- Runs Ubuntu 24.04 LTS\n"+
			"- Likes minimal dependencies\n"+
			"- Dislikes verbose output and \"glazing\"\n"+
			"- Communication style: direct, concise\n")

	// John's advisor agent
	advisor := f.ensureAgent(t, "john", "advisor")
	writeFile(t, filepath.Join(advisor.AgentDir, "ADVISOR.md"),
		"# Intent\nYou are a financial advisor specializing in index funds and ETFs.\n\n"+
			"# Status\nActive.\n")
	writeFile(t, filepath.Join(advisor.AgentDir, "SOUL.md"),
		"# Soul\n\n"+
			"Be direct and data-driven. Avoid hype.\n"+
			"When uncertain, say so. Cite sources when possible.\n")

	// Advisor memories
	advisorMem := filepath.Join(advisor.AgentDir, "memory", "memories.jsonl")
	appendJSONL(t, advisorMem, map[string]string{"text": "User prefers low-cost index funds over individual stocks", "ts": "2026-02-10T10:00:00Z"})
	appendJSONL(t, advisorMem, map[string]string{"text": "User has a long-term investment horizon of 20+ years", "ts": "2026-02-11T09:00:00Z"})
	appendJSONL(t, advisorMem, map[string]string{"text": "User is skeptical of cryptocurrency", "ts": "2026-02-12T14:00:00Z"})

	// John's coder agent
	coder := f.ensureAgent(t, "john", "coder")
	writeFile(t, filepath.Join(coder.AgentDir, "CODER.md"),
		"# Intent\nYou are a Go programming assistant.\n\n"+
			"# Status\nActive.\n")
	writeFile(t, filepath.Join(coder.AgentDir, "SOUL.md"),
		"# Soul\n\n"+
			"Write minimal, idiomatic Go. No frameworks unless asked.\n"+
			"Prefer stdlib. Explain trade-offs briefly.\n")

	// --- Alice ---
	alice := f.ensureUser(t, "alice")
	writeFile(t, filepath.Join(alice.HomeDir, "USER.md"),
		"# User Profile\n\n"+
			"- Uses macOS Sequoia\n"+
			"- Prefers Python and TypeScript\n"+
			"- Enjoys detailed explanations with examples\n")

	// Alice's researcher agent
	researcher := f.ensureAgent(t, "alice", "researcher")
	writeFile(t, filepath.Join(researcher.AgentDir, "RESEARCHER.md"),
		"# Intent\nYou are a research assistant that finds and summarizes academic papers.\n\n"+
			"# Status\nActive.\n")
	writeFile(t, filepath.Join(researcher.AgentDir, "SOUL.md"),
		"# Soul\n\n"+
			"Be thorough and cite sources. Provide balanced viewpoints.\n")

	return f
}

// UserHome returns the home directory path for a user.
func (f *Fixture) UserHome(userID string) string {
	return filepath.Join(f.DataDir, "home", userID)
}

// AgentDir returns the agent directory path.
func (f *Fixture) AgentDir(userID, agentName string) string {
	return filepath.Join(f.DataDir, "home", userID, "agents", agentName)
}

// MemoriesPath returns the path to an agent's memories.jsonl.
func (f *Fixture) MemoriesPath(userID, agentName string) string {
	return filepath.Join(f.AgentDir(userID, agentName), "memory", "memories.jsonl")
}

func (f *Fixture) ensureUser(t *testing.T, userID string) User {
	t.Helper()
	homeDir := filepath.Join(f.DataDir, "home", userID)
	os.MkdirAll(homeDir, 0755)
	os.MkdirAll(filepath.Join(homeDir, "sessions"), 0755)
	return User{ID: userID, HomeDir: homeDir}
}

func (f *Fixture) ensureAgent(t *testing.T, userID, agentName string) Agent {
	t.Helper()
	agentDir := filepath.Join(f.DataDir, "home", userID, "agents", agentName)
	os.MkdirAll(filepath.Join(agentDir, "memory"), 0755)
	os.MkdirAll(filepath.Join(agentDir, "sessions"), 0755)
	os.MkdirAll(filepath.Join(agentDir, "nodes"), 0755)
	return Agent{Name: agentName, AgentDir: agentDir}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	writeFile(t, path, string(data))
}

func appendJSONL(t *testing.T, path string, v any) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	data, _ := json.Marshal(v)
	f.Write(append(data, '\n'))
}
