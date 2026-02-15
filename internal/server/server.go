package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"goagent/internal/agent"
	"goagent/internal/auth"
	"goagent/internal/llm"
	"goagent/internal/memory"
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

// memoryTopK is the number of memories to retrieve per agent.chat call.
const memoryTopK = 5

// memoryMinScore is the minimum cosine similarity for a memory to be included.
const memoryMinScore = 0.4

// maxHistoryMessages caps the number of chat messages sent to the LLM.
// Full history stays in chat.jsonl — this only limits the context window.
const maxHistoryMessages = 20

type Server struct {
	addr       string
	dataDir    string
	auth       *auth.Manager
	store      *persistence.Store
	llm        *llm.Client
	embed      *llm.EmbedClient
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
		dataDir:  store.DataDir,
		auth:     auth.New(store.DataDir),
		store:    store,
		llm:      llm.NewClient(),
		embed:    llm.NewEmbedClient(),
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
	userID, ok := s.auth.Identify(token)
	if !ok {
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

	// Ensure the user's home directory and starter USER.md exist.
	agent.EnsureUserHome(s.dataDir, userID)

	sess := session.New(ws, s.store, s.dataDir, userID)
	s.sessions.Store(sess.ID(), sess)

	go s.store.AutoCommit(fmt.Sprintf("session:started:%s:user:%s", sess.ID(), userID))
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
			go s.store.AutoCommit("tool:web_search")
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

		err := s.CreateAgentFileSystem(sess.UserID(), params.Name, params.Prompt)
		if err != nil {
			sess.WriteError(req.ID, -32000, "FS error: "+err.Error(), nil)
		} else {
			sess.WriteResponse(req.ID, map[string]string{"status": "agent_born"}, nil)
			go s.store.AutoCommit(fmt.Sprintf("agent:created:%s:user:%s", params.Name, sess.UserID()))
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

		userID := sess.UserID()

		// Assemble system prompt: USER.md → SOUL.md → AGENT.md intent → skills → memories
		var systemPrompt string

		// 1. Global user profile (shared by all agents).
		if userProfile, err := agent.LoadUserProfile(s.dataDir, userID); err == nil && userProfile != "" {
			systemPrompt += userProfile + "\n\n"
		}

		// 2. Agent personality (SOUL.md).
		if soul, err := agent.LoadSoul(s.dataDir, userID, params.Agent); err == nil && soul != "" {
			systemPrompt += soul + "\n\n"
		}

		// 3. Agent intent (AGENT.md # Intent section).
		intentPrompt, err := agent.LoadSystemPrompt(s.dataDir, userID, params.Agent)
		if err != nil {
			sess.WriteError(req.ID, -32000, "Load agent: "+err.Error(), nil)
			return
		}
		systemPrompt += intentPrompt

		// 4. Skill instructions.
		if skillPrompt := s.skills.BuildPrompt(); skillPrompt != "" {
			systemPrompt += skillPrompt
		}

		// 5. Retrieve relevant memories and inject into system prompt.
		memProvider := memory.NewProvider(s.dataDir, userID, params.Agent, s.embed)
		memProvider.Load()
		if memProvider.Count() > 0 {
			if results, err := memProvider.Search(params.Message, memoryTopK); err == nil {
				if memCtx := memory.BuildContext(results, memoryMinScore); memCtx != "" {
					systemPrompt += memCtx
				}
			}
		}

		// 6. Chat history.
		history, err := agent.LoadHistory(s.dataDir, userID, params.Agent)
		if err != nil {
			sess.WriteError(req.ID, -32000, "Load history: "+err.Error(), nil)
			return
		}

		// Sliding window: only send the last N messages to the LLM.
		if len(history) > maxHistoryMessages {
			history = history[len(history)-maxHistoryMessages:]
		}

		userMsg := llm.Message{Role: "user", Content: params.Message}
		history = append(history, userMsg)

		// Build the full message list with system prompt prepended.
		msgs := make([]llm.Message, 0, len(history)+1)
		msgs = append(msgs, llm.Message{Role: "system", Content: systemPrompt})
		msgs = append(msgs, history...)

		// Include both registry tools and memory tools in the LLM call.
		toolDefs := append(s.registry.ToolDefs(), tools.MemoryToolDefs...)
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
				var toolResult string
				var toolErr error

				switch tc.Function.Name {
				case "memory_save":
					var res any
					res, toolErr = tools.MemorySave(memProvider, json.RawMessage(tc.Function.Arguments))
					if toolErr == nil {
						out, _ := json.Marshal(res)
						toolResult = string(out)
					}
				case "memory_recall":
					var res any
					res, toolErr = tools.MemoryRecall(memProvider, json.RawMessage(tc.Function.Arguments))
					if toolErr == nil {
						out, _ := json.Marshal(res)
						toolResult = string(out)
					}
				default:
					toolResult, toolErr = s.registry.Call(tc.Function.Name, tc.Function.Arguments)
				}

				if toolErr != nil {
					toolResult = `{"error":"` + toolErr.Error() + `"}`
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

		if err := agent.AppendHistory(s.dataDir, userID, params.Agent, userMsg, assistantMsg); err != nil {
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
		// Background: extract facts from this exchange and save to memory.
		go func() {
			memory.ExtractAndSave(s.llm, memProvider, params.Message, response)
			s.store.AutoCommit(fmt.Sprintf("agent:chat:%s:user:%s", params.Agent, userID))
		}()

	default:
		sess.WriteError(req.ID, -32601, "Method not found", nil)
	}
}

// CreateAgentFileSystem creates the git-friendly agent layout under a user's home.
func (s *Server) CreateAgentFileSystem(userID, name string, prompt string) error {
	agentDir := filepath.Join(s.dataDir, "home", userID, "agents", name)
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

	// AGENT.md — intent and status.
	mdName := strings.ToUpper(name) + ".md"
	defPath := filepath.Join(agentDir, mdName)
	content := "# Intent\n" + prompt + "\n\n# Status\nInitialized via GoAgent."
	if err := os.WriteFile(defPath, []byte(content), 0644); err != nil {
		return err
	}

	// SOUL.md — agent personality template.
	soulPath := filepath.Join(agentDir, "SOUL.md")
	soulContent := "# Soul\n\n" +
		"<!-- Define this agent's personality, tone, and behavioral guidelines. -->\n"
	return os.WriteFile(soulPath, []byte(soulContent), 0644)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
