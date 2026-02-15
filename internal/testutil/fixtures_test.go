package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goagent/internal/agent"
	"goagent/internal/auth"
	"goagent/internal/memory"
)

func TestFixtureCreation(t *testing.T) {
	fix := NewFixture(t)

	// Verify directory structure exists.
	paths := []string{
		filepath.Join(fix.DataDir, "users.json"),
		filepath.Join(fix.UserHome("john"), "USER.md"),
		filepath.Join(fix.UserHome("alice"), "USER.md"),
		filepath.Join(fix.AgentDir("john", "advisor"), "ADVISOR.md"),
		filepath.Join(fix.AgentDir("john", "advisor"), "SOUL.md"),
		filepath.Join(fix.AgentDir("john", "coder"), "CODER.md"),
		filepath.Join(fix.AgentDir("alice", "researcher"), "RESEARCHER.md"),
		fix.MemoriesPath("john", "advisor"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist: %v", p, err)
		}
	}
}

func TestFixtureAuth(t *testing.T) {
	fix := NewFixture(t)
	mgr := auth.New(fix.DataDir)

	// Valid tokens.
	uid, ok := mgr.Identify("tok-john")
	if !ok || uid != "john" {
		t.Errorf("Identify(tok-john) = (%q, %v), want (john, true)", uid, ok)
	}

	uid, ok = mgr.Identify("tok-alice")
	if !ok || uid != "alice" {
		t.Errorf("Identify(tok-alice) = (%q, %v), want (alice, true)", uid, ok)
	}

	// Invalid token.
	_, ok = mgr.Identify("tok-nobody")
	if ok {
		t.Error("Identify(tok-nobody) should return false")
	}
}

func TestFixtureUserProfile(t *testing.T) {
	fix := NewFixture(t)

	profile, err := agent.LoadUserProfile(fix.DataDir, "john")
	if err != nil {
		t.Fatalf("LoadUserProfile() error: %v", err)
	}
	if !strings.Contains(profile, "Ubuntu 24.04") {
		t.Error("john's profile should mention Ubuntu 24.04")
	}
	if !strings.Contains(profile, "minimal dependencies") {
		t.Error("john's profile should mention minimal dependencies")
	}

	profile, err = agent.LoadUserProfile(fix.DataDir, "alice")
	if err != nil {
		t.Fatalf("LoadUserProfile() error: %v", err)
	}
	if !strings.Contains(profile, "macOS") {
		t.Error("alice's profile should mention macOS")
	}
}

func TestFixtureSoul(t *testing.T) {
	fix := NewFixture(t)

	soul, err := agent.LoadSoul(fix.DataDir, "john", "advisor")
	if err != nil {
		t.Fatalf("LoadSoul() error: %v", err)
	}
	if !strings.Contains(soul, "data-driven") {
		t.Error("advisor soul should mention data-driven")
	}

	soul, err = agent.LoadSoul(fix.DataDir, "john", "coder")
	if err != nil {
		t.Fatalf("LoadSoul() error: %v", err)
	}
	if !strings.Contains(soul, "idiomatic Go") {
		t.Error("coder soul should mention idiomatic Go")
	}
}

func TestFixtureSystemPrompt(t *testing.T) {
	fix := NewFixture(t)

	prompt, err := agent.LoadSystemPrompt(fix.DataDir, "john", "advisor")
	if err != nil {
		t.Fatalf("LoadSystemPrompt() error: %v", err)
	}
	if !strings.Contains(prompt, "financial advisor") {
		t.Error("advisor prompt should contain 'financial advisor'")
	}
}

func TestFixtureMemories(t *testing.T) {
	fix := NewFixture(t)

	store := &memory.KeywordStore{}
	// Use the exported path via MemoriesPath.
	store.SetPath(fix.MemoriesPath("john", "advisor"))
	if err := store.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if store.Count() != 3 {
		t.Fatalf("expected 3 advisor memories, got %d", store.Count())
	}

	results, err := store.Search("index funds investment", 5)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results for 'index funds'")
	}
	if !strings.Contains(results[0].Text, "index funds") {
		t.Errorf("top result should be about index funds, got: %q", results[0].Text)
	}
}

func TestFixtureUserIsolation(t *testing.T) {
	fix := NewFixture(t)

	// John has an advisor, Alice does not.
	_, err := agent.LoadSystemPrompt(fix.DataDir, "john", "advisor")
	if err != nil {
		t.Fatalf("john/advisor should exist: %v", err)
	}

	_, err = agent.LoadSystemPrompt(fix.DataDir, "alice", "advisor")
	if err == nil {
		t.Error("alice/advisor should NOT exist")
	}

	// Alice has a researcher, John does not.
	_, err = agent.LoadSystemPrompt(fix.DataDir, "alice", "researcher")
	if err != nil {
		t.Fatalf("alice/researcher should exist: %v", err)
	}

	_, err = agent.LoadSystemPrompt(fix.DataDir, "john", "researcher")
	if err == nil {
		t.Error("john/researcher should NOT exist")
	}
}
