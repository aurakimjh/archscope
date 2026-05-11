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
