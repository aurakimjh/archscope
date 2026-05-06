// Header extraction ported from
// archscope_engine.parsers.gc_log_header. Real-world GC logs open
// with several lines describing the JVM build, host hardware, and
// effective JVM flags — this module is a pure scanner that surfaces
// the fields without judging or interpreting them. The frontend is
// responsible for highlighting suspicious combinations (e.g.
// `-XX:ParallelGCThreads=1` on a multi-core host).

package gclog

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

// HeaderInfo carries the extracted JVM / system metadata. All
// fields are optional — missing values stay as their zero values
// and are excluded from `ToMap` so JSON serialisation matches the
// Python "omit if absent" shape.
type HeaderInfo struct {
	VMBanner                    string
	VMVersion                   string
	VMBuild                     string
	Platform                    string
	Collector                   string
	CommandLine                 string
	CPUsTotal                   *int
	CPUsAvailable               *int
	MemoryMB                    *float64
	HeapRegionSizeMB            *float64
	HeapMinMB                   *float64
	HeapInitialMB               *float64
	HeapMaxMB                   *float64
	ParallelWorkers             *int
	ConcurrentWorkers           *int
	ConcurrentRefinementWorkers *int
	LargePages                  string
	NUMA                        string
	CompressedOops              string
	PreTouch                    string
	PeriodicGC                  string
	PageSizeKB                  *int
	RawLines                    []string
}

// ToMap projects HeaderInfo onto a key/value map matching the
// Python `extract_jvm_header` dict shape — useful when feeding into
// `AnalysisResult.Metadata.Extra`.
func (h HeaderInfo) ToMap() map[string]any {
	out := make(map[string]any)
	if h.VMBanner != "" {
		out["vm_banner"] = h.VMBanner
	}
	if h.VMVersion != "" {
		out["vm_version"] = h.VMVersion
	}
	if h.VMBuild != "" {
		out["vm_build"] = h.VMBuild
	}
	if h.Platform != "" {
		out["platform"] = h.Platform
	}
	if h.Collector != "" {
		out["collector"] = h.Collector
	}
	if h.CommandLine != "" {
		out["command_line"] = h.CommandLine
	}
	if h.CPUsTotal != nil {
		out["cpus_total"] = *h.CPUsTotal
	}
	if h.CPUsAvailable != nil {
		out["cpus_available"] = *h.CPUsAvailable
	}
	if h.MemoryMB != nil {
		out["memory_mb"] = *h.MemoryMB
	}
	if h.HeapRegionSizeMB != nil {
		out["heap_region_size_mb"] = *h.HeapRegionSizeMB
	}
	if h.HeapMinMB != nil {
		out["heap_min_mb"] = *h.HeapMinMB
	}
	if h.HeapInitialMB != nil {
		out["heap_initial_mb"] = *h.HeapInitialMB
	}
	if h.HeapMaxMB != nil {
		out["heap_max_mb"] = *h.HeapMaxMB
	}
	if h.ParallelWorkers != nil {
		out["parallel_workers"] = *h.ParallelWorkers
	}
	if h.ConcurrentWorkers != nil {
		out["concurrent_workers"] = *h.ConcurrentWorkers
	}
	if h.ConcurrentRefinementWorkers != nil {
		out["concurrent_refinement_workers"] = *h.ConcurrentRefinementWorkers
	}
	if h.LargePages != "" {
		out["large_pages"] = h.LargePages
	}
	if h.NUMA != "" {
		out["numa"] = h.NUMA
	}
	if h.CompressedOops != "" {
		out["compressed_oops"] = h.CompressedOops
	}
	if h.PreTouch != "" {
		out["pre_touch"] = h.PreTouch
	}
	if h.PeriodicGC != "" {
		out["periodic_gc"] = h.PeriodicGC
	}
	if h.PageSizeKB != nil {
		out["page_size_kb"] = *h.PageSizeKB
	}
	if len(h.RawLines) > 0 {
		out["raw_lines"] = h.RawLines
	}
	return out
}

// maxHeaderLines mirrors Python's _MAX_HEADER_LINES — generous
// upper bound to tolerate `-XX:+PrintFlagsFinal` style logs.
const maxHeaderLines = 400

// ─── Unified format ─────────────────────────────────────────────────

// unifiedInitLine matches a `[…][info][gc,init]` or `[gc,metaspace]`
// payload line.
var unifiedInitLine = regexp.MustCompile(
	`\]\s*\[\s*info\s*\]\s*\[\s*gc,(?:init|metaspace)\s*\]\s*(?P<payload>.+?)\s*$`,
)

// unifiedEventLine signals the first GC event — used as a stop
// marker because everything below it is per-event noise.
var unifiedEventLine = regexp.MustCompile(
	`\]\s*\[\s*info\s*\]\s*\[\s*gc(?:[,\s][^\]]*)?\]\s+GC\(\d+\)`,
)

// ─── JDK 8 / legacy format ──────────────────────────────────────────

var jdk8VMLine = regexp.MustCompile(`^Java\s+HotSpot|^OpenJDK`)
var jdk8MemoryLine = regexp.MustCompile(`^Memory:\s+(?P<rest>.+)$`)
var jdk8FlagsLine = regexp.MustCompile(`^CommandLine flags:\s*(?P<flags>.+)$`)

// jdk8GCBlockStart marks the first GC event in a JDK 8 log — either
// a `{Heap before GC` block or a leading datestamp.
var jdk8GCBlockStart = regexp.MustCompile(`^\s*\{?Heap before GC|^\s*\d{4}-\d{2}-\d{2}T`)

// ExtractHeader scans up to the first ~400 lines of `path` and
// returns whatever JVM/system metadata is recognised. Empty
// HeaderInfo is returned (with `RawLines` empty) when the file is
// missing or unreadable — same fail-soft behaviour as Python.
func ExtractHeader(path string) HeaderInfo {
	info := HeaderInfo{RawLines: []string{}}

	lines, err := textio.IterTextLines(path, "")
	if err != nil {
		return info
	}

	for i, raw := range lines {
		if i >= maxHeaderLines {
			break
		}
		stripped := strings.TrimSpace(raw)
		if stripped == "" {
			continue
		}

		// Stop conditions.
		if unifiedEventLine.MatchString(stripped) {
			break
		}
		if jdk8GCBlockStart.MatchString(stripped) {
			break
		}

		// Unified [gc,init] / [gc,metaspace] payloads.
		if m := unifiedInitLine.FindStringSubmatch(stripped); m != nil {
			payload := strings.TrimSpace(namedGroup(unifiedInitLine, m, "payload"))
			ingestUnifiedPayload(&info, payload)
			info.RawLines = append(info.RawLines, stripped)
			continue
		}

		// JDK 8 free-form header lines.
		if jdk8VMLine.MatchString(stripped) {
			if info.VMBanner == "" {
				info.VMBanner = stripped
			}
			extractJDK8Version(&info, stripped)
			info.RawLines = append(info.RawLines, stripped)
			continue
		}

		if m := jdk8MemoryLine.FindStringSubmatch(stripped); m != nil {
			rest := namedGroup(jdk8MemoryLine, m, "rest")
			ingestJDK8Memory(&info, rest)
			info.RawLines = append(info.RawLines, stripped)
			continue
		}

		if m := jdk8FlagsLine.FindStringSubmatch(stripped); m != nil {
			flags := strings.TrimSpace(namedGroup(jdk8FlagsLine, m, "flags"))
			info.CommandLine = flags
			inferCollectorFromFlags(&info, flags)
			info.RawLines = append(info.RawLines, stripped)
			continue
		}
	}

	return info
}

// ─── Unified helpers ────────────────────────────────────────────────

var (
	versionRe          = regexp.MustCompile(`^Version:\s+(?P<version>.+)$`)
	cpusRe             = regexp.MustCompile(`^CPUs:\s+(?P<total>\d+)\s+total(?:,\s+(?P<avail>\d+)\s+available)?`)
	memoryUnifiedRe    = regexp.MustCompile(`^Memory:\s+(?P<size>[\d.]+)(?P<unit>[KMG])\b`)
	commandLineFlagsRe = regexp.MustCompile(`^CommandLine flags:\s+(?P<flags>.+)$`)
)

// heapCapacityLabels covers "Heap …: <size><unit>" payload forms in
// priority order so `ingestUnifiedPayload` can short-circuit at the
// first hit.
var heapCapacityLabels = []struct {
	field func(*HeaderInfo, *float64)
	label string
	re    *regexp.Regexp
}{
	{func(h *HeaderInfo, v *float64) { h.HeapRegionSizeMB = v }, "Heap Region Size",
		regexp.MustCompile(`Heap Region Size:\s+(?P<size>[\d.]+)(?P<unit>[KMG])\b`)},
	{func(h *HeaderInfo, v *float64) { h.HeapMinMB = v }, "Heap Min Capacity",
		regexp.MustCompile(`Heap Min Capacity:\s+(?P<size>[\d.]+)(?P<unit>[KMG])\b`)},
	{func(h *HeaderInfo, v *float64) { h.HeapInitialMB = v }, "Heap Initial Capacity",
		regexp.MustCompile(`Heap Initial Capacity:\s+(?P<size>[\d.]+)(?P<unit>[KMG])\b`)},
	{func(h *HeaderInfo, v *float64) { h.HeapMaxMB = v }, "Heap Max Capacity",
		regexp.MustCompile(`Heap Max Capacity:\s+(?P<size>[\d.]+)(?P<unit>[KMG])\b`)},
}

var workerLabels = []struct {
	field func(*HeaderInfo, int)
	re    *regexp.Regexp
}{
	{func(h *HeaderInfo, n int) { h.ParallelWorkers = &n },
		regexp.MustCompile(`Parallel Workers:\s+(?P<n>\d+)`)},
	{func(h *HeaderInfo, n int) { h.ConcurrentWorkers = &n },
		regexp.MustCompile(`Concurrent Workers:\s+(?P<n>\d+)`)},
	{func(h *HeaderInfo, n int) { h.ConcurrentRefinementWorkers = &n },
		regexp.MustCompile(`Concurrent Refinement Workers:\s+(?P<n>\d+)`)},
}

var booleanLabels = []struct {
	field func(*HeaderInfo, string)
	re    *regexp.Regexp
}{
	{func(h *HeaderInfo, v string) { h.LargePages = v },
		regexp.MustCompile(`Large Page Support:\s+(?P<value>.+)$`)},
	{func(h *HeaderInfo, v string) { h.NUMA = v },
		regexp.MustCompile(`NUMA Support:\s+(?P<value>.+)$`)},
	{func(h *HeaderInfo, v string) { h.CompressedOops = v },
		regexp.MustCompile(`Compressed Oops:\s+(?P<value>.+)$`)},
	{func(h *HeaderInfo, v string) { h.PreTouch = v },
		regexp.MustCompile(`Pre-touch:\s+(?P<value>.+)$`)},
	{func(h *HeaderInfo, v string) { h.PeriodicGC = v },
		regexp.MustCompile(`Periodic GC:\s+(?P<value>.+)$`)},
}

func ingestUnifiedPayload(info *HeaderInfo, payload string) {
	// "Using G1" → collector. Python uses `setdefault`, so the first
	// observation wins.
	if strings.HasPrefix(payload, "Using ") {
		if info.Collector == "" {
			info.Collector = strings.TrimSpace(payload[len("Using "):])
		}
		return
	}

	if m := versionRe.FindStringSubmatch(payload); m != nil {
		info.VMVersion = strings.TrimSpace(namedGroup(versionRe, m, "version"))
		return
	}

	if m := cpusRe.FindStringSubmatch(payload); m != nil {
		total, _ := strconv.Atoi(namedGroup(cpusRe, m, "total"))
		info.CPUsTotal = &total
		if avail := namedGroup(cpusRe, m, "avail"); avail != "" {
			a, _ := strconv.Atoi(avail)
			info.CPUsAvailable = &a
		}
		return
	}

	if m := memoryUnifiedRe.FindStringSubmatch(payload); m != nil {
		v := unifiedSizeToMB(namedGroup(memoryUnifiedRe, m, "size"), namedGroup(memoryUnifiedRe, m, "unit"))
		info.MemoryMB = &v
		return
	}

	for _, entry := range heapCapacityLabels {
		if m := entry.re.FindStringSubmatch(payload); m != nil {
			v := unifiedSizeToMB(namedGroup(entry.re, m, "size"), namedGroup(entry.re, m, "unit"))
			entry.field(info, &v)
			return
		}
	}

	for _, entry := range workerLabels {
		if m := entry.re.FindStringSubmatch(payload); m != nil {
			n, _ := strconv.Atoi(namedGroup(entry.re, m, "n"))
			entry.field(info, n)
			return
		}
	}

	for _, entry := range booleanLabels {
		if m := entry.re.FindStringSubmatch(payload); m != nil {
			entry.field(info, strings.TrimSpace(namedGroup(entry.re, m, "value")))
			return
		}
	}

	if m := commandLineFlagsRe.FindStringSubmatch(payload); m != nil {
		flags := strings.TrimSpace(namedGroup(commandLineFlagsRe, m, "flags"))
		info.CommandLine = flags
		inferCollectorFromFlags(info, flags)
		return
	}
}

// ─── JDK 8 helpers ──────────────────────────────────────────────────

var (
	jdk8BannerBuildRe = regexp.MustCompile(`\(([\d.]+(?:[-_]b?\d+)?)\)\s+for\s+(?P<platform>[\w\-]+)`)
	jdk8BannerJRERe   = regexp.MustCompile(`JRE\s+\(([^)]+)\)`)
	jdk8MemoryPhysRe  = regexp.MustCompile(`physical\s+(\d+)k`)
	jdk8MemoryPageRe  = regexp.MustCompile(`(\d+)k\s*page`)
)

func extractJDK8Version(info *HeaderInfo, banner string) {
	if m := jdk8BannerBuildRe.FindStringSubmatch(banner); m != nil {
		info.VMBuild = m[1]
		info.Platform = namedGroup(jdk8BannerBuildRe, m, "platform")
	}
	if m := jdk8BannerJRERe.FindStringSubmatch(banner); m != nil {
		info.VMVersion = m[1]
	}
}

func ingestJDK8Memory(info *HeaderInfo, rest string) {
	if m := jdk8MemoryPhysRe.FindStringSubmatch(rest); m != nil {
		// physical is in KB; convert to MB (integer truncation matches Python `//`).
		kb, _ := strconv.Atoi(m[1])
		v := float64(kb / 1024)
		info.MemoryMB = &v
	}
	if m := jdk8MemoryPageRe.FindStringSubmatch(rest); m != nil {
		n, _ := strconv.Atoi(m[1])
		info.PageSizeKB = &n
	}
}

func inferCollectorFromFlags(info *HeaderInfo, flags string) {
	if info.Collector != "" {
		return
	}
	switch {
	case strings.Contains(flags, "+UseG1GC"):
		info.Collector = "G1"
	case strings.Contains(flags, "+UseShenandoahGC"):
		info.Collector = "Shenandoah"
	case strings.Contains(flags, "+UseZGC"):
		info.Collector = "ZGC"
	case strings.Contains(flags, "+UseConcMarkSweepGC"):
		info.Collector = "CMS"
	case strings.Contains(flags, "+UseParallelGC"), strings.Contains(flags, "+UseParallelOldGC"):
		info.Collector = "Parallel"
	case strings.Contains(flags, "+UseSerialGC"):
		info.Collector = "Serial"
	}
}

// ─── Misc ───────────────────────────────────────────────────────────

// unifiedSizeToMB ports the header module's local `_to_mb` — *not*
// the parser's. The header function uses 2-decimal rounding to match
// the Python module, so we keep it separate from `toMBPtr` rather
// than try to reconcile the two.
func unifiedSizeToMB(size, unit string) float64 {
	v, err := strconv.ParseFloat(size, 64)
	if err != nil {
		return 0
	}
	switch unit {
	case "K":
		return round2(v / 1024.0)
	case "G":
		return round2(v * 1024.0)
	default:
		return round2(v)
	}
}

func round2(v float64) float64 {
	if v < 0 {
		return -float64(int(-v*100+0.5)) / 100
	}
	return float64(int(v*100+0.5)) / 100
}

// namedGroup returns the named capture from a `FindStringSubmatch`
// result without rebuilding the full map. Safe to call with names
// that don't exist in the regex (returns "").
func namedGroup(re *regexp.Regexp, match []string, name string) string {
	idx := re.SubexpIndex(name)
	if idx < 0 || idx >= len(match) {
		return ""
	}
	return match[idx]
}
