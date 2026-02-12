package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// duckduckgoSearch queries the DuckDuckGo instant answer API (no API key required).
func duckduckgoSearch(query string, count int) ([]SearchResult, error) {
	endpoint := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1",
		url.QueryEscape(query))

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Abstract       string `json:"Abstract"`
		AbstractURL    string `json:"AbstractURL"`
		AbstractSource string `json:"AbstractSource"`
		RelatedTopics  []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []SearchResult

	// Include the abstract as the first result if present.
	if data.Abstract != "" {
		results = append(results, SearchResult{
			Title:   data.AbstractSource,
			URL:     data.AbstractURL,
			Snippet: data.Abstract,
		})
	}

	// Normalize related topics into search results.
	for _, topic := range data.RelatedTopics {
		if topic.FirstURL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   "",
			URL:     topic.FirstURL,
			Snippet: topic.Text,
		})
		if len(results) >= count {
			break
		}
	}

	return results, nil
}
