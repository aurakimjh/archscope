package profile

import (
	"os"
	"path/filepath"
	"testing"

	coreprofiler "github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
)

func TestParsePprofGzip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.pb.gz")
	if err := coreprofiler.ExportToPprof(map[string]int{
		"main.main;service.checkout;db.query": 7,
	}, path, "samples", "count", 100); err != nil {
		t.Fatalf("export pprof: %v", err)
	}
	parsed, diags, err := ParseFile(path, "auto", Options{})
	if err != nil {
		t.Fatalf("parse pprof: %v", err)
	}
	if parsed.Format != "pprof-gz" {
		t.Fatalf("format = %q", parsed.Format)
	}
	if diags.ParsedRecords != 1 {
		t.Fatalf("parsed records = %d", diags.ParsedRecords)
	}
	if got := parsed.Samples[0].Value; got != 7 {
		t.Fatalf("sample value = %d", got)
	}
	if len(parsed.Samples[0].Stack) != 3 {
		t.Fatalf("stack depth = %d", len(parsed.Samples[0].Stack))
	}
}

func TestParseSpeedscopeAndPyspy(t *testing.T) {
	dir := t.TempDir()
	speedPath := filepath.Join(dir, "dotnet-speedscope.json")
	if err := os.WriteFile(speedPath, []byte(`{
  "$schema": "https://www.speedscope.app/file-format-schema.json",
  "shared": {"frames": [{"name": "Controller.Handle"}, {"name": "Service.CheckoutAsync"}]},
  "profiles": [{"type": "sampled", "samples": [[0, 1]], "weights": [5]}]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	speed, _, err := ParseFile(speedPath, "auto", Options{})
	if err != nil {
		t.Fatalf("parse speedscope: %v", err)
	}
	if speed.Format != "dotnet-speedscope-json" {
		t.Fatalf("format = %q", speed.Format)
	}
	if speed.Samples[0].Value != 5 || !speed.Samples[0].Stack[1].Async {
		t.Fatalf("speedscope sample not normalized: %+v", speed.Samples[0])
	}

	pyspyPath := filepath.Join(dir, "py-spy.txt")
	if err := os.WriteFile(pyspyPath, []byte(`Process 123: python app.py
Python v3.12
Thread 123 "MainThread"
  app.py:10 - handle
  service.py:20 - checkout
`), 0o644); err != nil {
		t.Fatal(err)
	}
	pyspy, _, err := ParseFile(pyspyPath, "auto", Options{})
	if err != nil {
		t.Fatalf("parse pyspy: %v", err)
	}
	if pyspy.Format != "pyspy-raw" {
		t.Fatalf("format = %q", pyspy.Format)
	}
	if pyspy.Samples[0].Language != "Python" {
		t.Fatalf("language = %q", pyspy.Samples[0].Language)
	}
}
