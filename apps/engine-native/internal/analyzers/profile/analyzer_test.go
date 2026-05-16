package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeProfileEvidence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.collapsed")
	body := "app.Controller.handle;app.Service.checkout;db.Query 9\n[unknown];libnative.so;poll 4\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Analyze(path, Options{Format: "async-profiler-collapsed", TopN: 5, IntervalMS: 100})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.Type != ResultType {
		t.Fatalf("type = %q", result.Type)
	}
	if result.Summary["total_samples"] != 13 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if len(result.Metadata.Findings) == 0 {
		t.Fatalf("expected profile findings")
	}
	if _, ok := result.Charts["flamegraph"]; !ok {
		t.Fatalf("missing flamegraph")
	}
	if rows, ok := result.Tables["frames"].([]map[string]any); !ok || len(rows) == 0 {
		t.Fatalf("missing frame rows: %#v", result.Tables["frames"])
	}
}
