package reporting

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Section defines a group of symbols to track (e.g. "US Magnificent 7").
type Section struct {
	Name    string   `json:"name"`
	Symbols []string `json:"symbols"`
}

// ReportConfig holds the user's daily report preferences.
type ReportConfig struct {
	Sections []Section `json:"sections"`
	LLMEnabled       bool `json:"llm_enabled"`
	WebSearchEnabled bool `json:"web_search_enabled"`
}

// DefaultConfig returns the built-in default config with 5 sections.
func DefaultConfig() ReportConfig {
	return ReportConfig{
		LLMEnabled:       true,
		WebSearchEnabled: false,
		Sections: []Section{
			{
				Name:    "US Magnificent 7",
				Symbols: []string{"AAPL", "MSFT", "GOOGL", "AMZN", "NVDA", "META", "TSLA"},
			},
			{
				Name:    "China Dragons",
				Symbols: []string{"BABA", "PDD", "JD", "BIDU", "NIO", "XPEV", "LI"},
			},
			{
				Name:    "Macro ETFs & Indexes",
				Symbols: []string{"SPY", "QQQ", "DIA", "IWM", "VTI", "EFA", "EEM"},
			},
			{
				Name:    "Precious Metals",
				Symbols: []string{"GLD", "SLV", "GDX", "IAU"},
			},
			{
				Name:    "Crypto",
				Symbols: []string{"BTC-USD", "ETH-USD", "SOL-USD", "XRP-USD"},
			},
		},
	}
}

// configPath returns the path to the user's daily report config file.
func configPath(dataDir, userID string) string {
	return filepath.Join(dataDir, "home", userID, "config", "daily_report.json")
}

// LoadConfig reads the user's daily report config, creating defaults if missing.
func LoadConfig(dataDir, userID string) (ReportConfig, error) {
	path := configPath(dataDir, userID)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if writeErr := SaveConfig(dataDir, userID, cfg); writeErr != nil {
				return cfg, nil // return defaults even if save fails
			}
			return cfg, nil
		}
		return ReportConfig{}, err
	}

	var cfg ReportConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ReportConfig{}, err
	}
	return cfg, nil
}

// SaveConfig writes the config to disk, creating directories as needed.
func SaveConfig(dataDir, userID string, cfg ReportConfig) error {
	path := configPath(dataDir, userID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
