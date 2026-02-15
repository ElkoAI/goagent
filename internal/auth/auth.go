package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const defaultUser = "default"

// Manager handles multi-user authentication via token→userID mapping.
type Manager struct {
	tokens map[string]string // token → userID
	noAuth bool              // true when no tokens configured — all connections map to defaultUser
}

// New loads user tokens from dataDir/users.json.
// If the file is missing or empty, all connections map to the "default" user.
func New(dataDir string) *Manager {
	m := &Manager{
		tokens: make(map[string]string),
	}

	data, err := os.ReadFile(filepath.Join(dataDir, "users.json"))
	if err != nil {
		m.noAuth = true
		return m
	}

	var registry struct {
		Users map[string]struct {
			ID string `json:"id"`
		} `json:"users"`
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		m.noAuth = true
		return m
	}

	for token, user := range registry.Users {
		if id := sanitizeUserID(user.ID); id != "" {
			m.tokens[token] = id
		}
	}

	if len(m.tokens) == 0 {
		m.noAuth = true
	}
	return m
}

// Identify returns the userID for the given token.
// When no auth is configured, all connections map to "default".
func (m *Manager) Identify(token string) (userID string, ok bool) {
	if m.noAuth {
		return defaultUser, true
	}
	uid, found := m.tokens[token]
	if !found {
		return "", false
	}
	return uid, true
}

// sanitizeUserID rejects IDs that could cause path traversal.
func sanitizeUserID(id string) string {
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") || id == "" {
		return ""
	}
	return id
}
