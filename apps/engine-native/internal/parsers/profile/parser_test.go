package profile

import (
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// timeDeltas reconstruct observation timestamps. Each sample owns the
	// interval until the next observation; the last sample owns the endTime
	// tail. This pins the direction instead of allowing a one-sample shift.
	if got := []int64{parsed.Samples[0].Value, parsed.Samples[1].Value, parsed.Samples[2].Value}; got[0] != 200 || got[1] != 300 || got[2] != 0 {
		t.Fatalf("unexpected V8 sample deltas: %#v", got)
	}
	if got := []int64{parsed.Samples[0].TimestampUS, parsed.Samples[1].TimestampUS, parsed.Samples[2].TimestampUS}; got[0] != 1_100 || got[1] != 1_300 || got[2] != 1_600 {
		t.Fatalf("unexpected V8 sample timestamps: %#v", got)
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

func TestParseChromeTraceAssemblesProfileChunks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.json")
	data := `{"traceEvents":[{"ph":"P","name":"Profile","args":{"data":{"cpuProfile":{"nodes":[{"id":1,"callFrame":{"functionName":"root"},"children":[2]},{"id":2,"callFrame":{"functionName":"paint","url":"app.js","lineNumber":0}}]}}}},{"ph":"P","name":"ProfileChunk","args":{"data":{"samples":[2,2],"timeDeltas":[10,30]}}}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, _, err := ParseFile(path, "auto", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Format != "chrome-trace-json" || len(parsed.Samples) != 2 || parsed.Samples[0].Value != 30 || parsed.Samples[1].Value != 0 {
		t.Fatalf("unexpected Chrome trace normalization: %+v", parsed)
	}
}

func TestParseChromeTraceAcceptsBareEventArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace-array.json")
	data := `[{"ph":"P","args":{"data":{"nodes":[{"id":1,"callFrame":{}}],"samples":[1,1],"timeDeltas":[10,20]}}}]`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, _, err := ParseFile(path, "chrome-trace-json", Options{})
	if err != nil {
		t.Fatalf("parse bare event array: %v", err)
	}
	if len(parsed.Samples) != 2 || parsed.Samples[0].Value != 20 || parsed.Samples[1].Value != 0 {
		t.Fatalf("unexpected bare-array trace parse: %#v", parsed.Samples)
	}
}

func TestParseChromeTraceReportsNegativeDelta(t *testing.T) {
	path := filepath.Join(t.TempDir(), "negative-trace.json")
	data := `{"traceEvents":[{"ph":"P","args":{"data":{"nodes":[{"id":1,"callFrame":{"functionName":"work"}}],"samples":[1,1,1],"timeDeltas":[10,-20,30]}}}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, diags, err := ParseFile(path, "chrome-trace-json", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Metadata["negative_delta_clamp_count"] != 1 {
		t.Fatalf("negative clamp metadata = %#v", parsed.Metadata)
	}
	found := false
	for _, warning := range diags.Warnings {
		if warning.Reason == "PROFILE_NEGATIVE_DELTA_CLAMPED" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing trace negative-delta warning: %#v", diags.Warnings)
	}
}

func TestParseGzipChromeTrace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.json.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	_, _ = gz.Write([]byte(`{"traceEvents":[{"ph":"P","args":{"data":{"nodes":[{"id":1,"callFrame":{}}],"samples":[1],"timeDeltas":[10]}}}]}`))
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	parsed, _, err := ParseFile(path, "auto", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Format != "chrome-trace-json" {
		t.Fatalf("format = %q", parsed.Format)
	}
}

func TestParseV8DownsamplesDeterministicallyAndMarksPartial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.cpuprofile")
	data := `{"nodes":[{"id":1,"callFrame":{}}],"samples":[1,1,1,1],"timeDeltas":[10,20,30,40]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, diags, err := ParseFile(path, "v8-cpuprofile", Options{MaxSamples: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Samples) != 2 || parsed.Samples[0].Value != 50 || parsed.Samples[1].Value != 40 {
		t.Fatalf("unexpected downsample: %+v", parsed.Samples)
	}
	if parsed.Metadata["partial_result"] != true || diags.WarningCount == 0 {
		t.Fatalf("missing partial diagnostic: %#v %#v", parsed.Metadata, diags)
	}
}

func TestParseChromeTraceStreamsGzipAndPreservesWeightedDuration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large-trace.json.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	trace := `{"traceEvents":[{"ph":"P","args":{"data":{"cpuProfile":{"startTime":100,"nodes":[{"id":1,"callFrame":{"functionName":"root"},"children":[2]},{"id":2,"callFrame":{"functionName":"work"}}]}}}},{"ph":"P","args":{"data":{"samples":[2,2,2,2,2,2],"timeDeltas":[10,20,30,40,50,60]}}}]}`
	if _, err := gz.Write([]byte(trace)); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	parsed, diags, err := ParseFile(path, "auto", Options{MaxSamples: 3})
	if err != nil {
		t.Fatalf("parse streamed gzip trace: %v", err)
	}
	if parsed.Format != "chrome-trace-json" || len(parsed.Samples) != 3 {
		t.Fatalf("unexpected streamed trace result: %#v", parsed)
	}
	if got := []int64{parsed.Samples[0].Value, parsed.Samples[1].Value, parsed.Samples[2].Value}; got[0] != 50 || got[1] != 90 || got[2] != 60 {
		t.Fatalf("weighted buckets = %#v", got)
	}
	if total := parsed.Samples[0].Value + parsed.Samples[1].Value + parsed.Samples[2].Value; total != 200 {
		t.Fatalf("weighted duration = %d, want 200", total)
	}
	if parsed.Metadata["partial_result"] != true || parsed.Metadata["downsampling"] != "deterministic_time_weighted_buckets" || diags.WarningCount != 1 {
		t.Fatalf("missing streaming partial-result contract: metadata=%#v diagnostics=%#v", parsed.Metadata, diags)
	}
}

func TestProfileStreamRejectsDecompressedLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversize.cpuprofile.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	data := fmt.Sprintf(`{"nodes":[{"id":1,"callFrame":{}}],"samples":[1],"padding":"%s"}`, strings.Repeat("x", 2048))
	if _, err := gz.Write([]byte(data)); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ParseFile(path, "v8-cpuprofile", Options{MaxBytes: 256}); err == nil || !strings.Contains(err.Error(), "decompressed profile exceeds") {
		t.Fatalf("expected decompressed size guard, got %v", err)
	}
}

func TestParseV8AttributesFinalSampleThroughEndTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tail.cpuprofile")
	data := `{"startTime":1000,"endTime":1800,"nodes":[{"id":1,"callFrame":{"functionName":"work"}}],"samples":[1,1],"timeDeltas":[100,200]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, diags, err := ParseFile(path, "v8-cpuprofile", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := []int64{parsed.Samples[0].Value, parsed.Samples[1].Value}; got[0] != 200 || got[1] != 500 {
		t.Fatalf("sample attribution = %#v, want [200 500]", got)
	}
	if parsed.Metadata["end_time_tail_us"] != int64(500) || diags.WarningCount != 0 {
		t.Fatalf("tail metadata/diagnostics = %#v / %#v", parsed.Metadata, diags)
	}
}

func TestParseV8ClampsEndTimeBeforeLastSample(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad-tail.cpuprofile")
	data := `{"startTime":1000,"endTime":1150,"nodes":[{"id":1,"callFrame":{"functionName":"work"}}],"samples":[1,1],"timeDeltas":[100,200]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, diags, err := ParseFile(path, "v8-cpuprofile", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Samples[1].Value != 0 || parsed.Metadata["end_time_tail_clamped"] != true {
		t.Fatalf("tail clamp missing: samples=%#v metadata=%#v", parsed.Samples, parsed.Metadata)
	}
	found := false
	for _, warning := range diags.Warnings {
		if warning.Reason == "PROFILE_END_TIME_BEFORE_LAST_SAMPLE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing endTime diagnostic: %#v", diags.Warnings)
	}
}
