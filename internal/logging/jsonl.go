package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type logEntry struct {
	Ts   string          `json:"ts"`
	Dir  string          `json:"dir"`
	Data json.RawMessage `json:"data"`
}

// SessionLogger appends JSONL entries for a single session.
type SessionLogger struct {
	f  *os.File
	mu sync.Mutex
}

// NewSessionLogger creates or opens a JSONL file at <dataDir>/home/<userID>/sessions/<sessionID>.jsonl.
func NewSessionLogger(dataDir, userID, sessionID string) (*SessionLogger, error) {
	dir := filepath.Join(dataDir, "home", userID, "sessions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, sessionID+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &SessionLogger{f: f}, nil
}

// Log appends a single JSONL line with timestamp, direction, and raw data.
func (l *SessionLogger) Log(dir string, data json.RawMessage) {
	entry := logEntry{
		Ts:   time.Now().UTC().Format(time.RFC3339Nano),
		Dir:  dir,
		Data: data,
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.f, "%s\n", line)
}

// Close closes the underlying file.
func (l *SessionLogger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f.Close()
}
