package profiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugLogVerdictAndPortableFilename(t *testing.T) {
	log := NewDebugLog("profiler_collapsed", "collapsed", "/tmp/sample.collapsed")
	if log.Verdict() != "CLEAN" {
		t.Fatalf("verdict should start CLEAN; got %q", log.Verdict())
	}
	log.AddParseError(10, "BAD", "bad row", "PATTERN", map[string]string{"target": "Authorization: Bearer abc"}, nil)
	if log.Verdict() != "PARTIAL_SUCCESS" {
		t.Fatalf("verdict should be PARTIAL_SUCCESS after error; got %q", log.Verdict())
	}
	log.AddException("analyze", "boom", "")
	if log.Verdict() != "FATAL_ERROR" {
		t.Fatalf("verdict should escalate to FATAL_ERROR; got %q", log.Verdict())
	}

	name := log.PortableFilename()
	if !strings.HasPrefix(name, "archscope-debug-collapsed-") {
		t.Fatalf("filename prefix; got %q", name)
	}
	if !strings.HasSuffix(name, ".json") {
		t.Fatalf("filename should be JSON; got %q", name)
	}
}

func TestDebugLogWriteRedactsContextAndCapsSamples(t *testing.T) {
	log := NewDebugLog("profiler_collapsed", "collapsed", "/tmp/sample.collapsed")
	for i := 0; i < 12; i++ {
		log.AddParseError(
			i,
			"INVALID",
			"bad",
			"PATTERN",
			map[string]string{"target": "Authorization: Bearer redact-me"},
			nil,
		)
	}
	dir := t.TempDir()
	path, err := log.WriteJSON(dir)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(body), "redact-me") {
		t.Fatalf("debug log must not contain unredacted token: %s", body)
	}
	if !strings.Contains(string(body), "TOKEN len=") {
		t.Fatalf("expected token placeholder in payload; got %s", body)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	errorsByType, ok := parsed["errors_by_type"].(map[string]any)
	if !ok {
		t.Fatalf("errors_by_type should be object; got %T", parsed["errors_by_type"])
	}
	invalidEntry, ok := errorsByType["INVALID"].(map[string]any)
	if !ok {
		t.Fatalf("errors_by_type[INVALID] should be object; got %T", errorsByType["INVALID"])
	}
	samples, ok := invalidEntry["samples"].([]any)
	if !ok {
		t.Fatalf("samples should be array; got %T", invalidEntry["samples"])
	}
	if len(samples) > log.maxSamplesByType {
		t.Fatalf("sample cap exceeded: got %d, max %d", len(samples), log.maxSamplesByType)
	}
}

func TestDebugLogHasContent(t *testing.T) {
	log := NewDebugLog("profiler_collapsed", "collapsed", "/tmp/sample.collapsed")
	if log.HasContent() {
		t.Fatalf("fresh log should be empty")
	}
	log.AddParseError(1, "X", "msg", "P", nil, nil)
	if !log.HasContent() {
		t.Fatalf("log should report content after AddParseError")
	}
}

func TestDebugLogWriteCreatesDir(t *testing.T) {
	log := NewDebugLog("profiler_collapsed", "svg", "/tmp/sample.svg")
	log.AddParseError(0, "INVALID", "bad", "PAT", nil, nil)
	dir := filepath.Join(t.TempDir(), "subdir", "nested")
	path, err := log.WriteJSON(dir)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.HasPrefix(path, dir) {
		t.Fatalf("path %q should live under %q", path, dir)
	}
}

func TestDebugLogPayloadMatchesPythonShape(t *testing.T) {
	log := NewDebugLog("profiler_collapsed", "flamegraph_svg", "/tmp/test.svg")
	log.SetEncodingDetected("utf-8")
	log.SetParserOptions(map[string]any{"interval_ms": 100})
	log.AddParseError(5, "UNPARSEABLE_TITLE", "bad title", "svgTitleRE",
		map[string]string{"target": "malformed (no samples)"}, nil)
	log.SetTotals(100, 95, 5)

	dir := t.TempDir()
	path, err := log.WriteJSON(dir)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify top-level keys match Python shape.
	requiredKeys := []string{"environment", "context", "redaction", "summary", "errors_by_type", "exceptions", "hints", "truncated"}
	for _, key := range requiredKeys {
		if _, ok := payload[key]; !ok {
			t.Errorf("missing required key %q", key)
		}
	}

	// Verify environment section.
	env, _ := payload["environment"].(map[string]any)
	for _, key := range []string{"archscope_version", "go_version", "os", "timestamp"} {
		if _, ok := env[key]; !ok {
			t.Errorf("missing environment.%s", key)
		}
	}

	// Verify context section.
	ctx, _ := payload["context"].(map[string]any)
	for _, key := range []string{"analyzer_type", "source_file", "source_file_name", "encoding_detected", "parser", "parser_options"} {
		if _, ok := ctx[key]; !ok {
			t.Errorf("missing context.%s", key)
		}
	}
	if ctx["analyzer_type"] != "profiler_collapsed" {
		t.Errorf("context.analyzer_type = %v, want profiler_collapsed", ctx["analyzer_type"])
	}
	if ctx["parser"] != "flamegraph_svg" {
		t.Errorf("context.parser = %v, want flamegraph_svg", ctx["parser"])
	}

	// Verify summary section.
	summary, _ := payload["summary"].(map[string]any)
	if summary["verdict"] != "PARTIAL_SUCCESS" {
		t.Errorf("summary.verdict = %v, want PARTIAL_SUCCESS", summary["verdict"])
	}
	if int(summary["total_lines"].(float64)) != 100 {
		t.Errorf("summary.total_lines = %v, want 100", summary["total_lines"])
	}
	if int(summary["parsed_ok"].(float64)) != 95 {
		t.Errorf("summary.parsed_ok = %v, want 95", summary["parsed_ok"])
	}

	// Verify errors_by_type.
	ebt, _ := payload["errors_by_type"].(map[string]any)
	entry, ok := ebt["UNPARSEABLE_TITLE"].(map[string]any)
	if !ok {
		t.Fatalf("missing errors_by_type[UNPARSEABLE_TITLE]")
	}
	if int(entry["count"].(float64)) != 1 {
		t.Errorf("UNPARSEABLE_TITLE count = %v, want 1", entry["count"])
	}
	if entry["failed_pattern"] != "svgTitleRE" {
		t.Errorf("failed_pattern = %v, want svgTitleRE", entry["failed_pattern"])
	}

	// Verify redaction section.
	redaction, _ := payload["redaction"].(map[string]any)
	if redaction["enabled"] != true {
		t.Errorf("redaction.enabled should be true")
	}
}

func TestDebugLogMajorityFailedVerdict(t *testing.T) {
	log := NewDebugLog("profiler_collapsed", "collapsed", "/tmp/test.collapsed")
	log.SetTotals(100, 30, 70)
	if log.Verdict() != "MAJORITY_FAILED" {
		t.Errorf("verdict should be MAJORITY_FAILED when >50%% skipped; got %q", log.Verdict())
	}
}

func TestInferFieldShapes(t *testing.T) {
	shapes := InferFieldShapes(`192.168.1.1 - - [10/Oct/2023:13:55:36 -0700] "GET /api/users?id=123 HTTP/1.1" 200 2326`)

	if shapes["timestamp_shape"] != "dd/Mon/yyyy:HH:mm:ss Z" {
		t.Errorf("timestamp_shape = %v, want dd/Mon/yyyy:HH:mm:ss Z", shapes["timestamp_shape"])
	}
	if shapes["request_shape"] != "METHOD PATH_WITH_QUERY PROTOCOL" {
		t.Errorf("request_shape = %v, want METHOD PATH_WITH_QUERY PROTOCOL", shapes["request_shape"])
	}
	if shapes["path_shape"] != "/api/users" {
		t.Errorf("path_shape = %v, want /api/users", shapes["path_shape"])
	}
	queryKeys, ok := shapes["query_keys"].([]string)
	if !ok || len(queryKeys) != 1 || queryKeys[0] != "id" {
		t.Errorf("query_keys = %v, want [id]", shapes["query_keys"])
	}
	if shapes["quote_count"] != 2 {
		t.Errorf("quote_count = %v, want 2", shapes["quote_count"])
	}
}

func TestInferFieldShapesISO8601(t *testing.T) {
	shapes := InferFieldShapes(`2023-10-10T13:55:36Z some log line`)
	if shapes["timestamp_shape"] != "ISO-8601" {
		t.Errorf("timestamp_shape = %v, want ISO-8601", shapes["timestamp_shape"])
	}
}

func TestDebugLogAutoInfersFieldShapes(t *testing.T) {
	log := NewDebugLog("profiler_collapsed", "collapsed", "/tmp/test.collapsed")
	log.AddParseError(1, "BAD", "msg", "", map[string]string{"target": `"GET /foo HTTP/1.1"`}, nil)

	if !log.HasContent() {
		t.Fatalf("log should have content")
	}
	if log.parseErrors[0].FieldShapes == nil {
		t.Fatalf("field_shapes should be auto-inferred when nil")
	}
	if log.parseErrors[0].FieldShapes["request_shape"] != "METHOD PATH PROTOCOL" {
		t.Errorf("request_shape = %v, want METHOD PATH PROTOCOL", log.parseErrors[0].FieldShapes["request_shape"])
	}
}

func TestDebugLogSvgParserPopulatesDebugLog(t *testing.T) {
	// A malformed SVG that will trigger UNPARSEABLE_TITLE.
	malformed := `<svg xmlns="http://www.w3.org/2000/svg">
<g><title>no-samples-marker</title><rect x="0" y="0" width="100" height="16"/></g>
</svg>`
	dl := NewDebugLog("profiler_collapsed", "flamegraph_svg", "/tmp/test.svg")
	result := parseSvgFlamegraphBytes([]byte(malformed), ParserDiagnostics{Format: "flamegraph_svg", SkippedByReason: map[string]int{}}, dl)
	_ = result
	if !dl.HasContent() {
		t.Fatalf("debug log should capture SVG parse errors")
	}
	found := false
	for _, err := range dl.parseErrors {
		if err.Reason == "UNPARSEABLE_TITLE" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UNPARSEABLE_TITLE in debug log parse errors")
	}
}

func TestDebugLogJenniferParserPopulatesDebugLog(t *testing.T) {
	// A CSV with missing required columns.
	csv := "foo,bar\n1,2\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.csv")
	if err := os.WriteFile(path, []byte(csv), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	dl := NewDebugLog("profiler_jennifer", "jennifer_flamegraph_csv", path)
	_, err := ParseJenniferFlamegraphCSV(path, dl)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !dl.HasContent() {
		t.Fatalf("debug log should capture Jennifer parse errors")
	}
	found := false
	for _, e := range dl.parseErrors {
		if e.Reason == "MISSING_REQUIRED_COLUMNS" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected MISSING_REQUIRED_COLUMNS in debug log")
	}
}

func TestDebugLogHtmlUnsupportedFormatPopulatesDebugLog(t *testing.T) {
	html := `<!doctype html><html><body><p>not a flamegraph</p></body></html>`
	dl := NewDebugLog("profiler_collapsed", "flamegraph_html", "/tmp/test.html")
	result := ParseHtmlProfilerText(html, dl)
	if result.DetectedFormat != "unsupported" {
		t.Fatalf("expected unsupported format; got %q", result.DetectedFormat)
	}
	if !dl.HasContent() {
		t.Fatalf("debug log should capture HTML parse errors")
	}
	found := false
	for _, e := range dl.parseErrors {
		if e.Reason == "UNSUPPORTED_HTML_FORMAT" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UNSUPPORTED_HTML_FORMAT in debug log")
	}
}
