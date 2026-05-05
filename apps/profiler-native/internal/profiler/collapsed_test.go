package profiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCollapsedFileAggregatesAndReportsBadLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wall.collapsed")
	err := os.WriteFile(path, []byte(
		"com.example.Job;com.example.Service.process 3\n"+
			"com.example.Job;com.example.Service.process 4\n"+
			"broken without count\n"+
			"com.example.Other -1\n",
	), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	stacks, diagnostics, err := ParseCollapsedFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := stacks["com.example.Job;com.example.Service.process"]; got != 7 {
		t.Fatalf("aggregated samples = %d, want 7", got)
	}
	if diagnostics.ParsedRecords != 2 {
		t.Fatalf("parsed records = %d, want 2", diagnostics.ParsedRecords)
	}
	if diagnostics.SkippedByReason["INVALID_SAMPLE_COUNT"] != 1 {
		t.Fatalf("invalid sample count diagnostics missing: %#v", diagnostics.SkippedByReason)
	}
	if diagnostics.SkippedByReason["NEGATIVE_SAMPLE_COUNT"] != 1 {
		t.Fatalf("negative sample count diagnostics missing: %#v", diagnostics.SkippedByReason)
	}
}
