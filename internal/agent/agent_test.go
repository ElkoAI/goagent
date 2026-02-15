package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goagent/internal/llm"
)

func testDataDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestLoadUserProfile_Missing(t *testing.T) {
	dataDir := testDataDir(t)

	profile, err := LoadUserProfile(dataDir, "nobody")
	if err != nil {
		t.Fatalf("LoadUserProfile() error: %v", err)
	}
	if profile != "" {
		t.Errorf("expected empty profile for missing USER.md, got: %q", profile)
	}
}

func TestLoadUserProfile_Exists(t *testing.T) {
	dataDir := testDataDir(t)
	userDir := filepath.Join(dataDir, "home", "john")
	os.MkdirAll(userDir, 0755)
	os.WriteFile(filepath.Join(userDir, "USER.md"), []byte("I use Ubuntu 24.04"), 0644)

	profile, err := LoadUserProfile(dataDir, "john")
	if err != nil {
		t.Fatalf("LoadUserProfile() error: %v", err)
	}
	if !strings.Contains(profile, "Ubuntu") {
		t.Errorf("profile should contain 'Ubuntu', got: %q", profile)
	}
}

func TestLoadSoul_Missing(t *testing.T) {
	dataDir := testDataDir(t)

	soul, err := LoadSoul(dataDir, "john", "advisor")
	if err != nil {
		t.Fatalf("LoadSoul() error: %v", err)
	}
	if soul != "" {
		t.Errorf("expected empty soul for missing SOUL.md, got: %q", soul)
	}
}

func TestLoadSoul_Exists(t *testing.T) {
	dataDir := testDataDir(t)
	agentDir := filepath.Join(dataDir, "home", "john", "agents", "advisor")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "SOUL.md"), []byte("Be concise and direct."), 0644)

	soul, err := LoadSoul(dataDir, "john", "advisor")
	if err != nil {
		t.Fatalf("LoadSoul() error: %v", err)
	}
	if !strings.Contains(soul, "concise") {
		t.Errorf("soul should contain 'concise', got: %q", soul)
	}
}

func TestEnsureUserHome(t *testing.T) {
	dataDir := testDataDir(t)

	if err := EnsureUserHome(dataDir, "alice"); err != nil {
		t.Fatalf("EnsureUserHome() error: %v", err)
	}

	// USER.md should exist.
	profilePath := filepath.Join(dataDir, "home", "alice", "USER.md")
	if _, err := os.Stat(profilePath); err != nil {
		t.Fatalf("USER.md not created: %v", err)
	}

	// Calling again should be a no-op.
	if err := EnsureUserHome(dataDir, "alice"); err != nil {
		t.Fatalf("EnsureUserHome() second call error: %v", err)
	}
}

func TestLoadSystemPrompt(t *testing.T) {
	dataDir := testDataDir(t)
	agentDir := filepath.Join(dataDir, "home", "john", "agents", "coder")
	os.MkdirAll(agentDir, 0755)

	content := "# Intent\nYou are a coding assistant.\n\n# Status\nReady."
	os.WriteFile(filepath.Join(agentDir, "CODER.md"), []byte(content), 0644)

	prompt, err := LoadSystemPrompt(dataDir, "john", "coder")
	if err != nil {
		t.Fatalf("LoadSystemPrompt() error: %v", err)
	}
	if !strings.Contains(prompt, "coding assistant") {
		t.Errorf("prompt should contain intent, got: %q", prompt)
	}
	if strings.Contains(prompt, "Status") {
		t.Errorf("prompt should NOT contain Status section, got: %q", prompt)
	}
}

func TestHistoryRoundTrip(t *testing.T) {
	dataDir := testDataDir(t)
	agentDir := filepath.Join(dataDir, "home", "john", "agents", "test", "memory")
	os.MkdirAll(agentDir, 0755)

	msgs := []llm.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	if err := AppendHistory(dataDir, "john", "test", msgs...); err != nil {
		t.Fatalf("AppendHistory() error: %v", err)
	}

	loaded, err := LoadHistory(dataDir, "john", "test")
	if err != nil {
		t.Fatalf("LoadHistory() error: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d messages, want 2", len(loaded))
	}
	if loaded[0].Content != "Hello" {
		t.Errorf("first message = %q, want 'Hello'", loaded[0].Content)
	}
}

func TestLoadHistory_Missing(t *testing.T) {
	dataDir := testDataDir(t)

	history, err := LoadHistory(dataDir, "john", "nonexistent")
	if err != nil {
		t.Fatalf("LoadHistory() error: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history for missing agent, got %d", len(history))
	}
}
