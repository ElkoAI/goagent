package reporting

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// RenderMarkdown produces a complete markdown report.
func RenderMarkdown(report *DailyReport) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Daily Market Report — %s\n\n", report.Date))
	b.WriteString(fmt.Sprintf("*Generated: %s | Timezone: %s*\n\n", report.Generated, report.Timezone))

	// Top movers summary.
	b.WriteString("## Top Movers (1D)\n\n")
	if len(report.TopGainers1D) > 0 {
		b.WriteString("### Gainers\n")
		b.WriteString("| Symbol | Close | 1D % |\n|--------|------:|-----:|\n")
		for _, sd := range report.TopGainers1D {
			b.WriteString(fmt.Sprintf("| %s | %.2f | %+.2f%% |\n", sd.Symbol, sd.LastClose, sd.Change1D))
		}
		b.WriteString("\n")
	}
	if len(report.TopLosers1D) > 0 {
		b.WriteString("### Losers\n")
		b.WriteString("| Symbol | Close | 1D % |\n|--------|------:|-----:|\n")
		for _, sd := range report.TopLosers1D {
			b.WriteString(fmt.Sprintf("| %s | %.2f | %+.2f%% |\n", sd.Symbol, sd.LastClose, sd.Change1D))
		}
		b.WriteString("\n")
	}

	// Per-section tables.
	for _, sec := range report.Sections {
		b.WriteString(fmt.Sprintf("## %s\n\n", sec.Name))
		b.WriteString("| Symbol | Close | 1D %% | 5D %% | 1M %% | YTD %% | Volume |\n")
		b.WriteString("|--------|------:|------:|------:|------:|-------:|-------:|\n")
		for _, sd := range sec.Data {
			if sd.Error != "" {
				b.WriteString(fmt.Sprintf("| %s | — | — | — | — | — | *%s* |\n", sd.Symbol, sd.Error))
			} else {
				b.WriteString(fmt.Sprintf("| %s | %.2f | %+.2f%% | %+.2f%% | %+.2f%% | %+.2f%% | %d |\n",
					sd.Symbol, sd.LastClose, sd.Change1D, sd.Change5D, sd.Change1M, sd.ChangeYTD, sd.Volume))
			}
		}
		b.WriteString("\n")

		if sec.Narrative != "" {
			b.WriteString(sec.Narrative + "\n\n")
		}
	}

	// Macro view.
	if report.MacroView != "" {
		b.WriteString("## Macro Overview\n\n")
		b.WriteString(report.MacroView + "\n")
	}

	return b.String()
}

var htmlTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"pctClass": func(v float64) string {
		if v > 0 {
			return "pos"
		}
		if v < 0 {
			return "neg"
		}
		return ""
	},
	"fmtPct": func(v float64) string {
		return fmt.Sprintf("%+.2f%%", v)
	},
	"fmtPrice": func(v float64) string {
		return fmt.Sprintf("%.2f", v)
	},
	"rawSVG": func(s string) template.HTML {
		return template.HTML(s)
	},
}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Daily Market Report — {{.Date}}</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; max-width: 960px; margin: 0 auto; padding: 20px; color: #1a1a1a; background: #fafafa; }
  h1 { font-size: 1.6em; margin-bottom: 4px; }
  h2 { font-size: 1.2em; margin: 24px 0 8px; border-bottom: 1px solid #ddd; padding-bottom: 4px; }
  h3 { font-size: 1em; margin: 12px 0 4px; }
  .meta { color: #666; font-size: 0.85em; margin-bottom: 16px; }
  table { border-collapse: collapse; width: 100%; margin-bottom: 12px; font-size: 0.9em; }
  th { background: #f0f0f0; text-align: left; padding: 6px 10px; border-bottom: 2px solid #ccc; }
  td { padding: 5px 10px; border-bottom: 1px solid #eee; }
  tr:nth-child(even) td { background: #f8f8f8; }
  .num { text-align: right; font-variant-numeric: tabular-nums; }
  .pos { color: #16a34a; }
  .neg { color: #dc2626; }
  .narrative { margin: 8px 0 16px; line-height: 1.5; white-space: pre-wrap; }
  .macro { margin: 8px 0; line-height: 1.6; white-space: pre-wrap; }
  .error { color: #999; font-style: italic; }
  .spark { text-align: center; padding: 2px 4px; }
  .spark svg { vertical-align: middle; }
  @media print { body { max-width: none; } }
</style>
</head>
<body>
<h1>Daily Market Report — {{.Date}}</h1>
<p class="meta">Generated: {{.Generated}} | Timezone: {{.Timezone}}</p>

{{if .TopGainers1D}}
<h2>Top Gainers (1D)</h2>
<table>
<tr><th>Symbol</th><th class="num">Close</th><th class="num">1D %</th></tr>
{{range .TopGainers1D}}
<tr><td>{{.Symbol}}</td><td class="num">{{fmtPrice .LastClose}}</td><td class="num pos">{{fmtPct .Change1D}}</td></tr>
{{end}}
</table>
{{end}}

{{if .TopLosers1D}}
<h2>Top Losers (1D)</h2>
<table>
<tr><th>Symbol</th><th class="num">Close</th><th class="num">1D %</th></tr>
{{range .TopLosers1D}}
<tr><td>{{.Symbol}}</td><td class="num">{{fmtPrice .LastClose}}</td><td class="num neg">{{fmtPct .Change1D}}</td></tr>
{{end}}
</table>
{{end}}

{{range .Sections}}
<h2>{{.Name}}</h2>
<table>
<tr><th>Symbol</th><th class="spark">5D</th><th class="num">Close</th><th class="num">1D %</th><th class="num">5D %</th><th class="num">1M %</th><th class="num">YTD %</th><th class="num">Volume</th></tr>
{{range .Data}}
{{if .Error}}
<tr><td>{{.Symbol}}</td><td colspan="7" class="error">{{.Error}}</td></tr>
{{else}}
<tr>
  <td>{{.Symbol}}</td>
  <td class="spark">{{if .Sparkline}}{{rawSVG .Sparkline}}{{end}}</td>
  <td class="num">{{fmtPrice .LastClose}}</td>
  <td class="num {{pctClass .Change1D}}">{{fmtPct .Change1D}}</td>
  <td class="num {{pctClass .Change5D}}">{{fmtPct .Change5D}}</td>
  <td class="num {{pctClass .Change1M}}">{{fmtPct .Change1M}}</td>
  <td class="num {{pctClass .ChangeYTD}}">{{fmtPct .ChangeYTD}}</td>
  <td class="num">{{.Volume}}</td>
</tr>
{{end}}
{{end}}
</table>
{{if .Narrative}}<div class="narrative">{{.Narrative}}</div>{{end}}
{{end}}

{{if .MacroView}}
<h2>Macro Overview</h2>
<div class="macro">{{.MacroView}}</div>
{{end}}

</body>
</html>`))

// RenderHTML produces a standalone HTML report.
func RenderHTML(report *DailyReport) (string, error) {
	var buf bytes.Buffer
	if err := htmlTmpl.Execute(&buf, report); err != nil {
		return "", fmt.Errorf("render HTML: %w", err)
	}
	return buf.String(), nil
}
