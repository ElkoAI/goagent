package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"goagent/internal/protocol"
	"nhooyr.io/websocket"
)

func main() {
	// Flags and environment variables for Linux-first usage.
	addr := flag.String("addr", "ws://127.0.0.1:18789", "Gateway WebSocket address")
	token := flag.String("token", os.Getenv("GOAGENT_TOKEN"), "Auth token (fallback to GOAGENT_TOKEN)")
	flag.Parse()

	if *token == "" {
		log.Println("⚠️  Warning: No token provided. Using unauthenticated connection.")
	}

	ctx := context.Background()
	// Connect to the gateway with the token passed as a query param.
	conn, _, err := websocket.Dial(ctx, *addr+"/ws?token="+*token, nil)
	if err != nil {
		log.Fatalf("❌ Failed to connect to gateway: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	args := flag.Args()
	if len(args) > 0 {
		// Execute a one-shot command from the terminal.
		execute(ctx, conn, args[0], args[1:])
		return
	}

	// Default to interactive REPL mode.
	runInteractive(ctx, conn)
}

func execute(ctx context.Context, conn *websocket.Conn, method string, args []string) {
	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"cli"`),
	}

	// Route CLI shortcuts to Gateway JSON-RPC methods.
	switch method {
	case "ping":
		req.Method = "ping"
	case "search":
		req.Method = "tool.web_search"
		query := strings.Join(args, " ")
		p, _ := json.Marshal(map[string]any{"query": query, "num_results": 10})
		req.Params = p
	case "create":
		req.Method = "agent.create"
		if len(args) < 2 {
			fmt.Println("Usage: create <name> <prompt>")
			return
		}
		// First arg is the agent name, the rest is the prompt.
		name := args[0]
		prompt := strings.Join(args[1:], " ")
		p, _ := json.Marshal(map[string]string{"name": name, "prompt": prompt})
		req.Params = p
	case "chat":
		req.Method = "agent.chat"
		if len(args) < 2 {
			fmt.Println("Usage: chat <agent-name> <message>")
			return
		}
		name := args[0]
		message := strings.Join(args[1:], " ")
		p, _ := json.Marshal(map[string]any{"agent": name, "message": message, "stream": true})
		req.Params = p
	default:
		fmt.Printf("Unknown command: %s (Available: ping, search, create, chat)\n", method)
		return
	}

	data, _ := json.Marshal(req)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		log.Printf("❌ Write error: %v", err)
		return
	}

	if method == "chat" {
		readStream(ctx, conn)
		return
	}

	// Wait for and display the response (non-streaming commands).
	_, respData, err := conn.Read(ctx)
	if err != nil {
		log.Printf("❌ Read error: %v", err)
		return
	}

	var resp protocol.Response
	if err := json.Unmarshal(respData, &resp); err != nil {
		fmt.Println("Raw Response:", string(respData))
	} else if resp.Error != nil {
		fmt.Printf("Error [%d]: %s\n", resp.Error.Code, resp.Error.Message)
	} else {
		pretty, _ := json.MarshalIndent(resp.Result, "", "  ")
		fmt.Println(string(pretty))
	}
}

// readStream reads streaming delta messages until done:true.
func readStream(ctx context.Context, conn *websocket.Conn) {
	for {
		_, respData, err := conn.Read(ctx)
		if err != nil {
			log.Printf("❌ Read error: %v", err)
			return
		}

		var resp protocol.Response
		if err := json.Unmarshal(respData, &resp); err != nil {
			fmt.Println("Raw Response:", string(respData))
			return
		}
		if resp.Error != nil {
			fmt.Printf("Error [%d]: %s\n", resp.Error.Code, resp.Error.Message)
			return
		}

		// Parse the result map to get delta/done fields.
		raw, _ := json.Marshal(resp.Result)
		var result struct {
			Delta    string `json:"delta"`
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			fmt.Println("Raw Response:", string(respData))
			return
		}

		if result.Done {
			fmt.Println()
			return
		}

		fmt.Print(result.Delta)
	}
}

func runInteractive(ctx context.Context, conn *websocket.Conn) {
	fmt.Println("🤖 GoAgent CLI (REPL). Commands: ping, search <q>, create <name> <prompt>, chat <agent> <msg>, exit")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("goagent> ")
		if !scanner.Scan() {
			break
		}
		input := strings.Fields(scanner.Text())
		if len(input) == 0 {
			continue
		}
		if input[0] == "exit" || input[0] == "quit" {
			break
		}
		execute(ctx, conn, input[0], input[1:])
	}
}
