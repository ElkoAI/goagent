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

var htmlTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
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
}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} — {{.Date}} ({{.Generated}})</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; max-width: 1000px; margin: 0 auto; padding: 20px; color: #1a1a1a; background: #fafafa; }
  h1 { font-size: 1.6em; margin-bottom: 4px; }
  h2 { font-size: 1.2em; margin: 28px 0 8px; border-bottom: 2px solid #e0e0e0; padding-bottom: 6px; }
  .meta { color: #666; font-size: 0.85em; margin-bottom: 8px; }
  .intro { font-size: 0.92em; margin: 8px 0 16px; padding: 14px 18px; background: #f8f8f2; border-left: 4px solid #c8a850; border-radius: 4px; color: #555; line-height: 1.6; }
  .summary { font-size: 1.05em; margin: 8px 0 20px; padding: 12px 16px; background: #f0f4ff; border-left: 4px solid #4a7dff; border-radius: 4px; white-space: pre-line; line-height: 1.6; }
  .at-a-glance { margin: 0 0 20px; padding: 0; }
  .at-a-glance li { list-style: none; padding: 8px 0; border-bottom: 1px solid #eee; font-size: 0.95em; }
  .at-a-glance li:last-child { border-bottom: none; }
  .at-a-glance .sec-name { font-weight: 600; }
  .at-a-glance .sec-summary { color: #555; }
  .at-a-glance .sec-about { display: block; color: #888; font-size: 0.85em; font-style: italic; margin-top: 2px; }
  .sec-desc { color: #666; font-size: 0.88em; font-style: italic; margin: 2px 0 6px; }
  .sec-summary-line { font-size: 0.95em; margin: 4px 0 10px; }
  table { border-collapse: collapse; width: 100%; margin-bottom: 12px; font-size: 0.88em; }
  th { background: #f0f0f0; text-align: left; padding: 7px 10px; border-bottom: 2px solid #ccc; font-weight: 600; font-size: 0.85em; text-transform: uppercase; letter-spacing: 0.03em; color: #555; user-select: none; }
  th.sortable { cursor: pointer; position: relative; padding-right: 18px; }
  th.sortable:hover { background: #e4e4e4; }
  th.sortable::after { content: "⇅"; position: absolute; right: 4px; top: 50%; transform: translateY(-50%); font-size: 0.8em; color: #bbb; }
  th.sortable.asc::after { content: "▲"; color: #555; }
  th.sortable.desc::after { content: "▼"; color: #555; }
  td { padding: 6px 10px; border-bottom: 1px solid #eee; }
  tr:nth-child(even) td { background: #f8f8f8; }
  tr:hover td { background: #f0f0f0; }
  .num { text-align: right; font-variant-numeric: tabular-nums; }
  .pos { color: #16a34a; font-weight: 500; }
  .neg { color: #dc2626; font-weight: 500; }
  .flat { color: #888; }
  .sym-up { color: #16a34a; font-weight: 600; }
  .sym-down { color: #dc2626; font-weight: 600; }
  .sym-flat { color: #555; font-weight: 600; }
  .sym-up a, .sym-down a, .sym-flat a { color: inherit; text-decoration: none; border-bottom: 1px dotted currentColor; }
  .sym-up a:hover, .sym-down a:hover, .sym-flat a:hover { border-bottom-style: solid; }
  .emoji { text-align: center; font-size: 1.1em; width: 30px; }
  .narrative { margin: 10px 0 16px; padding: 12px 16px; line-height: 1.6; white-space: pre-wrap; background: #fafff5; border-left: 3px solid #c0d8a0; border-radius: 4px; font-size: 0.92em; }
  .macro { margin: 10px 0; padding: 14px 18px; line-height: 1.7; white-space: pre-wrap; background: #f5f5ff; border-left: 3px solid #a0a0d8; border-radius: 4px; font-size: 0.95em; }
  .error { color: #999; font-style: italic; }
  .spark { text-align: center; padding: 2px 4px; }
  .spark svg { vertical-align: middle; }
  .vol { color: #888; font-size: 0.9em; }
  .movers { display: flex; gap: 20px; flex-wrap: wrap; }
  .movers table { flex: 1; min-width: 300px; }
  .footer { margin-top: 40px; padding: 16px 0; border-top: 2px solid #e0e0e0; text-align: center; color: #999; font-size: 0.8em; letter-spacing: 0.05em; }
  @media print { body { max-width: none; } .summary { background: #f8f8f8; } }
</style>
</head>
<body>
<h1>📊 {{.Title}} — {{.Date}} <span class="meta">({{.Generated}})</span></h1>

{{if .Intro}}<div class="intro">{{.Intro}}</div>{{end}}
{{if .Summary}}<div class="summary">{{.Summary}}</div>{{end}}

<h2>📋 At a Glance</h2>
<ul class="at-a-glance">
{{range .Sections}}
<li><span class="sec-name">{{.Name}}</span> — <span class="sec-summary">{{.Summary}}</span></li>
{{end}}
</ul>

<h2>🏆 Top Movers (1D)</h2>
<div class="movers">
{{if .TopGainers1D}}
<table>
<tr><th></th><th>Gainers</th><th class="num">Close</th><th class="num">1D %</th></tr>
{{range .TopGainers1D}}
<tr><td class="emoji">{{emoji .Change1D}}</td><td class="sym-up"><a href="{{yfinLink .Symbol}}" target="_blank">{{.Symbol}}</a></td><td class="num">{{fmtPrice .LastClose}}</td><td class="num pos">{{fmtPct .Change1D}}</td></tr>
{{end}}
</table>
{{end}}
{{if .TopLosers1D}}
<table>
<tr><th></th><th>Losers</th><th class="num">Close</th><th class="num">1D %</th></tr>
{{range .TopLosers1D}}
<tr><td class="emoji">{{emoji .Change1D}}</td><td class="sym-down"><a href="{{yfinLink .Symbol}}" target="_blank">{{.Symbol}}</a></td><td class="num">{{fmtPrice .LastClose}}</td><td class="num neg">{{fmtPct .Change1D}}</td></tr>
{{end}}
</table>
{{end}}
</div>

{{range .Sections}}
<h2>{{.Name}}</h2>
{{if .Description}}<p class="sec-desc">{{.Description}}</p>{{end}}
{{if .Summary}}<p class="sec-summary-line">{{.Summary}}</p>{{end}}
<table>
<tr><th></th><th class="sortable" data-col="1" data-type="text">Symbol</th><th class="spark">5 Day</th><th class="num sortable" data-col="3" data-type="num">Close</th><th class="num sortable" data-col="4" data-type="pct">1D %</th><th class="num sortable" data-col="5" data-type="pct">5D %</th><th class="num sortable" data-col="6" data-type="pct">1M %</th><th class="num sortable" data-col="7" data-type="pct">YTD %</th><th class="num sortable" data-col="8" data-type="vol">Vol</th></tr>
{{range .Data}}
{{if .Error}}
<tr><td class="emoji">⚠️</td><td>{{.Symbol}}</td><td colspan="7" class="error">{{.Error}}</td></tr>
{{else}}
<tr>
  <td class="emoji">{{emoji .Change1D}}</td>
  <td class="{{symClass .Change1D}}"><a href="{{yfinLink .Symbol}}" target="_blank">{{.Symbol}}</a></td>
  <td class="spark">{{if .Sparkline}}{{rawSVG .Sparkline}}{{end}}</td>
  <td class="num">{{fmtPrice .LastClose}}</td>
  <td class="num {{pctClass .Change1D}}">{{fmtPct .Change1D}}</td>
  <td class="num {{pctClass .Change5D}}">{{fmtPct .Change5D}}</td>
  <td class="num {{pctClass .Change1M}}">{{fmtPct .Change1M}}</td>
  <td class="num {{pctClass .ChangeYTD}}">{{fmtPct .ChangeYTD}}</td>
  <td class="num vol">{{fmtVol .Volume}}</td>
</tr>
{{end}}
{{end}}
</table>
{{if .Narrative}}<div class="narrative">{{.Narrative}}</div>{{end}}
{{end}}

{{if .MacroView}}
<h2>🌍 Macro Overview</h2>
<div class="macro">{{.MacroView}}</div>
{{end}}

<footer class="footer">© 2026-2027 Elko AI Financial. All rights reserved.</footer>

<script>
document.querySelectorAll('th.sortable').forEach(th => {
  th.addEventListener('click', function() {
    const table = this.closest('table');
    const col = parseInt(this.dataset.col);
    const type = this.dataset.type;
    const tbody = table.querySelector('tbody') || table;
    const rows = Array.from(tbody.querySelectorAll('tr')).filter(r => !r.querySelector('th'));
    const isAsc = this.classList.contains('asc');
    table.querySelectorAll('th.sortable').forEach(h => h.classList.remove('asc','desc'));
    this.classList.add(isAsc ? 'desc' : 'asc');
    const dir = isAsc ? -1 : 1;
    rows.sort((a, b) => {
      const ca = a.cells[col], cb = b.cells[col];
      if (!ca || !cb) return 0;
      let va, vb;
      if (type === 'text') {
        va = ca.textContent.trim(); vb = cb.textContent.trim();
        return dir * va.localeCompare(vb);
      }
      if (type === 'pct') {
        va = parseFloat(ca.textContent.replace(/[%,]/g,'')); vb = parseFloat(cb.textContent.replace(/[%,]/g,''));
      } else if (type === 'vol') {
        va = parseVol(ca.textContent.trim()); vb = parseVol(cb.textContent.trim());
      } else {
        va = parseFloat(ca.textContent.replace(/,/g,'')); vb = parseFloat(cb.textContent.replace(/,/g,''));
      }
      if (isNaN(va)) va = 0; if (isNaN(vb)) vb = 0;
      return dir * (va - vb);
    });
    rows.forEach(r => tbody.appendChild(r));
  });
});
function parseVol(s) {
  s = s.trim().toUpperCase();
  const n = parseFloat(s);
  if (s.endsWith('B')) return n * 1e9;
  if (s.endsWith('M')) return n * 1e6;
  if (s.endsWith('K')) return n * 1e3;
  return n || 0;
}
</script>

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
