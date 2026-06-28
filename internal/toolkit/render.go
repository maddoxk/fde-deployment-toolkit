package toolkit

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"strings"
)

// RenderStatusPage writes a self-contained static HTML status page for the
// report to path. It embeds inline SVG sparkline charts, metric cards and a
// checks table -- no external JS/CSS dependencies.
func RenderStatusPage(rep StatusReport, path string) error {
	funcs := template.FuncMap{
		"reqPath": func() template.HTML { return sparkPathInt(rep.Metrics.RequestSeries) },
		"latPath": func() template.HTML { return sparkPathFloat(rep.Metrics.LatencySeries) },
		"badge":   badgeClass,
		"checkRow": func(b bool) string {
			if b {
				return "pass"
			}
			return "fail"
		},
		"checkTxt": func(b bool) string {
			if b {
				return "PASS"
			}
			return "FAIL"
		},
	}
	tmpl, err := template.New("status").Funcs(funcs).Parse(statusTmpl)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, rep); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func badgeClass(s string) string {
	switch s {
	case "healthy":
		return "ok"
	case "degraded":
		return "warn"
	default:
		return "down"
	}
}

// sparkPathInt builds an SVG polyline points string scaled into a 300x70 box.
func sparkPathInt(series []int) template.HTML {
	f := make([]float64, len(series))
	for i, v := range series {
		f[i] = float64(v)
	}
	return sparkPathFloat(f)
}

func sparkPathFloat(series []float64) template.HTML {
	if len(series) == 0 {
		return template.HTML("")
	}
	w, h := 300.0, 70.0
	min, max := series[0], series[0]
	for _, v := range series {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	if span == 0 {
		span = 1
	}
	var pts []string
	n := len(series)
	for i, v := range series {
		x := float64(i) / float64(n-1) * w
		y := h - ((v-min)/span)*(h-8) - 4
		pts = append(pts, fmt.Sprintf("%.1f,%.1f", x, y))
	}
	return template.HTML(strings.Join(pts, " "))
}

const statusTmpl = `<!DOCTYPE html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Service.Customer.Name}} — Live Status | fdetk</title>
<style>
:root{--bg:#0d1117;--card:#161b22;--bd:#30363d;--fg:#e6edf3;--mut:#8b949e;--ok:#3fb950;--warn:#d29922;--down:#f85149;--ac:#58a6ff}
*{box-sizing:border-box}body{margin:0;font:15px/1.5 -apple-system,Segoe UI,Roboto,sans-serif;background:var(--bg);color:var(--fg)}
.wrap{max-width:980px;margin:0 auto;padding:28px 18px}
header{display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:12px;margin-bottom:8px}
h1{font-size:22px;margin:0}.sub{color:var(--mut);font-size:13px}
.badge{padding:5px 14px;border-radius:999px;font-weight:600;font-size:13px;text-transform:uppercase;letter-spacing:.04em}
.badge.ok{background:rgba(63,185,80,.15);color:var(--ok);border:1px solid var(--ok)}
.badge.warn{background:rgba(210,153,34,.15);color:var(--warn);border:1px solid var(--warn)}
.badge.down{background:rgba(248,81,73,.15);color:var(--down);border:1px solid var(--down)}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px;margin:18px 0}
.card{background:var(--card);border:1px solid var(--bd);border-radius:10px;padding:14px 16px}
.card .k{color:var(--mut);font-size:12px;text-transform:uppercase;letter-spacing:.05em}
.card .v{font-size:26px;font-weight:700;margin-top:4px}
.charts{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin:18px 0}
@media(max-width:680px){.charts{grid-template-columns:1fr}}
.chart{background:var(--card);border:1px solid var(--bd);border-radius:10px;padding:14px 16px}
.chart h3{margin:0 0 8px;font-size:13px;color:var(--mut);font-weight:600;text-transform:uppercase;letter-spacing:.05em}
svg{width:100%;height:auto;display:block}
table{width:100%;border-collapse:collapse;background:var(--card);border:1px solid var(--bd);border-radius:10px;overflow:hidden;margin-top:8px}
th,td{text-align:left;padding:10px 14px;border-bottom:1px solid var(--bd);font-size:14px}
th{color:var(--mut);font-size:12px;text-transform:uppercase;letter-spacing:.04em}
tr:last-child td{border-bottom:none}
.pill{padding:2px 9px;border-radius:6px;font-size:12px;font-weight:600}
.pill.pass{background:rgba(63,185,80,.15);color:var(--ok)}
.pill.fail{background:rgba(248,81,73,.15);color:var(--down)}
.kind{color:var(--mut);font-size:12px}
.meta{color:var(--mut);font-size:12px;margin-top:24px;border-top:1px solid var(--bd);padding-top:12px}
a{color:var(--ac)}
h2{font-size:15px;margin:24px 0 4px;text-transform:uppercase;letter-spacing:.05em;color:var(--mut)}
</style></head><body><div class="wrap">
<header>
<div><h1>{{.Service.Customer.Name}} · {{.Service.ServiceName}}</h1>
<div class="sub">{{.Service.Customer.Environment}} · {{.Service.Customer.Region}} · tier {{.Service.Customer.Tier}} · {{.Service.Customer.Replicas}} replicas</div></div>
<span class="badge {{badge .OverallHealth}}">{{.OverallHealth}}</span>
</header>
<div class="grid">
<div class="card"><div class="k">Uptime (24h)</div><div class="v">{{printf "%.2f" .Metrics.UptimePct}}%</div></div>
<div class="card"><div class="k">Requests (24h)</div><div class="v">{{.Metrics.RequestCount}}</div></div>
<div class="card"><div class="k">Error rate</div><div class="v">{{printf "%.2f" .Metrics.ErrorRatePct}}%</div></div>
<div class="card"><div class="k">RPS (avg)</div><div class="v">{{printf "%.2f" .Metrics.RPS}}</div></div>
<div class="card"><div class="k">p50 latency</div><div class="v">{{printf "%.1f" .Metrics.P50LatencyMs}}ms</div></div>
<div class="card"><div class="k">p95 latency</div><div class="v">{{printf "%.1f" .Metrics.P95LatencyMs}}ms</div></div>
<div class="card"><div class="k">p99 latency</div><div class="v">{{printf "%.1f" .Metrics.P99LatencyMs}}ms</div></div>
<div class="card"><div class="k">Checks</div><div class="v">{{.ChecksPassed}}/{{.ChecksTotal}}</div></div>
</div>
<div class="charts">
<div class="chart"><h3>Requests / hour (24h)</h3>
<svg viewBox="0 0 300 70" preserveAspectRatio="none"><polyline fill="none" stroke="#58a6ff" stroke-width="2" points="{{reqPath}}"/></svg></div>
<div class="chart"><h3>Latency p50 / hour (ms)</h3>
<svg viewBox="0 0 300 70" preserveAspectRatio="none"><polyline fill="none" stroke="#3fb950" stroke-width="2" points="{{latPath}}"/></svg></div>
</div>
<h2>Check results</h2>
<table><thead><tr><th>Check</th><th>Kind</th><th>Target</th><th>Latency</th><th>Result</th></tr></thead><tbody>
{{range .Checks}}<tr>
<td>{{.Name}}<div class="kind">{{.Detail}}</div></td>
<td class="kind">{{.Kind}}</td>
<td class="kind">{{.Target}}</td>
<td>{{printf "%.1f" .LatencyMs}}ms</td>
<td><span class="pill {{checkRow .Passed}}">{{checkTxt .Passed}}</span></td>
</tr>{{end}}
</tbody></table>
<div class="meta">Generated by <a href="https://github.com/maddoxk/fde-deployment-toolkit">fdetk</a> · {{.GeneratedAt.Format "2006-01-02 15:04:05 UTC"}} · image {{.Service.Image}}</div>
</div></body></html>`
