package reporting

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Section defines a group of symbols to track (e.g. "US Magnificent 7").
type Section struct {
	Name        string   `json:"name"`
	Description string   `json:"description"` // what this section tracks and why it matters
	Symbols     []string `json:"symbols"`
}

// ReportConfig holds the user's daily report preferences.
type ReportConfig struct {
	Title            string    `json:"title"`
	Intro            string    `json:"intro"` // why this report exists and why it matters
	Sections         []Section `json:"sections"`
	LLMEnabled       bool      `json:"llm_enabled"`
	WebSearchEnabled bool      `json:"web_search_enabled"`
}

// DefaultConfig returns the built-in default config with 5 sections.
func DefaultConfig() ReportConfig {
	return ReportConfig{
		Title: "ChartGeometry Daily Market Report",
		Intro: "This report tracks 29 symbols across five sectors that together form a real-time " +
			"map of global capital flows. US mega-cap tech (the \"Magnificent 7\") drives index " +
			"weighting and sentiment worldwide. Chinese ADRs (the \"Dragons\") signal Asia's " +
			"economic pulse and regulatory climate. Macro ETFs reveal broad risk appetite across " +
			"large-cap, small-cap, and international markets. Precious metals gauge inflation " +
			"expectations and flight-to-safety demand. Crypto tracks speculative liquidity and " +
			"digital asset sentiment. When these five sectors diverge — tech sells off while gold " +
			"rallies, or emerging markets outpace the S&P — it reveals rotation patterns, regime " +
			"shifts, and macro inflection points often days before headlines catch up. That's why " +
			"we watch them. That's why you should too.",
		LLMEnabled:       true,
		WebSearchEnabled: false,
		Sections: []Section{
			{
				Name:        "US Magnificent 7",
				Description: "The seven mega-cap tech stocks that dominate S&P 500 weighting and global market sentiment.",
				Symbols:     []string{"AAPL", "MSFT", "GOOGL", "AMZN", "NVDA", "META", "TSLA"},
			},
			{
				Name:        "China Dragons",
				Description: "Major Chinese ADRs listed in the US — a barometer for China's tech economy and regulatory climate.",
				Symbols:     []string{"BABA", "PDD", "JD", "BIDU", "NIO", "XPEV", "LI"},
			},
			{
				Name:        "Macro ETFs & Indexes",
				Description: "Broad market and international ETFs tracking US large-cap, small-cap, developed, and emerging markets.",
				Symbols:     []string{"SPY", "QQQ", "DIA", "IWM", "VTI", "EFA", "EEM"},
			},
			{
				Name:        "Precious Metals",
				Description: "Gold, silver, and mining ETFs — traditional safe-haven and inflation-hedge instruments.",
				Symbols:     []string{"GLD", "SLV", "GDX", "IAU"},
			},
			{
				Name:        "Crypto",
				Description: "Major cryptocurrencies — digital assets reflecting risk appetite and liquidity conditions.",
				Symbols:     []string{"BTC-USD", "ETH-USD", "SOL-USD", "XRP-USD"},
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
