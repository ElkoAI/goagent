package reporting

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"goagent/internal/tools"
)

// SymbolData holds computed metrics for a single symbol.
type SymbolData struct {
	Symbol    string  `json:"symbol"`
	Currency  string  `json:"currency"`
	LastClose float64 `json:"last_close"`
	Change1D  float64 `json:"change_1d"`  // percent
	Change5D  float64 `json:"change_5d"`  // percent
	Change1M  float64 `json:"change_1m"`  // percent
	ChangeYTD float64 `json:"change_ytd"` // percent
	Volume    int64   `json:"volume"`
	Sparkline string  `json:"sparkline,omitempty"` // inline SVG path from 5D closes
	Error     string  `json:"error,omitempty"`
}

// ComputeReturns calculates percentage changes from raw OHLCV bars.
func ComputeReturns(raw *SymbolRawData) SymbolData {
	sd := SymbolData{
		Symbol:   raw.Symbol,
		Currency: raw.Currency,
		Error:    raw.Error,
	}

	if raw.Error != "" {
		return sd
	}

	// Last close from the most recent bar available.
	if last := lastBar(raw.Bars5D); last != nil {
		sd.LastClose = last.Close
		sd.Volume = last.Volume
	} else if last := lastBar(raw.Bars1M); last != nil {
		sd.LastClose = last.Close
		sd.Volume = last.Volume
	}

	// 1D change: use 5D bars (last two) to get today vs yesterday.
	sd.Change1D = pctChange(secondToLast(raw.Bars5D), lastBar(raw.Bars5D))

	// 5D change: first bar vs last bar of the 5d range.
	sd.Change5D = pctChange(firstBar(raw.Bars5D), lastBar(raw.Bars5D))

	// 1M change: first bar vs last bar of the 1mo range.
	sd.Change1M = pctChange(firstBar(raw.Bars1M), lastBar(raw.Bars1M))

	// YTD change: first bar vs last bar of the ytd range.
	sd.ChangeYTD = pctChange(firstBar(raw.BarsYTD), lastBar(raw.BarsYTD))

	// Sparkline from 5D close prices.
	sd.Sparkline = SparklineSVG(raw.Bars5D, 80, 20)

	return sd
}

// pctChange calculates percentage change between two bars' close prices.
// Returns 0 if either bar is nil or the base close is zero.
func pctChange(from, to *tools.OHLCVBar) float64 {
	if from == nil || to == nil || from.Close == 0 {
		return 0
	}
	pct := (to.Close - from.Close) / from.Close * 100
	return roundTo(pct, 2)
}

func firstBar(bars []tools.OHLCVBar) *tools.OHLCVBar {
	if len(bars) == 0 {
		return nil
	}
	return &bars[0]
}

func lastBar(bars []tools.OHLCVBar) *tools.OHLCVBar {
	if len(bars) == 0 {
		return nil
	}
	return &bars[len(bars)-1]
}

func secondToLast(bars []tools.OHLCVBar) *tools.OHLCVBar {
	if len(bars) < 2 {
		return nil
	}
	return &bars[len(bars)-2]
}

func roundTo(val float64, places int) float64 {
	pow := math.Pow(10, float64(places))
	return math.Round(val*pow) / pow
}

// RankMovers sorts symbols by the given field and returns the top N.
// The getter function extracts the sort value from each SymbolData.
func RankMovers(all []SymbolData, getter func(SymbolData) float64, n int, descending bool) []SymbolData {
	// Filter out errored symbols.
	var valid []SymbolData
	for _, sd := range all {
		if sd.Error == "" {
			valid = append(valid, sd)
		}
	}

	sort.Slice(valid, func(i, j int) bool {
		if descending {
			return getter(valid[i]) > getter(valid[j])
		}
		return getter(valid[i]) < getter(valid[j])
	})

	if n > len(valid) {
		n = len(valid)
	}
	return valid[:n]
}

// TopGainers returns the top N symbols by the given field (highest first).
func TopGainers(all []SymbolData, getter func(SymbolData) float64, n int) []SymbolData {
	return RankMovers(all, getter, n, true)
}

// TopLosers returns the top N symbols by the given field (lowest first).
func TopLosers(all []SymbolData, getter func(SymbolData) float64, n int) []SymbolData {
	return RankMovers(all, getter, n, false)
}

// SparklineSVG generates an inline SVG sparkline from OHLCV bar close prices.
// Returns a complete <svg> element string. Width/height are in pixels.
func SparklineSVG(bars []tools.OHLCVBar, width, height int) string {
	if len(bars) < 2 {
		return ""
	}

	// Extract close prices.
	closes := make([]float64, len(bars))
	minV, maxV := bars[0].Close, bars[0].Close
	for i, bar := range bars {
		closes[i] = bar.Close
		if bar.Close < minV {
			minV = bar.Close
		}
		if bar.Close > maxV {
			maxV = bar.Close
		}
	}

	// Avoid division by zero for flat lines.
	spread := maxV - minV
	if spread == 0 {
		spread = 1
	}

	// Build polyline points. Y is inverted (SVG origin is top-left).
	padding := 2.0
	w := float64(width) - 2*padding
	h := float64(height) - 2*padding

	var points []string
	for i, v := range closes {
		x := padding + (float64(i)/float64(len(closes)-1))*w
		y := padding + (1-(v-minV)/spread)*h
		points = append(points, fmt.Sprintf("%.1f,%.1f", x, y))
	}

	// Color: green if last > first, red if last < first, gray if flat.
	color := "#888"
	if closes[len(closes)-1] > closes[0] {
		color = "#16a34a"
	} else if closes[len(closes)-1] < closes[0] {
		color = "#dc2626"
	}

	return fmt.Sprintf(
		`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg">`+
			`<polyline fill="none" stroke="%s" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" points="%s"/>`+
			`</svg>`,
		width, height, width, height, color, strings.Join(points, " "))
}
