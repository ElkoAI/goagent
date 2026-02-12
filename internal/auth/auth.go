package auth

// Manager handles authentication logic for the gateway.
type Manager struct {
	token string
}

// New creates a new authentication manager with a given secret token.
func New(token string) *Manager {
	return &Manager{token: token}
}

// Validate checks if the provided token matches the expected gateway token.
// If the gateway was started without a token, validation always passes. [cite: 31, 165]
func (m *Manager) Validate(t string) bool {
	if m.token == "" {
		return true
	}
	return t == m.token
}
