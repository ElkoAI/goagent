package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"goagent/internal/config"
	"goagent/internal/reporting"
)

func main() {
	config.LoadDotEnv(".env")

	// Subcommand: only "daily" is supported for now.
	if len(os.Args) < 2 || os.Args[1] != "daily" {
		fmt.Fprintf(os.Stderr, "Usage: goagent-report daily [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Generates a daily market report with OHLCV data and optional LLM narrative.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fmt.Fprintf(os.Stderr, "  --user    User ID (default: \"default\")\n")
		fmt.Fprintf(os.Stderr, "  --date    Report date YYYY-MM-DD (default: today)\n")
		fmt.Fprintf(os.Stderr, "  --data    Data directory (default: ~/.goagent)\n")
		fmt.Fprintf(os.Stderr, "  --out     Output directory override\n")
		fmt.Fprintf(os.Stderr, "  --no-llm  Skip LLM narrative generation\n")
		os.Exit(1)
	}

	// Parse flags after the "daily" subcommand.
	fs := flag.NewFlagSet("daily", flag.ExitOnError)
	user := fs.String("user", "default", "user ID")
	date := fs.String("date", time.Now().Format("2006-01-02"), "report date (YYYY-MM-DD)")
	dataDir := fs.String("data", "~/.goagent", "data directory")
	outDir := fs.String("out", "", "output directory override")
	noLLM := fs.Bool("no-llm", false, "skip LLM narrative generation")
	fs.Parse(os.Args[2:])

	// Expand home directory.
	if *dataDir == "~/.goagent" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Cannot determine home directory: %v", err)
		}
		*dataDir = filepath.Join(home, ".goagent")
	}

	log.Printf("Generating daily report for user=%s date=%s", *user, *date)

	opts := reporting.Options{
		DataDir: *dataDir,
		UserID:  *user,
		Date:    *date,
		OutDir:  *outDir,
		NoLLM:   *noLLM,
	}

	if err := reporting.GenerateDaily(opts); err != nil {
		log.Fatalf("Report generation failed: %v", err)
	}
}
