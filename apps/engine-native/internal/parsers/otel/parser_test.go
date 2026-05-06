package otel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeJSONL marshals the slice of objects one-per-line — same shape
// as the Python `_write_jsonl` helper in `test_otel_parser.py`.
func writeJSONL(t *testing.T, dir string, payloads []map[string]any) string {
	t.Helper()
	path := filepath.Join(dir, "otel.jsonl")
	parts := make([]string, 0, len(payloads))
	for _, p := range payloads {
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		parts = append(parts, string(raw))
	}
	if err := os.WriteFile(path, []byte(strings.Join(parts, "\n")), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func strDeref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// Field-alias suite (mirrors Python TestOtelParserFieldAliases).
func TestParsesStandardOTelFields(t *testing.T) {
	path := writeJSONL(t, t.TempDir(), []map[string]any{
		{
			"timestamp":     "2026-01-01T00:00:00Z",
			"trace_id":      "abc123",
			"span_id":       "span1",
			"severity_text": "ERROR",
			"service_name":  "order-service",
			"body":          "Connection timeout",
		},
	})
	records, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	r := records[0]
	if strDeref(r.TraceID) != "abc123" {
		t.Errorf("TraceID = %q", strDeref(r.TraceID))
	}
	if strDeref(r.SpanID) != "span1" {
		t.Errorf("SpanID = %q", strDeref(r.SpanID))
	}
	if r.Severity != "ERROR" {
		t.Errorf("Severity = %q", r.Severity)
	}
	if strDeref(r.ServiceName) != "order-service" {
		t.Errorf("ServiceName = %q", strDeref(r.ServiceName))
	}
	if r.Body != "Connection timeout" {
		t.Errorf("Body = %q", r.Body)
	}
}

func TestParsesCamelCaseFieldAliases(t *testing.T) {
	path := writeJSONL(t, t.TempDir(), []map[string]any{
		{
			"traceId":      "t1",
			"spanId":       "s1",
			"parentSpanId": "ps1",
			"severityText": "WARN",
			"service.name": "api-gw",
			"body":         "Slow response",
		},
	})
	records, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	r := records[0]
	if strDeref(r.TraceID) != "t1" {
		t.Errorf("TraceID = %q", strDeref(r.TraceID))
	}
	if strDeref(r.SpanID) != "s1" {
		t.Errorf("SpanID = %q", strDeref(r.SpanID))
	}
	if strDeref(r.ParentSpanID) != "ps1" {
		t.Errorf("ParentSpanID = %q", strDeref(r.ParentSpanID))
	}
	if strDeref(r.ServiceName) != "api-gw" {
		t.Errorf("ServiceName = %q", strDeref(r.ServiceName))
	}
}

func TestExtractsServiceFromResourceAttributes(t *testing.T) {
	path := writeJSONL(t, t.TempDir(), []map[string]any{
		{
			"trace_id": "t1",
			"body":     "test",
			"resource": map[string]any{
				"attributes": map[string]any{"service.name": "payment-service"},
			},
		},
	})
	records, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if strDeref(records[0].ServiceName) != "payment-service" {
		t.Errorf("ServiceName = %q", strDeref(records[0].ServiceName))
	}
}

func TestExtractsServiceFromTopLevelAttributes(t *testing.T) {
	path := writeJSONL(t, t.TempDir(), []map[string]any{
		{
			"trace_id":   "t1",
			"body":       "test",
			"attributes": map[string]any{"service.name": "user-service"},
		},
	})
	records, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if strDeref(records[0].ServiceName) != "user-service" {
		t.Errorf("ServiceName = %q", strDeref(records[0].ServiceName))
	}
}

func TestExtractsParentSpanFromAttributes(t *testing.T) {
	path := writeJSONL(t, t.TempDir(), []map[string]any{
		{
			"trace_id":   "t1",
			"body":       "test",
			"attributes": map[string]any{"parentSpanId": "parent1"},
		},
	})
	records, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if strDeref(records[0].ParentSpanID) != "parent1" {
		t.Errorf("ParentSpanID = %q", strDeref(records[0].ParentSpanID))
	}
}

func TestBodyFromNestedStringValue(t *testing.T) {
	path := writeJSONL(t, t.TempDir(), []map[string]any{
		{
			"trace_id": "t1",
			"body":     map[string]any{"stringValue": "Nested body text"},
		},
	})
	records, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if records[0].Body != "Nested body text" {
		t.Errorf("Body = %q", records[0].Body)
	}
}

func TestBodyFromMessageField(t *testing.T) {
	path := writeJSONL(t, t.TempDir(), []map[string]any{
		{"trace_id": "t1", "message": "A log message"},
	})
	records, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if records[0].Body != "A log message" {
		t.Errorf("Body = %q", records[0].Body)
	}
}

func TestNumericTraceIDConvertedToString(t *testing.T) {
	path := writeJSONL(t, t.TempDir(), []map[string]any{
		{"trace_id": 12345, "body": "numeric id"},
	})
	records, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if strDeref(records[0].TraceID) != "12345" {
		t.Errorf("TraceID = %q, want 12345", strDeref(records[0].TraceID))
	}
}

func TestBooleanBodyConvertedToString(t *testing.T) {
	path := writeJSONL(t, t.TempDir(), []map[string]any{
		{"trace_id": "t1", "body": true},
	})
	records, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	// Python `str(True)` -> "True" with capital T; the Python test
	// asserts this exact spelling.
	if records[0].Body != "True" {
		t.Errorf("Body = %q, want True", records[0].Body)
	}
}

// Diagnostics suite (mirrors Python TestOtelParserDiagnostics).
func TestSkipsInvalidJSONLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "otel.jsonl")
	body := `{"trace_id": "t1", "body": "ok"}` + "\n" +
		"not json at all\n" +
		`{"trace_id": "t2", "body": "ok2"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	records, diags, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	if diags.SkippedLines != 1 {
		t.Errorf("SkippedLines = %d, want 1", diags.SkippedLines)
	}
	if _, ok := diags.SkippedByReason[ReasonInvalidOTelJSON]; !ok {
		t.Errorf("SkippedByReason missing %q: %+v", ReasonInvalidOTelJSON, diags.SkippedByReason)
	}
}

func TestSkipsJSONWithoutOTelFields(t *testing.T) {
	path := writeJSONL(t, t.TempDir(), []map[string]any{
		{"unrelated_key": "value", "another": 42},
		{"trace_id": "t1", "body": "valid"},
	})
	records, diags, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if diags.SkippedLines != 1 {
		t.Errorf("SkippedLines = %d, want 1", diags.SkippedLines)
	}
	if _, ok := diags.SkippedByReason[ReasonInvalidOTelRecord]; !ok {
		t.Errorf("SkippedByReason missing %q: %+v", ReasonInvalidOTelRecord, diags.SkippedByReason)
	}
}

func TestSkipsNonDictJSONValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "otel.jsonl")
	body := `"just a string"` + "\n" +
		`[1,2,3]` + "\n" +
		`{"trace_id":"t1","body":"ok"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	records, diags, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if diags.SkippedLines != 2 {
		t.Errorf("SkippedLines = %d, want 2", diags.SkippedLines)
	}
}

func TestBlankLinesAreIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "otel.jsonl")
	body := "\n" +
		`{"trace_id":"t1","body":"ok"}` + "\n\n\n" +
		`{"trace_id":"t2","body":"ok2"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	records, diags, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	if diags.SkippedLines != 0 {
		t.Errorf("SkippedLines = %d, want 0", diags.SkippedLines)
	}
}

func TestSeverityDefaultsToUnspecified(t *testing.T) {
	path := writeJSONL(t, t.TempDir(), []map[string]any{
		{"trace_id": "t1", "body": "no severity"},
	})
	records, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if records[0].Severity != "UNSPECIFIED" {
		t.Errorf("Severity = %q, want UNSPECIFIED", records[0].Severity)
	}
}

// ParseLine-direct tests — mirror accesslog conventions.
func TestParseLineRejectsInvalidJSON(t *testing.T) {
	rec, perr := ParseLine("not json at all")
	if rec != nil {
		t.Fatalf("expected nil record, got %+v", rec)
	}
	if perr == nil || perr.Reason != ReasonInvalidOTelJSON {
		t.Fatalf("expected INVALID_OTEL_JSON, got %+v", perr)
	}
}

func TestParseLineRejectsEmptyOTelObject(t *testing.T) {
	rec, perr := ParseLine(`{"unrelated": "value"}`)
	if rec != nil {
		t.Fatalf("expected nil record, got %+v", rec)
	}
	if perr == nil || perr.Reason != ReasonInvalidOTelRecord {
		t.Fatalf("expected INVALID_OTEL_RECORD, got %+v", perr)
	}
}

func TestParseLineRejectsArrayPayload(t *testing.T) {
	_, perr := ParseLine(`[1,2,3]`)
	if perr == nil || perr.Reason != ReasonInvalidOTelRecord {
		t.Fatalf("expected INVALID_OTEL_RECORD, got %+v", perr)
	}
}

func TestParseLineRawLinePreserved(t *testing.T) {
	line := `{"trace_id":"t1","body":"ok"}`
	rec, perr := ParseLine(line)
	if perr != nil {
		t.Fatalf("unexpected parse error: %+v", perr)
	}
	if rec.RawLine != line {
		t.Errorf("RawLine = %q, want %q", rec.RawLine, line)
	}
}

// ParseFile feature tests — Strict mode, MaxLines, EMPTY_FILE warning.
func TestParseFileStrictModeFailsFast(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "otel.jsonl")
	body := "not json\n" + `{"trace_id":"t1","body":"ok"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _, err := ParseFile(path, Options{Strict: true})
	if err == nil {
		t.Fatalf("expected strict-mode error")
	}
	if !strings.Contains(err.Error(), ReasonInvalidOTelJSON) {
		t.Fatalf("error should mention %s, got %v", ReasonInvalidOTelJSON, err)
	}
}

func TestParseFileMaxLinesCutsoff(t *testing.T) {
	path := writeJSONL(t, t.TempDir(), []map[string]any{
		{"trace_id": "t1", "body": "1"},
		{"trace_id": "t2", "body": "2"},
		{"trace_id": "t3", "body": "3"},
	})
	records, diags, err := ParseFile(path, Options{MaxLines: 2})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	if diags.TotalLines != 2 {
		t.Errorf("TotalLines = %d, want 2", diags.TotalLines)
	}
}

func TestParseFileRejectsNegativeMaxLines(t *testing.T) {
	_, _, err := ParseFile("/tmp/none.jsonl", Options{MaxLines: -1})
	if err == nil {
		t.Fatalf("expected error for negative max_lines")
	}
}

func TestParseFileEmptyFileWarns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	records, diags, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records non-empty: %+v", records)
	}
	if diags.WarningCount != 1 {
		t.Errorf("WarningCount = %d, want 1", diags.WarningCount)
	}
	if len(diags.Warnings) == 0 || diags.Warnings[0].Reason != "EMPTY_FILE" {
		t.Errorf("expected EMPTY_FILE warning, got %+v", diags.Warnings)
	}
}

// Fixture parity: walk the example shipped with the Python engine and
// confirm we extract the same four service names. This locks us to
// the canonical sample so future schema additions surface here first.
func TestParseFileWithSampleFixture(t *testing.T) {
	// Locate the fixture relative to the repo root. Test cwd is the
	// package dir; walk up until we hit the `examples` folder.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	var fixture string
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "examples", "otel", "sample-otel-logs.jsonl")
		if _, err := os.Stat(candidate); err == nil {
			fixture = candidate
			break
		}
		dir = filepath.Dir(dir)
	}
	if fixture == "" {
		t.Skip("sample-otel-logs.jsonl fixture not found from cwd; skipping parity check")
	}
	records, diags, err := ParseFile(fixture, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("records = %d, want 4 (fixture has 4 lines)", len(records))
	}
	if diags.SkippedLines != 0 {
		t.Errorf("SkippedLines = %d, want 0 for canonical fixture", diags.SkippedLines)
	}
	wantServices := []string{"demo-api", "demo-payment", "demo-api", "demo-api"}
	for i, want := range wantServices {
		got := strDeref(records[i].ServiceName)
		if got != want {
			t.Errorf("records[%d].ServiceName = %q, want %q", i, got, want)
		}
	}
}
