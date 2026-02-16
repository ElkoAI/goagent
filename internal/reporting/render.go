package reporting

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
)

//go:embed templates/report.html
var templateFS embed.FS

// templateFuncs are shared template functions for HTML rendering.
var templateFuncs = template.FuncMap{
	"pctClass": func(v float64) string {
		if v > 0 {
			return "pos"
		}
		if v < 0 {
			return "neg"
		}
		return "flat"
	},
	"symClass": func(v float64) string {
		if v > 0 {
			return "sym-up"
		}
		if v < 0 {
			return "sym-down"
		}
		return "sym-flat"
	},
	"fmtPct": func(v float64) string {
		return fmt.Sprintf("%+.2f%%", v)
	},
	"fmtPrice": func(v float64) string {
		return fmt.Sprintf("%.2f", v)
	},
	"fmtVol": func(v int64) string {
		switch {
		case v >= 1_000_000_000:
			return fmt.Sprintf("%.1fB", float64(v)/1e9)
		case v >= 1_000_000:
			return fmt.Sprintf("%.1fM", float64(v)/1e6)
		case v >= 1_000:
			return fmt.Sprintf("%.1fK", float64(v)/1e3)
		default:
			return fmt.Sprintf("%d", v)
		}
	},
	"emoji": func(v float64) string {
		return PerformanceEmoji(v)
	},
	"rawSVG": func(s string) template.HTML {
		return template.HTML(s)
	},
	"yfinLink": func(symbol string) string {
		return "https://finance.yahoo.com/quote/" + symbol
	},
}

var htmlTmpl = template.Must(
	template.New("report.html").Funcs(templateFuncs).ParseFS(templateFS, "templates/report.html"),
)

// RenderMarkdown produces a complete markdown report.
func RenderMarkdown(report *DailyReport) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s — %s (%s)\n\n", report.Title, report.Date, report.Generated))

	// Report intro — why this report exists.
	if report.Intro != "" {
		b.WriteString(fmt.Sprintf("> %s\n\n", report.Intro))
	}

	// Market pulse.
	if report.Summary != "" {
		b.WriteString(report.Summary + "\n\n")
	}

	// Section overview with emojis.
	b.WriteString("## At a Glance\n\n")
	for _, sec := range report.Sections {
		b.WriteString(fmt.Sprintf("**%s** — %s\n", sec.Name, sec.Summary))
	}
	b.WriteString("\n")

	// Top movers summary.
	b.WriteString("## Top Movers (1D)\n\n")
	if len(report.TopGainers1D) > 0 {
		b.WriteString("### Gainers\n")
		b.WriteString("| | Symbol | Close | 1D % |\n|---|--------|------:|-----:|\n")
		for _, sd := range report.TopGainers1D {
			b.WriteString(fmt.Sprintf("| %s | %s | %.2f | %+.2f%% |\n",
				PerformanceEmoji(sd.Change1D), sd.Symbol, sd.LastClose, sd.Change1D))
		}
		b.WriteString("\n")
	}
	if len(report.TopLosers1D) > 0 {
		b.WriteString("### Losers\n")
		b.WriteString("| | Symbol | Close | 1D % |\n|---|--------|------:|-----:|\n")
		for _, sd := range report.TopLosers1D {
			b.WriteString(fmt.Sprintf("| %s | %s | %.2f | %+.2f%% |\n",
				PerformanceEmoji(sd.Change1D), sd.Symbol, sd.LastClose, sd.Change1D))
		}
		b.WriteString("\n")
	}

	// Per-section tables.
	for _, sec := range report.Sections {
		b.WriteString(fmt.Sprintf("## %s\n\n", sec.Name))
		if sec.Description != "" {
			b.WriteString(fmt.Sprintf("*%s*\n\n", sec.Description))
		}
		if sec.Summary != "" {
			b.WriteString(sec.Summary + "\n\n")
		}
		b.WriteString("| | Symbol | Close | 1D %% | 5D %% | 1M %% | YTD %% | Volume |\n")
		b.WriteString("|---|--------|------:|------:|------:|------:|-------:|-------:|\n")
		for _, sd := range sec.Data {
			if sd.Error != "" {
				b.WriteString(fmt.Sprintf("| ⚠️ | %s | — | — | — | — | — | *%s* |\n", sd.Symbol, sd.Error))
			} else {
				b.WriteString(fmt.Sprintf("| %s | %s | %.2f | %+.2f%% | %+.2f%% | %+.2f%% | %+.2f%% | %d |\n",
					PerformanceEmoji(sd.Change1D), sd.Symbol, sd.LastClose,
					sd.Change1D, sd.Change5D, sd.Change1M, sd.ChangeYTD, sd.Volume))
			}
		}
		b.WriteString("\n")

		if sec.Narrative != "" {
			b.WriteString(sec.Narrative + "\n\n")
		}
	}

	// Macro view.
	if report.MacroView != "" {
		b.WriteString("## 🌍 Macro Overview\n\n")
		b.WriteString(report.MacroView + "\n\n")
	}

	b.WriteString("---\n\n*© 2026-2027 Elko AI Financial. All rights reserved.*\n")

	return b.String()
}

// RenderHTML produces a standalone HTML report.
func RenderHTML(report *DailyReport) (string, error) {
	var buf bytes.Buffer
	if err := htmlTmpl.Execute(&buf, report); err != nil {
		return "", fmt.Errorf("render HTML: %w", err)
	}
	return buf.String(), nil
}
