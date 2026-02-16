package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"
)

// OHLCVBar represents a single candlestick bar.
type OHLCVBar struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

// yahooAuth holds the crumb and HTTP client with session cookies.
type yahooAuth struct {
	crumb  string
	client *http.Client
}

var (
	cachedAuth *yahooAuth
	authOnce   sync.Once
	authErr    error

	// Overridable for testing.
	yahooBaseURL  = "https://query2.finance.yahoo.com"
	yahooCrumbURL = "https://query2.finance.yahoo.com/v1/test/getcrumb"
	yahooCookieURL = "https://fc.yahoo.com/"
)

var validRanges = map[string]bool{
	"1d": true, "5d": true, "1mo": true, "3mo": true, "6mo": true,
	"1y": true, "2y": true, "5y": true, "10y": true, "ytd": true, "max": true,
}

var validIntervals = map[string]bool{
	"1d": true, "1wk": true, "1mo": true,
}

// fetchCrumb obtains a Yahoo Finance crumb+cookie pair for API auth.
func fetchCrumb(client *http.Client) (string, error) {
	// Hit the cookie consent page to obtain session cookies.
	resp, err := client.Get(yahooCookieURL)
	if err != nil {
		return "", fmt.Errorf("yahoo cookie fetch: %w", err)
	}
	resp.Body.Close()

	// Fetch the crumb using the session cookies.
	resp, err = client.Get(yahooCrumbURL)
	if err != nil {
		return "", fmt.Errorf("yahoo crumb fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("yahoo crumb read: %w", err)
	}

	crumb := strings.TrimSpace(string(body))
	if crumb == "" {
		return "", fmt.Errorf("yahoo returned empty crumb")
	}
	return crumb, nil
}

// uaTransport wraps http.RoundTripper to inject a browser User-Agent header.
type uaTransport struct {
	base http.RoundTripper
}

func (t *uaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	return t.base.RoundTrip(req)
}

// getAuth returns a cached Yahoo auth (crumb + cookie client), initializing on first call.
func getAuth() (*yahooAuth, error) {
	authOnce.Do(func() {
		jar, err := cookiejar.New(nil)
		if err != nil {
			authErr = err
			return
		}
		client := &http.Client{
			Jar:       jar,
			Timeout:   15 * time.Second,
			Transport: &uaTransport{base: http.DefaultTransport},
		}
		crumb, err := fetchCrumb(client)
		if err != nil {
			authErr = err
			return
		}
		cachedAuth = &yahooAuth{crumb: crumb, client: client}
	})
	return cachedAuth, authErr
}

// resetAuth clears the cached auth (used in tests).
func resetAuth() {
	authOnce = sync.Once{}
	cachedAuth = nil
	authErr = nil
}

// fetchOHLCV retrieves OHLCV bars from Yahoo Finance for the given symbol.
func fetchOHLCV(auth *yahooAuth, symbol, dataRange, interval string) ([]OHLCVBar, string, error) {
	url := fmt.Sprintf("%s/v8/finance/chart/%s?range=%s&interval=%s&crumb=%s",
		yahooBaseURL, symbol, dataRange, interval, auth.crumb)

	resp, err := auth.client.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("yahoo chart fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("yahoo chart API %d: %s", resp.StatusCode, string(body))
	}

	var chart struct {
		Chart struct {
			Result []struct {
				Meta struct {
					Currency string `json:"currency"`
					Symbol   string `json:"symbol"`
				} `json:"meta"`
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []int64   `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
			Error *struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		} `json:"chart"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&chart); err != nil {
		return nil, "", fmt.Errorf("yahoo chart parse: %w", err)
	}

	if chart.Chart.Error != nil {
		return nil, "", fmt.Errorf("yahoo chart error: %s — %s",
			chart.Chart.Error.Code, chart.Chart.Error.Description)
	}

	if len(chart.Chart.Result) == 0 {
		return nil, "", fmt.Errorf("yahoo chart: no results for %s", symbol)
	}

	r := chart.Chart.Result[0]
	if len(r.Indicators.Quote) == 0 {
		return nil, "", fmt.Errorf("yahoo chart: no quote data for %s", symbol)
	}

	q := r.Indicators.Quote[0]
	bars := make([]OHLCVBar, 0, len(r.Timestamp))
	for i, ts := range r.Timestamp {
		bar := OHLCVBar{
			Date: time.Unix(ts, 0).UTC().Format("2006-01-02"),
		}
		if i < len(q.Open) {
			bar.Open = q.Open[i]
		}
		if i < len(q.High) {
			bar.High = q.High[i]
		}
		if i < len(q.Low) {
			bar.Low = q.Low[i]
		}
		if i < len(q.Close) {
			bar.Close = q.Close[i]
		}
		if i < len(q.Volume) {
			bar.Volume = q.Volume[i]
		}
		bars = append(bars, bar)
	}

	return bars, r.Meta.Currency, nil
}

// BarsToCSV converts OHLCV bars into a CSV string with a header row.
func BarsToCSV(bars []OHLCVBar) string {
	var b strings.Builder
	b.WriteString("date,open,high,low,close,volume\n")
	for _, bar := range bars {
		fmt.Fprintf(&b, "%s,%.2f,%.2f,%.2f,%.2f,%d\n",
			bar.Date, bar.Open, bar.High, bar.Low, bar.Close, bar.Volume)
	}
	return b.String()
}

// FetchBars is the public API for fetching OHLCV bars directly (no JSON-RPC).
// Used by the reporting package and anything else that needs raw bar data.
func FetchBars(symbol, dataRange, interval string) ([]OHLCVBar, string, error) {
	auth, err := getAuth()
	if err != nil {
		return nil, "", fmt.Errorf("yahoo auth: %w", err)
	}
	return fetchOHLCV(auth, symbol, dataRange, interval)
}

// OHLCV is the tool entry point — called from the JSON-RPC dispatcher.
func OHLCV(params json.RawMessage) (any, error) {
	var req struct {
		Symbol   string `json:"symbol"`
		Range    string `json:"range"`
		Interval string `json:"interval"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}

	if req.Symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	if req.Range == "" {
		req.Range = "1mo"
	}
	if req.Interval == "" {
		req.Interval = "1d"
	}

	if !validRanges[req.Range] {
		return nil, fmt.Errorf("invalid range %q", req.Range)
	}
	if !validIntervals[req.Interval] {
		return nil, fmt.Errorf("invalid interval %q", req.Interval)
	}

	auth, err := getAuth()
	if err != nil {
		return nil, fmt.Errorf("yahoo auth: %w", err)
	}

	bars, currency, err := fetchOHLCV(auth, req.Symbol, req.Range, req.Interval)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"symbol":   req.Symbol,
		"currency": currency,
		"bars":     bars,
	}, nil
}
