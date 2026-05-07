// Package demosite — index.html renderer. Mirrors `render_demo_index`
// and the per-row helpers (`_run_row`, `_link`, `_summary_cards`,
// `_analysis_summary_rows`, `_analysis_summaries`, `_key_metrics`,
// `_finding_label`, `_index_stylesheet`).
//
// Design note: the Python source assembles the page with f-strings +
// `html.escape`. We do the same with `strings.Builder` + html.Escape
// from the stdlib so every dynamic value passes through one escaper —
// no template engine in this file. The CSS is inlined verbatim.
package demosite

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// osReadFile is an alias used by readFile to keep that helper a
// single-arg call shape (mirrors Python `Path.read_text(path)`).
func osReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

// RenderDemoIndex returns the per-scenario index.html body. Mirrors
// `render_demo_index`.
func RenderDemoIndex(
	scenario, dataSource string,
	manifest map[string]any,
	runs []DemoAnalyzerRun,
	skippedFiles, referenceFiles []map[string]any,
	comparisonPaths []string,
) string {
	summaries := analysisSummaries(runs)
	cards := summaryCards(runs, skippedFiles, referenceFiles, summaries)
	summaryRows := analysisSummaryRows(summaries)

	var rowsBuf strings.Builder
	for _, r := range runs {
		rowsBuf.WriteString(runRow(r))
		rowsBuf.WriteByte('\n')
	}
	rows := strings.TrimRight(rowsBuf.String(), "\n")
	if rows == "" {
		rows = `<tr><td colspan="7">No analyzer outputs.</td></tr>`
	}

	var skippedBuf strings.Builder
	for _, item := range skippedFiles {
		skippedBuf.WriteString("<tr><td>")
		skippedBuf.WriteString(html.EscapeString(stringOr(item["file"], "")))
		skippedBuf.WriteString("</td><td>")
		skippedBuf.WriteString(html.EscapeString(stringOr(item["analyzer_type"], "")))
		skippedBuf.WriteString("</td><td>")
		skippedBuf.WriteString(html.EscapeString(stringOr(item["reason"], "")))
		skippedBuf.WriteString("</td></tr>\n")
	}
	skippedRows := strings.TrimRight(skippedBuf.String(), "\n")
	if skippedRows == "" {
		skippedRows = `<tr><td colspan="3">No skipped manifest files.</td></tr>`
	}

	var refBuf strings.Builder
	for _, item := range referenceFiles {
		refBuf.WriteString("<tr><td>")
		refBuf.WriteString(html.EscapeString(stringOr(item["file"], "")))
		refBuf.WriteString("</td><td>")
		refBuf.WriteString(html.EscapeString(stringOr(item["description"], "")))
		refBuf.WriteString("</td><td>")
		refBuf.WriteString(html.EscapeString(stringOr(item["path"], "")))
		refBuf.WriteString("</td></tr>\n")
	}
	refRows := strings.TrimRight(refBuf.String(), "\n")
	if refRows == "" {
		refRows = `<tr><td colspan="3">No reference-only files.</td></tr>`
	}

	var cmpBuf strings.Builder
	for _, p := range comparisonPaths {
		name := filepath.Base(p)
		cmpBuf.WriteString("<li><a href=\"")
		cmpBuf.WriteString(html.EscapeString(name))
		cmpBuf.WriteString("\">")
		cmpBuf.WriteString(html.EscapeString(name))
		cmpBuf.WriteString("</a></li>\n")
	}
	cmpItems := strings.TrimRight(cmpBuf.String(), "\n")
	if cmpItems == "" {
		cmpItems = "<li>No baseline comparison generated.</li>"
	}

	expectedHTML := "<p>No expected signals in manifest.</p>"
	if exp, ok := manifest["expected_key_signals"].(map[string]any); ok {
		body, err := json.MarshalIndent(exp, "", "  ")
		if err == nil {
			expectedHTML = "<pre>" + html.EscapeString(string(body)) + "</pre>"
		}
	}
	description := html.EscapeString(stringOr(manifest["description"], ""))

	parts := []string{
		"<!doctype html>",
		`<html lang="en">`,
		"<head>",
		`  <meta charset="utf-8">`,
		`  <meta name="viewport" content="width=device-width, initial-scale=1">`,
		fmt.Sprintf("  <title>ArchScope Demo Bundle - %s</title>", html.EscapeString(scenario)),
		"  <style>",
		indexStylesheet(),
		"  </style>",
		"</head>",
		"<body>",
		fmt.Sprintf("  <h1>%s</h1>", html.EscapeString(scenario)),
		fmt.Sprintf(`  <p class="meta">Data source: %s</p>`, html.EscapeString(dataSource)),
		fmt.Sprintf("  <p>%s</p>", description),
		"  <h2>Scenario Executive Summary</h2>",
		fmt.Sprintf(`  <div class="summary-grid">%s</div>`, cards),
		"  <h2>Analyzer Result Summary</h2>",
		"  <table><thead><tr><th>Analyzer</th><th>File</th><th>Key metrics</th><th>Findings</th><th>Skipped lines</th></tr></thead>",
		fmt.Sprintf("  <tbody>%s</tbody></table>", summaryRows),
		"  <h2>Analyzer Outputs</h2>",
		"  <table><thead><tr><th>File</th><th>Analyzer</th><th>Status</th><th>Skipped lines</th><th>JSON</th><th>HTML</th><th>PPTX</th></tr></thead>",
		fmt.Sprintf("  <tbody>%s</tbody></table>", rows),
		"  <h2>Manifest Files Not Analyzed</h2>",
		"  <table><thead><tr><th>File</th><th>Analyzer</th><th>Reason</th></tr></thead>",
		fmt.Sprintf("  <tbody>%s</tbody></table>", skippedRows),
		"  <h2>Correlation Context</h2>",
		"  <table><thead><tr><th>File</th><th>Description</th><th>Path</th></tr></thead>",
		fmt.Sprintf("  <tbody>%s</tbody></table>", refRows),
		"  <h2>Baseline Comparison</h2>",
		fmt.Sprintf("  <ul>%s</ul>", cmpItems),
		"  <h2>Expected Signals</h2>",
		expectedHTML,
		"</body>",
		"</html>",
	}
	return strings.Join(parts, "\n")
}

// runRow renders the per-analyzer row in the "Analyzer Outputs" table.
func runRow(run DemoAnalyzerRun) string {
	status := "ok"
	if run.Failed {
		status = "failed"
	}
	if run.Error != nil && *run.Error != "" {
		status = status + ": " + *run.Error
	}
	jsonLink := link(run.JSONPath)
	htmlLink := link(run.HTMLPath)
	pptxLink := link(run.PPTXPath)
	return fmt.Sprintf(
		"<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>",
		html.EscapeString(run.File),
		html.EscapeString(run.AnalyzerType),
		html.EscapeString(status),
		run.SkippedLines,
		jsonLink,
		htmlLink,
		pptxLink,
	)
}

// link wraps a *string path as <a href="basename">basename</a> or "".
func link(p *string) string {
	if p == nil || *p == "" {
		return ""
	}
	name := filepath.Base(*p)
	return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(name), html.EscapeString(name))
}

// summaryCards renders the executive-summary grid.
func summaryCards(
	runs []DemoAnalyzerRun,
	skippedFiles, referenceFiles []map[string]any,
	summaries []map[string]any,
) string {
	ok := 0
	failed := 0
	skipped := 0
	for _, r := range runs {
		if r.Failed {
			failed++
		} else {
			ok++
		}
		skipped += r.SkippedLines
	}
	findings := 0
	for _, s := range summaries {
		findings += intOr(s["finding_count"], 0)
	}
	cards := []struct {
		label string
		value int
	}{
		{"Analyzer outputs", ok},
		{"Failed analyzers", failed},
		{"Skipped lines", skipped},
		{"Finding count", findings},
		{"Skipped manifest files", len(skippedFiles)},
		{"Reference files", len(referenceFiles)},
	}
	var b strings.Builder
	for _, c := range cards {
		b.WriteString("<div><span>")
		b.WriteString(html.EscapeString(c.label))
		b.WriteString("</span><strong>")
		b.WriteString(html.EscapeString(strconv.Itoa(c.value)))
		b.WriteString("</strong></div>")
	}
	return b.String()
}

// analysisSummaryRows renders the per-analyzer key-metric table.
func analysisSummaryRows(summaries []map[string]any) string {
	if len(summaries) == 0 {
		return `<tr><td colspan="5">No analyzer summaries.</td></tr>`
	}
	var rows []string
	for _, summary := range summaries {
		metrics := dictOf(summary["key_metrics"])
		keys := make([]string, 0, len(metrics))
		for k := range metrics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s: %v", k, metrics[k]))
		}
		findings, _ := summary["top_findings"].([]string)
		findingText := strings.Join(findings, "; ")
		if findingText == "" {
			findingText = strconv.Itoa(intOr(summary["finding_count"], 0))
		}
		rows = append(rows, fmt.Sprintf(
			"<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			html.EscapeString(stringOr(summary["analyzer_type"], "")),
			html.EscapeString(stringOr(summary["file"], "")),
			html.EscapeString(strings.Join(parts, ", ")),
			html.EscapeString(findingText),
			html.EscapeString(strconv.Itoa(intOr(summary["skipped_lines"], 0))),
		))
	}
	return strings.Join(rows, "\n")
}

// analysisSummaries reads each successful run's JSON output and
// projects the fields needed for the index page.
func analysisSummaries(runs []DemoAnalyzerRun) []map[string]any {
	out := []map[string]any{}
	for _, r := range runs {
		if r.JSONPath == nil || r.Failed {
			continue
		}
		body, err := readFile(*r.JSONPath)
		if err != nil {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			continue
		}
		metadata := dictOf(payload["metadata"])
		diagnostics := dictOf(metadata["diagnostics"])
		findings, _ := metadata["findings"].([]any)
		topFindings := []string{}
		findingCount := 0
		if findings != nil {
			findingCount = len(findings)
			for i, f := range findings {
				if i >= 5 {
					break
				}
				m, ok := f.(map[string]any)
				if !ok {
					continue
				}
				topFindings = append(topFindings, findingLabel(m))
			}
		}
		out = append(out, map[string]any{
			"file":          r.File,
			"analyzer_type": r.AnalyzerType,
			"result_type":   payload["type"],
			"json_path":     *r.JSONPath,
			"html_path":     deref(r.HTMLPath),
			"pptx_path":     deref(r.PPTXPath),
			"key_metrics":   keyMetrics(dictOf(payload["summary"])),
			"finding_count": findingCount,
			"top_findings":  topFindings,
			"skipped_lines": intOr(diagnostics["skipped_lines"], 0),
		})
	}
	return out
}

// keyMetrics picks a curated list of summary keys (or falls back to
// the first 5 scalars). Mirrors `_key_metrics`.
func keyMetrics(summary map[string]any) map[string]any {
	preferred := []string{
		"total_requests", "error_rate", "p95_response_ms",
		"total_records", "unique_traces", "failed_traces",
		"total_samples", "estimated_seconds", "total_events",
		"max_pause_ms", "total_threads", "blocked_threads",
		"total_exceptions", "unique_exception_types",
	}
	out := map[string]any{}
	for _, k := range preferred {
		if v, ok := summary[k]; ok && !isContainer(v) {
			out[k] = v
		}
	}
	if len(out) > 0 {
		return out
	}
	// Fallback: first 5 scalar keys (lex order to be deterministic —
	// Python's dict preserves insertion order, which the engine
	// guarantees when populating summary; we sort to make Go's
	// map-iteration determinism explicit).
	keys := make([]string, 0, len(summary))
	for k := range summary {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	count := 0
	for _, k := range keys {
		v := summary[k]
		if isContainer(v) {
			continue
		}
		out[k] = v
		count++
		if count == 5 {
			break
		}
	}
	return out
}

// findingLabel builds "severity - code - message". Mirrors `_finding_label`.
func findingLabel(f map[string]any) string {
	parts := []string{}
	for _, k := range []string{"severity", "code", "message"} {
		if v, ok := f[k]; ok && v != nil {
			s := fmt.Sprintf("%v", v)
			if s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, " - ")
}

// isContainer reports whether v is a dict or list (per Python's
// `isinstance(value, (dict, list))`).
func isContainer(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}

// readFile is a one-arg os.ReadFile wrapper kept here to match the
// Python `Path.read_text` style that the runner replays.
func readFile(path string) ([]byte, error) {
	return osReadFile(path)
}

// WriteTopLevelIndex writes a single index.html at `path` listing
// every scenario in `runs`. Used by `demo-site run-all` so a user
// browsing the output root has a one-click entry point.
func WriteTopLevelIndex(runs []DemoScenarioRun, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	body := renderTopLevelIndex(runs, filepath.Dir(path))
	return os.WriteFile(path, []byte(body), 0o644)
}

func renderTopLevelIndex(runs []DemoScenarioRun, root string) string {
	var rowsB strings.Builder
	for _, run := range runs {
		ok := 0
		failed := 0
		skipped := 0
		for _, r := range run.Runs {
			if r.Failed {
				failed++
			} else {
				ok++
			}
			skipped += r.SkippedLines
		}
		var indexHref string
		if run.IndexPath != nil {
			rel, err := filepath.Rel(root, *run.IndexPath)
			if err == nil {
				indexHref = filepath.ToSlash(rel)
			} else {
				indexHref = filepath.Base(*run.IndexPath)
			}
		}
		rowsB.WriteString(fmt.Sprintf(
			`<tr><td><a href="%s">%s</a></td><td>%s</td><td>%d</td><td>%d</td><td>%d</td></tr>`,
			html.EscapeString(indexHref),
			html.EscapeString(run.Scenario),
			html.EscapeString(run.DataSource),
			ok, failed, skipped,
		))
		rowsB.WriteByte('\n')
	}
	rows := strings.TrimRight(rowsB.String(), "\n")
	if rows == "" {
		rows = `<tr><td colspan="5">No scenarios.</td></tr>`
	}
	parts := []string{
		"<!doctype html>",
		`<html lang="en">`,
		"<head>",
		`  <meta charset="utf-8">`,
		`  <meta name="viewport" content="width=device-width, initial-scale=1">`,
		"  <title>ArchScope Demo Bundle</title>",
		"  <style>",
		indexStylesheet(),
		"  </style>",
		"</head>",
		"<body>",
		"  <h1>ArchScope Demo Bundle</h1>",
		`  <p class="meta">All scenarios</p>`,
		"  <table><thead><tr><th>Scenario</th><th>Data source</th><th>OK</th><th>Failed</th><th>Skipped lines</th></tr></thead>",
		fmt.Sprintf("  <tbody>%s</tbody></table>", rows),
		"</body>",
		"</html>",
	}
	return strings.Join(parts, "\n")
}

func indexStylesheet() string {
	return `
body{max-width:1180px;margin:32px auto;padding:0 24px;color:#111827;background:#f8fafc}
body{font-family:Inter,Arial,sans-serif}
h1{margin-bottom:4px}h2{margin-top:28px}.meta{color:#475569;font-weight:700}
.summary-grid{display:grid;grid-template-columns:repeat(6,1fr);gap:12px}
.summary-grid div{padding:12px;border:1px solid #dbe3ef;background:#fff}
.summary-grid span{display:block;color:#64748b;font-size:12px;font-weight:700}
.summary-grid strong{display:block;margin-top:6px;font-size:22px}
table{width:100%;border-collapse:collapse;background:#fff;border:1px solid #dbe3ef}
th,td{padding:10px 12px;border-bottom:1px solid #e5e7eb;text-align:left;vertical-align:top}
th{color:#334155;background:#eef2f7}a{color:#1d4ed8;font-weight:700}
pre{overflow:auto;padding:12px;border:1px solid #dbe3ef;background:#fff}
`
}
