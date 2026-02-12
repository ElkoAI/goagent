package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// resolveBackend returns the search backend to use.
// Checks SEARCH_BACKEND env var first, then auto-detects based on available keys.
func resolveBackend() string {
	if b := os.Getenv("SEARCH_BACKEND"); b != "" {
		return strings.ToLower(b)
	}
	if os.Getenv("BRAVE_SEARCH_API_KEY") != "" {
		return "brave"
	}
	return "duckduckgo"
}

// WebSearch dispatches to the configured search backend.
func WebSearch(params json.RawMessage) (any, error) {
	var req struct {
		Query string `json:"query"`
		Count int    `json:"num_results"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}
	if req.Count == 0 {
		req.Count = 10
	}

	var results []SearchResult
	var err error

	switch resolveBackend() {
	case "brave":
		results, err = braveSearch(req.Query, req.Count)
	case "duckduckgo":
		results, err = duckduckgoSearch(req.Query, req.Count)
	default:
		return nil, fmt.Errorf("unknown search backend: %s", resolveBackend())
	}

	if err != nil {
		return nil, err
	}
	return map[string]any{"results": results}, nil
}

// braveSearch queries the Brave Search API.
func braveSearch(query string, count int) ([]SearchResult, error) {
	apiKey := os.Getenv("BRAVE_SEARCH_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("BRAVE_SEARCH_API_KEY env var missing")
	}

	endpoint := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), count)

	httpReq, _ := http.NewRequest("GET", endpoint, nil)
	httpReq.Header.Set("X-Subscription-Token", apiKey)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{}).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Web struct {
			Results []SearchResult `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data.Web.Results, nil
}
