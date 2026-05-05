package profiler

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/pprof/profile"
)

func TestExportToPprofRoundTrip(t *testing.T) {
	stacks := map[string]int{
		"main;worker;compute": 7,
		"main;worker;render":  3,
		"main;idle":           5,
	}
	out := filepath.Join(t.TempDir(), "profile.pb.gz")
	if err := ExportToPprof(stacks, out, "samples", "count", 100); err != nil {
		t.Fatalf("export: %v", err)
	}

	bytes, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	prof, err := profile.Parse(newGzipReader(t, bytes))
	if err != nil {
		t.Fatalf("parse pprof: %v", err)
	}
	// Two sample types now: samples + duration (because intervalMs > 0).
	if got := len(prof.SampleType); got != 2 {
		t.Fatalf("sample_types = %d, want 2", got)
	}
	if got := prof.SampleType[0].Type; got != "samples" {
		t.Fatalf("primary sample_type = %q", got)
	}

	totalSamples := int64(0)
	for _, sample := range prof.Sample {
		totalSamples += sample.Value[0]
	}
	if totalSamples != 15 {
		t.Fatalf("total samples = %d, want 15", totalSamples)
	}

	// Locations are emitted leaf-first: ensure the first sample's first
	// location maps to a leaf frame (compute / render / idle).
	if len(prof.Sample) == 0 {
		t.Fatalf("no samples")
	}
	first := prof.Sample[0]
	leaf := first.Location[0].Line[0].Function.Name
	if leaf != "compute" && leaf != "render" && leaf != "idle" {
		t.Fatalf("expected leaf-first ordering; got first frame %q", leaf)
	}
}

func TestExportToPprofIgnoresEmptyStacks(t *testing.T) {
	stacks := map[string]int{
		"":      4,
		";;":    2,
		"good":  3,
	}
	out := filepath.Join(t.TempDir(), "profile.pb.gz")
	if err := ExportToPprof(stacks, out, "", "", 0); err != nil {
		t.Fatalf("export: %v", err)
	}
}

func newGzipReader(t *testing.T, payload []byte) io.Reader {
	t.Helper()
	if len(payload) >= 2 && payload[0] == 0x1f && payload[1] == 0x8b {
		gz, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("gzip: %v", err)
		}
		return gz
	}
	return bytes.NewReader(payload)
}
