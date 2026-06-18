package handler

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"marginalia/agent"
)

type eventRow struct {
	Time     string
	Kind     string
	Session  string
	CourseID string
	ToolName string
	Dur      string
	OK       string
}

type metricsData struct {
	Window  string
	Summary agent.EventSummary
	Events  []eventRow
}

var metricsTempl = template.Must(template.New("metrics").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Metrics</title>
<style>
body{font-family:monospace;max-width:960px;margin:2rem auto;padding:0 1rem}
h1{font-size:1.2rem}h2{font-size:1rem;margin-top:2rem;border-bottom:1px solid #ccc}
.windows a{margin-right:1rem}.windows a.active{font-weight:bold;text-decoration:none}
table{border-collapse:collapse;width:100%;font-size:.85rem}
th,td{border:1px solid #ddd;padding:4px 8px;text-align:left}th{background:#f5f5f5}
.stat{display:inline-block;margin-right:2rem}
</style>
</head>
<body>
<h1>Metrics</h1>
<div class="windows">
  <a href="?window=7d"{{if eq .Window "7d"}} class="active"{{end}}>7d</a>
  <a href="?window=30d"{{if eq .Window "30d"}} class="active"{{end}}>30d</a>
  <a href="?window=90d"{{if eq .Window "90d"}} class="active"{{end}}>90d</a>
</div>
<h2>Chat</h2>
<span class="stat">Turns: {{.Summary.TurnCount}}</span>
<span class="stat">Avg latency: {{.Summary.AvgLatencyMs}}ms</span>
<span class="stat">p95: {{.Summary.P95LatencyMs}}ms</span>
<span class="stat">Tokens in: {{.Summary.InputTokens}}</span>
<span class="stat">Tokens out: {{.Summary.OutputTokens}}</span>
<h2>Top Tools</h2>
{{if .Summary.ToolCounts}}<table><tr><th>Tool</th><th>Calls</th></tr>
{{range $n,$c := .Summary.ToolCounts}}<tr><td>{{$n}}</td><td>{{$c}}</td></tr>{{end}}
</table>{{else}}<p>No tool events.</p>{{end}}
<h2>Active Courses</h2>
{{if .Summary.CourseCounts}}<table><tr><th>Course</th><th>Sessions</th></tr>
{{range $c,$n := .Summary.CourseCounts}}<tr><td>{{$c}}</td><td>{{$n}}</td></tr>{{end}}
</table>{{else}}<p>No session events.</p>{{end}}
<h2>Plan Toggles</h2>
<span class="stat">Done: {{.Summary.PlanDone}}</span>
<span class="stat">Undone: {{.Summary.PlanUndone}}</span>
<h2>PDF Opens</h2>
<span class="stat">{{.Summary.PDFOpens}}</span>
<h2>Recent Events (last 200)</h2>
{{if .Events}}<table>
<tr><th>Time</th><th>Kind</th><th>Session</th><th>Course</th><th>Tool</th><th>Dur</th><th>OK</th></tr>
{{range .Events}}<tr><td>{{.Time}}</td><td>{{.Kind}}</td><td>{{.Session}}</td><td>{{.CourseID}}</td><td>{{.ToolName}}</td><td>{{.Dur}}</td><td>{{.OK}}</td></tr>{{end}}
</table>{{else}}<p>No events yet.</p>{{end}}
</body></html>`))

func (h *Handler) handleDebugMetrics(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}

	window := r.URL.Query().Get("window")
	if window != "7d" && window != "90d" {
		window = "30d"
	}
	days := map[string]int{"7d": 7, "30d": 30, "90d": 90}
	since := time.Now().Add(-time.Duration(days[window]) * 24 * time.Hour)

	summary, err := h.App.QueryEventSummary(since)
	if err != nil {
		writeServerError(w, "query event summary", err)
		return
	}

	rawEvs, err := h.App.ListRecentEvents(200)
	if err != nil {
		writeServerError(w, "list recent events", err)
		return
	}

	rows := make([]eventRow, len(rawEvs))
	for i, e := range rawEvs {
		row := eventRow{
			Time:     time.UnixMilli(e.CreatedAt).UTC().Format("01-02 15:04:05"),
			Kind:     e.Kind,
			CourseID: e.CourseID,
			ToolName: e.ToolName,
		}
		if e.SessionID != nil {
			row.Session = fmt.Sprintf("%d", *e.SessionID)
		}
		if e.DurationMs > 0 {
			row.Dur = fmt.Sprintf("%.1fs", float64(e.DurationMs)/1000)
		}
		if e.OK != nil {
			if *e.OK {
				row.OK = "✓"
			} else {
				row.OK = "✗"
			}
		}
		rows[i] = row
	}

	data := metricsData{Window: window, Summary: summary, Events: rows}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := metricsTempl.Execute(w, data); err != nil {
		slog.Warn("render metrics template", "err", err)
	}
}
