package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goagent/internal/config"
	"goagent/internal/server"
)

func main() {
	config.LoadDotEnv(".env")

	// Defaults match the OpenClaw architecture for easier interop[cite: 2].
	addr := flag.String("addr", "127.0.0.1:18789", "listen address (default: loopback only)")
	token := flag.String("token", os.Getenv("GOAGENT_TOKEN"), "auth token (or set GOAGENT_TOKEN)")
	data := flag.String("data", "~/.goagent", "data directory for git-friendly storage")
	flag.Parse()

	if *token == "" {
		log.Println("⚠️  Warning: No token provided. Authentication is disabled.")
	}

	// Initialize the server with Linux-first defaults.
	gw := server.New(*addr, *token, *data)

	// Context for managing the server lifecycle.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the gateway in a goroutine.
	go func() {
		log.Printf("🚀 GoAgent Gateway listening on ws://%s", *addr)
		if err := gw.Start(ctx); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown handling for Linux (SIGINT/SIGTERM)[cite: 25].
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down gracefully...")

	// Allow up to 10 seconds for active sessions to close.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := gw.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}

	log.Println("👋 Gateway stopped.")
}
