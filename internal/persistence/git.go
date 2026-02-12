package persistence

import (
	"log"
	"os/exec"
	"time"
)

// AutoCommit watches the data directory and commits changes.
// In a Linux environment, this is best handled with a simple debounce loop.
func (s *Store) AutoCommit(message string) {
	// Debounce to avoid spamming commits during rapid file writes
	time.Sleep(2 * time.Second)

	log.Printf("💾 Git: Committing state changes to %s...", s.DataDir)

	// Stage all changes (sessions.json, .jsonl, .md files)
	addCmd := exec.Command("git", "-C", s.DataDir, "add", ".")
	if err := addCmd.Run(); err != nil {
		log.Printf("❌ Git add failed: %v", err)
		return
	}

	// Commit with a timestamped message
	commitMsg := message + " " + time.Now().Format(time.RFC3339)
	commitCmd := exec.Command("git", "-C", s.DataDir, "commit", "-m", commitMsg)

	if err := commitCmd.Run(); err != nil {
		// It's okay if there's nothing to commit
		return
	}

	log.Println("✅ Git: State preserved.")
}
