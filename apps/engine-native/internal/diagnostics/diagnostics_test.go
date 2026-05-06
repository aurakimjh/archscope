package diagnostics

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewIsJSONFriendly(t *testing.T) {
	d := New("access_log")
	body, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`"format":"access_log"`,
		`"skipped_by_reason":{}`,
		`"samples":[]`,
		`"warnings":[]`,
		`"errors":[]`,
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("expected %q in marshalled output; got %s", want, body)
		}
	}
}

func TestAddSkippedBumpsCountersAndKeepsSampleBounded(t *testing.T) {
	d := New("collapsed")
	for i := 0; i < MaxDiagnosticSamples+10; i++ {
		d.AddSkipped(i, "BAD_LINE", "bad", "raw")
	}
	if d.SkippedLines != MaxDiagnosticSamples+10 {
		t.Fatalf("SkippedLines = %d, want %d", d.SkippedLines, MaxDiagnosticSamples+10)
	}
	if d.SkippedByReason["BAD_LINE"] != MaxDiagnosticSamples+10 {
		t.Fatalf("per-reason count drift: got %d", d.SkippedByReason["BAD_LINE"])
	}
	if len(d.Samples) != MaxDiagnosticSamples {
		t.Fatalf("Samples len = %d, want bounded to %d", len(d.Samples), MaxDiagnosticSamples)
	}
	if len(d.Errors) != MaxDiagnosticSamples {
		t.Fatalf("Errors len = %d, want bounded to %d", len(d.Errors), MaxDiagnosticSamples)
	}
	if d.ErrorCount != MaxDiagnosticSamples+10 {
		t.Fatalf("ErrorCount = %d, want %d", d.ErrorCount, MaxDiagnosticSamples+10)
	}
}

func TestAddWarningWithoutSkipDoesNotBumpSkipCounter(t *testing.T) {
	d := New("svg")
	d.AddWarning(0, "ENCODING_FALLBACK", "fell back to cp949", "raw", false)
	if d.SkippedLines != 0 {
		t.Fatalf("SkippedLines should stay 0; got %d", d.SkippedLines)
	}
	if d.WarningCount != 1 {
		t.Fatalf("WarningCount = %d, want 1", d.WarningCount)
	}
	if d.Warnings[0].Reason != "ENCODING_FALLBACK" {
		t.Fatalf("Reason mismatch: %+v", d.Warnings[0])
	}
}

func TestAddWarningWithSkipBumpsBothCounters(t *testing.T) {
	d := New("jennifer_csv")
	d.AddWarning(5, "DUPLICATE_KEY", "key reuse", "raw", true)
	if d.SkippedLines != 1 || d.SkippedByReason["DUPLICATE_KEY"] != 1 {
		t.Fatalf("skip bookkeeping wrong: %+v / %+v", d.SkippedLines, d.SkippedByReason)
	}
	if d.WarningCount != 1 {
		t.Fatalf("WarningCount = %d, want 1", d.WarningCount)
	}
}

func TestRawPreviewIsTruncated(t *testing.T) {
	d := New("access_log")
	long := strings.Repeat("x", RawPreviewLimit*2)
	d.AddSkipped(1, "OVERSIZE", "too long", long)
	if got := len(d.Samples[0].RawPreview); got != RawPreviewLimit {
		t.Fatalf("RawPreview len = %d, want %d", got, RawPreviewLimit)
	}
}

func TestSetSourceFile(t *testing.T) {
	d := New("gc_log")
	d.SetSourceFile("/tmp/gc.log")
	if d.SourceFile == nil || *d.SourceFile != "/tmp/gc.log" {
		t.Fatalf("SourceFile mismatch: %+v", d.SourceFile)
	}
}
