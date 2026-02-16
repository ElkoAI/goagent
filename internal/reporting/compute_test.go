package reporting

import (
	"strings"
	"testing"

	"goagent/internal/tools"
)

func TestPctChange(t *testing.T) {
	tests := []struct {
		name     string
		from     *tools.OHLCVBar
		to       *tools.OHLCVBar
		expected float64
	}{
		{"10% gain", &tools.OHLCVBar{Close: 100}, &tools.OHLCVBar{Close: 110}, 10.0},
		{"10% loss", &tools.OHLCVBar{Close: 100}, &tools.OHLCVBar{Close: 90}, -10.0},
		{"no change", &tools.OHLCVBar{Close: 100}, &tools.OHLCVBar{Close: 100}, 0.0},
		{"nil from", nil, &tools.OHLCVBar{Close: 100}, 0.0},
		{"nil to", &tools.OHLCVBar{Close: 100}, nil, 0.0},
		{"zero base", &tools.OHLCVBar{Close: 0}, &tools.OHLCVBar{Close: 100}, 0.0},
		{"fractional", &tools.OHLCVBar{Close: 200}, &tools.OHLCVBar{Close: 203}, 1.5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pctChange(tc.from, tc.to)
			if got != tc.expected {
				t.Errorf("pctChange = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestComputeReturns(t *testing.T) {
	raw := &SymbolRawData{
		Symbol:   "TEST",
		Currency: "USD",
		Bars5D: []tools.OHLCVBar{
			{Date: "2026-02-10", Close: 100, Volume: 1000},
			{Date: "2026-02-11", Close: 102, Volume: 1100},
			{Date: "2026-02-12", Close: 105, Volume: 1200},
			{Date: "2026-02-13", Close: 103, Volume: 900},
			{Date: "2026-02-14", Close: 110, Volume: 1500},
		},
		Bars1M: []tools.OHLCVBar{
			{Date: "2026-01-15", Close: 95},
			{Date: "2026-02-14", Close: 110},
		},
		BarsYTD: []tools.OHLCVBar{
			{Date: "2026-01-02", Close: 90},
			{Date: "2026-02-14", Close: 110},
		},
	}

	sd := ComputeReturns(raw)

	if sd.Symbol != "TEST" {
		t.Errorf("symbol = %q, want TEST", sd.Symbol)
	}
	if sd.LastClose != 110 {
		t.Errorf("last_close = %v, want 110", sd.LastClose)
	}
	if sd.Volume != 1500 {
		t.Errorf("volume = %v, want 1500", sd.Volume)
	}

	// 1D: (110 - 103) / 103 * 100 = 6.80
	if sd.Change1D != 6.80 {
		t.Errorf("change_1d = %v, want 6.80", sd.Change1D)
	}

	// 5D: (110 - 100) / 100 * 100 = 10.0
	if sd.Change5D != 10.0 {
		t.Errorf("change_5d = %v, want 10.0", sd.Change5D)
	}

	// 1M: (110 - 95) / 95 * 100 = 15.79
	if sd.Change1M != 15.79 {
		t.Errorf("change_1m = %v, want 15.79", sd.Change1M)
	}

	// YTD: (110 - 90) / 90 * 100 = 22.22
	if sd.ChangeYTD != 22.22 {
		t.Errorf("change_ytd = %v, want 22.22", sd.ChangeYTD)
	}
}

func TestComputeReturns_WithError(t *testing.T) {
	raw := &SymbolRawData{
		Symbol: "FAIL",
		Error:  "fetch failed",
	}

	sd := ComputeReturns(raw)
	if sd.Error != "fetch failed" {
		t.Errorf("error = %q, want 'fetch failed'", sd.Error)
	}
	if sd.LastClose != 0 {
		t.Error("expected zero last_close for errored symbol")
	}
}

func TestTopGainers(t *testing.T) {
	all := []SymbolData{
		{Symbol: "A", Change1D: 5.0},
		{Symbol: "B", Change1D: -2.0},
		{Symbol: "C", Change1D: 10.0},
		{Symbol: "D", Change1D: 3.0, Error: "failed"},
		{Symbol: "E", Change1D: 8.0},
	}

	getter := func(sd SymbolData) float64 { return sd.Change1D }
	top := TopGainers(all, getter, 3)

	if len(top) != 3 {
		t.Fatalf("got %d, want 3", len(top))
	}
	if top[0].Symbol != "C" {
		t.Errorf("top[0] = %q, want C", top[0].Symbol)
	}
	if top[1].Symbol != "E" {
		t.Errorf("top[1] = %q, want E", top[1].Symbol)
	}
	if top[2].Symbol != "A" {
		t.Errorf("top[2] = %q, want A", top[2].Symbol)
	}
}

func TestTopLosers(t *testing.T) {
	all := []SymbolData{
		{Symbol: "A", Change1D: 5.0},
		{Symbol: "B", Change1D: -2.0},
		{Symbol: "C", Change1D: -7.0},
	}

	getter := func(sd SymbolData) float64 { return sd.Change1D }
	bottom := TopLosers(all, getter, 2)

	if len(bottom) != 2 {
		t.Fatalf("got %d, want 2", len(bottom))
	}
	if bottom[0].Symbol != "C" {
		t.Errorf("bottom[0] = %q, want C", bottom[0].Symbol)
	}
	if bottom[1].Symbol != "B" {
		t.Errorf("bottom[1] = %q, want B", bottom[1].Symbol)
	}
}

func TestRankMovers_MoreThanAvailable(t *testing.T) {
	all := []SymbolData{
		{Symbol: "A", Change1D: 5.0},
		{Symbol: "B", Change1D: 3.0},
	}

	getter := func(sd SymbolData) float64 { return sd.Change1D }
	top := TopGainers(all, getter, 10)

	if len(top) != 2 {
		t.Errorf("got %d, want 2 (capped to available)", len(top))
	}
}

func TestSparklineSVG(t *testing.T) {
	bars := []tools.OHLCVBar{
		{Close: 100}, {Close: 105}, {Close: 102}, {Close: 108}, {Close: 110},
	}

	svg := SparklineSVG(bars, 80, 20)
	if svg == "" {
		t.Fatal("expected SVG output")
	}
	if !strings.Contains(svg, "<svg") {
		t.Error("missing <svg tag")
	}
	if !strings.Contains(svg, "polyline") {
		t.Error("missing polyline element")
	}
	// Gained overall → green
	if !strings.Contains(svg, "#16a34a") {
		t.Error("expected green color for upward trend")
	}
}

func TestSparklineSVG_Downward(t *testing.T) {
	bars := []tools.OHLCVBar{
		{Close: 110}, {Close: 105}, {Close: 100},
	}

	svg := SparklineSVG(bars, 80, 20)
	// Lost overall → red
	if !strings.Contains(svg, "#dc2626") {
		t.Error("expected red color for downward trend")
	}
}

func TestSparklineSVG_TooFewBars(t *testing.T) {
	svg := SparklineSVG([]tools.OHLCVBar{{Close: 100}}, 80, 20)
	if svg != "" {
		t.Error("expected empty string for <2 bars")
	}
}

func TestRoundTo(t *testing.T) {
	if got := roundTo(3.14159, 2); got != 3.14 {
		t.Errorf("roundTo(3.14159, 2) = %v, want 3.14", got)
	}
	if got := roundTo(3.145, 2); got != 3.15 {
		t.Errorf("roundTo(3.145, 2) = %v, want 3.15", got)
	}
}
