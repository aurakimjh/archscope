package observability

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeLokiAndGrafanaEvidence(t *testing.T) {
	dir := t.TempDir()
	loki := filepath.Join(dir, "loki.json")
	if err := os.WriteFile(loki, []byte(`{"data":{"result":[{"stream":{"service":"api","level":"error","trace_id":"trace-1"},"values":[["1778916000000000000","request failed"]]}]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Analyze(loki, Options{Format: "auto"})
	if err != nil {
		t.Fatalf("Analyze loki: %v", err)
	}
	if result.Summary["error_count"] != 1 || result.Summary["trace_reference_count"] != 1 {
		t.Fatalf("loki summary=%+v", result.Summary)
	}
	dash := filepath.Join(dir, "dashboard.json")
	if err := os.WriteFile(dash, []byte(`{"uid":"svc","title":"Service","panels":[{"id":1,"title":"Latency","type":"timeseries"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err = Analyze(dash, Options{Format: "auto"})
	if err != nil {
		t.Fatalf("Analyze dashboard: %v", err)
	}
	if result.Summary["dashboard_panel_count"] != 1 {
		t.Fatalf("dashboard summary=%+v", result.Summary)
	}
}
