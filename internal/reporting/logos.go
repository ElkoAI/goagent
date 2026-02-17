package reporting

import (
	"embed"
	"fmt"
	"html/template"
	"strings"
	"sync"
)

//go:embed templates/logos
var logosFS embed.FS

// SymbolMeta holds display metadata for a tracked symbol.
type SymbolMeta struct {
	Symbol    string // ticker symbol (e.g. "AAPL")
	LongName  string // full company/asset name
	ShortName string // abbreviated display name
	Sector    string // sector grouping
	Type      string // "stock", "etf", "crypto", "commodity"
}

// symbolRegistry maps ticker symbols to their metadata.
var symbolRegistry = map[string]SymbolMeta{
	// US Magnificent 7
	"AAPL":  {Symbol: "AAPL", LongName: "Apple Inc.", ShortName: "Apple", Sector: "US Mag 7", Type: "stock"},
	"MSFT":  {Symbol: "MSFT", LongName: "Microsoft Corporation", ShortName: "Microsoft", Sector: "US Mag 7", Type: "stock"},
	"GOOGL": {Symbol: "GOOGL", LongName: "Alphabet Inc.", ShortName: "Alphabet", Sector: "US Mag 7", Type: "stock"},
	"AMZN":  {Symbol: "AMZN", LongName: "Amazon.com Inc.", ShortName: "Amazon", Sector: "US Mag 7", Type: "stock"},
	"NVDA":  {Symbol: "NVDA", LongName: "NVIDIA Corporation", ShortName: "NVIDIA", Sector: "US Mag 7", Type: "stock"},
	"META":  {Symbol: "META", LongName: "Meta Platforms Inc.", ShortName: "Meta", Sector: "US Mag 7", Type: "stock"},
	"TSLA":  {Symbol: "TSLA", LongName: "Tesla Inc.", ShortName: "Tesla", Sector: "US Mag 7", Type: "stock"},

	// China Dragons
	"BABA": {Symbol: "BABA", LongName: "Alibaba Group Holding", ShortName: "Alibaba", Sector: "China Dragons", Type: "stock"},
	"PDD":  {Symbol: "PDD", LongName: "PDD Holdings Inc.", ShortName: "PDD", Sector: "China Dragons", Type: "stock"},
	"JD":   {Symbol: "JD", LongName: "JD.com Inc.", ShortName: "JD.com", Sector: "China Dragons", Type: "stock"},
	"BIDU": {Symbol: "BIDU", LongName: "Baidu Inc.", ShortName: "Baidu", Sector: "China Dragons", Type: "stock"},
	"NIO":  {Symbol: "NIO", LongName: "NIO Inc.", ShortName: "NIO", Sector: "China Dragons", Type: "stock"},
	"XPEV": {Symbol: "XPEV", LongName: "XPeng Inc.", ShortName: "XPeng", Sector: "China Dragons", Type: "stock"},
	"LI":   {Symbol: "LI", LongName: "Li Auto Inc.", ShortName: "Li Auto", Sector: "China Dragons", Type: "stock"},

	// Macro ETFs & Indexes
	"SPY": {Symbol: "SPY", LongName: "SPDR S&P 500 ETF Trust", ShortName: "S&P 500", Sector: "Macro ETFs", Type: "etf"},
	"QQQ": {Symbol: "QQQ", LongName: "Invesco QQQ Trust", ShortName: "Nasdaq 100", Sector: "Macro ETFs", Type: "etf"},
	"DIA": {Symbol: "DIA", LongName: "SPDR Dow Jones Industrial Avg ETF", ShortName: "Dow Jones", Sector: "Macro ETFs", Type: "etf"},
	"IWM": {Symbol: "IWM", LongName: "iShares Russell 2000 ETF", ShortName: "Russell 2000", Sector: "Macro ETFs", Type: "etf"},
	"VTI": {Symbol: "VTI", LongName: "Vanguard Total Stock Market ETF", ShortName: "Total Market", Sector: "Macro ETFs", Type: "etf"},
	"EFA": {Symbol: "EFA", LongName: "iShares MSCI EAFE ETF", ShortName: "Intl Developed", Sector: "Macro ETFs", Type: "etf"},
	"EEM": {Symbol: "EEM", LongName: "iShares MSCI Emerging Markets ETF", ShortName: "Emerging Mkts", Sector: "Macro ETFs", Type: "etf"},

	// Precious Metals
	"GLD": {Symbol: "GLD", LongName: "SPDR Gold Shares", ShortName: "Gold", Sector: "Precious Metals", Type: "commodity"},
	"SLV": {Symbol: "SLV", LongName: "iShares Silver Trust", ShortName: "Silver", Sector: "Precious Metals", Type: "commodity"},
	"GDX": {Symbol: "GDX", LongName: "VanEck Gold Miners ETF", ShortName: "Gold Miners", Sector: "Precious Metals", Type: "etf"},
	"IAU": {Symbol: "IAU", LongName: "iShares Gold Trust", ShortName: "Gold Trust", Sector: "Precious Metals", Type: "commodity"},

	// Crypto
	"BTC-USD": {Symbol: "BTC-USD", LongName: "Bitcoin", ShortName: "Bitcoin", Sector: "Crypto", Type: "crypto"},
	"ETH-USD": {Symbol: "ETH-USD", LongName: "Ethereum", ShortName: "Ethereum", Sector: "Crypto", Type: "crypto"},
	"SOL-USD": {Symbol: "SOL-USD", LongName: "Solana", ShortName: "Solana", Sector: "Crypto", Type: "crypto"},
	"XRP-USD": {Symbol: "XRP-USD", LongName: "XRP", ShortName: "XRP", Sector: "Crypto", Type: "crypto"},
}

// GetSymbolMeta returns metadata for a symbol, or a zero-value with the symbol set if unknown.
func GetSymbolMeta(symbol string) SymbolMeta {
	if m, ok := symbolRegistry[symbol]; ok {
		return m
	}
	return SymbolMeta{Symbol: symbol, ShortName: symbol}
}

// logoCache caches loaded SVG content by symbol.
var (
	logoCache   map[string]string
	logoCacheOn sync.Once
)

// loadLogos reads all SVG files from the embedded logos directory.
func loadLogos() map[string]string {
	logoCacheOn.Do(func() {
		logoCache = make(map[string]string)
		entries, err := logosFS.ReadDir("templates/logos")
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".svg") {
				continue
			}
			data, err := logosFS.ReadFile("templates/logos/" + e.Name())
			if err != nil {
				continue
			}
			sym := strings.TrimSuffix(e.Name(), ".svg")
			logoCache[sym] = string(data)
		}
	})
	return logoCache
}

// LogoSVG returns inline SVG HTML for a symbol's logo (20x20).
// Falls back to a styled text badge if no logo file exists.
func LogoSVG(symbol string) template.HTML {
	logos := loadLogos()
	if svg, ok := logos[symbol]; ok {
		return template.HTML(fmt.Sprintf(`<span class="logo">%s</span>`, svg))
	}
	// Fallback: first 1-2 letters as a badge.
	label := symbol
	if len(label) > 2 {
		label = label[:2]
	}
	return template.HTML(fmt.Sprintf(
		`<span class="logo"><span class="logo-badge">%s</span></span>`, label))
}
