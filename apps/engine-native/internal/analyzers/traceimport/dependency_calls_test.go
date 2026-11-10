package traceimport

import (
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/traceimport"
	"testing"
)

func logicalCallSpans() []parser.Span {
	client := elapsedSpan("c", "", 0, 80)
	client.ServiceName = "frontend"
	client.RemoteService = "backend"
	client.Kind = "CLIENT"
	server := elapsedSpan("s", "c", 5, 70)
	server.ServiceName = "backend"
	server.RemoteService = "frontend"
	server.Kind = "SERVER"
	return []parser.Span{client, server}
}
func dependencyForTest(t *testing.T, spans []parser.Span) map[string]any {
	t.Helper()
	r := Build(spans, "synthetic", "otlp-json", Options{})
	rows := r.Tables["service_dependencies"].([]map[string]any)
	if len(rows) != 1 {
		t.Fatal(rows)
	}
	return rows[0]
}
func TestPairedClientServerCountOnceWithClientTiming(t *testing.T) {
	spans := logicalCallSpans()
	spans[1].Error = true
	row := dependencyForTest(t, spans)
	if row["call_count"] != 1 || row["total_duration_ms"] != float64(80) || row["avg_duration_ms"] != float64(80) || row["error_count"] != 1 || row["caller"] != "frontend" || row["callee"] != "backend" {
		t.Fatal(row)
	}
}
func TestDependencySingleSidedAndDistinctRetries(t *testing.T) {
	spans := logicalCallSpans()
	for _, one := range [][]parser.Span{{spans[0]}, {spans[1]}} {
		row := dependencyForTest(t, one)
		if row["call_count"] != 1 || row["caller"] != "frontend" || row["callee"] != "backend" {
			t.Fatal(row)
		}
	}
	retry := logicalCallSpans()
	retry[0].SpanID = "c2"
	retry[1].SpanID = "s2"
	retry[1].ParentSpanID = "c2"
	row := dependencyForTest(t, append(spans, retry...))
	if row["call_count"] != 2 || row["total_duration_ms"] != float64(160) {
		t.Fatal(row)
	}
	retry[0].TraceID = "other"
	retry[1].TraceID = "other"
	retry[0].SpanID = "c"
	retry[1].SpanID = "s"
	retry[1].ParentSpanID = "c"
	row = dependencyForTest(t, append(spans, retry...))
	if row["call_count"] != 2 {
		t.Fatal("cross-trace ID collision", row)
	}
}
func TestDependencyMultipleServerAttemptsRetainSeparateCalls(t *testing.T) {
	spans := logicalCallSpans()
	other := spans[1]
	other.SpanID = "s2"
	other.DurationNanos = 60_000_000
	other.Error = true
	spans = append(spans, other)
	row := dependencyForTest(t, spans)
	if row["call_count"] != 2 || row["total_duration_ms"] != float64(130) || row["error_count"] != 1 {
		t.Fatal(row)
	}
}

func TestPairedServerSkipDoesNotDropDifferentRemoteChild(t *testing.T) {
	spans := logicalCallSpans()
	other := spans[1]
	other.SpanID = "other-server"
	other.ServiceName = "other-backend"
	spans = append(spans, other)
	rows := Build(spans, "synthetic", "otlp-json", Options{}).Tables["service_dependencies"].([]map[string]any)
	if len(rows) != 2 {
		t.Fatal(rows)
	}
}
