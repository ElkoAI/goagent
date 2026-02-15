package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"goagent/internal/logging"
	"goagent/internal/persistence"
	"goagent/internal/protocol"

	"nhooyr.io/websocket"
)

// Session represents a single connected client (CLI, bot, or dashboard).
type Session struct {
	id     string
	userID string
	ws     *websocket.Conn
	store  *persistence.Store
	logger *logging.SessionLogger
	closed atomic.Bool
}

// New initializes a new session with a unique ID, store reference, and JSONL logger.
func New(ws *websocket.Conn, store *persistence.Store, dataDir, userID string) *Session {
	id := fmt.Sprintf("sess-%d", time.Now().UnixNano())
	var logger *logging.SessionLogger
	if dataDir != "" {
		l, err := logging.NewSessionLogger(dataDir, userID, id)
		if err != nil {
			log.Printf("session logger init failed: %v", err)
		} else {
			logger = l
		}
	}
	return &Session{
		id:     id,
		userID: userID,
		ws:     ws,
		store:  store,
		logger: logger,
	}
}

// ID returns the unique session identifier.
func (s *Session) ID() string { return s.id }

// UserID returns the authenticated user who owns this session.
func (s *Session) UserID() string { return s.userID }

// Read listens for the next message from the WebSocket.
func (s *Session) Read() (websocket.MessageType, []byte, error) {
	typ, data, err := s.ws.Read(context.Background())
	if err == nil && s.logger != nil {
		s.logger.Log("in", json.RawMessage(data))
	}
	return typ, data, err
}

// Write sends a JSON-marshaled object to the client.
func (s *Session) Write(v any) error {
	if s.closed.Load() {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if s.logger != nil {
		s.logger.Log("out", json.RawMessage(data))
	}
	return s.ws.Write(context.Background(), websocket.MessageText, data)
}

// WriteResponse sends a standard JSON-RPC success response.
func (s *Session) WriteResponse(id json.RawMessage, result any, err error) {
	resp := protocol.Response{
		JSONRPC: "2.0",
		ID:      id,
	}
	if err != nil {
		resp.Error = &protocol.Error{Code: -32000, Message: err.Error()}
	} else {
		resp.Result = result
	}
	_ = s.Write(resp)
}

// WriteError sends a standard JSON-RPC error response.
func (s *Session) WriteError(id json.RawMessage, code int, msg string, data any) {
	resp := protocol.Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &protocol.Error{Code: code, Message: msg, Data: data},
	}
	_ = s.Write(resp)
}

// Close gracefully terminates the session logger and WebSocket connection.
func (s *Session) Close() {
	if s.closed.CompareAndSwap(false, true) {
		if s.logger != nil {
			s.logger.Close()
		}
		s.ws.Close(websocket.StatusNormalClosure, "")
	}
}
