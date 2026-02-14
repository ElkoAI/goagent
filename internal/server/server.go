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
	"goagent/internal/skills"
	"goagent/internal/tools"

	"nhooyr.io/websocket"
)

// maxToolRounds limits how many tool-call/result round-trips the agent can do
// before the server forces it to produce a text response.
const maxToolRounds = 10

type Server struct {
	addr       string
	token      string
	dataDir    string
	auth       *auth.Manager
	store      *persistence.Store
	llm        *llm.Client
	skills     *skills.Loader
	registry   *tools.Registry
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
	sl := skills.NewLoader(store.DataDir)
	sl.LoadAll()

	return &Server{
		addr:     addr,
		token:    token,
		dataDir:  store.DataDir,
		auth:     auth.New(token),
		store:    store,
		llm:      llm.NewClient(),
		skills:   sl,
		registry: tools.NewRegistry(),
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

	case "tool.ohlcv":
		result, err := tools.OHLCV(req.Params)
		if err != nil {
			sess.WriteError(req.ID, -32000, err.Error(), nil)
		} else {
			sess.WriteResponse(req.ID, result, nil)
			go s.store.AutoCommit("tool:ohlcv")
		}

	case "tool.exec":
		result, err := tools.Exec(req.Params)
		if err != nil {
			sess.WriteError(req.ID, -32000, err.Error(), nil)
		} else {
			sess.WriteResponse(req.ID, result, nil)
			go s.store.AutoCommit("tool:exec")
		}

	case "skill.list":
		type entry struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Ready       bool     `json:"ready"`
			Missing     []string `json:"missing,omitempty"`
		}
		var list []entry
		for _, sk := range s.skills.Skills {
			list = append(list, entry{
				Name:        sk.Name,
				Description: sk.Description,
				Ready:       sk.Ready,
				Missing:     sk.Missing,
			})
		}
		sess.WriteResponse(req.ID, map[string]any{"skills": list}, nil)

	case "skill.refresh":
		if err := s.skills.LoadAll(); err != nil {
			sess.WriteError(req.ID, -32000, "refresh: "+err.Error(), nil)
		} else {
			sess.WriteResponse(req.ID, map[string]any{
				"loaded": len(s.skills.Skills),
				"ready":  len(s.skills.Ready()),
			}, nil)
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
			Stream  bool   `json:"stream"`
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

		// Inject loaded skill instructions into the agent's system prompt.
		if skillPrompt := s.skills.BuildPrompt(); skillPrompt != "" {
			systemPrompt += skillPrompt
		}

		history, err := agent.LoadHistory(s.dataDir, params.Agent)
		if err != nil {
			sess.WriteError(req.ID, -32000, "Load history: "+err.Error(), nil)
			return
		}

		userMsg := llm.Message{Role: "user", Content: params.Message}
		history = append(history, userMsg)

		// Build the full message list with system prompt prepended.
		msgs := make([]llm.Message, 0, len(history)+1)
		msgs = append(msgs, llm.Message{Role: "system", Content: systemPrompt})
		msgs = append(msgs, history...)

		toolDefs := s.registry.ToolDefs()
		var response string

		// Tool-calling loop: call LLM → execute tool calls → feed results → repeat.
		for round := 0; round < maxToolRounds; round++ {
			result, err := s.llm.ChatWithTools(msgs, toolDefs)
			if err != nil {
				sess.WriteError(req.ID, -32000, "LLM error: "+err.Error(), nil)
				return
			}

			// If the LLM returned a text response (no tool calls), we're done.
			if len(result.ToolCalls) == 0 {
				response = result.Content
				break
			}

			// Notify the client that tools are being called.
			for _, tc := range result.ToolCalls {
				sess.Write(protocol.Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]any{
						"tool_call": tc.Function.Name,
						"arguments": tc.Function.Arguments,
						"done":      false,
					},
				})
			}

			// Append the assistant's tool-call message to the conversation.
			assistantToolMsg := llm.Message{
				Role:      "assistant",
				ToolCalls: result.ToolCalls,
			}
			msgs = append(msgs, assistantToolMsg)

			// Execute each tool call and feed results back.
			for _, tc := range result.ToolCalls {
				toolResult, err := s.registry.Call(tc.Function.Name, tc.Function.Arguments)
				if err != nil {
					toolResult = `{"error":"` + err.Error() + `"}`
				}

				msgs = append(msgs, llm.Message{
					Role:       "tool",
					Content:    toolResult,
					ToolCallID: tc.ID,
				})
			}
		}

		// If streaming was requested, re-send the final response as a stream
		// (the tool-calling rounds were non-streaming for speed).
		assistantMsg := llm.Message{Role: "assistant", Content: response}

		if err := agent.AppendHistory(s.dataDir, params.Agent, userMsg, assistantMsg); err != nil {
			sess.WriteError(req.ID, -32000, "Save history: "+err.Error(), nil)
			return
		}

		if params.Stream {
			sess.Write(protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"delta": response,
					"done":  false,
				},
			})
			sess.Write(protocol.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"response": response,
					"done":     true,
				},
			})
		} else {
			sess.WriteResponse(req.ID, map[string]string{"response": response}, nil)
		}
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
