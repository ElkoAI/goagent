package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"goagent/internal/agent"
	"goagent/internal/auth"
	"goagent/internal/llm"
	"goagent/internal/persistence"
	"goagent/internal/protocol"
	"goagent/internal/session"
	"goagent/internal/tools"

	"nhooyr.io/websocket"
)

type Server struct {
	addr       string
	token      string
	dataDir    string
	auth       *auth.Manager
	store      *persistence.Store
	llm        *llm.Client
	sessions   sync.Map
	httpServer *http.Server
}

func New(addr, token, dataDir string) *Server {
	// Standard Linux home directory expansion for the data store
	if dataDir == "~/.goagent" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".goagent")
	}

	store := persistence.New(dataDir)
	return &Server{
		addr:    addr,
		token:   token,
		dataDir: store.DataDir,
		auth:    auth.New(token),
		store:   store,
		llm:     llm.NewClient(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	s.httpServer = &http.Server{Addr: s.addr, Handler: mux}

	go func() {
		<-ctx.Done()
		s.httpServer.Shutdown(context.Background())
	}()

	return s.httpServer.ListenAndServe()
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if !s.auth.Validate(token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		OriginPatterns:     []string{"*"},
	})
	if err != nil {
		return
	}

	sess := session.New(ws, s.store, s.dataDir)
	s.sessions.Store(sess.ID(), sess)

	// Start tracking this session in the git-friendly persistence layer
	go s.store.AutoCommit("session:started:" + sess.ID())
	go s.handleMessages(sess)
}

func (s *Server) handleMessages(sess *session.Session) {
	defer func() {
		s.sessions.Delete(sess.ID())
		sess.Close()
	}()

	for {
		_, data, err := sess.Read()
		if err != nil {
			return
		}

		req, err := protocol.ParseRequest(data)
		if err != nil {
			sess.WriteError(nil, -32600, "Parse error", nil)
			continue
		}

		s.dispatch(sess, req)
	}
}

func (s *Server) dispatch(sess *session.Session, req *protocol.Request) {
	switch req.Method {
	case "ping":
		sess.WriteResponse(req.ID, "pong", nil)

	case "tool.web_search":
		result, err := tools.WebSearch(req.Params)
		if err != nil {
			sess.WriteError(req.ID, -32000, err.Error(), nil)
		} else {
			sess.WriteResponse(req.ID, result, nil)
			go s.store.AutoCommit("tool:web_search") //
		}

	case "agent.create":
		var params struct {
			Name   string `json:"name"`
			Prompt string `json:"prompt"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sess.WriteError(req.ID, -32602, "Invalid params", nil)
			return
		}

		// Logic to build the agent's Linux folder structure
		err := s.CreateAgentFileSystem(params.Name, params.Prompt)
		if err != nil {
			sess.WriteError(req.ID, -32000, "FS error: "+err.Error(), nil)
		} else {
			sess.WriteResponse(req.ID, map[string]string{"status": "agent_born"}, nil)
			go s.store.AutoCommit("agent:created:" + params.Name) //
		}

	case "agent.chat":
		var params struct {
			Agent   string `json:"agent"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sess.WriteError(req.ID, -32602, "Invalid params", nil)
			return
		}

		systemPrompt, err := agent.LoadSystemPrompt(s.dataDir, params.Agent)
		if err != nil {
			sess.WriteError(req.ID, -32000, "Load agent: "+err.Error(), nil)
			return
		}

		history, err := agent.LoadHistory(s.dataDir, params.Agent)
		if err != nil {
			sess.WriteError(req.ID, -32000, "Load history: "+err.Error(), nil)
			return
		}

		userMsg := llm.Message{Role: "user", Content: params.Message}
		history = append(history, userMsg)

		response, err := s.llm.Chat(systemPrompt, history)
		if err != nil {
			sess.WriteError(req.ID, -32000, "LLM error: "+err.Error(), nil)
			return
		}

		assistantMsg := llm.Message{Role: "assistant", Content: response}

		if err := agent.AppendHistory(s.dataDir, params.Agent, userMsg, assistantMsg); err != nil {
			sess.WriteError(req.ID, -32000, "Save history: "+err.Error(), nil)
			return
		}

		sess.WriteResponse(req.ID, map[string]string{"response": response}, nil)
		go s.store.AutoCommit("agent:chat:" + params.Agent)

	default:
		sess.WriteError(req.ID, -32601, "Method not found", nil)
	}
}

// CreateAgentFileSystem implements the OpenClaw-inspired git-friendly layout
func (s *Server) CreateAgentFileSystem(name string, prompt string) error {
	agentDir := filepath.Join(s.dataDir, "agents", name)
	dirs := []string{
		filepath.Join(agentDir, "memory"),
		filepath.Join(agentDir, "sessions"),
		filepath.Join(agentDir, "nodes"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	mdName := strings.ToUpper(name) + ".md"
	defPath := filepath.Join(agentDir, mdName)
	content := "# Intent\n" + prompt + "\n\n# Status\nInitialized via GoAgent."
	return os.WriteFile(defPath, []byte(content), 0644)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
