package reporting

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goagent/internal/llm"
	"goagent/internal/persistence"
)

// topMoversCount is the number of symbols in the top gainers/losers lists.
const topMoversCount = 5

// SectionReport holds computed data and optional narrative for a config section.
type SectionReport struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"` // what this section tracks
	Summary     string       `json:"summary,omitempty"`     // one-liner with emoji
	Data        []SymbolData `json:"data"`
	Narrative   string       `json:"narrative,omitempty"`
}

// DailyReport is the final assembled report ready for rendering.
type DailyReport struct {
	Title        string          `json:"title"`
	Intro        string          `json:"intro,omitempty"` // why this report exists
	Date         string          `json:"date"`
	Timezone     string          `json:"timezone"`
	Generated    string          `json:"generated"`
	Summary      string          `json:"summary,omitempty"` // market pulse
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

	title := cfg.Title
	if title == "" {
		title = "Daily Market Report"
	}

	report := &DailyReport{
		Title:        title,
		Intro:        cfg.Intro,
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
		sr := SectionReport{
			Name:        sec.Name,
			Description: sec.Description,
		}
		for _, sym := range sec.Symbols {
			if sd, ok := computed[sym]; ok {
				sr.Data = append(sr.Data, sd)
			} else {
				sr.Data = append(sr.Data, SymbolData{Symbol: sym, Error: "not fetched"})
			}
		}

		// Compute section summary line with emoji.
		sr.Summary = SectionSummaryLine(sr.Data)

		// Generate per-section narrative.
		if client != nil {
			log.Printf("Generating narrative for %s...", sec.Name)
			sr.Narrative = GenerateSectionNarrative(client, sec.Name, sr.Data)
		}

		report.Sections = append(report.Sections, sr)
	}

	// Build report-level summary.
	report.Summary = buildReportSummary(report, countSymbols(cfg.Sections))

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

// buildReportSummary creates a compelling top-of-report market pulse.
func buildReportSummary(report *DailyReport, symbolCount int) string {
	// Count breadth: gainers vs losers vs flat.
	var gainers, losers, flat, totalVol int64
	var sumChange float64
	var count int

	for _, sec := range report.Sections {
		for _, sd := range sec.Data {
			if sd.Error != "" {
				continue
			}
			count++
			sumChange += sd.Change1D
			totalVol += sd.Volume
			switch {
			case sd.Change1D > 0.1:
				gainers++
			case sd.Change1D < -0.1:
				losers++
			default:
				flat++
			}
		}
	}

	// Market sentiment based on breadth ratio.
	var sentiment string
	if count > 0 {
		ratio := float64(gainers) / float64(count)
		switch {
		case ratio >= 0.7:
			sentiment = "🟢 Risk-On"
		case ratio >= 0.5:
			sentiment = "⚖️ Mixed"
		case ratio >= 0.3:
			sentiment = "🟡 Cautious"
		default:
			sentiment = "🔴 Risk-Off"
		}
	}

	// Best gainer and worst loser.
	var topGainer, topLoser string
	if len(report.TopGainers1D) > 0 {
		g := report.TopGainers1D[0]
		topGainer = fmt.Sprintf("%s %s (%+.2f%%)", PerformanceEmoji(g.Change1D), g.Symbol, g.Change1D)
	}
	if len(report.TopLosers1D) > 0 {
		l := report.TopLosers1D[0]
		topLoser = fmt.Sprintf("%s %s (%+.2f%%)", PerformanceEmoji(l.Change1D), l.Symbol, l.Change1D)
	}

	// Format volume.
	var volStr string
	switch {
	case totalVol >= 1_000_000_000:
		volStr = fmt.Sprintf("%.1fB", float64(totalVol)/1e9)
	case totalVol >= 1_000_000:
		volStr = fmt.Sprintf("%.0fM", float64(totalVol)/1e6)
	default:
		volStr = fmt.Sprintf("%d", totalVol)
	}

	// Build the pulse line.
	parts := []string{
		fmt.Sprintf("%s Sentiment: %s", "📊", sentiment),
		fmt.Sprintf("Breadth: %d↑ %d↓ %d➖ of %d symbols", gainers, losers, flat, count),
		fmt.Sprintf("Volume: %s shares", volStr),
	}

	summary := strings.Join(parts, " | ")

	if topGainer != "" || topLoser != "" {
		summary += "\n"
		if topGainer != "" {
			summary += fmt.Sprintf("Top gainer: %s", topGainer)
		}
		if topLoser != "" {
			if topGainer != "" {
				summary += " · "
			}
			summary += fmt.Sprintf("Top loser: %s", topLoser)
		}
	}

	return summary
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
