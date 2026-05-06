// Package html ports archscope_engine.exporters.html_exporter — a
// single-file portable HTML report renderer for AnalysisResult or
// parser-debug JSON payloads.
//
// Sibling of the JSON exporter (T-340). The public surface mirrors
// `internal/exporters/json` (Write / Marshal) so callers can swap
// exporters by package alias.
//
// Strategy:
//
//   - The page shell (doctype, head, embedded <style>) is rendered via
//     `html/template` so the inlined CSS and the user-supplied title go
//     through Go's contextual auto-escaper. The CSS itself is embedded
//     from `assets/report.css` via `//go:embed` (mirrors T-339's pattern
//     for static config) — this is what makes the report a single file.
//   - The body is built with an `htmlBuilder` that wraps every dynamic
//     value through `html.EscapeString`, matching the Python source's
//     line-by-line use of `html.escape`. The composed body is then
//     handed to the template as `template.HTML`, which marks it as
//     already-escaped so html/template will not double-escape it.
//
// This keeps parity with the Python renderer while still routing every
// untrusted string through a stdlib escaper.
package html

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html"
	htmltmpl "html/template"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed assets/report.css
var assetsFS embed.FS

// Options tweaks rendering. `SourcePath` mirrors Python's
// `source_path` keyword (shown in the metadata block), and `Title`
// overrides the auto-derived report title.
type Options struct {
	SourcePath string
	Title      string
}

// Write renders `result` (typically `models.AnalysisResult` or any
// JSON-decoded payload — both are accepted as `any`) to a single
// portable HTML file at `path`, creating any missing parent
// directories. Mirrors Python `write_html_report(input_path,
// output_path)`.
//
// `result` may be a `*models.AnalysisResult`, a `map[string]any`
// (already-decoded JSON), or any other JSON-marshalable value; we
// re-marshal-then-decode internally so the renderer always works
// against a uniform `map[string]any` — same as the Python source which
// `json.loads`es the input.
func Write(path string, result any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	body, err := Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// WriteWithOptions is the keyword-arg variant of Write.
func WriteWithOptions(path string, result any, opts Options) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	body, err := MarshalWithOptions(result, opts)
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// Marshal returns the rendered HTML bytes that `Write` would persist.
func Marshal(result any) ([]byte, error) {
	return MarshalWithOptions(result, Options{})
}

// MarshalWithOptions is Marshal with the keyword-arg surface.
func MarshalWithOptions(result any, opts Options) ([]byte, error) {
	payload, err := toMap(result)
	if err != nil {
		return nil, err
	}

	title := opts.Title
	if title == "" {
		title = defaultTitle(payload)
	}

	var body string
	if isDebugLog(payload) {
		body = renderDebugLog(payload, opts.SourcePath)
	} else {
		body = renderAnalysisResult(payload, opts.SourcePath)
	}

	return renderPage(title, body)
}

// renderPage stitches the doctype, <head>, embedded CSS, and the
// already-escaped body together. Uses `html/template` for the shell so
// the title flows through the contextual escaper and the embedded CSS
// is interpolated as trusted CSS.
func renderPage(title, body string) ([]byte, error) {
	css, err := assetsFS.ReadFile("assets/report.css")
	if err != nil {
		return nil, fmt.Errorf("read embedded css: %w", err)
	}

	const shell = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
{{.CSS}}
  </style>
</head>
<body>
  <h1>{{.Title}}</h1>
{{.Body}}
</body>
</html>`

	tmpl, err := htmltmpl.New("report").Parse(shell)
	if err != nil {
		return nil, fmt.Errorf("parse shell template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, struct {
		Title string
		CSS   htmltmpl.CSS
		Body  htmltmpl.HTML
	}{
		Title: title,
		CSS:   htmltmpl.CSS(string(css)),
		Body:  htmltmpl.HTML(body),
	})
	if err != nil {
		return nil, fmt.Errorf("execute shell template: %w", err)
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------
// Body renderers — mirror html_exporter.py.
// ---------------------------------------------------------------------

func renderAnalysisResult(payload map[string]any, sourcePath string) string {
	resultType := stringOr(payload["type"], "unknown")
	metadata := asMap(payload["metadata"])
	diagnostics := asMap(metadata["diagnostics"])

	parts := []string{
		section("Report Metadata", definitionList([]kv{
			{"Input JSON", optionalString(sourcePath)},
			{"Result type", resultType},
			{"Created at", payload["created_at"]},
			{"Schema version", metadata["schema_version"]},
			{"Source files", strings.Join(stringSlice(payload["source_files"]), ", ")},
		})),
		section("Summary", renderKeyValues(asMap(payload["summary"]))),
	}

	if findings, ok := payload["metadata"].(map[string]any); ok {
		if list, ok := findings["findings"].([]any); ok && len(list) > 0 {
			parts = append(parts, section("Findings", renderRecords(list, 20)))
		} else if list, ok := findings["findings"].([]map[string]any); ok && len(list) > 0 {
			// AnalysisResult.Metadata uses []map[string]any.
			anyList := make([]any, len(list))
			for i, row := range list {
				anyList[i] = row
			}
			parts = append(parts, section("Findings", renderRecords(anyList, 20)))
		}
	}

	if len(diagnostics) > 0 {
		parts = append(parts, section("Parser Diagnostics", renderKeyValues(diagnostics)))
	}

	parts = append(parts, renderSeriesAndTables(payload)...)

	charts := asMap(payload["charts"])
	if flame, ok := charts["flamegraph"].(map[string]any); ok {
		parts = append(parts, section("Flamegraph", renderFlamegraph(flame)))
	}
	if len(charts) > 0 {
		parts = append(parts, section("Chart Data", renderJSONPreview(charts)))
	}
	_ = resultType
	return strings.Join(parts, "\n")
}

func renderDebugLog(payload map[string]any, sourcePath string) string {
	summary := asMap(payload["summary"])
	context := asMap(payload["context"])
	redaction := asMap(payload["redaction"])

	redactionLabel := "disabled"
	if b, ok := redaction["enabled"].(bool); ok && b {
		redactionLabel = "enabled"
	}

	parts := []string{
		section("Debug Log Metadata", definitionList([]kv{
			{"Input JSON", optionalString(sourcePath)},
			{"Analyzer", context["analyzer_type"]},
			{"Parser", context["parser"]},
			{"Source file", context["source_file_name"]},
			{"Encoding", context["encoding_detected"]},
			{"Redaction", redactionLabel},
		})),
		section("Summary", renderKeyValues(summary)),
	}

	errorsByType := asMap(payload["errors_by_type"])
	if len(errorsByType) > 0 {
		parts = append(parts, section("Parse Errors", renderDebugErrors(errorsByType)))
	}
	if exceptions, ok := payload["exceptions"].([]any); ok && len(exceptions) > 0 {
		parts = append(parts, section("Exceptions", renderRecords(exceptions, 10)))
	}
	if hints, ok := payload["hints"].([]any); ok && len(hints) > 0 {
		var sb strings.Builder
		sb.WriteString("<ul>")
		for _, h := range hints {
			sb.WriteString("<li>")
			sb.WriteString(html.EscapeString(stringify(h)))
			sb.WriteString("</li>")
		}
		sb.WriteString("</ul>")
		parts = append(parts, section("Hints", sb.String()))
	}
	return strings.Join(parts, "\n")
}

func renderSeriesAndTables(payload map[string]any) []string {
	out := []string{}
	for _, groupName := range []string{"series", "tables"} {
		group := asMap(payload[groupName])
		if len(group) == 0 {
			continue
		}
		// Python preserves dict insertion order. Go maps are
		// randomised — we keep the order stable by sorting on key for
		// deterministic snapshot tests; this loses Python's exact
		// order but each section is still self-contained.
		keys := sortedKeys(group)
		for _, name := range keys {
			value := group[name]
			title := strings.ToTitle(groupName[:1]) + groupName[1:] + ": " + name
			if list, ok := value.([]any); ok {
				out = append(out, section(title, renderRecords(list, 50)))
			} else {
				out = append(out, section(title, renderJSONPreview(value)))
			}
		}
	}
	return out
}

func renderDebugErrors(errorsByType map[string]any) string {
	var blocks []string
	for _, reason := range sortedKeys(errorsByType) {
		entry := errorsByType[reason]
		errMap := asMap(entry)
		var sampleHTML string
		if samples, ok := errMap["samples"].([]any); ok {
			sampleHTML = renderRecords(samples, 5)
		}
		var sb strings.Builder
		sb.WriteString(`<article class="subsection"><h3>`)
		sb.WriteString(html.EscapeString(reason))
		sb.WriteString("</h3>")
		sb.WriteString(definitionList([]kv{
			{"Count", errMap["count"]},
			{"Description", errMap["description"]},
			{"Failed pattern", errMap["failed_pattern"]},
		}))
		sb.WriteString(sampleHTML)
		sb.WriteString("</article>")
		blocks = append(blocks, sb.String())
	}
	return strings.Join(blocks, "\n")
}

func renderFlamegraph(root map[string]any) string {
	totalSamples := numberOr(root["samples"], 1)
	if totalSamples == 0 {
		totalSamples = 1
	}
	return `<div class="flamegraph" role="tree">` +
		renderFlameNode(root, totalSamples, 0) +
		"</div>"
}

func renderFlameNode(node map[string]any, totalSamples float64, depth int) string {
	var childNodes []map[string]any
	if children, ok := node["children"].([]any); ok {
		for _, c := range children {
			if m, ok := c.(map[string]any); ok {
				childNodes = append(childNodes, m)
			}
		}
	}

	samples := numberOr(node["samples"], 0)
	var ratio float64
	if totalSamples != 0 {
		ratio = math.Max(0.5, math.Min(100.0, (samples/totalSamples)*100))
	}
	name := html.EscapeString(stringOr(node["name"], "(unknown)"))
	sampleText := html.EscapeString(stringOr(node["samples"], "0"))

	var childHTML strings.Builder
	for _, child := range childNodes {
		childHTML.WriteString(renderFlameNode(child, totalSamples, depth+1))
	}

	row := fmt.Sprintf(
		`<div class="flame-row" style="margin-left:%dpx"><div class="flame-bar" style="width:%.2f%%"><span>%s</span><small>%s</small></div></div>`,
		depth*14, ratio, name, sampleText,
	)
	if childHTML.Len() == 0 {
		return row
	}
	return "<details open><summary>" + row + "</summary>" + childHTML.String() + "</details>"
}

func renderKeyValues(values map[string]any) string {
	if len(values) == 0 {
		return "<p>No data.</p>"
	}
	var simple []kv
	type complexEntry struct {
		key   string
		value any
	}
	var complexValues []complexEntry
	for _, k := range sortedKeys(values) {
		v := values[k]
		switch v.(type) {
		case map[string]any, []any:
			complexValues = append(complexValues, complexEntry{k, v})
		default:
			simple = append(simple, kv{k, v})
		}
	}
	out := definitionList(simple)
	for _, c := range complexValues {
		out += "<h3>" + html.EscapeString(c.key) + "</h3>" + renderJSONPreview(c.value)
	}
	return out
}

func renderRecords(records []any, limit int) string {
	if limit > len(records) {
		limit = len(records)
	}
	clip := records[:limit]
	var rows []map[string]any
	for _, r := range clip {
		if m, ok := r.(map[string]any); ok {
			rows = append(rows, m)
		}
	}
	if len(rows) == 0 {
		// Python falls back to a JSON preview of the trimmed slice.
		return renderJSONPreview(any(clip))
	}

	var columns []string
	seen := map[string]bool{}
	for _, row := range rows {
		// dict iteration order in Python is insertion order; for Go
		// maps we keep encounter order via sorted-key fallback. To
		// match Python output as closely as possible we use the
		// row-key alphabetical order which is at least stable across
		// runs.
		for _, k := range sortedKeys(row) {
			if !seen[k] {
				seen[k] = true
				columns = append(columns, k)
			}
		}
	}

	var headerHTML strings.Builder
	for _, col := range columns {
		headerHTML.WriteString("<th>")
		headerHTML.WriteString(html.EscapeString(col))
		headerHTML.WriteString("</th>")
	}

	var bodyHTML strings.Builder
	for _, row := range rows {
		bodyHTML.WriteString("<tr>")
		for _, col := range columns {
			bodyHTML.WriteString("<td>")
			bodyHTML.WriteString(formatCell(row[col]))
			bodyHTML.WriteString("</td>")
		}
		bodyHTML.WriteString("</tr>")
	}

	note := ""
	if len(records) > limit {
		note = fmt.Sprintf(`<p class="muted">Showing %d of %d rows.</p>`, limit, len(records))
	}
	return note + `<div class="table-wrap"><table><thead><tr>` +
		headerHTML.String() + "</tr></thead><tbody>" +
		bodyHTML.String() + "</tbody></table></div>"
}

type kv struct {
	key   string
	value any
}

func definitionList(values []kv) string {
	var rows []string
	for _, pair := range values {
		if pair.value == nil {
			continue
		}
		// Filter empty strings out only when caller used
		// optionalString() to flag a missing source path.
		if s, ok := pair.value.(omittedString); ok {
			if !s.present {
				continue
			}
			rows = append(rows, "<div><dt>"+html.EscapeString(pair.key)+
				"</dt><dd>"+formatCell(s.value)+"</dd></div>")
			continue
		}
		rows = append(rows, "<div><dt>"+html.EscapeString(pair.key)+
			"</dt><dd>"+formatCell(pair.value)+"</dd></div>")
	}
	if len(rows) == 0 {
		return "<p>No data.</p>"
	}
	return "<dl>" + strings.Join(rows, "") + "</dl>"
}

func section(title, content string) string {
	return "<section><h2>" + html.EscapeString(title) + "</h2>" + content + "</section>"
}

func renderJSONPreview(value any) string {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		// Match Python's `default=str` fallback.
		return "<pre>" + html.EscapeString(stringify(value)) + "</pre>"
	}
	return "<pre>" + html.EscapeString(string(body)) + "</pre>"
}

func formatCell(value any) string {
	switch v := value.(type) {
	case nil:
		return `<span class="muted">null</span>`
	case map[string]any, []any:
		return renderJSONPreview(v)
	default:
		return html.EscapeString(stringify(v))
	}
}

// ---------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------

// omittedString is the Go analogue of Python's "skip key whose value
// is None" rule: when source_path is empty we want to omit the row,
// not render an empty string.
type omittedString struct {
	value   string
	present bool
}

func optionalString(s string) any {
	if s == "" {
		return omittedString{present: false}
	}
	return omittedString{value: s, present: true}
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func stringOr(v any, fallback string) string {
	if v == nil {
		return fallback
	}
	return stringify(v)
}

func stringSlice(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, stringify(item))
	}
	return out
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		// Trim trailing .0 to match Python `str(float)` style for
		// integer-valued floats inside JSON-decoded payloads.
		if t == math.Trunc(t) && !math.IsInf(t, 0) {
			return strconv.FormatFloat(t, 'f', -1, 64)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return string(t)
	default:
		// Best-effort fallthrough (mirrors Python `str(x)`).
		return fmt.Sprintf("%v", t)
	}
}

func numberOr(v any, fallback float64) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return f
		}
	}
	return fallback
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isDebugLog(payload map[string]any) bool {
	_, hasErrors := payload["errors_by_type"]
	_, hasContext := payload["context"]
	_, hasEnv := payload["environment"]
	return hasErrors && hasContext && hasEnv
}

func defaultTitle(payload map[string]any) string {
	if isDebugLog(payload) {
		analyzer := stringOr(asMap(payload["context"])["analyzer_type"], "parser")
		return "ArchScope Parser Debug Report - " + analyzer
	}
	return "ArchScope Analysis Report - " + stringOr(payload["type"], "unknown")
}

// toMap normalises any JSON-serializable input into a map[string]any
// the renderer can walk uniformly. Mirrors the Python source which
// always operates on `json.loads(...)` output.
func toMap(v any) (map[string]any, error) {
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	var out map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	// Convert json.Number back to float64 so downstream type
	// switches match without a special case.
	out = convertNumbers(out).(map[string]any)
	return out, nil
}

func convertNumbers(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			t[k] = convertNumbers(val)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = convertNumbers(val)
		}
		return t
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return string(t)
	default:
		return t
	}
}
