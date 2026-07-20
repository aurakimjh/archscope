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

func TestParseV8CPUProfileUsesMicrosecondDeltasAndCanonicalFrames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkout.cpuprofile")
	if err := os.WriteFile(path, []byte(`{
  "startTime": 1000,
  "endTime": 1600,
  "nodes": [
    {"id": 1, "callFrame": {"functionName": "(root)", "url": "", "lineNumber": 0}, "children": [2]},
    {"id": 2, "callFrame": {"functionName": "renderList", "url": "https://example.test/app.js?token=secret", "lineNumber": 41}}
  ],
  "samples": [2, 2, 2],
  "timeDeltas": [100, 200, 300]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, diags, err := ParseFile(path, "auto", Options{})
	if err != nil {
		t.Fatalf("parse V8 profile: %v", err)
	}
	if parsed.Format != "v8-cpuprofile" || parsed.ValueUnit != "microseconds" {
		t.Fatalf("unexpected V8 normalization: format=%q unit=%q", parsed.Format, parsed.ValueUnit)
	}
	if diags.ParsedRecords != 3 {
		t.Fatalf("parsed records = %d", diags.ParsedRecords)
	}
	// Chrome attributes delta i to sample i-1; the first sample has no prior
	// interval and is deliberately zero rather than fabricated from hitCount.
	if got := []int64{parsed.Samples[0].Value, parsed.Samples[1].Value, parsed.Samples[2].Value}; got[0] != 0 || got[1] != 200 || got[2] != 300 {
		t.Fatalf("unexpected V8 sample deltas: %#v", got)
	}
	if got := parsed.Samples[2].Stack[1]; got.Name != "renderList" || got.File != "https://example.test/app.js?token=<TOKEN len=6>" || got.Line != 42 || got.Runtime != "V8" {
		t.Fatalf("unexpected canonical V8 frame: %+v", got)
	}
}

func TestParseV8RejectsBrokenGraph(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.cpuprofile")
	if err := os.WriteFile(path, []byte(`{"nodes":[{"id":1,"callFrame":{},"children":[99]}],"samples":[1],"timeDeltas":[1]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ParseFile(path, "v8-cpuprofile", Options{}); err == nil {
		t.Fatal("expected malformed V8 graph to be rejected")
	}
}
