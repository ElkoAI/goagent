package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// cannedYahooChart is a minimal Yahoo Finance chart API response.
const cannedYahooChart = `{
  "chart": {
    "result": [{
      "meta": {"currency": "USD", "symbol": "AAPL"},
      "timestamp": [1736899200, 1736985600, 1737072000],
      "indicators": {
        "quote": [{
          "open":   [228.47, 229.50, 230.00],
          "high":   [230.82, 231.00, 232.50],
          "low":    [227.58, 228.90, 229.10],
          "close":  [229.26, 230.10, 231.80],
          "volume": [43627500, 38100000, 41200000]
        }]
      }
    }],
    "error": null
  }
}`

func TestOHLCV(t *testing.T) {
	// Set up a mock Yahoo Finance server.
	mux := http.NewServeMux()

	// Mock cookie endpoint.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "A3", Value: "test"})
		w.WriteHeader(http.StatusOK)
	})

	// Mock crumb endpoint.
	mux.HandleFunc("/v1/test/getcrumb", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test-crumb-123"))
	})

	// Mock chart endpoint.
	mux.HandleFunc("/v8/finance/chart/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cannedYahooChart))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Override the Yahoo URLs to point to our test server.
	origBase := yahooBaseURL
	origCrumb := yahooCrumbURL
	origCookie := yahooCookieURL
	yahooBaseURL = ts.URL
	yahooCrumbURL = ts.URL + "/v1/test/getcrumb"
	yahooCookieURL = ts.URL + "/"
	defer func() {
		yahooBaseURL = origBase
		yahooCrumbURL = origCrumb
		yahooCookieURL = origCookie
	}()

	// Reset cached auth so it uses our test server.
	resetAuth()
	defer resetAuth()

	params, _ := json.Marshal(map[string]string{
		"symbol":   "AAPL",
		"range":    "1mo",
		"interval": "1d",
	})

	result, err := OHLCV(params)
	if err != nil {
		t.Fatalf("OHLCV() error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	if m["symbol"] != "AAPL" {
		t.Errorf("symbol = %v, want AAPL", m["symbol"])
	}
	if m["currency"] != "USD" {
		t.Errorf("currency = %v, want USD", m["currency"])
	}

	bars, ok := m["bars"].([]OHLCVBar)
	if !ok {
		t.Fatalf("bars type = %T, want []OHLCVBar", m["bars"])
	}
	if len(bars) != 3 {
		t.Fatalf("len(bars) = %d, want 3", len(bars))
	}

	// Verify first bar values.
	b := bars[0]
	if b.Date != "2025-01-15" {
		t.Errorf("bars[0].Date = %q, want 2025-01-15", b.Date)
	}
	if b.Open != 228.47 {
		t.Errorf("bars[0].Open = %v, want 228.47", b.Open)
	}
	if b.High != 230.82 {
		t.Errorf("bars[0].High = %v, want 230.82", b.High)
	}
	if b.Low != 227.58 {
		t.Errorf("bars[0].Low = %v, want 227.58", b.Low)
	}
	if b.Close != 229.26 {
		t.Errorf("bars[0].Close = %v, want 229.26", b.Close)
	}
	if b.Volume != 43627500 {
		t.Errorf("bars[0].Volume = %v, want 43627500", b.Volume)
	}

	// Verify last bar.
	last := bars[2]
	if last.Volume != 41200000 {
		t.Errorf("bars[2].Volume = %v, want 41200000", last.Volume)
	}
}

func TestOHLCV_Validation(t *testing.T) {
	// Missing symbol.
	params, _ := json.Marshal(map[string]string{"range": "1mo"})
	_, err := OHLCV(params)
	if err == nil || !strings.Contains(err.Error(), "symbol is required") {
		t.Errorf("expected symbol required error, got: %v", err)
	}

	// Invalid range.
	params, _ = json.Marshal(map[string]string{"symbol": "AAPL", "range": "99y"})
	_, err = OHLCV(params)
	if err == nil || !strings.Contains(err.Error(), "invalid range") {
		t.Errorf("expected invalid range error, got: %v", err)
	}

	// Invalid interval.
	params, _ = json.Marshal(map[string]string{"symbol": "AAPL", "interval": "5m"})
	_, err = OHLCV(params)
	if err == nil || !strings.Contains(err.Error(), "invalid interval") {
		t.Errorf("expected invalid interval error, got: %v", err)
	}
}

func TestBarsToCSV(t *testing.T) {
	bars := []OHLCVBar{
		{Date: "2025-01-15", Open: 228.47, High: 230.82, Low: 227.58, Close: 229.26, Volume: 43627500},
		{Date: "2025-01-16", Open: 229.50, High: 231.00, Low: 228.90, Close: 230.10, Volume: 38100000},
	}

	csv := BarsToCSV(bars)
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 rows), got %d", len(lines))
	}
	if lines[0] != "date,open,high,low,close,volume" {
		t.Errorf("header = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "2025-01-15,228.47,") {
		t.Errorf("row 1 = %q", lines[1])
	}
}

func TestIntegration_OHLCV(t *testing.T) {
	if os.Getenv("OHLCV_INTEGRATION") != "true" {
		t.Skip("Skipping integration test (set OHLCV_INTEGRATION=true to run)")
	}

	// Reset auth so we hit real Yahoo.
	resetAuth()
	defer resetAuth()

	params, _ := json.Marshal(map[string]string{
		"symbol":   "AAPL",
		"range":    "5d",
		"interval": "1d",
	})

	result, err := OHLCV(params)
	if err != nil {
		t.Fatalf("Integration OHLCV() error: %v", err)
	}

	m := result.(map[string]any)
	bars := m["bars"].([]OHLCVBar)
	if len(bars) == 0 {
		t.Fatal("expected at least 1 bar from real Yahoo Finance")
	}

	t.Logf("Got %d bars for AAPL, currency=%s", len(bars), m["currency"])
	t.Logf("First bar: %+v", bars[0])
}
