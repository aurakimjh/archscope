package accesslog

import (
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/accesslog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func analyzeLatencyLines(t *testing.T, lines ...string) mapResultForLatency {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic.log")
	os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600)
	result, err := Analyze(path, "nginx", Options{})
	if err != nil {
		t.Fatal(err)
	}
	return mapResultForLatency{result.Summary, result.Series, result.Tables}
}

type mapResultForLatency struct{ summary, series, tables map[string]any }

const latencyLine = `127.0.0.1 - - [10/Nov/2026:10:00:00 +0900] "GET /synthetic HTTP/1.1" 200 10 "-" "synthetic"`

func TestMissingLatencyDoesNotBecomeZero(t *testing.T) {
	r := analyzeLatencyLines(t, latencyLine, latencyLine+" 1.000")
	for _, key := range []string{"avg_response_ms", "p50_response_ms", "p95_response_ms", "api_avg_response_ms"} {
		if r.summary[key] != float64(1000) {
			t.Fatal(key, r.summary[key])
		}
	}
	if r.summary["total_requests"] != 2 || r.summary["response_time_sample_count"] != 1 || r.summary["response_time_unknown_count"] != 1 {
		t.Fatal(r.summary)
	}
	minute := r.series["p95_response_time_per_minute"].([]map[string]any)[0]
	if minute["value"] != float64(1000) || minute["sample_count"] != 1 || minute["unknown_count"] != 1 {
		t.Fatal(minute)
	}
	row := r.tables["url_stats"].([]map[string]any)[0]
	if row["count"] != 2 || row["avg_response_ms"] != float64(1000) {
		t.Fatal(row)
	}
	samples := r.tables["sample_records"].([]map[string]any)
	if samples[0]["response_time_ms"] != nil || samples[0]["response_time_available"] != false {
		t.Fatal(samples[0])
	}
}
func TestAllMissingAndRealZeroLatency(t *testing.T) {
	r := analyzeLatencyLines(t, latencyLine, latencyLine)
	if r.summary["avg_response_ms"] != nil || r.summary["p95_response_ms"] != nil || r.summary["total_requests"] != 2 || r.summary["total_bytes"] != 20 {
		t.Fatal(r.summary)
	}
	if r.series["p95_response_time_per_minute"].([]map[string]any)[0]["value"] != nil {
		t.Fatal(r.series)
	}
	if r.tables["top_urls_by_avg_response_time"].([]map[string]any)[0]["avg_response_ms"] != nil {
		t.Fatal(r.tables)
	}
	zero := analyzeLatencyLines(t, latencyLine+" 0.000", latencyLine)
	if zero.summary["avg_response_ms"] != float64(0) || zero.summary["response_time_sample_count"] != 1 {
		t.Fatal(zero.summary)
	}
}
func TestUnknownLatencyKeepsRouteEdgeAndErrorCounters(t *testing.T) {
	rows := []parser.Record{{Timestamp: time.Now(), URI: "/r", Route: "/r", ServiceName: "front", UpstreamService: "back", Status: 500, BytesSent: 10, ResponseTimeUnavailable: true}, {Timestamp: time.Now(), URI: "/r", Route: "/r", ServiceName: "front", UpstreamService: "back", Status: 200, BytesSent: 20, ResponseTimeMS: 1000}}
	r := Build(rows, "synthetic", "nginx", nil, Options{})
	edge := r.Tables["service_dependencies"].([]map[string]any)[0]
	if edge["call_count"] != 2 || edge["error_count"] != 1 || edge["duration_sample_count"] != 1 || edge["avg_duration_ms"] != float64(1000) {
		t.Fatal(edge)
	}
	route := r.Tables["route_stats"].([]map[string]any)[0]
	if route["count"] != 2 || route["response_time_unknown_count"] != 1 || route["avg_response_ms"] != float64(1000) {
		t.Fatal(route)
	}
}
