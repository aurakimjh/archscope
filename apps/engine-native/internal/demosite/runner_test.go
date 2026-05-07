// Tests mirroring engines/python/tests/test_demo_site.py
// (TestDemoSiteRunner). Same fixtures + the sample nginx access log
// from <repo>/examples/access-logs/sample-nginx-access.log.
package demosite

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot walks up from this test file to the workspace root. We
// rely on it for the access-log fixture path so tests work regardless
// of cwd.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is .../apps/engine-native/internal/demosite/runner_test.go
	// repo  is file/../../../../..
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverManifestsFindsNestedJSON(t *testing.T) {
	tmp := t.TempDir()
	scenarioDir := filepath.Join(tmp, "synthetic", "test-scenario")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(scenarioDir, "manifest.json")
	if err := os.WriteFile(manifest, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := DiscoverDemoManifests(tmp)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(found) != 1 || found[0] != manifest {
		t.Fatalf("discover returned %v; want [%s]", found, manifest)
	}
}

func TestDiscoverManifestsReturnsSingleFile(t *testing.T) {
	tmp := t.TempDir()
	manifest := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifest, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := DiscoverDemoManifests(manifest)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(found) != 1 || found[0] != manifest {
		t.Fatalf("got %v; want [%s]", found, manifest)
	}
}

func TestRunDemoManifestWithAccessLog(t *testing.T) {
	root := repoRoot(t)
	sample := filepath.Join(root, "examples", "access-logs", "sample-nginx-access.log")
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("sample not available: %v", err)
	}

	tmp := t.TempDir()
	mappingFile := filepath.Join(tmp, "analyzer_type_mapping.json")
	writeJSON(t, mappingFile, map[string]any{
		"mappings": map[string]any{
			"access_log": map[string]any{
				"command":      []string{"access-log", "analyze"},
				"input_option": "--file",
			},
		},
	})

	scenarioDir := filepath.Join(tmp, "synthetic", "smoke")
	copyFile(t, sample, filepath.Join(scenarioDir, "access.log"))

	manifest := filepath.Join(scenarioDir, "manifest.json")
	writeJSON(t, manifest, map[string]any{
		"scenario":    "smoke",
		"data_source": "synthetic",
		"description": "smoke test",
		"files": []map[string]any{
			{"file": "access.log", "analyzer_type": "access_log", "format": "nginx"},
		},
	})

	out := filepath.Join(tmp, "output")
	result, err := RunDemoSiteManifest(RunOptions{
		ManifestPath: manifest,
		OutputRoot:   out,
		WritePPTX:    false,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Scenario != "smoke" {
		t.Fatalf("scenario %q want smoke", result.Scenario)
	}
	if len(result.Runs) != 1 {
		t.Fatalf("got %d runs; want 1", len(result.Runs))
	}
	r := result.Runs[0]
	if r.Failed {
		t.Fatalf("run failed: %s", deref(r.Error))
	}
	if r.JSONPath == nil {
		t.Fatal("nil JSONPath")
	}
	if _, err := os.Stat(*r.JSONPath); err != nil {
		t.Fatalf("json output missing: %v", err)
	}

	// The access-log analyzer should report exactly 6 entries from the
	// sample log fixture (matches the Python parity assertion).
	body, err := os.ReadFile(*r.JSONPath)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["type"] != "access_log" {
		t.Fatalf("type %v want access_log", payload["type"])
	}
	summary := payload["summary"].(map[string]any)
	if total := summary["total_requests"]; total != float64(6) {
		t.Fatalf("total_requests = %v; want 6", total)
	}

	// Index page should mention the scenario name.
	if result.IndexPath == nil {
		t.Fatal("no index path")
	}
	indexBody, err := os.ReadFile(*result.IndexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(indexBody), "smoke") {
		t.Fatalf("index.html does not mention scenario name; first 400 bytes: %q", string(indexBody)[:min(len(indexBody), 400)])
	}
	// Demo metadata annotation should round-trip into the JSON output.
	metadata := payload["metadata"].(map[string]any)
	demoSite := metadata["demo_site"].(map[string]any)
	if demoSite["scenario"] != "smoke" {
		t.Fatalf("demo_site.scenario = %v", demoSite["scenario"])
	}
}

func TestRunDemoManifestSkipsMissingAnalyzerType(t *testing.T) {
	tmp := t.TempDir()
	mappingFile := filepath.Join(tmp, "analyzer_type_mapping.json")
	writeJSON(t, mappingFile, map[string]any{
		"mappings": map[string]any{
			"access_log": map[string]any{"command": []string{"a"}, "input_option": "--f"},
		},
	})

	scenarioDir := filepath.Join(tmp, "synthetic", "bad")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(scenarioDir, "manifest.json")
	writeJSON(t, manifest, map[string]any{
		"scenario": "bad",
		"files": []map[string]any{
			{"file": "x.log", "analyzer_type": "unknown_type"},
			{"file": "", "analyzer_type": "access_log"},
		},
	})

	out := filepath.Join(tmp, "output")
	result, err := RunDemoSiteManifest(RunOptions{
		ManifestPath: manifest,
		OutputRoot:   out,
		WritePPTX:    false,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(result.Runs) != 0 {
		t.Fatalf("got %d runs; want 0", len(result.Runs))
	}
	if len(result.SkippedFiles) != 2 {
		t.Fatalf("got %d skipped; want 2", len(result.SkippedFiles))
	}
}

func TestDemoScenarioRunProperties(t *testing.T) {
	jp := "/tmp/a.json"
	errStr := "file not found"
	runOK := DemoAnalyzerRun{
		AnalyzerType: "access_log",
		File:         "a.log",
		Command:      []string{"access-log", "analyze"},
		JSONPath:     &jp,
	}
	runFail := DemoAnalyzerRun{
		AnalyzerType: "gc_log",
		File:         "b.log",
		Command:      []string{"gc-log", "analyze"},
		Failed:       true,
		Error:        &errStr,
	}
	scenario := DemoScenarioRun{
		Scenario:     "test",
		DataSource:   "synthetic",
		ManifestPath: "/m.json",
		OutputDir:    "/out",
		Runs:         []DemoAnalyzerRun{runOK, runFail},
	}
	failed := scenario.FailedRuns()
	if len(failed) != 1 || failed[0].File != "b.log" {
		t.Fatalf("failed_runs wrong: %+v", failed)
	}
	jsonPaths := scenario.JSONPaths()
	if len(jsonPaths) != 1 || jsonPaths[0] != jp {
		t.Fatalf("json_paths wrong: %+v", jsonPaths)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
