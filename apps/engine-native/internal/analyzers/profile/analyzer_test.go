package profile

import (
	"os"
	"path/filepath"
	"testing"

	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	coreprofiler "github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
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

func TestBuildEmitsSampledCPURunsWithoutClaimingLongTasks(t *testing.T) {
	frame := parser.Frame{Name: "renderList", Function: "renderList", Runtime: "V8", Language: "JavaScript"}
	parsed := parser.Parsed{Format: "v8-cpuprofile", ValueUnit: "microseconds", Samples: []parser.Sample{{Stack: []parser.Frame{frame}, TimestampUS: 0, Value: 0}, {Stack: []parser.Frame{frame}, TimestampUS: 1_000, Value: 120_000}}}
	result := Build(parsed, "profile.cpuprofile", nil, Options{TopN: 10, ProfileKind: "cpu"})
	runs, ok := result.Tables["cpu_sample_runs"].([]map[string]any)
	if !ok || len(runs) != 1 || runs[0]["duration_ms"] != 120.0 {
		t.Fatalf("unexpected runs: %#v", result.Tables["cpu_sample_runs"])
	}
	if result.Metadata.Extra["temporal_semantics"] != "sampled_cpu_runs; not browser Long Tasks" {
		t.Fatal("temporal semantics must forbid Long Task claim")
	}
	found := false
	for _, finding := range result.Metadata.Findings {
		if finding["code"] == "SAMPLED_CPU_HOTSPOT" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected sampled CPU hotspot finding")
	}
}

func TestBrowserFlamegraphClassificationIsSourceAware(t *testing.T) {
	frame := parser.Frame{Name: "node_modules/react/index.js", Function: "render", Runtime: "V8"}
	result := Build(parser.Parsed{Format: "v8-cpuprofile", ValueUnit: "microseconds", Samples: []parser.Sample{{Stack: []parser.Frame{frame}, Value: 10}}}, "browser.cpuprofile", nil, Options{})
	flame := result.Charts["flamegraph"].(coreprofiler.FlameNode)
	if len(flame.Children) != 1 || flame.Children[0].Category == nil || *flame.Children[0].Category != "dependency" {
		t.Fatalf("unexpected browser category: %+v", flame)
	}
}
