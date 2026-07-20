package httpcapture

import (
	"testing"
	"time"

	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/httpcapture"
)

func TestBuildHTTPHARAnalysisIsDeterministicAndBounded(t *testing.T) {
	entries := []parser.Entry{{StartedAt: time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC), Method: "GET", Path: "/orders", Host: "api", Status: 200, DurationMS: 25}, {StartedAt: time.Date(2026, 7, 20, 10, 0, 5, 0, time.UTC), Method: "GET", Path: "/orders", Host: "api", Status: 503, DurationMS: 50, Error: true}}
	result := Build(entries, "sample.har", "har", "chrome", Options{TopN: 1})
	if result.Type != ResultType || result.Summary["total_transactions"] != 2 || result.Summary["error_transactions"] != 1 {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
	if rows, ok := result.Tables["endpoints"].([]map[string]any); !ok || len(rows) != 1 || rows[0]["avg_duration_ms"] != 37.5 {
		t.Fatalf("unexpected endpoints: %#v", result.Tables["endpoints"])
	}
	if result.Metadata.Extra["http_capture"].(map[string]any)["fidelity"] != "har_import" {
		t.Fatal("missing fidelity metadata")
	}
	if _, ok := result.Metadata.Extra["capture_aggregate_snapshot"]; !ok {
		t.Fatal("HAR must use capture aggregate snapshot")
	}
}
