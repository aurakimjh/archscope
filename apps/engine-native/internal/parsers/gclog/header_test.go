package gclog

import (
	"strings"
	"testing"
)

// ───────────────────────────────────────────────────────────────────
// Unified header
// ───────────────────────────────────────────────────────────────────

func TestExtractHeaderUnifiedBasic(t *testing.T) {
	body := strings.Join([]string{
		"[0.001s][info][gc,init] Version: 17.0.7+8 (release)",
		"[0.001s][info][gc,init] CPUs: 8 total, 8 available",
		"[0.001s][info][gc,init] Memory: 16384M",
		"[0.001s][info][gc,init] Heap Region Size: 4M",
		"[0.001s][info][gc,init] Heap Min Capacity: 8M",
		"[0.001s][info][gc,init] Heap Initial Capacity: 256M",
		"[0.001s][info][gc,init] Heap Max Capacity: 4G",
		"[0.001s][info][gc,init] Using G1",
		"[0.002s][info][gc,init] Parallel Workers: 4",
		"[0.002s][info][gc,init] Concurrent Workers: 1",
		"[0.002s][info][gc,init] Concurrent Refinement Workers: 4",
		"[0.002s][info][gc,init] Large Page Support: Disabled",
		"[0.002s][info][gc,init] NUMA Support: Disabled",
		"[0.002s][info][gc,init] Compressed Oops: Enabled (32-bit)",
		"[0.002s][info][gc,init] Pre-touch: Disabled",
		"[0.002s][info][gc,init] Periodic GC: Disabled",
		"[0.003s][info][gc] GC(0) Pause Young (Normal) (G1 Evacuation Pause) 100M->50M(200M) 25.000ms",
		"",
	}, "\n")
	path := writeTempFile(t, "h.log", body)
	info := ExtractHeader(path)

	if info.VMVersion != "17.0.7+8 (release)" {
		t.Errorf("VMVersion = %q", info.VMVersion)
	}
	if info.CPUsTotal == nil || *info.CPUsTotal != 8 {
		t.Errorf("CPUsTotal = %v, want 8", info.CPUsTotal)
	}
	if info.CPUsAvailable == nil || *info.CPUsAvailable != 8 {
		t.Errorf("CPUsAvailable = %v, want 8", info.CPUsAvailable)
	}
	if info.MemoryMB == nil || *info.MemoryMB != 16384 {
		t.Errorf("MemoryMB = %v, want 16384", info.MemoryMB)
	}
	if info.HeapRegionSizeMB == nil || *info.HeapRegionSizeMB != 4 {
		t.Errorf("HeapRegionSizeMB = %v, want 4", info.HeapRegionSizeMB)
	}
	if info.HeapMaxMB == nil || *info.HeapMaxMB != 4096 {
		t.Errorf("HeapMaxMB = %v, want 4096", info.HeapMaxMB)
	}
	if info.Collector != "G1" {
		t.Errorf("Collector = %q, want G1", info.Collector)
	}
	if info.ParallelWorkers == nil || *info.ParallelWorkers != 4 {
		t.Errorf("ParallelWorkers = %v, want 4", info.ParallelWorkers)
	}
	if info.ConcurrentRefinementWorkers == nil || *info.ConcurrentRefinementWorkers != 4 {
		t.Errorf("ConcurrentRefinementWorkers = %v", info.ConcurrentRefinementWorkers)
	}
	if info.CompressedOops != "Enabled (32-bit)" {
		t.Errorf("CompressedOops = %q", info.CompressedOops)
	}
	if len(info.RawLines) == 0 {
		t.Errorf("RawLines should not be empty")
	}
}

func TestExtractHeaderStopsAtFirstGCEvent(t *testing.T) {
	body := strings.Join([]string{
		"[0.001s][info][gc,init] Using G1",
		"[0.002s][info][gc] GC(0) Pause Young (G1 Evacuation Pause) 100M->50M(200M) 25.000ms",
		"[0.003s][info][gc,init] CPUs: 8 total",
		"",
	}, "\n")
	path := writeTempFile(t, "h.log", body)
	info := ExtractHeader(path)
	// The CPUs line after the first event should not be ingested.
	if info.CPUsTotal != nil {
		t.Errorf("CPUsTotal should be nil (post-event line ignored), got %v", *info.CPUsTotal)
	}
	if info.Collector != "G1" {
		t.Errorf("Collector = %q, want G1", info.Collector)
	}
}

func TestExtractHeaderUnifiedCommandLineFlagsInfersCollector(t *testing.T) {
	body := strings.Join([]string{
		"[0.001s][info][gc,init] CommandLine flags: -Xmx4g -XX:+UseShenandoahGC",
		"",
	}, "\n")
	path := writeTempFile(t, "h.log", body)
	info := ExtractHeader(path)
	if info.Collector != "Shenandoah" {
		t.Errorf("Collector = %q, want Shenandoah", info.Collector)
	}
	if info.CommandLine != "-Xmx4g -XX:+UseShenandoahGC" {
		t.Errorf("CommandLine = %q", info.CommandLine)
	}
}

// ───────────────────────────────────────────────────────────────────
// JDK 8 / legacy header
// ───────────────────────────────────────────────────────────────────

func TestExtractHeaderJDK8Banner(t *testing.T) {
	body := strings.Join([]string{
		"Java HotSpot(TM) 64-Bit Server VM (25.181-b13) for solaris-sparc JRE (1.8.0_181-b13), built on Jul 16 2018 23:46:54 by \"java_re\" with Sun Studio 12u1",
		"Memory: 8k page, physical 132644864k(99543560k free)",
		"CommandLine flags: -XX:+UseG1GC -Xmx4g",
		"2026-04-27T10:00:00.000+0900: 1.234: [GC pause (young), 0.001 secs]",
		"",
	}, "\n")
	path := writeTempFile(t, "h.log", body)
	info := ExtractHeader(path)
	if info.VMBuild != "25.181-b13" {
		t.Errorf("VMBuild = %q, want 25.181-b13", info.VMBuild)
	}
	if info.Platform != "solaris-sparc" {
		t.Errorf("Platform = %q, want solaris-sparc", info.Platform)
	}
	if info.VMVersion != "1.8.0_181-b13" {
		t.Errorf("VMVersion = %q, want 1.8.0_181-b13", info.VMVersion)
	}
	if info.Collector != "G1" {
		t.Errorf("Collector = %q, want G1", info.Collector)
	}
	if info.PageSizeKB == nil || *info.PageSizeKB != 8 {
		t.Errorf("PageSizeKB = %v, want 8", info.PageSizeKB)
	}
	if info.MemoryMB == nil {
		t.Errorf("MemoryMB nil")
	} else if *info.MemoryMB != float64(132644864/1024) {
		t.Errorf("MemoryMB = %v, want %v", *info.MemoryMB, 132644864/1024)
	}
	if info.CommandLine != "-XX:+UseG1GC -Xmx4g" {
		t.Errorf("CommandLine = %q", info.CommandLine)
	}
}

func TestExtractHeaderMissingFile(t *testing.T) {
	info := ExtractHeader("/nonexistent/path.log")
	if len(info.RawLines) != 0 {
		t.Errorf("RawLines should be empty on missing file")
	}
	if info.Collector != "" {
		t.Errorf("Collector = %q, should be empty", info.Collector)
	}
}

func TestExtractHeaderUsingFirstObservationWins(t *testing.T) {
	// Two "Using …" lines — first one must win.
	body := strings.Join([]string{
		"[0.001s][info][gc,init] Using G1",
		"[0.002s][info][gc,init] Using ZGC",
		"",
	}, "\n")
	path := writeTempFile(t, "h.log", body)
	info := ExtractHeader(path)
	if info.Collector != "G1" {
		t.Errorf("Collector = %q, want G1 (first observation wins)", info.Collector)
	}
}

// ───────────────────────────────────────────────────────────────────
// HeaderInfo.ToMap
// ───────────────────────────────────────────────────────────────────

func TestHeaderInfoToMapOmitsZeroes(t *testing.T) {
	four := 4
	h := HeaderInfo{
		Collector:       "G1",
		ParallelWorkers: &four,
		RawLines:        []string{"line"},
	}
	m := h.ToMap()
	if m["collector"] != "G1" {
		t.Errorf("collector = %v", m["collector"])
	}
	if m["parallel_workers"] != 4 {
		t.Errorf("parallel_workers = %v, want 4", m["parallel_workers"])
	}
	if _, ok := m["cpus_total"]; ok {
		t.Errorf("cpus_total should be omitted")
	}
	if _, ok := m["vm_banner"]; ok {
		t.Errorf("vm_banner should be omitted")
	}
}
