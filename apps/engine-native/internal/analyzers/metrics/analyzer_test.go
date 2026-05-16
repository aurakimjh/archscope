package metrics

import (
	"testing"

	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrics"
)

func TestAnalyzeMetricsSnapshot(t *testing.T) {
	result := Build([]parser.Sample{
		{Name: "http_request_duration_seconds", Labels: map[string]string{"service": "api"}, Value: 0.25},
		{Name: "http_requests_total", Labels: map[string]string{"service": "api", "code": "200"}, Value: 10},
		{Name: "http_errors_total", Labels: map[string]string{"service": "api"}, Value: 1},
	}, "metrics.prom", nil, Options{})
	if result.Summary["metric_count"] != 3 {
		t.Fatalf("summary=%+v", result.Summary)
	}
	if rows := result.Tables["golden_signal_candidates"].([]map[string]any); len(rows) != 3 {
		t.Fatalf("golden_signal_candidates=%+v", rows)
	}
	if len(result.Metadata.Findings) != 1 || result.Metadata.Findings[0]["code"] != "METRICS_ERROR_SIGNALS_PRESENT" {
		t.Fatalf("findings=%+v", result.Metadata.Findings)
	}
}
