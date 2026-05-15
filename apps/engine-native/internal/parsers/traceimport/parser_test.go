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
	jaeger, err := ParseFile(filepath.Join(root, "sample-jaeger-query.json"), Options{})
	if err != nil {
		t.Fatalf("auto jaeger: %v", err)
	}
	if jaeger.Format != FormatJaegerQueryJSON {
		t.Fatalf("auto jaeger format = %q", jaeger.Format)
	}
	skywalking, err := ParseFile(filepath.Join(root, "sample-skywalking-query-trace.json"), Options{})
	if err != nil {
		t.Fatalf("auto skywalking: %v", err)
	}
	if skywalking.Format != FormatSkyWalkingGraphQL {
		t.Fatalf("auto skywalking format = %q", skywalking.Format)
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

func TestParseJaegerQueryJSON(t *testing.T) {
	path := filepath.Join(examplesRoot(t), "sample-jaeger-query.json")
	result, err := ParseFile(path, Options{Format: FormatJaegerQueryJSON})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got, want := len(result.Spans), 3; got != want {
		t.Fatalf("spans = %d, want %d", got, want)
	}
	if result.Spans[0].ServiceName != "checkout-service" {
		t.Fatalf("service = %q", result.Spans[0].ServiceName)
	}
	if result.Spans[1].RemoteService != "inventory-service" {
		t.Fatalf("remote service = %q", result.Spans[1].RemoteService)
	}
	if result.Spans[2].ParentSpanID != "inventory" {
		t.Fatalf("parent span = %q", result.Spans[2].ParentSpanID)
	}
}

func TestParseSkyWalkingGraphQLTrace(t *testing.T) {
	path := filepath.Join(examplesRoot(t), "sample-skywalking-query-trace.json")
	result, err := ParseFile(path, Options{Format: FormatSkyWalkingGraphQL})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got, want := len(result.Spans), 2; got != want {
		t.Fatalf("spans = %d, want %d", got, want)
	}
	if result.Spans[1].SpanID != "sw-seg-1:1" {
		t.Fatalf("span id = %q", result.Spans[1].SpanID)
	}
	if result.Spans[1].ParentSpanID != "sw-seg-1:0" {
		t.Fatalf("parent span id = %q", result.Spans[1].ParentSpanID)
	}
	if result.Spans[0].DurationNanos != 180_000_000 {
		t.Fatalf("duration nanos = %d", result.Spans[0].DurationNanos)
	}
}

func TestSkyWalkingUnsupportedSchemaReportsDiagnostics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skywalking-basic.json")
	if err := os.WriteFile(path, []byte(`{"data":{"queryBasicTraces":{"traces":[]}}}`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	result, err := ParseFile(path, Options{Format: FormatSkyWalkingGraphQL})
	if err != nil {
		t.Fatalf("ParseFile should return diagnostics without fatal error: %v", err)
	}
	if result.Diagnostics == nil || result.Diagnostics.ErrorCount == 0 {
		t.Fatalf("expected schema diagnostic: %#v", result.Diagnostics)
	}
	if result.Diagnostics.Errors[0].Reason != ReasonSkyWalkingSchema {
		t.Fatalf("reason = %q", result.Diagnostics.Errors[0].Reason)
	}
}
