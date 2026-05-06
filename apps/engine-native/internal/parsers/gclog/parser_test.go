package gclog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ───────────────────────────────────────────────────────────────────
// Fixtures
// ───────────────────────────────────────────────────────────────────

const sampleHotspot = `[0.001s][info][gc] GC(0) Pause Young (Normal) (G1 Evacuation Pause) 100M->50M(200M) 25.000ms
[5.001s][info][gc] GC(1) Pause Full (System.gc()) 150M->100M(200M) 190.120ms
`

func writeTempFile(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// repoFixture returns the absolute path to a checked-in GC log
// fixture. The walk-up looks for the repo root by going six levels up
// from this file (apps/engine-native/internal/parsers/gclog → repo).
func repoFixture(t *testing.T, rel string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Walk up looking for the examples/ directory so this test runs
	// regardless of where `go test` is invoked from.
	dir := wd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skipf("fixture %s not found from %s", rel, wd)
	return ""
}

// ───────────────────────────────────────────────────────────────────
// DetectFormat
// ───────────────────────────────────────────────────────────────────

func TestDetectFormatUnified(t *testing.T) {
	path := writeTempFile(t, "u.log", sampleHotspot)
	if got := DetectFormat(path); got != FormatUnified {
		t.Fatalf("DetectFormat = %q, want %q", got, FormatUnified)
	}
}

func TestDetectFormatG1Legacy(t *testing.T) {
	body := "2026-04-27T10:00:00.000+0900: 1.234: [GC pause (G1 Evacuation Pause) (young), 0.0123 secs]\n"
	path := writeTempFile(t, "g.log", body)
	if got := DetectFormat(path); got != FormatG1Legacy {
		t.Fatalf("DetectFormat = %q, want %q", got, FormatG1Legacy)
	}
}

func TestDetectFormatLegacy(t *testing.T) {
	body := "2026-04-27T10:00:00.000+0900: 1.234: [GC [PSYoungGen: 1024K->0K(2048K)] 1024K->0K(4096K), 0.001 secs]\n"
	path := writeTempFile(t, "l.log", body)
	if got := DetectFormat(path); got != FormatLegacy {
		t.Fatalf("DetectFormat = %q, want %q", got, FormatLegacy)
	}
}

func TestDetectFormatUnknown(t *testing.T) {
	body := "completely unrelated text\nand another line\n"
	path := writeTempFile(t, "x.log", body)
	if got := DetectFormat(path); got != FormatUnknown {
		t.Fatalf("DetectFormat = %q, want %q", got, FormatUnknown)
	}
}

// ───────────────────────────────────────────────────────────────────
// Unified parser — ports test_gc_log_parser_hotspot_unified_sample
// ───────────────────────────────────────────────────────────────────

func TestParseFileHotspotUnifiedFixture(t *testing.T) {
	path := repoFixture(t, "examples/gc-logs/sample-hotspot-gc.log")

	events, diags, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].GCType != "Pause Young" {
		t.Errorf("events[0].GCType = %q, want Pause Young", events[0].GCType)
	}
	if events[0].Cause == nil || *events[0].Cause != "G1 Evacuation Pause" {
		t.Errorf("events[0].Cause = %v, want G1 Evacuation Pause", events[0].Cause)
	}
	if events[1].GCType != "Pause Full" {
		t.Errorf("events[1].GCType = %q, want Pause Full", events[1].GCType)
	}
	if events[1].PauseMS == nil || *events[1].PauseMS != 190.120 {
		t.Errorf("events[1].PauseMS = %v, want 190.120", events[1].PauseMS)
	}
	if diags.ParsedRecords != 2 {
		t.Errorf("ParsedRecords = %d, want 2", diags.ParsedRecords)
	}
	if diags.Format != FormatUnified {
		t.Errorf("Format = %q, want unified", diags.Format)
	}
}

func TestParseUnifiedHeapValuesAndCommitted(t *testing.T) {
	path := writeTempFile(t, "u.log", sampleHotspot)
	events, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0].HeapBeforeMB == nil || *events[0].HeapBeforeMB != 100.0 {
		t.Errorf("HeapBeforeMB = %v, want 100", events[0].HeapBeforeMB)
	}
	if events[0].HeapAfterMB == nil || *events[0].HeapAfterMB != 50.0 {
		t.Errorf("HeapAfterMB = %v, want 50", events[0].HeapAfterMB)
	}
	if events[0].HeapCommittedMB == nil || *events[0].HeapCommittedMB != 200.0 {
		t.Errorf("HeapCommittedMB = %v, want 200", events[0].HeapCommittedMB)
	}
}

func TestParseUnifiedMetaspaceCompanionMerges(t *testing.T) {
	body := strings.Join([]string{
		"[0.001s][info][gc] GC(0) Pause Young (Normal) (G1 Evacuation Pause) 100M->50M(200M) 25.000ms",
		"[0.002s][info][gc,metaspace] GC(0) Metaspace: 12345K->12300K(15345K)",
		"[5.001s][info][gc] GC(1) Pause Full (System.gc()) 150M->100M(200M) 190.120ms",
		"",
	}, "\n")
	path := writeTempFile(t, "m.log", body)
	events, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].MetaspaceBeforeMB == nil {
		t.Fatalf("MetaspaceBeforeMB nil")
	}
	if got := *events[0].MetaspaceBeforeMB; got <= 12 || got >= 13 {
		t.Errorf("MetaspaceBeforeMB = %v, want ~12.06", got)
	}
	// Second event has no metaspace companion.
	if events[1].MetaspaceBeforeMB != nil {
		t.Errorf("events[1].MetaspaceBeforeMB should be nil, got %v", *events[1].MetaspaceBeforeMB)
	}
}

func TestParseUnifiedMetaspaceCompanionMismatchedGCID(t *testing.T) {
	body := strings.Join([]string{
		"[0.001s][info][gc] GC(0) Pause Young (G1 Evacuation Pause) 100M->50M(200M) 25.000ms",
		"[0.002s][info][gc,metaspace] GC(99) Metaspace: 999K->888K(1000K)",
		"",
	}, "\n")
	path := writeTempFile(t, "m.log", body)
	events, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].MetaspaceBeforeMB != nil {
		t.Errorf("Metaspace should not merge across mismatched GC IDs, got %v",
			*events[0].MetaspaceBeforeMB)
	}
}

func TestParseUnifiedNoGCFormatMatchSkipped(t *testing.T) {
	body := strings.Join([]string{
		"[0.001s][info][gc] GC(0) Pause Young (G1 Evacuation Pause) 100M->50M(200M) 25.000ms",
		"this line is garbage",
		"",
	}, "\n")
	path := writeTempFile(t, "m.log", body)
	events, diags, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if diags.SkippedByReason[ReasonNoGCFormatMatch] != 1 {
		t.Errorf("SkippedByReason[NO_GC_FORMAT_MATCH] = %d, want 1",
			diags.SkippedByReason[ReasonNoGCFormatMatch])
	}
	if diags.WarningCount != 1 {
		t.Errorf("WarningCount = %d, want 1", diags.WarningCount)
	}
}

func TestParseUnifiedStrictModeFailsFast(t *testing.T) {
	body := strings.Join([]string{
		"[0.001s][info][gc] GC(0) Pause Young (G1 Evacuation Pause) 100M->50M(200M) 25.000ms",
		"this line is garbage",
		"[5.001s][info][gc] GC(1) Pause Full (System.gc()) 150M->100M(200M) 190.120ms",
		"",
	}, "\n")
	path := writeTempFile(t, "m.log", body)
	events, _, err := ParseFile(path, Options{Strict: true})
	if err == nil {
		t.Fatalf("expected strict-mode error")
	}
	if !strings.Contains(err.Error(), ReasonNoGCFormatMatch) {
		t.Fatalf("error should mention NO_GC_FORMAT_MATCH, got %v", err)
	}
	// First event was already buffered + flushed before the error.
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1 (the one before the bad line)", len(events))
	}
}

// ───────────────────────────────────────────────────────────────────
// G1 Legacy
// ───────────────────────────────────────────────────────────────────

func TestParseG1LegacyMultiLineEvent(t *testing.T) {
	body := strings.Join([]string{
		"2026-04-27T10:00:00.000+0900: 1.234: [GC pause (G1 Evacuation Pause) (young), 0.0123 secs]",
		"   [Eden: 4096.0M(4096.0M)->0.0B(3584.0M) Survivors: 0.0B->512.0M Heap: 5120.0M(8192.0M)->1024.0M(8192.0M)]",
		"   [Metaspace: 12345K->12300K(15345K)]",
		"",
	}, "\n")
	path := writeTempFile(t, "g.log", body)
	events, diags, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	e := events[0]
	if e.GCType != "G1 Young" {
		t.Errorf("GCType = %q, want G1 Young", e.GCType)
	}
	if e.Cause == nil || *e.Cause != "G1 Evacuation Pause" {
		t.Errorf("Cause = %v, want G1 Evacuation Pause", e.Cause)
	}
	if e.PauseMS == nil || *e.PauseMS != 12.3 {
		t.Errorf("PauseMS = %v, want 12.3", e.PauseMS)
	}
	if e.HeapBeforeMB == nil || *e.HeapBeforeMB != 5120.0 {
		t.Errorf("HeapBeforeMB = %v", e.HeapBeforeMB)
	}
	if e.HeapAfterMB == nil || *e.HeapAfterMB != 1024.0 {
		t.Errorf("HeapAfterMB = %v", e.HeapAfterMB)
	}
	if e.MetaspaceBeforeMB == nil {
		t.Errorf("MetaspaceBeforeMB nil")
	}
	if e.UptimeSec == nil || *e.UptimeSec != 1.234 {
		t.Errorf("UptimeSec = %v, want 1.234", e.UptimeSec)
	}
	if diags.ParsedRecords != 1 {
		t.Errorf("ParsedRecords = %d, want 1", diags.ParsedRecords)
	}
}

func TestParseG1LegacyCleanup(t *testing.T) {
	// Prefix with a `GC pause (young)` line so DetectFormat picks
	// g1_legacy (matches Python's detection heuristic).
	body := strings.Join([]string{
		"2026-04-27T10:00:00.000+0900: 1.0: [GC pause (young) 4096K->3936K(16M), 0.005 secs]",
		"2026-04-27T10:00:00.000+0900: 2.345: [GC cleanup 75M->25M(103M), 0.005 secs]",
		"",
	}, "\n")
	path := writeTempFile(t, "g.log", body)
	events, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[1].GCType != "G1 Cleanup" {
		t.Errorf("GCType = %q", events[1].GCType)
	}
	if events[1].HeapBeforeMB == nil || *events[1].HeapBeforeMB != 75.0 {
		t.Errorf("HeapBeforeMB = %v", events[1].HeapBeforeMB)
	}
	if events[1].HeapCommittedMB == nil || *events[1].HeapCommittedMB != 103.0 {
		t.Errorf("HeapCommittedMB = %v", events[1].HeapCommittedMB)
	}
}

func TestParseG1LegacyInlineHeap(t *testing.T) {
	body := "2026-04-27T10:00:00.000+0900: 1.0: [GC pause (young) 4096K->3936K(16M), 0.005 secs]\n"
	path := writeTempFile(t, "g.log", body)
	events, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].GCType != "G1 Young" {
		t.Errorf("GCType = %q, want G1 Young", events[0].GCType)
	}
	if events[0].HeapBeforeMB == nil || *events[0].HeapBeforeMB != 4.0 {
		t.Errorf("HeapBeforeMB = %v, want 4.0", events[0].HeapBeforeMB)
	}
	if events[0].HeapCommittedMB == nil || *events[0].HeapCommittedMB != 16.0 {
		t.Errorf("HeapCommittedMB = %v, want 16.0", events[0].HeapCommittedMB)
	}
}

func TestParseG1LegacyRemark(t *testing.T) {
	body := strings.Join([]string{
		"2026-04-27T10:00:00.000+0900: 1.0: [GC pause (young) 4096K->3936K(16M), 0.005 secs]",
		"2026-04-27T10:00:00.000+0900: 3.0: [GC remark, 0.001 secs]",
		"",
	}, "\n")
	path := writeTempFile(t, "g.log", body)
	events, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[1].GCType != "G1 Remark" {
		t.Errorf("GCType = %q, want G1 Remark", events[1].GCType)
	}
}

func TestParseG1LegacyMixedWithModifier(t *testing.T) {
	body := "2026-04-27T10:00:00.000+0900: 4.0: [GC pause (G1 Evacuation Pause) (mixed) (initial-mark), 0.002 secs]\n"
	path := writeTempFile(t, "g.log", body)
	events, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0].GCType != "G1 Mixed (initial-mark)" {
		t.Errorf("GCType = %q, want \"G1 Mixed (initial-mark)\"", events[0].GCType)
	}
}

// ───────────────────────────────────────────────────────────────────
// Legacy (Serial / Parallel / CMS)
// ───────────────────────────────────────────────────────────────────

func TestParseLegacyParallelYoung(t *testing.T) {
	body := "2026-04-27T10:00:00.000+0900: 1.234: [GC [PSYoungGen: 1024K->0K(2048K)] 1024K->0K(4096K), 0.001 secs]\n"
	path := writeTempFile(t, "l.log", body)
	events, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0].GCType != "Young GC (Parallel)" {
		t.Errorf("GCType = %q, want \"Young GC (Parallel)\"", events[0].GCType)
	}
	if events[0].HeapBeforeMB == nil || *events[0].HeapBeforeMB != 1.0 {
		t.Errorf("HeapBeforeMB = %v, want 1.0 (1024K → 1.0M)", events[0].HeapBeforeMB)
	}
	if events[0].HeapCommittedMB == nil || *events[0].HeapCommittedMB != 4.0 {
		t.Errorf("HeapCommittedMB = %v, want 4.0", events[0].HeapCommittedMB)
	}
	if events[0].PauseMS == nil || *events[0].PauseMS != 1.0 {
		t.Errorf("PauseMS = %v, want 1.0 (0.001 secs)", events[0].PauseMS)
	}
}

func TestParseLegacyFullCMS(t *testing.T) {
	// Prefix with a CMS-marker line so DetectFormat picks `legacy`.
	body := strings.Join([]string{
		"2026-04-27T10:00:00.000+0900: 4.0: [GC [ParNew: 1024K->0K(2048K), 0.001 secs] [CMS-concurrent-mark-start]",
		"2026-04-27T10:00:00.000+0900: 5.0: [Full GC (System) [CMS: 100K->50K(200K)] 100K->50K(400K), 0.010 secs]",
		"",
	}, "\n")
	path := writeTempFile(t, "l.log", body)
	events, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	// Find the Full GC event — first line may or may not parse cleanly.
	var fullGC *Event
	for i := range events {
		if events[i].GCType == "Full GC (CMS)" {
			fullGC = &events[i]
			break
		}
	}
	if fullGC == nil {
		t.Fatalf("Full GC (CMS) event not found in %+v", events)
	}
	if fullGC.Cause == nil || *fullGC.Cause != "System" {
		t.Errorf("Cause = %v, want System", fullGC.Cause)
	}
}

func TestParseLegacyDefNewYoung(t *testing.T) {
	body := "2026-04-27T10:00:00.000+0900: 6.0: [GC [DefNew: 1024K->0K(2048K)] 1024K->0K(4096K), 0.001 secs]\n"
	path := writeTempFile(t, "l.log", body)
	events, _, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0].GCType != "Young GC (Serial)" {
		t.Errorf("GCType = %q, want \"Young GC (Serial)\"", events[0].GCType)
	}
}

// ───────────────────────────────────────────────────────────────────
// Diagnostics edge cases
// ───────────────────────────────────────────────────────────────────

func TestParseFileEmptyFileWarns(t *testing.T) {
	path := writeTempFile(t, "empty.log", "")
	events, diags, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events non-empty for empty file: %d", len(events))
	}
	if diags.WarningCount != 1 {
		t.Errorf("WarningCount = %d, want 1", diags.WarningCount)
	}
	if len(diags.Warnings) == 0 || diags.Warnings[0].Reason != ReasonEmptyFile {
		t.Errorf("expected EMPTY_FILE warning, got %+v", diags.Warnings)
	}
}

func TestParseFileNoSupportedEvents(t *testing.T) {
	body := "blah\nblah\nblah\n"
	path := writeTempFile(t, "junk.log", body)
	_, diags, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	// Three NO_GC_FORMAT_MATCH skips + one NO_SUPPORTED_GC_EVENTS warning.
	if diags.SkippedByReason[ReasonNoGCFormatMatch] != 3 {
		t.Errorf("SkippedByReason[NO_GC_FORMAT_MATCH] = %d, want 3",
			diags.SkippedByReason[ReasonNoGCFormatMatch])
	}
	foundNoSupported := false
	for _, w := range diags.Warnings {
		if w.Reason == ReasonNoSupportedGCEvent {
			foundNoSupported = true
		}
	}
	if !foundNoSupported {
		t.Errorf("expected NO_SUPPORTED_GC_EVENTS warning; got %+v", diags.Warnings)
	}
}

func TestParseFileMaxLines(t *testing.T) {
	body := strings.Join([]string{
		"[0.001s][info][gc] GC(0) Pause Young (Normal) (G1 Evacuation Pause) 100M->50M(200M) 25.000ms",
		"[0.002s][info][gc] GC(1) Pause Young (Normal) (G1 Evacuation Pause) 100M->50M(200M) 25.000ms",
		"[0.003s][info][gc] GC(2) Pause Young (Normal) (G1 Evacuation Pause) 100M->50M(200M) 25.000ms",
		"",
	}, "\n")
	path := writeTempFile(t, "n.log", body)
	events, _, err := ParseFile(path, Options{MaxLines: 2})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
}

func TestParseFileSetsSourceAndFormat(t *testing.T) {
	path := writeTempFile(t, "u.log", sampleHotspot)
	_, diags, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if diags.SourceFile == nil || *diags.SourceFile != path {
		t.Errorf("SourceFile = %v, want %q", diags.SourceFile, path)
	}
	if diags.Format != FormatUnified {
		t.Errorf("Format = %q, want unified", diags.Format)
	}
}

// ───────────────────────────────────────────────────────────────────
// _to_mb / unit handling
// ───────────────────────────────────────────────────────────────────

func TestToMBPtrUnits(t *testing.T) {
	cases := []struct {
		v, u string
		want float64
	}{
		{"1024", "K", 1.0},
		{"1024", "M", 1024.0},
		{"1", "G", 1024.0},
		{"1048576", "B", 1.0},
		{"100", "M", 100.0},
	}
	for _, c := range cases {
		got := toMBPtr(c.v, c.u)
		if got == nil {
			t.Errorf("toMBPtr(%q,%q) = nil, want %v", c.v, c.u, c.want)
			continue
		}
		if *got != c.want {
			t.Errorf("toMBPtr(%q,%q) = %v, want %v", c.v, c.u, *got, c.want)
		}
	}
}

func TestSafeAddNilHandling(t *testing.T) {
	if got := safeAdd(nil, nil); got != nil {
		t.Errorf("safeAdd(nil,nil) = %v, want nil", got)
	}
	a := 1.5
	b := 2.5
	if got := safeAdd(&a, &b); got == nil || *got != 4.0 {
		t.Errorf("safeAdd(1.5,2.5) = %v, want 4.0", got)
	}
	if got := safeAdd(&a, nil); got == nil || *got != 1.5 {
		t.Errorf("safeAdd(1.5,nil) = %v, want 1.5", got)
	}
}

func TestSplitUnifiedLabel(t *testing.T) {
	gc, cause := splitUnifiedLabel("Pause Young (Normal) (G1 Evacuation Pause)")
	if gc != "Pause Young" {
		t.Errorf("gcType = %q", gc)
	}
	if cause == nil || *cause != "G1 Evacuation Pause" {
		t.Errorf("cause = %v", cause)
	}

	gc, cause = splitUnifiedLabel("Pause Full")
	if gc != "Pause Full" {
		t.Errorf("gcType = %q", gc)
	}
	if cause != nil {
		t.Errorf("cause = %v, want nil", cause)
	}
}
