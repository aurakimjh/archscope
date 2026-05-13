package traceimport

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func examplesRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	return filepath.Join(repo, "examples", "traces")
}

func TestParseOTLPJSONFile(t *testing.T) {
	path := filepath.Join(examplesRoot(t), "sample-otlp-traces.jsonl")
	result, err := ParseFile(path, Options{Format: FormatOTLPJSON})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if result.Format != FormatOTLPJSON {
		t.Fatalf("Format = %q", result.Format)
	}
	if got, want := len(result.Spans), 3; got != want {
		t.Fatalf("spans = %d, want %d", got, want)
	}
	if result.Diagnostics.ParsedRecords != 3 {
		t.Fatalf("ParsedRecords = %d", result.Diagnostics.ParsedRecords)
	}
	if result.Spans[0].ServiceName != "checkout-service" {
		t.Fatalf("service = %q", result.Spans[0].ServiceName)
	}
	if !result.Spans[2].Error {
		t.Fatalf("payment server span should be marked error")
	}
}

func TestParseZipkinV2JSONFile(t *testing.T) {
	path := filepath.Join(examplesRoot(t), "sample-zipkin-v2.json")
	result, err := ParseFile(path, Options{Format: FormatZipkinV2})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got, want := len(result.Spans), 3; got != want {
		t.Fatalf("spans = %d, want %d", got, want)
	}
	if result.Spans[1].RemoteService != "inventory-service" {
		t.Fatalf("remote service = %q", result.Spans[1].RemoteService)
	}
	if result.Spans[2].DurationNanos != 40_000_000 {
		t.Fatalf("duration nanos = %d", result.Spans[2].DurationNanos)
	}
}

func TestAutoDetectFormats(t *testing.T) {
	root := examplesRoot(t)
	otlp, err := ParseFile(filepath.Join(root, "sample-otlp-traces.jsonl"), Options{})
	if err != nil {
		t.Fatalf("auto otlp: %v", err)
	}
	if otlp.Format != FormatOTLPJSON {
		t.Fatalf("auto otlp format = %q", otlp.Format)
	}
	zipkin, err := ParseFile(filepath.Join(root, "sample-zipkin-v2.json"), Options{})
	if err != nil {
		t.Fatalf("auto zipkin: %v", err)
	}
	if zipkin.Format != FormatZipkinV2 {
		t.Fatalf("auto zipkin format = %q", zipkin.Format)
	}
	elastic, err := ParseFile(filepath.Join(root, "sample-elastic-apm-search.json"), Options{})
	if err != nil {
		t.Fatalf("auto elastic: %v", err)
	}
	if elastic.Format != FormatElasticSearchJSON {
		t.Fatalf("auto elastic format = %q", elastic.Format)
	}
	wrappedSourcePath := filepath.Join(t.TempDir(), "elastic-source-hit.ndjson")
	if err := os.WriteFile(wrappedSourcePath, []byte(`{"_source":{"trace":{"id":"trace-a"},"transaction":{"id":"tx-a","name":"GET /a"},"service":{"name":"api"}}}`+"\n"), 0o600); err != nil {
		t.Fatalf("write wrapped source: %v", err)
	}
	wrappedSource, err := ParseFile(wrappedSourcePath, Options{})
	if err != nil {
		t.Fatalf("auto elastic source: %v", err)
	}
	if wrappedSource.Format != FormatElasticSourceNDJSON {
		t.Fatalf("auto elastic source format = %q", wrappedSource.Format)
	}
}

func TestParseElasticAPMSearchJSON(t *testing.T) {
	path := filepath.Join(examplesRoot(t), "sample-elastic-apm-search.json")
	result, err := ParseFile(path, Options{Format: FormatElasticSearchJSON})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got, want := len(result.Spans), 3; got != want {
		t.Fatalf("spans = %d, want %d", got, want)
	}
	if result.Spans[1].RemoteService != "inventory-service" {
		t.Fatalf("remote service = %q", result.Spans[1].RemoteService)
	}
	if !result.Spans[1].Error {
		t.Fatalf("failure outcome should be marked error")
	}
}

func TestParseElasticAPMSourceNDJSON(t *testing.T) {
	path := filepath.Join(examplesRoot(t), "sample-elastic-apm-source.ndjson")
	result, err := ParseFile(path, Options{Format: FormatElasticSourceNDJSON})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got, want := len(result.Spans), 2; got != want {
		t.Fatalf("spans = %d, want %d", got, want)
	}
	if result.Spans[0].SourceFormat != FormatElasticSourceNDJSON {
		t.Fatalf("source format = %q", result.Spans[0].SourceFormat)
	}
}
