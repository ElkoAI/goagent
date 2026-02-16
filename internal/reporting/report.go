package reporting

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"goagent/internal/llm"
	"goagent/internal/persistence"
)

// topMoversCount is the number of symbols in the top gainers/losers lists.
const topMoversCount = 5

// SectionReport holds computed data and optional narrative for a config section.
type SectionReport struct {
	Name      string       `json:"name"`
	Data      []SymbolData `json:"data"`
	Narrative string       `json:"narrative,omitempty"`
}

// DailyReport is the final assembled report ready for rendering.
type DailyReport struct {
	Date         string          `json:"date"`
	Timezone     string          `json:"timezone"`
	Generated    string          `json:"generated"`
	TopGainers1D []SymbolData    `json:"top_gainers_1d"`
	TopLosers1D  []SymbolData    `json:"top_losers_1d"`
	TopGainers5D []SymbolData    `json:"top_gainers_5d"`
	TopLosers5D  []SymbolData    `json:"top_losers_5d"`
	Sections     []SectionReport `json:"sections"`
	MacroView    string          `json:"macro_view,omitempty"`
}

// Options configures a daily report generation run.
type Options struct {
	DataDir string // base data directory (e.g. ~/.goagent)
	UserID  string // user home directory name
	Date    string // YYYY-MM-DD (defaults to today)
	OutDir  string // override output directory (empty = auto)
	NoLLM   bool   // skip narrative generation
}

// GenerateDaily is the main orchestrator: fetch → compute → narrate → render → write.
func GenerateDaily(opts Options) error {
	if opts.Date == "" {
		opts.Date = time.Now().Format("2006-01-02")
	}

	// Determine output directory.
	outDir := opts.OutDir
	if outDir == "" {
		outDir = filepath.Join(opts.DataDir, "home", opts.UserID, "reports", "daily", opts.Date)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Load config.
	cfg, err := LoadConfig(opts.DataDir, opts.UserID)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log.Printf("Fetching data for %d sections (%d symbols)...",
		len(cfg.Sections), countSymbols(cfg.Sections))

	// Fetch OHLCV data for all symbols.
	rawData := FetchAllSymbols(cfg.Sections)

	// Compute returns for each symbol.
	computed := make(map[string]SymbolData)
	var allSymbols []SymbolData
	for sym, raw := range rawData {
		sd := ComputeReturns(raw)
		computed[sym] = sd
		allSymbols = append(allSymbols, sd)
	}

	// Build section reports.
	getter1D := func(sd SymbolData) float64 { return sd.Change1D }

	report := &DailyReport{
		Date:         opts.Date,
		Timezone:     time.Now().Location().String(),
		Generated:    time.Now().UTC().Format(time.RFC3339),
		TopGainers1D: TopGainers(allSymbols, getter1D, topMoversCount),
		TopLosers1D:  TopLosers(allSymbols, getter1D, topMoversCount),
		TopGainers5D: TopGainers(allSymbols, func(sd SymbolData) float64 { return sd.Change5D }, topMoversCount),
		TopLosers5D:  TopLosers(allSymbols, func(sd SymbolData) float64 { return sd.Change5D }, topMoversCount),
	}

	// Optional LLM client for narratives.
	var client *llm.Client
	if !opts.NoLLM {
		client = llm.NewClient()
	}

	for _, sec := range cfg.Sections {
		sr := SectionReport{Name: sec.Name}
		for _, sym := range sec.Symbols {
			if sd, ok := computed[sym]; ok {
				sr.Data = append(sr.Data, sd)
			} else {
				sr.Data = append(sr.Data, SymbolData{Symbol: sym, Error: "not fetched"})
			}
		}

		// Generate per-section narrative.
		if client != nil {
			log.Printf("Generating narrative for %s...", sec.Name)
			sr.Narrative = GenerateSectionNarrative(client, sec.Name, sr.Data)
		}

		report.Sections = append(report.Sections, sr)
	}

	// Generate macro overview.
	if client != nil {
		log.Println("Generating macro overview...")
		report.MacroView = GenerateMacroView(client, report)
	}

	// Write outputs.
	log.Printf("Writing report to %s", outDir)

	// 1. data.json — raw structured data.
	dataJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal data.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "data.json"), dataJSON, 0644); err != nil {
		return fmt.Errorf("write data.json: %w", err)
	}

	// 2. market_report.md
	md := RenderMarkdown(report)
	if err := os.WriteFile(filepath.Join(outDir, "market_report.md"), []byte(md), 0644); err != nil {
		return fmt.Errorf("write markdown: %w", err)
	}

	// 3. market_report.html
	html, err := RenderHTML(report)
	if err != nil {
		return fmt.Errorf("render HTML: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "market_report.html"), []byte(html), 0644); err != nil {
		return fmt.Errorf("write HTML: %w", err)
	}

	// Git commit (best-effort).
	store := persistence.New(opts.DataDir)
	go store.AutoCommit(fmt.Sprintf("daily report %s user:%s", opts.Date, opts.UserID))

	log.Printf("Done. Report written to %s", outDir)
	return nil
}

func countSymbols(sections []Section) int {
	seen := make(map[string]bool)
	for _, sec := range sections {
		for _, sym := range sec.Symbols {
			seen[sym] = true
		}
	}
	return len(seen)
}
