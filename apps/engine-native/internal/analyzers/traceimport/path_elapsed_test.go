package traceimport

import (
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/traceimport"
	"strconv"
	"testing"
)

func elapsedSpan(id, parent string, start, duration int64) parser.Span {
	return parser.Span{TraceID: "t", SpanID: id, ParentSpanID: parent, Name: id, ServiceName: id, StartUnixNanos: 1_700_000_000_000_000_000 + start*1_000_000, DurationNanos: duration * 1_000_000}
}
func TestObservedPathDoesNotAddInclusiveDurations(t *testing.T) {
	cases := map[string][]parser.Span{
		"nested":     {elapsedSpan("r", "", 0, 100), elapsedSpan("a", "r", 10, 80), elapsedSpan("b", "a", 15, 70)},
		"sequential": {elapsedSpan("r", "", 0, 100), elapsedSpan("a", "r", 10, 30), elapsedSpan("b", "r", 50, 40)},
		"parallel":   {elapsedSpan("r", "", 0, 100), elapsedSpan("a", "r", 10, 80), elapsedSpan("b", "r", 20, 60)},
	}
	for name, spans := range cases {
		t.Run(name, func(t *testing.T) {
			r := Build(spans, "synthetic", "otlp-json", Options{})
			row := r.Tables["critical_paths"].([]map[string]any)[0]
			if row["observed_path_elapsed_ms"] != float64(100) || row["critical_path_ms"] != float64(100) {
				t.Fatal(row)
			}
			if row["path_semantics"] != "observed_span_chain_elapsed_estimate_v1" || row["causal_critical_path_proven"] != false {
				t.Fatal(row)
			}
			if name == "nested" && row["selected_chain_cumulative_ms"] != float64(250) {
				t.Fatal("cumulative work disappeared", row)
			}
		})
	}
}
func TestObservedPathUnavailableGraphs(t *testing.T) {
	missing := elapsedSpan("a", "", 0, 10)
	missing.StartUnixNanos = 0
	cases := map[string][]parser.Span{"missing_timestamp": {missing}, "duplicate_span_id": {elapsedSpan("a", "", 0, 10), elapsedSpan("a", "", 0, 10)}, "cyclic_parent_graph": {elapsedSpan("a", "b", 0, 10), elapsedSpan("b", "a", 0, 10)}, "parent_child_clock_skew": {elapsedSpan("r", "", 0, 100), elapsedSpan("a", "r", 90, 30)}}
	for reason, spans := range cases {
		t.Run(reason, func(t *testing.T) {
			row := Build(spans, "synthetic", "otlp-json", Options{}).Tables["critical_paths"].([]map[string]any)[0]
			if row["observed_path_elapsed_ms"] != nil || row["critical_path_ms"] != nil || row["duration_unavailable_reason"] != reason {
				t.Fatal(row)
			}
		})
	}
}
func TestObservedPathDeepGraphAndZero(t *testing.T) {
	spans := make([]parser.Span, 20000)
	for i := range spans {
		parent := ""
		if i > 0 {
			parent = spans[i-1].SpanID
		}
		spans[i] = elapsedSpan(strconv.Itoa(i), parent, 0, 100)
	}
	p := longestSpanChain("t", spans)
	if p.SpanCount != len(spans) || p.DurationNanos != 100_000_000 {
		t.Fatal(p.SpanCount, p.DurationNanos)
	}
	row := criticalPathRows([]criticalPath{longestSpanChain("t", []parser.Span{elapsedSpan("z", "", 0, 0)})}, 1)[0]
	if row["observed_path_elapsed_ms"] != float64(0) {
		t.Fatal(row)
	}
}
