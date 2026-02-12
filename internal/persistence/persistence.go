package persistence

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	DataDir      string
	sessionsFile string
	mu           sync.Mutex
}

func New(dataDir string) *Store {
	if dataDir == "~/.goagent" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".goagent")
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("⚠️  Warning: Could not create data directory: %v", err)
	}

	return &Store{
		DataDir:      dataDir,
		sessionsFile: filepath.Join(dataDir, "sessions.json"),
	}
}

func (s *Store) AtomicWriteJSON(path string, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

