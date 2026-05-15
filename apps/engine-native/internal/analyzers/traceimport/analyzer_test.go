package traceimport

import (
	"path/filepath"
	"runtime"
	"testing"

	traceparser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/traceimport"
)

func traceExamplesRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	return filepath.Join(repo, "examples", "traces")
}

func TestAnalyzeOTLPTraceImport(t *testing.T) {
	result, err := Analyze(filepath.Join(traceExamplesRoot(t), "sample-otlp-traces.jsonl"), Options{
		Format: traceparser.FormatOTLPJSON,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Type != ResultType {
		t.Fatalf("type = %q", result.Type)
	}
	if result.Summary["total_spans"] != 3 {
		t.Fatalf("total_spans = %#v", result.Summary["total_spans"])
	}
	if result.Summary["error_spans"] != 1 {
		t.Fatalf("error_spans = %#v", result.Summary["error_spans"])
	}
	if len(result.Metadata.Findings) == 0 {
		t.Fatalf("expected findings")
	}
	if rows, ok := result.Tables["critical_paths"].([]map[string]any); !ok || len(rows) == 0 {
		t.Fatalf("critical_paths = %#v", result.Tables["critical_paths"])
	}
}

func TestAnalyzeZipkinTraceImport(t *testing.T) {
	result, err := Analyze(filepath.Join(traceExamplesRoot(t), "sample-zipkin-v2.json"), Options{
		Format: traceparser.FormatZipkinV2,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Summary["unique_dependencies"] != 1 {
		t.Fatalf("unique_dependencies = %#v", result.Summary["unique_dependencies"])
	}
	rows, ok := result.Tables["service_dependencies"].([]map[string]any)
	if !ok {
		t.Fatalf("service_dependencies type = %T", result.Tables["service_dependencies"])
	}
	if len(rows) != 1 || rows[0]["caller"] != "order-service" || rows[0]["callee"] != "inventory-service" {
		t.Fatalf("dependency rows = %#v", rows)
	}
}

func TestAnalyzeElasticTraceImport(t *testing.T) {
	result, err := Analyze(filepath.Join(traceExamplesRoot(t), "sample-elastic-apm-search.json"), Options{
		Format: traceparser.FormatElasticSearchJSON,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Summary["total_spans"] != 3 {
		t.Fatalf("total_spans = %#v", result.Summary["total_spans"])
	}
	if result.Summary["unique_dependencies"] != 1 {
		t.Fatalf("unique_dependencies = %#v", result.Summary["unique_dependencies"])
	}
	if result.Summary["high_error_service_edges"] != 1 {
		t.Fatalf("high_error_service_edges = %#v", result.Summary["high_error_service_edges"])
	}
	if result.Summary["unbalanced_service_count"] != 1 {
		t.Fatalf("unbalanced_service_count = %#v", result.Summary["unbalanced_service_count"])
	}
	if !hasFinding(result.Metadata.Findings, "SLOW_TRACE_P95") {
		t.Fatalf("expected SLOW_TRACE_P95 finding: %#v", result.Metadata.Findings)
	}
	if !hasFinding(result.Metadata.Findings, "HIGH_ERROR_SERVICE_EDGE") {
		t.Fatalf("expected HIGH_ERROR_SERVICE_EDGE finding: %#v", result.Metadata.Findings)
	}
	rows, ok := result.Tables["critical_paths"].([]map[string]any)
	if !ok || len(rows) == 0 {
		t.Fatalf("critical_paths = %#v", result.Tables["critical_paths"])
	}
	if rows[0]["trace_id"] != "elastic-trace-1" {
		t.Fatalf("critical path rows = %#v", rows)
	}
}

func TestAnalyzeJaegerTraceImport(t *testing.T) {
	result, err := Analyze(filepath.Join(traceExamplesRoot(t), "sample-jaeger-query.json"), Options{
		Format: traceparser.FormatJaegerQueryJSON,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Summary["source_format"] != traceparser.FormatJaegerQueryJSON {
		t.Fatalf("source_format = %#v", result.Summary["source_format"])
	}
	if result.Summary["unique_dependencies"] != 1 {
		t.Fatalf("unique_dependencies = %#v", result.Summary["unique_dependencies"])
	}
	rows, ok := result.Tables["service_dependencies"].([]map[string]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("service_dependencies = %#v", result.Tables["service_dependencies"])
	}
	if rows[0]["caller"] != "checkout-service" || rows[0]["callee"] != "inventory-service" {
		t.Fatalf("dependency rows = %#v", rows)
	}
}

func TestAnalyzeSkyWalkingTraceImport(t *testing.T) {
	result, err := Analyze(filepath.Join(traceExamplesRoot(t), "sample-skywalking-query-trace.json"), Options{
		Format: traceparser.FormatSkyWalkingGraphQL,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Summary["source_format"] != traceparser.FormatSkyWalkingGraphQL {
		t.Fatalf("source_format = %#v", result.Summary["source_format"])
	}
	if result.Summary["total_spans"] != 2 {
		t.Fatalf("total_spans = %#v", result.Summary["total_spans"])
	}
	if result.Summary["unique_dependencies"] != 1 {
		t.Fatalf("unique_dependencies = %#v", result.Summary["unique_dependencies"])
	}
}

func TestAnalyzeClockSkewFinding(t *testing.T) {
	result := Build([]traceparser.Span{
		{
			TraceID:        "trace-a",
			SpanID:         "root",
			Name:           "root",
			ServiceName:    "frontend",
			StartUnixNanos: 1_000_000_000,
			DurationNanos:  100_000_000,
		},
		{
			TraceID:        "trace-a",
			SpanID:         "child",
			ParentSpanID:   "root",
			Name:           "child",
			ServiceName:    "backend",
			StartUnixNanos: 980_000_000,
			DurationNanos:  20_000_000,
		},
	}, "synthetic.json", traceparser.FormatOTLPJSON, Options{})
	if result.Summary["clock_skew_suspected_spans"] != 1 {
		t.Fatalf("clock_skew_suspected_spans = %#v", result.Summary["clock_skew_suspected_spans"])
	}
	if !hasFinding(result.Metadata.Findings, "CLOCK_SKEW_SUSPECTED") {
		t.Fatalf("expected CLOCK_SKEW_SUSPECTED finding: %#v", result.Metadata.Findings)
	}
}

func hasFinding(findings []map[string]any, code string) bool {
	for _, finding := range findings {
		if finding["code"] == code {
			return true
		}
	}
	return false
}
