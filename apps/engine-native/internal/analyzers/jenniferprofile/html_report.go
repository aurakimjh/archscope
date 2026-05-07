package jenniferprofile

import (
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// RenderHTMLReport produces a self-contained HTML view of the
// AnalysisResult per spec §23.3 — file summary, error summary,
// signature statistics, top slow root response, top network gap
// edges, top SQL execute, top fetch, parallelism table.
//
// MVP4 keeps the styling minimal (inline CSS) so the report is one
// file with no external assets. Open in any modern browser.
func RenderHTMLReport(result models.AnalysisResult) string {
	var b strings.Builder
	b.WriteString(htmlHead())
	b.WriteString("<body>\n")

	fmt.Fprintf(&b, "<h1>%s</h1>\n", html.EscapeString("ArchScope — Jennifer MSA Timeline Report"))
	if result.CreatedAt != "" {
		fmt.Fprintf(&b, "<p class=\"meta\">Generated %s</p>\n", html.EscapeString(result.CreatedAt))
	}

	renderSummarySection(&b, result.Summary)
	renderFileSummary(&b, getMapSlice(result.Series, "file_summary"))
	renderFileErrors(&b, getMapSlice(result.Tables, "file_errors"))
	renderSignatureStats(&b, getMapSlice(result.Series, "signature_statistics"))
	renderGuidGroups(&b, getMapSlice(result.Series, "guid_groups"))
	renderTopRoots(&b, getMapSlice(result.Series, "guid_groups"))
	renderTopGaps(&b, getMapSlice(result.Tables, "msa_edges"))
	renderProfileTable(&b, getMapSlice(result.Tables, "profiles"))

	b.WriteString("</body></html>\n")
	return b.String()
}

func htmlHead() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Jennifer MSA Timeline Report</title>
<style>
:root { --bg: #f8fafc; --card: #ffffff; --border: #e2e8f0; --text: #0f172a; --muted: #64748b; --accent: #4f46e5; --danger: #dc2626; --warn: #d97706; }
body { background: var(--bg); color: var(--text); font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", sans-serif; margin: 24px; font-size: 13px; line-height: 1.5; }
h1 { font-size: 22px; margin: 0 0 8px; }
h2 { font-size: 16px; margin: 28px 0 8px; padding-bottom: 4px; border-bottom: 1px solid var(--border); }
.meta { color: var(--muted); margin: 0 0 24px; font-size: 12px; }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 8px; margin-bottom: 12px; }
.card { background: var(--card); border: 1px solid var(--border); border-radius: 6px; padding: 10px 12px; }
.card .label { font-size: 11px; text-transform: uppercase; letter-spacing: 0.04em; color: var(--muted); margin-bottom: 2px; }
.card .value { font-size: 18px; font-weight: 600; font-variant-numeric: tabular-nums; }
table { width: 100%; border-collapse: collapse; background: var(--card); border: 1px solid var(--border); border-radius: 6px; overflow: hidden; font-size: 12px; }
thead tr { background: #f1f5f9; }
th, td { padding: 6px 10px; text-align: left; border-bottom: 1px solid var(--border); }
th { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--muted); }
td.num, th.num { text-align: right; font-variant-numeric: tabular-nums; }
.mono { font-family: "JetBrains Mono", Consolas, monospace; font-size: 11px; }
.pill { display: inline-block; padding: 1px 6px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.pill.ok { background: rgba(79, 70, 229, 0.12); color: var(--accent); }
.pill.danger { background: rgba(220, 38, 38, 0.12); color: var(--danger); }
.pill.warn { background: rgba(217, 119, 6, 0.12); color: var(--warn); }
.empty { color: var(--muted); font-style: italic; }
</style>
</head>
`
}

func renderSummarySection(b *strings.Builder, summary map[string]any) {
	b.WriteString("<h2>Summary</h2>\n<div class=\"cards\">\n")
	cards := []struct {
		label string
		key   string
	}{
		{"Total files", "total_files"},
		{"Total profiles", "total_profiles"},
		{"Full profiles", "full_profile_count"},
		{"GUID groups", "guid_group_count"},
		{"Signatures", "unique_signature_count"},
		{"Matched edges", "matched_external_call_count"},
		{"Unmatched edges", "unmatched_external_call_count"},
		{"Net gap (cum ms)", "total_network_gap_cum_ms"},
		{"SQL exec ms", "sql_execute_cum_ms"},
		{"Fetch rows", "fetch_total_rows"},
	}
	for _, c := range cards {
		v := summary[c.key]
		fmt.Fprintf(b, "<div class=\"card\"><div class=\"label\">%s</div><div class=\"value\">%s</div></div>\n",
			html.EscapeString(c.label), html.EscapeString(fmtScalar(v)))
	}
	b.WriteString("</div>\n")
}

func renderFileSummary(b *strings.Builder, rows []map[string]any) {
	if len(rows) == 0 {
		return
	}
	b.WriteString("<h2>Files</h2>\n<table><thead><tr>")
	headers := []string{"Source file", "Declared", "Detected", "Profiles"}
	for _, h := range headers {
		fmt.Fprintf(b, "<th>%s</th>", html.EscapeString(h))
	}
	b.WriteString("</tr></thead><tbody>\n")
	for _, r := range rows {
		fmt.Fprintf(b, "<tr><td class=\"mono\">%s</td><td class=\"num\">%s</td><td class=\"num\">%s</td><td class=\"num\">%s</td></tr>\n",
			html.EscapeString(fmtScalar(r["source_file"])),
			html.EscapeString(fmtScalar(r["declared_transaction_count"])),
			html.EscapeString(fmtScalar(r["detected_transaction_count"])),
			html.EscapeString(fmtScalar(r["profile_count"])),
		)
	}
	b.WriteString("</tbody></table>\n")
}

func renderFileErrors(b *strings.Builder, rows []map[string]any) {
	if len(rows) == 0 {
		return
	}
	b.WriteString("<h2>File errors</h2>\n<table><thead><tr><th>Source</th><th>Code</th><th>Message</th></tr></thead><tbody>\n")
	for _, r := range rows {
		fmt.Fprintf(b, "<tr><td class=\"mono\">%s</td><td><span class=\"pill danger\">%s</span></td><td>%s</td></tr>\n",
			html.EscapeString(fmtScalar(r["source_file"])),
			html.EscapeString(fmtScalar(r["code"])),
			html.EscapeString(fmtScalar(r["message"])),
		)
	}
	b.WriteString("</tbody></table>\n")
}

func renderSignatureStats(b *strings.Builder, rows []map[string]any) {
	if len(rows) == 0 {
		return
	}
	b.WriteString("<h2>MSA timeline signatures</h2>\n<table><thead><tr>")
	for _, h := range []string{"Hash", "Root", "Edges", "Samples", "Resp avg", "Resp p95", "Ext avg", "Ext p95", "Wall p95", "Gap avg", "Gap p95", "Mode"} {
		fmt.Fprintf(b, "<th class=\"num\">%s</th>", html.EscapeString(h))
	}
	b.WriteString("</tr></thead><tbody>\n")
	for _, s := range rows {
		hash := fmtScalar(s["signature_hash"])
		short := hash
		if len(short) > 12 {
			short = "…" + short[len(short)-12:]
		}
		m := getMap(s, "metrics")
		fmt.Fprintf(b, "<tr>")
		fmt.Fprintf(b, "<td class=\"mono\" title=\"%s\">%s</td>", html.EscapeString(hash), html.EscapeString(short))
		fmt.Fprintf(b, "<td class=\"mono\">%s</td>", html.EscapeString(fmtScalar(s["root_application"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtScalar(s["edge_count"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtScalar(s["sample_count"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtStat(getMap(m, "root_response_time_ms"), "avg")))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtStat(getMap(m, "root_response_time_ms"), "p95")))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtStat(getMap(m, "total_external_call_cumulative_ms"), "avg")))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtStat(getMap(m, "total_external_call_cumulative_ms"), "p95")))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtStat(getMap(m, "total_external_call_wall_time_ms"), "p95")))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtStat(getMap(m, "total_network_gap_cumulative_ms"), "avg")))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtStat(getMap(m, "total_network_gap_cumulative_ms"), "p95")))
		// Pick the dominant mode by inspecting parallelism ratio.
		ratioMap := getMap(m, "external_call_parallelism_ratio")
		ratioP95 := floatOf(ratioMap["p95"])
		mode := "SEQUENTIAL"
		pillClass := "ok"
		if ratioP95 > 1.05 {
			mode = "PARALLEL"
			pillClass = "warn"
		}
		fmt.Fprintf(b, "<td><span class=\"pill %s\">%s</span></td>", pillClass, mode)
		fmt.Fprintf(b, "</tr>\n")
	}
	b.WriteString("</tbody></table>\n")
}

func renderGuidGroups(b *strings.Builder, rows []map[string]any) {
	if len(rows) == 0 {
		return
	}
	b.WriteString("<h2>GUID groups (parallelism)</h2>\n<table><thead><tr>")
	for _, h := range []string{"GUID", "Root", "Profiles", "Cumulative ms", "Wall-clock ms", "Ratio", "Concurrency", "Mode"} {
		fmt.Fprintf(b, "<th>%s</th>", html.EscapeString(h))
	}
	b.WriteString("</tr></thead><tbody>\n")
	for _, r := range rows {
		guid := fmtScalar(r["guid"])
		short := guid
		if len(short) > 16 {
			short = "…" + short[len(short)-16:]
		}
		m := getMap(r, "metrics")
		mode := fmtScalar(m["group_execution_mode"])
		pillClass := "ok"
		switch mode {
		case "PARALLEL":
			pillClass = "warn"
		case "MIXED":
			pillClass = "warn"
		}
		fmt.Fprintf(b, "<tr>")
		fmt.Fprintf(b, "<td class=\"mono\" title=\"%s\">%s</td>", html.EscapeString(guid), html.EscapeString(short))
		fmt.Fprintf(b, "<td class=\"mono\">%s</td>", html.EscapeString(fmtScalar(r["root_application"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtScalar(r["profile_count"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtScalar(m["total_external_call_cumulative_ms"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtScalar(m["total_external_call_wall_time_ms"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtRatio(m["external_call_parallelism_ratio"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtScalar(m["max_external_call_concurrency"])))
		fmt.Fprintf(b, "<td><span class=\"pill %s\">%s</span></td>", pillClass, html.EscapeString(mode))
		fmt.Fprintf(b, "</tr>\n")
	}
	b.WriteString("</tbody></table>\n")
}

func renderTopRoots(b *strings.Builder, rows []map[string]any) {
	if len(rows) == 0 {
		return
	}
	type entry struct {
		root   string
		guid   string
		respMs int
	}
	all := []entry{}
	for _, r := range rows {
		rt := 0
		if v, ok := r["root_response_time_ms"]; ok {
			rt = numberOf(v)
		}
		all = append(all, entry{
			root:   fmtScalar(r["root_application"]),
			guid:   fmtScalar(r["guid"]),
			respMs: rt,
		})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].respMs > all[j].respMs })
	if len(all) > 10 {
		all = all[:10]
	}
	b.WriteString("<h2>Top slow root response</h2>\n<table><thead><tr><th>Root</th><th>GUID</th><th class=\"num\">Response ms</th></tr></thead><tbody>\n")
	for _, e := range all {
		fmt.Fprintf(b, "<tr><td class=\"mono\">%s</td><td class=\"mono\">%s</td><td class=\"num\">%d</td></tr>\n",
			html.EscapeString(e.root), html.EscapeString(e.guid), e.respMs)
	}
	b.WriteString("</tbody></table>\n")
}

func renderTopGaps(b *strings.Builder, rows []map[string]any) {
	if len(rows) == 0 {
		return
	}
	type entry struct {
		caller string
		callee string
		gap    int
	}
	matched := []entry{}
	for _, r := range rows {
		if fmtScalar(r["match_status"]) != "MATCHED" {
			continue
		}
		matched = append(matched, entry{
			caller: fmtScalar(r["caller_application"]),
			callee: fmtScalar(r["callee_application"]),
			gap:    numberOf(r["adjusted_network_gap_ms"]),
		})
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].gap > matched[j].gap })
	if len(matched) > 10 {
		matched = matched[:10]
	}
	if len(matched) == 0 {
		return
	}
	b.WriteString("<h2>Top network gap edges</h2>\n<table><thead><tr><th>Caller</th><th>Callee</th><th class=\"num\">Adj gap ms</th></tr></thead><tbody>\n")
	for _, e := range matched {
		fmt.Fprintf(b, "<tr><td class=\"mono\">%s</td><td class=\"mono\">%s</td><td class=\"num\">%d</td></tr>\n",
			html.EscapeString(e.caller), html.EscapeString(e.callee), e.gap)
	}
	b.WriteString("</tbody></table>\n")
}

func renderProfileTable(b *strings.Builder, rows []map[string]any) {
	if len(rows) == 0 {
		return
	}
	b.WriteString("<h2>Transaction profiles</h2>\n<table><thead><tr>")
	for _, h := range []string{"TXID", "Application", "Resp ms", "SQL ms", "Check ms", "2PC ms", "Fetch ms", "Ext call", "Issues"} {
		fmt.Fprintf(b, "<th>%s</th>", html.EscapeString(h))
	}
	b.WriteString("</tr></thead><tbody>\n")
	for _, r := range rows {
		bm := getMap(r, "body_metrics")
		hdr := getMap(r, "header")
		errCount := lenOf(r["errors"])
		warnCount := lenOf(r["warnings"])
		issues := errCount + warnCount
		fmt.Fprintf(b, "<tr>")
		fmt.Fprintf(b, "<td class=\"mono\">%s</td>", html.EscapeString(fmtScalar(r["txid"])))
		fmt.Fprintf(b, "<td class=\"mono\">%s</td>", html.EscapeString(fmtScalar(r["application"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtScalar(hdr["response_time_ms"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtScalar(bm["sql_execute_cum_ms"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtScalar(bm["check_query_cum_ms"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtScalar(bm["two_pc_cum_ms"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtScalar(bm["fetch_cum_ms"])))
		fmt.Fprintf(b, "<td class=\"num\">%s</td>", html.EscapeString(fmtScalar(bm["external_call_count"])))
		if issues > 0 {
			fmt.Fprintf(b, "<td><span class=\"pill danger\">%d</span></td>", issues)
		} else {
			fmt.Fprintf(b, "<td>—</td>")
		}
		fmt.Fprintf(b, "</tr>\n")
	}
	b.WriteString("</tbody></table>\n")
}

// ── helpers ─────────────────────────────────────────────────────────

func getMapSlice(src map[string]any, key string) []map[string]any {
	if v, ok := src[key]; ok {
		switch vt := v.(type) {
		case []map[string]any:
			return vt
		case []any:
			out := make([]map[string]any, 0, len(vt))
			for _, e := range vt {
				if m, ok := e.(map[string]any); ok {
					out = append(out, m)
				}
			}
			return out
		}
	}
	return nil
}

func getMap(src map[string]any, key string) map[string]any {
	if src == nil {
		return nil
	}
	if v, ok := src[key]; ok {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}

func fmtScalar(v any) string {
	if v == nil {
		return "—"
	}
	switch t := v.(type) {
	case string:
		if t == "" {
			return "—"
		}
		return t
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%.2f", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	}
	return fmt.Sprintf("%v", v)
}

func fmtStat(stat map[string]any, key string) string {
	if stat == nil {
		return "—"
	}
	v, ok := stat[key]
	if !ok {
		return "—"
	}
	switch t := v.(type) {
	case float64:
		if t == 0 {
			return "0"
		}
		return fmt.Sprintf("%d", int64(t))
	case int:
		return fmt.Sprintf("%d", t)
	}
	return fmtScalar(v)
}

func fmtRatio(v any) string {
	if v == nil {
		return "—"
	}
	if f, ok := v.(float64); ok {
		return fmt.Sprintf("%.2f", f)
	}
	return fmtScalar(v)
}

func numberOf(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	}
	return 0
}

func floatOf(v any) float64 {
	switch t := v.(type) {
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case float64:
		return t
	}
	return 0
}

func lenOf(v any) int {
	switch t := v.(type) {
	case []any:
		return len(t)
	case []map[string]any:
		return len(t)
	}
	return 0
}
