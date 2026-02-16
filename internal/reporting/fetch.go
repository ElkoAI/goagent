package reporting

import (
	"log"
	"sync"

	"goagent/internal/tools"
)

// fetchPoolSize limits concurrent Yahoo Finance requests.
const fetchPoolSize = 5

// SymbolRawData holds the raw OHLCV bars fetched for a single symbol.
type SymbolRawData struct {
	Symbol   string
	Currency string
	Bars5D   []tools.OHLCVBar
	Bars1M   []tools.OHLCVBar
	BarsYTD  []tools.OHLCVBar
	Error    string
}

// fetchJob is a work item for the parallel fetch pool.
type fetchJob struct {
	symbol string
}

// FetchAllSymbols fetches OHLCV data for every symbol across all sections.
// Uses a bounded goroutine pool to avoid hammering Yahoo Finance.
func FetchAllSymbols(sections []Section) map[string]*SymbolRawData {
	// Deduplicate symbols across sections.
	seen := make(map[string]bool)
	var symbols []string
	for _, sec := range sections {
		for _, sym := range sec.Symbols {
			if !seen[sym] {
				seen[sym] = true
				symbols = append(symbols, sym)
			}
		}
	}

	results := make(map[string]*SymbolRawData)
	var mu sync.Mutex

	jobs := make(chan fetchJob, len(symbols))
	var wg sync.WaitGroup

	// Start worker pool.
	for i := 0; i < fetchPoolSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				raw := fetchSymbol(job.symbol)
				mu.Lock()
				results[job.symbol] = raw
				mu.Unlock()
			}
		}()
	}

	// Enqueue jobs.
	for _, sym := range symbols {
		jobs <- fetchJob{symbol: sym}
	}
	close(jobs)

	wg.Wait()
	return results
}

// fetchSymbol fetches all time ranges for a single symbol.
func fetchSymbol(symbol string) *SymbolRawData {
	raw := &SymbolRawData{Symbol: symbol}

	// We skip the "1d" range — 1D change is computed from the last two 5D bars.
	// Each range failure is logged but non-fatal; we keep what we can get.
	ranges := []struct {
		name      string
		dataRange string
		interval  string
	}{
		{"5d", "5d", "1d"},
		{"1mo", "1mo", "1d"},
		{"ytd", "ytd", "1d"},
	}

	var lastErr string
	for _, r := range ranges {
		bars, currency, err := tools.FetchBars(symbol, r.dataRange, r.interval)
		if err != nil {
			log.Printf("fetch %s/%s: %v", symbol, r.name, err)
			lastErr = err.Error()
			continue // try remaining ranges
		}
		if raw.Currency == "" {
			raw.Currency = currency
		}

		switch r.name {
		case "5d":
			raw.Bars5D = bars
		case "1mo":
			raw.Bars1M = bars
		case "ytd":
			raw.BarsYTD = bars
		}
	}

	// Only mark as errored if ALL ranges failed (no usable data).
	// Truncate error messages to avoid bloating LLM prompts with HTML error pages.
	if raw.Bars5D == nil && raw.Bars1M == nil && raw.BarsYTD == nil && lastErr != "" {
		if len(lastErr) > 120 {
			lastErr = lastErr[:120] + "..."
		}
		raw.Error = lastErr
	}

	return raw
}
