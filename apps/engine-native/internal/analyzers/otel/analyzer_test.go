package otel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	otelparser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/otel"
)

// writeJSONL writes a list of payload objects, one JSON object per
// line. Mirrors the writeJSONL helper in the otel parser tests so the
// fixture shape is identical across the two suites.
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

// fixturePath walks up from the test's cwd looking for the canonical
// example shipped with the repo. Returns "" if not found.
func fixturePath(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "examples", "otel", "sample-otel-logs.jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

// rowSlice extracts a `[]map[string]any` value from a tables map and
// fails the test if it's missing or has the wrong type.
func rowSlice(t *testing.T, tables map[string]any, key string) []map[string]any {
	t.Helper()
	raw, ok := tables[key]
	if !ok {
		t.Fatalf("tables[%q] missing", key)
	}
	rows, ok := raw.([]map[string]any)
	if !ok {
		t.Fatalf("tables[%q] is not []map[string]any (got %T)", key, raw)
	}
	return rows
}

// Mirrors test_otel_jsonl_analyzer_builds_trace_correlation.
func TestAnalyzeFixtureBuildsTraceCorrelation(t *testing.T) {
	sample := fixturePath(t)
	if sample == "" {
		t.Skip("sample-otel-logs.jsonl fixture not found from cwd")
	}
	result, err := Analyze(sample, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Type != "otel_logs" {
		t.Fatalf("Type = %q, want otel_logs", result.Type)
	}
	if got := asInt(result.Summary["total_records"]); got != 4 {
		t.Errorf("total_records = %d, want 4", got)
	}
	if got := asInt(result.Summary["unique_traces"]); got != 2 {
		t.Errorf("unique_traces = %d, want 2", got)
	}
	if got := asInt(result.Summary["cross_service_traces"]); got != 1 {
		t.Errorf("cross_service_traces = %d, want 1", got)
	}
	if got := asInt(result.Summary["error_records"]); got != 2 {
		t.Errorf("error_records = %d, want 2", got)
	}
	if got := asInt(result.Summary["failed_traces"]); got != 1 {
		t.Errorf("failed_traces = %d, want 1", got)
	}
	traceServicePaths := rowSlice(t, result.Tables, "trace_service_paths")
	if len(traceServicePaths) == 0 {
		t.Fatal("trace_service_paths empty")
	}
	traceFailures := rowSlice(t, result.Tables, "trace_failures")
	if len(traceFailures) == 0 {
		t.Fatal("trace_failures empty")
	}
	if got := asInt(traceFailures[0]["error_count"]); got != 2 {
		t.Errorf("trace_failures[0].error_count = %d, want 2", got)
	}
}

// Mirrors test_otel_analyzer_uses_parent_span_path_and_failure_propagation.
func TestAnalyzerUsesParentSpanPathAndFailurePropagation(t *testing.T) {
	dir := t.TempDir()
	sample := writeJSONL(t, dir, []map[string]any{
		{
			"timestamp":     "2026-04-30T10:00:00Z",
			"trace_id":      "trace-1",
			"span_id":       "root",
			"service_name":  "gateway",
			"severity_text": "INFO",
			"body":          "request started",
		},
		{
			"timestamp":      "2026-04-30T10:00:01Z",
			"trace_id":       "trace-1",
			"span_id":        "payment",
			"parent_span_id": "order",
			"service_name":   "payment",
			"severity_text":  "INFO",
			"body":           "payment call started",
		},
		{
			"timestamp":      "2026-04-30T10:00:02Z",
			"trace_id":       "trace-1",
			"span_id":        "order",
			"parent_span_id": "root",
			"service_name":   "order",
			"severity_text":  "ERROR",
			"body":           "order failed",
		},
	})
	result, err := Analyze(sample, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	tracePaths := rowSlice(t, result.Tables, "trace_service_paths")
	if len(tracePaths) == 0 {
		t.Fatal("trace_service_paths empty")
	}
	got := tracePaths[0]["service_path"].(string)
	if got != "gateway -> order -> payment" {
		t.Errorf("service_path = %q, want %q", got, "gateway -> order -> payment")
	}
	if got := asInt(result.Summary["failure_propagation_traces"]); got != 1 {
		t.Errorf("failure_propagation_traces = %d, want 1", got)
	}
	prop := rowSlice(t, result.Tables, "service_failure_propagation")
	if len(prop) == 0 {
		t.Fatal("service_failure_propagation empty")
	}
	if first := prop[0]["first_failing_service"].(string); first != "order" {
		t.Errorf("first_failing_service = %q, want order", first)
	}
	downstream, ok := prop[0]["downstream_services"].([]string)
	if !ok || len(downstream) != 1 || downstream[0] != "payment" {
		t.Errorf("downstream_services = %#v, want [payment]", prop[0]["downstream_services"])
	}
	codes := findingCodes(result.Metadata.Findings)
	if !codes["OTEL_FAILURE_PROPAGATION"] {
		t.Errorf("missing OTEL_FAILURE_PROPAGATION; got %v", codes)
	}
}

// Mirrors test_otel_analyzer_reports_span_parent_cycles.
func TestAnalyzerReportsSpanParentCycles(t *testing.T) {
	dir := t.TempDir()
	sample := writeJSONL(t, dir, []map[string]any{
		{
			"timestamp":      "2026-04-30T10:00:00Z",
			"trace_id":       "trace-cycle",
			"span_id":        "span-a",
			"parent_span_id": "span-b",
			"service_name":   "gateway",
			"severity_text":  "INFO",
			"body":           "request started",
		},
		{
			"timestamp":      "2026-04-30T10:00:01Z",
			"trace_id":       "trace-cycle",
			"span_id":        "span-b",
			"parent_span_id": "span-a",
			"service_name":   "order",
			"severity_text":  "INFO",
			"body":           "order started",
		},
		{
			"timestamp":      "2026-04-30T10:00:02Z",
			"trace_id":       "trace-self",
			"span_id":        "span-self",
			"parent_span_id": "span-self",
			"service_name":   "payment",
			"severity_text":  "INFO",
			"body":           "self parent",
		},
	})
	result, err := Analyze(sample, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := asInt(result.Summary["span_topology_warnings"]); got != 2 {
		t.Errorf("span_topology_warnings = %d, want 2", got)
	}
	warnings := rowSlice(t, result.Tables, "span_topology_warnings")
	reasons := map[string]bool{}
	for _, row := range warnings {
		reasons[row["reason"].(string)] = true
	}
	if !reasons["PARENT_CYCLE"] || !reasons["SELF_PARENT"] {
		t.Errorf("warning reasons = %v, want PARENT_CYCLE and SELF_PARENT", reasons)
	}
	codes := findingCodes(result.Metadata.Findings)
	if !codes["OTEL_SPAN_TOPOLOGY_INCONSISTENT"] {
		t.Errorf("missing OTEL_SPAN_TOPOLOGY_INCONSISTENT; got %v", codes)
	}
}

// Mirrors
// test_otel_error_detection_prioritizes_severity_before_body_keywords.
// "INFO" with "error" in body is NOT an error; missing severity
// (UNSPECIFIED) with "failed" in body IS an error.
func TestErrorDetectionPrioritizesSeverityBeforeBodyKeywords(t *testing.T) {
	dir := t.TempDir()
	sample := writeJSONL(t, dir, []map[string]any{
		{
			"timestamp":     "2026-04-30T10:00:00Z",
			"trace_id":      "trace-info",
			"span_id":       "span-info",
			"service_name":  "gateway",
			"severity_text": "INFO",
			"body":          "error budget check passed",
		},
		{
			"timestamp":    "2026-04-30T10:00:01Z",
			"trace_id":     "trace-unspecified",
			"span_id":      "span-unspecified",
			"service_name": "worker",
			"body":         "job failed before severity was attached",
		},
	})
	result, err := Analyze(sample, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := asInt(result.Summary["failed_traces"]); got != 1 {
		t.Errorf("failed_traces = %d, want 1", got)
	}
	want := []map[string]any{
		{
			"trace_id":      "trace-info",
			"service_path":  "gateway",
			"service_count": 1,
			"record_count":  1,
			"has_error":     false,
			"max_severity":  "INFO",
		},
		{
			"trace_id":      "trace-unspecified",
			"service_path":  "worker",
			"service_count": 1,
			"record_count":  1,
			"has_error":     true,
			"max_severity":  "UNSPECIFIED",
		},
	}
	tracePaths := rowSlice(t, result.Tables, "trace_service_paths")
	if len(tracePaths) != len(want) {
		t.Fatalf("trace_service_paths len = %d, want %d", len(tracePaths), len(want))
	}
	for i, expected := range want {
		got := tracePaths[i]
		for key, ev := range expected {
			gv := got[key]
			switch tv := ev.(type) {
			case int:
				if gi := asInt(gv); gi != tv {
					t.Errorf("row[%d].%s = %v, want %v", i, key, gv, ev)
				}
			default:
				if gv != ev {
					t.Errorf("row[%d].%s = %v, want %v", i, key, gv, ev)
				}
			}
		}
	}
}

// Verifies the AnalysisResult envelope matches the Python contract:
// `metadata.parser`, `metadata.schema_version`, type, source_files
// shape, and that empty findings list / Counter ordering survive a
// JSON round-trip.
func TestEnvelopeMetadataMatchesPythonContract(t *testing.T) {
	dir := t.TempDir()
	sample := writeJSONL(t, dir, []map[string]any{
		{"trace_id": "t1", "body": "hello"},
	})
	result, err := Analyze(sample, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	meta := decoded["metadata"].(map[string]any)
	if got := meta["parser"]; got != "otel_jsonl" {
		t.Errorf("metadata.parser = %v, want otel_jsonl", got)
	}
	if got := meta["schema_version"]; got != "0.1.0" {
		t.Errorf("metadata.schema_version = %v, want 0.1.0", got)
	}
	if _, ok := meta["findings"]; !ok {
		t.Errorf("metadata.findings missing")
	}
	sources := decoded["source_files"].([]any)
	if len(sources) != 1 || sources[0] != sample {
		t.Errorf("source_files = %v, want [%s]", sources, sample)
	}
}

// Cross-service trace table is keyed alphabetically by trace_id and
// truncated to topN. Mirrors Python `sorted(cross_service_traces)`.
func TestCrossServiceTracesTableSortedAndTruncated(t *testing.T) {
	dir := t.TempDir()
	payloads := []map[string]any{}
	// Three traces, each spanning 2 services. Reverse order on disk so
	// the sort is observable.
	for _, traceID := range []string{"trace-c", "trace-b", "trace-a"} {
		payloads = append(payloads,
			map[string]any{"trace_id": traceID, "span_id": "s1", "service_name": "svc-x", "body": "ok"},
			map[string]any{"trace_id": traceID, "span_id": "s2", "service_name": "svc-y", "body": "ok"},
		)
	}
	sample := writeJSONL(t, dir, payloads)
	result, err := Analyze(sample, Options{TopN: 2})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	rows := rowSlice(t, result.Tables, "cross_service_traces")
	if len(rows) != 2 {
		t.Fatalf("cross_service_traces len = %d, want 2 (truncated)", len(rows))
	}
	if rows[0]["trace_id"] != "trace-a" || rows[1]["trace_id"] != "trace-b" {
		t.Errorf("cross_service_traces order = %v %v, want trace-a trace-b", rows[0]["trace_id"], rows[1]["trace_id"])
	}
	// services should be sorted ascending.
	services := rows[0]["services"].([]string)
	if len(services) != 2 || services[0] != "svc-x" || services[1] != "svc-y" {
		t.Errorf("services = %v, want [svc-x svc-y]", services)
	}
}

// Ordered services fall back to ingestion order when no parent edges
// exist, deduping consecutive same-service runs (mirrors
// `_ordered_services` else-branch).
func TestServicePathFallsBackToTimestampOrderWithoutParents(t *testing.T) {
	dir := t.TempDir()
	sample := writeJSONL(t, dir, []map[string]any{
		{"timestamp": "2026-04-30T10:00:02Z", "trace_id": "t1", "span_id": "s2", "service_name": "b", "body": "two"},
		{"timestamp": "2026-04-30T10:00:00Z", "trace_id": "t1", "span_id": "s1", "service_name": "a", "body": "one"},
		{"timestamp": "2026-04-30T10:00:03Z", "trace_id": "t1", "span_id": "s3", "service_name": "b", "body": "three"},
		{"timestamp": "2026-04-30T10:00:04Z", "trace_id": "t1", "span_id": "s4", "service_name": "c", "body": "four"},
	})
	result, err := Analyze(sample, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	tracePaths := rowSlice(t, result.Tables, "trace_service_paths")
	if len(tracePaths) != 1 {
		t.Fatalf("trace_service_paths len = %d, want 1", len(tracePaths))
	}
	if got := tracePaths[0]["service_path"].(string); got != "a -> b -> c" {
		t.Errorf("service_path = %q, want %q", got, "a -> b -> c")
	}
}

// Findings are NOT emitted when nothing happened — empty findings
// list survives but stays empty. Mirrors Python's `_findings(summary)`
// when every counter is zero.
func TestFindingsEmptyForCleanTrace(t *testing.T) {
	dir := t.TempDir()
	sample := writeJSONL(t, dir, []map[string]any{
		{"trace_id": "t1", "span_id": "s1", "service_name": "svc", "severity_text": "INFO", "body": "hi"},
	})
	result, err := Analyze(sample, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Metadata.Findings) != 0 {
		t.Errorf("findings = %v, want empty", result.Metadata.Findings)
	}
}

// Severity / service distributions are projected via Counter.most_common.
// Larger counts come first; ties keep insertion order (Python dict).
func TestSeverityAndServiceDistributionsOrdering(t *testing.T) {
	dir := t.TempDir()
	sample := writeJSONL(t, dir, []map[string]any{
		{"trace_id": "t1", "span_id": "s1", "service_name": "first", "severity_text": "INFO", "body": "1"},
		{"trace_id": "t1", "span_id": "s2", "service_name": "second", "severity_text": "ERROR", "body": "2"},
		{"trace_id": "t1", "span_id": "s3", "service_name": "second", "severity_text": "INFO", "body": "3"},
	})
	result, err := Analyze(sample, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	severityRows := result.Series["severity_distribution"].([]map[string]any)
	if len(severityRows) != 2 {
		t.Fatalf("severity_distribution len = %d, want 2", len(severityRows))
	}
	// INFO has count 2, comes first; ERROR has count 1.
	if severityRows[0]["severity"] != "INFO" || asInt(severityRows[0]["count"]) != 2 {
		t.Errorf("severity_distribution[0] = %v", severityRows[0])
	}
	serviceRows := result.Series["service_distribution"].([]map[string]any)
	if len(serviceRows) != 2 {
		t.Fatalf("service_distribution len = %d, want 2", len(serviceRows))
	}
	if serviceRows[0]["service"] != "second" || asInt(serviceRows[0]["count"]) != 2 {
		t.Errorf("service_distribution[0] = %v", serviceRows[0])
	}
}

// _trace_failures.error_services is a comma-separated *sorted* set.
func TestTraceFailuresErrorServicesSortedAndDeduped(t *testing.T) {
	dir := t.TempDir()
	sample := writeJSONL(t, dir, []map[string]any{
		{"timestamp": "2026-04-30T10:00:00Z", "trace_id": "tx", "span_id": "s1", "service_name": "zeta", "severity_text": "ERROR", "body": "boom"},
		{"timestamp": "2026-04-30T10:00:01Z", "trace_id": "tx", "span_id": "s2", "service_name": "alpha", "severity_text": "ERROR", "body": "boom2"},
		{"timestamp": "2026-04-30T10:00:02Z", "trace_id": "tx", "span_id": "s3", "service_name": "alpha", "severity_text": "ERROR", "body": "boom3"},
	})
	result, err := Analyze(sample, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	failures := rowSlice(t, result.Tables, "trace_failures")
	if len(failures) != 1 {
		t.Fatalf("trace_failures len = %d, want 1", len(failures))
	}
	// Sorted unique: "alpha, zeta".
	if got := failures[0]["error_services"].(string); got != "alpha, zeta" {
		t.Errorf("error_services = %q, want %q", got, "alpha, zeta")
	}
	if got := asInt(failures[0]["error_count"]); got != 3 {
		t.Errorf("error_count = %d, want 3", got)
	}
}

// Span topology rows are keyed by sorted span_id and report parent +
// child counts derived from span_records. Verifies the
// `_trace_span_topology` projection used by the renderer.
func TestSpanTopologyExposesChildCountsAndParentLinks(t *testing.T) {
	dir := t.TempDir()
	sample := writeJSONL(t, dir, []map[string]any{
		{"timestamp": "2026-04-30T10:00:00Z", "trace_id": "topo", "span_id": "root", "service_name": "gateway", "severity_text": "INFO", "body": "rt"},
		{"timestamp": "2026-04-30T10:00:01Z", "trace_id": "topo", "span_id": "child-a", "parent_span_id": "root", "service_name": "svc-a", "severity_text": "INFO", "body": "a"},
		{"timestamp": "2026-04-30T10:00:02Z", "trace_id": "topo", "span_id": "child-b", "parent_span_id": "root", "service_name": "svc-b", "severity_text": "INFO", "body": "b"},
	})
	result, err := Analyze(sample, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	rows := rowSlice(t, result.Tables, "trace_span_topology")
	// 3 rows. sorted by span_id alphabetically: child-a, child-b, root.
	if len(rows) != 3 {
		t.Fatalf("trace_span_topology len = %d, want 3", len(rows))
	}
	rootRow := rows[2]
	if rootRow["span_id"] != "root" {
		t.Fatalf("rootRow.span_id = %v, want root (rows = %v)", rootRow["span_id"], rows)
	}
	if asInt(rootRow["child_count"]) != 2 {
		t.Errorf("root.child_count = %v, want 2", rootRow["child_count"])
	}
	childA := rows[0]
	if childA["span_id"] != "child-a" {
		t.Fatalf("childA.span_id = %v, want child-a", childA["span_id"])
	}
	if childA["parent_span_id"] != "root" {
		t.Errorf("child-a.parent_span_id = %v, want root", childA["parent_span_id"])
	}
}

// service_trace_matrix flattens trace × service counts. Sorted by
// trace_id alphabetically; within each trace, services sorted
// alphabetically.
func TestServiceTraceMatrixFlattensCounts(t *testing.T) {
	dir := t.TempDir()
	sample := writeJSONL(t, dir, []map[string]any{
		{"trace_id": "t-1", "span_id": "s1", "service_name": "alpha", "body": "1"},
		{"trace_id": "t-1", "span_id": "s2", "service_name": "alpha", "body": "2"},
		{"trace_id": "t-1", "span_id": "s3", "service_name": "beta", "body": "3"},
	})
	result, err := Analyze(sample, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	rows := rowSlice(t, result.Tables, "service_trace_matrix")
	if len(rows) != 2 {
		t.Fatalf("service_trace_matrix len = %d, want 2", len(rows))
	}
	if rows[0]["service"] != "alpha" || asInt(rows[0]["record_count"]) != 2 {
		t.Errorf("matrix[0] = %v", rows[0])
	}
	if rows[1]["service"] != "beta" || asInt(rows[1]["record_count"]) != 1 {
		t.Errorf("matrix[1] = %v", rows[1])
	}
}

// Build skips the parser entirely — verify we can hand-feed records.
// Mirrors the access-log analyzer's Analyze/Build split.
func TestBuildAcceptsManuallyConstructedRecords(t *testing.T) {
	// Construct two records: one with parent edge, one without.
	tid := "manual"
	sidA, sidB := "sa", "sb"
	svcA, svcB := "svc-a", "svc-b"
	recs := []manualRecord{
		{trace: &tid, span: &sidA, service: &svcA, severity: "INFO", body: "alpha"},
		{trace: &tid, span: &sidB, parent: &sidA, service: &svcB, severity: "ERROR", body: "boom"},
	}
	parsed := buildManualRecords(recs)
	result := Build(parsed, "manual.jsonl", nil, Options{})
	if got := asInt(result.Summary["total_records"]); got != 2 {
		t.Errorf("total_records = %d, want 2", got)
	}
	if got := asInt(result.Summary["error_records"]); got != 1 {
		t.Errorf("error_records = %d, want 1", got)
	}
	tracePaths := rowSlice(t, result.Tables, "trace_service_paths")
	if len(tracePaths) != 1 {
		t.Fatalf("trace_service_paths len = %d, want 1", len(tracePaths))
	}
	if got := tracePaths[0]["service_path"].(string); got != "svc-a -> svc-b" {
		t.Errorf("service_path = %q, want %q", got, "svc-a -> svc-b")
	}
}

// Helper for TestBuildAcceptsManuallyConstructedRecords.
type manualRecord struct {
	trace    *string
	span     *string
	parent   *string
	service  *string
	severity string
	body     string
}

func buildManualRecords(in []manualRecord) []otelparser.Record {
	out := make([]otelparser.Record, 0, len(in))
	for _, r := range in {
		out = append(out, otelparser.Record{
			TraceID:      r.trace,
			SpanID:       r.span,
			ParentSpanID: r.parent,
			ServiceName:  r.service,
			Severity:     r.severity,
			Body:         r.body,
		})
	}
	return out
}

// findingCodes collects all `code` strings from a findings list.
func findingCodes(findings []map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, f := range findings {
		if code, ok := f["code"].(string); ok {
			out[code] = true
		}
	}
	return out
}
