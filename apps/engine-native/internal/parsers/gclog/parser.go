// Package gclog ports archscope_engine.parsers.gc_log_parser plus
// archscope_engine.parsers.gc_log_header.
//
// Three on-disk formats are recognised — JDK 9+ unified
// (`[2026-04-27T10:00:00+0900][info][gc] GC(123) Pause Young ... 25.000ms`),
// JDK 8 G1 legacy (multi-line, datestamp-prefixed) and JDK 4-8
// Serial / Parallel / CMS legacy (single-line). The format is
// auto-detected by sampling the first 8 KiB and surfaced through the
// shared diagnostics builder so renderers can group reports.
package gclog


import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

// Format constants surface the four labels Python's
// `detect_gc_log_format` returns. The string values match Python
// verbatim so diagnostics output stays cross-engine identical.
const (
	FormatUnified  = "unified"
	FormatG1Legacy = "g1_legacy"
	FormatLegacy   = "legacy"
	FormatUnknown  = "unknown"
)

// Skip / warning reason names — Python verbatim.
const (
	ReasonNoGCFormatMatch    = "NO_GC_FORMAT_MATCH"
	ReasonEmptyFile          = "EMPTY_FILE"
	ReasonNoSupportedGCEvent = "NO_SUPPORTED_GC_EVENTS"
)

// Event mirrors `models.gc_event.GcEvent` field-for-field. Optional
// numeric fields are pointers so a missing value (e.g. metaspace not
// reported on a particular line) round-trips as JSON `null` rather
// than `0`.
type Event struct {
	Timestamp         *time.Time
	UptimeSec         *float64
	GCType            string
	Cause             *string
	PauseMS           *float64
	HeapBeforeMB      *float64
	HeapAfterMB       *float64
	HeapCommittedMB   *float64
	YoungBeforeMB     *float64
	YoungAfterMB      *float64
	OldBeforeMB       *float64
	OldAfterMB        *float64
	MetaspaceBeforeMB *float64
	MetaspaceAfterMB  *float64
	RawLine           string
}

// Options carries parser-level filters. Currently only `Strict` and
// `MaxLines` — the GC parser does not expose start/end time filters
// in Python, so we keep symmetry with the reference.
type Options struct {
	MaxLines int
	Strict   bool
}

// ─── JDK 9+ Unified GC Log ──────────────────────────────────────────

// unifiedGCRe matches a unified-format pause line. The `label` group
// stays greedy / non-greedy so causes like `(G1 Evacuation Pause)` are
// preserved for `_split_unified_label`.
var unifiedGCRe = regexp.MustCompile(
	`^\[(?P<timestamp>[^\]]+)\].*?GC\((?P<gc_id>\d+)\)\s+` +
		`(?P<label>.*?)\s+` +
		`(?P<before>\d+(?:\.\d+)?)(?P<before_unit>[KMG])->` +
		`(?P<after>\d+(?:\.\d+)?)(?P<after_unit>[KMG])` +
		`(?:\((?P<committed>\d+(?:\.\d+)?)(?P<committed_unit>[KMG])\))?\s+` +
		`(?P<pause>\d+(?:\.\d+)?)ms`,
)

// unifiedMetaspaceRe matches the companion Metaspace line that
// follows a pause event — same GC(id) ties the two together.
var unifiedMetaspaceRe = regexp.MustCompile(
	`GC\((?P<gc_id>\d+)\)\s+Metaspace:\s+` +
		`(?P<before>\d+(?:\.\d+)?)(?P<before_unit>[KMG])` +
		`(?:\([^)]+\))?->` +
		`(?P<after>\d+(?:\.\d+)?)(?P<after_unit>[KMG])` +
		`(?:\((?P<committed>\d+(?:\.\d+)?)(?P<committed_unit>[KMG])\))?`,
)

// ─── JDK 8 G1GC Legacy ──────────────────────────────────────────────

var g1PauseRe = regexp.MustCompile(
	`^(?:(?P<datestamp>\d{4}-\d{2}-\d{2}T[\d:.]+[+-]\d{4}):\s+)?` +
		`(?P<uptime>[\d.,]+):\s+` +
		`(?:#\d+:\s+)?` +
		`\[(?P<label>GC\s+(?:pause|remark|cleanup).*),\s+` +
		`(?P<pause>[\d.]+)\s+secs\]\s*$`,
)

var g1InlineHeapRe = regexp.MustCompile(
	`\s+([\d.]+)([KMG])\s*->\s*([\d.]+)([KMG])\(([\d.]+)([KMG])\)\s*$`,
)

var g1MemoryRe = regexp.MustCompile(
	`\[Eden:\s*([\d.]+)([BKMG])\([^)]+\)->([\d.]+)([BKMG])\([^)]+\)\s+` +
		`Survivors:\s*([\d.]+)([BKMG])->([\d.]+)([BKMG])\s+` +
		`Heap:\s*([\d.]+)([BKMG])\(([\d.]+)([BKMG])\)->([\d.]+)([BKMG])\(([\d.]+)([BKMG])\)`,
)

var g1MetaspaceRe = regexp.MustCompile(
	`\[Metaspace:\s*([\d.]+)([BKMG])->([\d.]+)([BKMG])\(([\d.]+)([BKMG])\)\]`,
)

var g1CleanupMemRe = regexp.MustCompile(
	`GC\s+cleanup\s+([\d.]+)([BKMG])->([\d.]+)([BKMG])\(([\d.]+)([BKMG])\)`,
)

var g1Phases = map[string]struct{}{
	"young":   {},
	"mixed":   {},
	"partial": {},
}

// ─── JDK 4-8 Serial / Parallel / CMS ────────────────────────────────

var legacyPauseRe = regexp.MustCompile(
	`^(?:(?P<datestamp>\d{4}-\d{2}-\d{2}T[\d:.]+[+-]\d{4}):\s+)?` +
		`(?P<uptime>[\d.,]+):\s+` +
		`\[(?P<label>(?:Full\s+)?GC(?:\s+\([^)]*\))?)`,
)

var legacyHeapRe = regexp.MustCompile(
	`(?P<before>[\d.]+)(?P<bu>[KMG])->(?P<after>[\d.]+)(?P<au>[KMG])` +
		`\((?P<committed>[\d.]+)(?P<cu>[KMG])\),\s+(?P<pause>[\d.]+)\s+secs`,
)

// tzFixRe converts Python-incompatible `+0900` to `+09:00` so
// `time.Parse` handles both. Go's `time.Parse` accepts `-0700` natively
// but the legacy datestamp uses ISO-8601 `2006-01-02T15:04:05.000+0900`
// which Go's RFC3339 layout does not match, so we coerce to colon
// form and run RFC3339 underneath.
var tzFixRe = regexp.MustCompile(`([+-]\d{2})(\d{2})$`)

// ═══════════════════════════════════════════════════════════════════
// Public API
// ═══════════════════════════════════════════════════════════════════

// DetectFormat samples the first 8 KiB of `path` and returns one of
// the four format constants. Mirrors `detect_gc_log_format`.
func DetectFormat(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return FormatUnknown
	}
	defer f.Close()
	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	sample := string(buf[:n])

	if strings.Contains(sample, "][gc") || (strings.Contains(sample, "] GC(") && strings.Contains(sample, "[info]")) {
		return FormatUnified
	}
	if strings.Contains(sample, "garbage-first heap") ||
		strings.Contains(sample, "G1 Evacuation Pause") ||
		strings.Contains(sample, "GC pause (young)") ||
		strings.Contains(sample, "GC pause (mixed)") {
		return FormatG1Legacy
	}
	if strings.Contains(sample, "PSYoungGen") ||
		strings.Contains(sample, "ParNew") ||
		strings.Contains(sample, "DefNew") ||
		strings.Contains(sample, "CMS-") ||
		strings.Contains(sample, "PSPermGen") {
		return FormatLegacy
	}
	return FormatUnknown
}

// ParseFile reads `path` end-to-end, auto-detects the format, and
// returns the parsed events alongside a populated diagnostics
// builder. Mirrors Python `iter_gc_log_events_with_diagnostics`.
//
// On `Strict=true`, the first malformed line is returned as a
// non-nil error after the diagnostics row has been recorded.
// Currently only the unified format emits per-line skips — G1 legacy
// and legacy silently ignore non-event lines (matching Python).
func ParseFile(path string, opts Options) ([]Event, *diagnostics.ParserDiagnostics, error) {
	if opts.MaxLines < 0 {
		return nil, nil, fmt.Errorf("max_lines must be a positive integer")
	}

	format := DetectFormat(path)
	diags := diagnostics.New(format)
	diags.SetSourceFile(path)

	lines, err := textio.IterTextLines(path, "")
	if err != nil {
		return nil, nil, err
	}

	switch format {
	case FormatG1Legacy:
		events := parseG1Legacy(lines, opts, diags)
		emitNoEventsWarning(diags, "No supported G1 legacy GC events were parsed.")
		return events, diags, nil
	case FormatLegacy:
		events := parseLegacy(lines, opts, diags)
		emitNoEventsWarning(diags, "No supported legacy GC events were parsed.")
		return events, diags, nil
	default:
		// FormatUnified and FormatUnknown both go through the unified
		// parser — Python falls through to `_iter_unified` for unknown.
		events, perr := parseUnified(lines, opts, diags)
		emitNoEventsWarning(diags, "No supported GC pause events were parsed.")
		if perr != nil {
			return events, diags, perr
		}
		return events, diags, nil
	}
}

// emitNoEventsWarning surfaces EMPTY_FILE / NO_SUPPORTED_GC_EVENTS at
// the end of a parse run so consumers can branch on "we read the file
// but found nothing useful". Shared across all three format branches.
func emitNoEventsWarning(diags *diagnostics.ParserDiagnostics, noEventsMsg string) {
	if diags.ParsedRecords > 0 {
		return
	}
	if diags.TotalLines == 0 {
		diags.AddWarning(0, ReasonEmptyFile, "GC log file is empty.", "", false)
		return
	}
	diags.AddWarning(0, ReasonNoSupportedGCEvent, noEventsMsg, "", false)
}

// ═══════════════════════════════════════════════════════════════════
// Unified (JDK 9+)
// ═══════════════════════════════════════════════════════════════════

// parseUnified runs the JDK 9+ pipeline. A pause event is buffered
// across iterations so the next-line Metaspace companion can be
// merged before flushing to the output.
func parseUnified(lines []string, opts Options, diags *diagnostics.ParserDiagnostics) ([]Event, error) {
	events := make([]Event, 0, len(lines))

	var pending *Event
	var pendingGCID *int

	flush := func() {
		if pending != nil {
			events = append(events, *pending)
			diags.ParsedRecords++
			pending = nil
			pendingGCID = nil
		}
	}

	for i, line := range lines {
		lineNumber := i + 1
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			break
		}
		diags.TotalLines++
		if isBlank(line) {
			continue
		}

		// Metaspace companion line — attach to buffered event when GC(id) matches.
		if meta := captureGroups(unifiedMetaspaceRe, line); meta != nil {
			if pending != nil && pendingGCID != nil {
				if mid, err := strconv.Atoi(meta["gc_id"]); err == nil && mid == *pendingGCID {
					pending.MetaspaceBeforeMB = toMBPtr(meta["before"], meta["before_unit"])
					pending.MetaspaceAfterMB = toMBPtr(meta["after"], meta["after_unit"])
				}
			}
			continue
		}

		event, gcID := parseUnifiedGCLine(line)
		if event == nil {
			diags.AddWarning(lineNumber, ReasonNoGCFormatMatch,
				"Line did not match the supported HotSpot unified GC format.",
				line, true)
			if opts.Strict {
				flush()
				return events, fmt.Errorf("%d: %s: Line did not match the supported HotSpot unified GC format.",
					lineNumber, ReasonNoGCFormatMatch)
			}
			continue
		}

		// New pause event — flush any prior buffered event first.
		flush()
		pending = event
		pendingGCID = gcID
	}

	flush()
	return events, nil
}

// parseUnifiedGCLine attempts to parse a single non-empty unified
// pause line. Returns (event, gcID) on success, (nil, nil) on no
// match. Mirrors `_parse_unified_gc_line`.
func parseUnifiedGCLine(line string) (*Event, *int) {
	groups := captureGroups(unifiedGCRe, line)
	if groups == nil {
		return nil, nil
	}

	label := strings.TrimSpace(groups["label"])
	gcType, cause := splitUnifiedLabel(label)

	pauseMS, err := strconv.ParseFloat(groups["pause"], 64)
	if err != nil {
		return nil, nil
	}

	ts := parseTimestamp(groups["timestamp"])
	event := &Event{
		Timestamp:    ts,
		GCType:       gcType,
		Cause:        cause,
		PauseMS:      &pauseMS,
		HeapBeforeMB: toMBPtr(groups["before"], groups["before_unit"]),
		HeapAfterMB:  toMBPtr(groups["after"], groups["after_unit"]),
		RawLine:      line,
	}
	if groups["committed"] != "" {
		event.HeapCommittedMB = toMBPtr(groups["committed"], groups["committed_unit"])
	}

	gcID, err := strconv.Atoi(groups["gc_id"])
	if err != nil {
		return event, nil
	}
	return event, &gcID
}

// ═══════════════════════════════════════════════════════════════════
// G1 Legacy (JDK 8 -XX:+UseG1GC)
// ═══════════════════════════════════════════════════════════════════

// g1Pending is the per-event accumulator the state machine builds up
// across pause / heap / metaspace lines before flushing.
type g1Pending struct {
	datestamp         string
	uptime            string
	label             string
	pauseMS           *float64
	heapBeforeMB      *float64
	heapAfterMB       *float64
	heapCommittedMB   *float64
	youngBeforeMB     *float64
	youngAfterMB      *float64
	metaspaceBeforeMB *float64
	metaspaceAfterMB  *float64
	rawLine           string
}

func parseG1Legacy(lines []string, opts Options, diags *diagnostics.ParserDiagnostics) []Event {
	events := make([]Event, 0, len(lines))
	var pending *g1Pending

	flush := func() {
		if pending != nil {
			events = append(events, buildG1Event(*pending))
			diags.ParsedRecords++
			pending = nil
		}
	}

	for i, line := range lines {
		lineNumber := i + 1
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			break
		}
		diags.TotalLines++
		if isBlank(line) {
			continue
		}

		// Eden/Survivors/Heap memory detail line — must be checked BEFORE
		// the pause regex because some logs prefix them with whitespace.
		if m := g1MemoryRe.FindStringSubmatch(line); m != nil && pending != nil {
			// Indices (1-based capture groups): 1-2 Eden before(unit),
			// 3-4 Eden after, 5-6 Survivors before, 7-8 Survivors after,
			// 9-10 Heap before, 11-12 Heap cap before, 13-14 Heap after,
			// 15-16 Heap cap after.
			pending.heapBeforeMB = toMBPtr(m[9], m[10])
			pending.heapAfterMB = toMBPtr(m[13], m[14])
			pending.heapCommittedMB = toMBPtr(m[15], m[16])
			edenBefore := toMBPtr(m[1], m[2])
			survivorsBefore := toMBPtr(m[5], m[6])
			edenAfter := toMBPtr(m[3], m[4])
			survivorsAfter := toMBPtr(m[7], m[8])
			pending.youngBeforeMB = safeAdd(edenBefore, survivorsBefore)
			pending.youngAfterMB = safeAdd(edenAfter, survivorsAfter)
			continue
		}

		// Metaspace companion line.
		if m := g1MetaspaceRe.FindStringSubmatch(line); m != nil && pending != nil {
			pending.metaspaceBeforeMB = toMBPtr(m[1], m[2])
			pending.metaspaceAfterMB = toMBPtr(m[3], m[4])
			continue
		}

		// GC pause / remark / cleanup line.
		if m := g1PauseRe.FindStringSubmatch(line); m != nil {
			if pending != nil {
				flush()
			}
			pauseSecsStr := matchGroup(g1PauseRe, m, "pause")
			pauseSecs, err := strconv.ParseFloat(pauseSecsStr, 64)
			pauseMS := 0.0
			if err == nil {
				pauseMS = pauseSecs * 1000.0
			}
			label := strings.TrimRight(strings.TrimSpace(matchGroup(g1PauseRe, m, "label")), ",")
			label = strings.TrimSpace(label)
			pending = &g1Pending{
				datestamp: matchGroup(g1PauseRe, m, "datestamp"),
				uptime:    matchGroup(g1PauseRe, m, "uptime"),
				label:     label,
				pauseMS:   &pauseMS,
				rawLine:   line,
			}
			// GC cleanup embeds heap sizes in its label.
			if cm := g1CleanupMemRe.FindStringSubmatch(line); cm != nil {
				pending.heapBeforeMB = toMBPtr(cm[1], cm[2])
				pending.heapAfterMB = toMBPtr(cm[3], cm[4])
				pending.heapCommittedMB = toMBPtr(cm[5], cm[6])
			} else if hm := g1InlineHeapRe.FindStringSubmatchIndex(label); hm != nil {
				// Single-line G1 format (without -XX:+PrintHeapAtGC):
				// "GC pause (young) 4096K->3936K(16M)" — heap embedded in label.
				groups := g1InlineHeapRe.FindStringSubmatch(label)
				pending.heapBeforeMB = toMBPtr(groups[1], groups[2])
				pending.heapAfterMB = toMBPtr(groups[3], groups[4])
				pending.heapCommittedMB = toMBPtr(groups[5], groups[6])
				pending.label = strings.TrimRight(label[:hm[0]], " ")
			}
			continue
		}

		// All other lines (heap block headers, worker detail lines, JVM headers): skip silently.
	}

	flush()
	return events
}

func buildG1Event(p g1Pending) Event {
	gcType, cause := parseG1Label(p.label)
	var uptime *float64
	if p.uptime != "" {
		if v, err := strconv.ParseFloat(strings.ReplaceAll(p.uptime, ",", "."), 64); err == nil {
			uptime = &v
		}
	}
	return Event{
		Timestamp:         parseLegacyTimestamp(p.datestamp),
		UptimeSec:         uptime,
		GCType:            gcType,
		Cause:             cause,
		PauseMS:           p.pauseMS,
		HeapBeforeMB:      p.heapBeforeMB,
		HeapAfterMB:       p.heapAfterMB,
		HeapCommittedMB:   p.heapCommittedMB,
		YoungBeforeMB:     p.youngBeforeMB,
		YoungAfterMB:      p.youngAfterMB,
		MetaspaceBeforeMB: p.metaspaceBeforeMB,
		MetaspaceAfterMB:  p.metaspaceAfterMB,
		RawLine:           p.rawLine,
	}
}

func parseG1Label(label string) (string, *string) {
	label = strings.TrimSpace(label)
	if strings.HasPrefix(label, "GC remark") {
		return "G1 Remark", nil
	}
	if strings.HasPrefix(label, "GC cleanup") {
		return "G1 Cleanup", nil
	}
	if !strings.HasPrefix(label, "GC pause") {
		return label, nil
	}

	groupRe := regexp.MustCompile(`\(([^)]+)\)`)
	matches := groupRe.FindAllStringSubmatch(label, -1)
	if len(matches) == 0 {
		return "G1 Young", nil
	}

	var cause *string
	phase := "young"
	var modifiers []string

	for _, m := range matches {
		g := m[1]
		gLower := strings.ToLower(g)
		if _, ok := g1Phases[gLower]; ok {
			phase = gLower
			continue
		}
		if gLower == "initial-mark" || gLower == "to-space exhausted" || gLower == "to-space overflow" {
			modifiers = append(modifiers, g)
			continue
		}
		v := g
		cause = &v
	}

	gcType := "G1 " + titleCase(phase)
	if len(modifiers) > 0 {
		gcType += " (" + strings.Join(modifiers, ", ") + ")"
	}
	return gcType, cause
}

// titleCase upper-cases the first rune (ASCII Python `.title()` is
// fine here — phase strings are always ASCII lowercase).
func titleCase(s string) string {
	if s == "" {
		return s
	}
	first := strings.ToUpper(s[:1])
	return first + s[1:]
}

// ═══════════════════════════════════════════════════════════════════
// Legacy Non-G1 (JDK 4-8 Serial / Parallel / CMS)
// ═══════════════════════════════════════════════════════════════════

func parseLegacy(lines []string, opts Options, diags *diagnostics.ParserDiagnostics) []Event {
	events := make([]Event, 0, len(lines))

	for i, line := range lines {
		lineNumber := i + 1
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			break
		}
		diags.TotalLines++
		if isBlank(line) {
			continue
		}

		pm := captureGroups(legacyPauseRe, line)
		if pm == nil {
			continue
		}

		// Use the LAST heap transition on the line — outer total heap
		// match comes after inner per-gen matches.
		allHeap := legacyHeapRe.FindAllStringSubmatchIndex(line, -1)
		if len(allHeap) == 0 {
			continue
		}
		lastIdx := allHeap[len(allHeap)-1]
		hm := captureGroupsFromIndex(legacyHeapRe, line, lastIdx)

		label := strings.TrimSpace(pm["label"])
		var uptime *float64
		if pm["uptime"] != "" {
			if v, err := strconv.ParseFloat(strings.ReplaceAll(pm["uptime"], ",", "."), 64); err == nil {
				uptime = &v
			}
		}

		gcType, cause := detectLegacyGCType(line, label)

		pauseSecs, err := strconv.ParseFloat(hm["pause"], 64)
		if err != nil {
			continue
		}
		pauseMS := pauseSecs * 1000.0

		events = append(events, Event{
			Timestamp:       parseLegacyTimestamp(pm["datestamp"]),
			UptimeSec:       uptime,
			GCType:          gcType,
			Cause:           cause,
			PauseMS:         &pauseMS,
			HeapBeforeMB:    toMBPtr(hm["before"], hm["bu"]),
			HeapAfterMB:     toMBPtr(hm["after"], hm["au"]),
			HeapCommittedMB: toMBPtr(hm["committed"], hm["cu"]),
			RawLine:         line,
		})
		diags.ParsedRecords++
	}
	return events
}

func detectLegacyGCType(line, label string) (string, *string) {
	isFull := strings.Contains(line, "Full GC") || strings.HasPrefix(label, "Full")

	var cause *string
	if m := regexp.MustCompile(`\(([^)]+)\)`).FindStringSubmatch(label); m != nil {
		v := m[1]
		cause = &v
	}

	if isFull {
		if strings.Contains(line, "PSYoungGen") || strings.Contains(line, "ParOldGen") {
			return "Full GC (Parallel)", cause
		}
		if strings.Contains(line, "CMS") {
			return "Full GC (CMS)", cause
		}
		return "Full GC", cause
	}

	switch {
	case strings.Contains(line, "DefNew"):
		return "Young GC (Serial)", cause
	case strings.Contains(line, "ParNew"):
		return "Young GC (CMS)", cause
	case strings.Contains(line, "PSYoungGen"):
		return "Young GC (Parallel)", cause
	case strings.Contains(line, "CMS-initial-mark"):
		return "CMS Initial Mark", cause
	case strings.Contains(line, "CMS-remark"):
		return "CMS Remark", cause
	}
	return "Young GC", cause
}

// ═══════════════════════════════════════════════════════════════════
// Shared utilities
// ═══════════════════════════════════════════════════════════════════

// splitUnifiedLabel mirrors `_split_unified_label`. The label is
// "<type>" or "<type> (<modifier>) (<cause>)" — gc_type is the prefix
// up to the first " (" and cause is the trailing parenthesised group
// when present.
func splitUnifiedLabel(label string) (string, *string) {
	gcType := strings.TrimSpace(strings.SplitN(label, " (", 2)[0])
	if gcType == "" {
		gcType = label
	}
	start := strings.LastIndex(label, " (")
	if start == -1 || !strings.HasSuffix(label, ")") {
		return gcType, nil
	}
	v := label[start+2 : len(label)-1]
	return gcType, &v
}

func parseTimestamp(value string) *time.Time {
	if value == "" {
		return nil
	}
	// Python's `datetime.fromisoformat` accepts both `+0900` and
	// `+09:00` from 3.11 onward. We coerce to the colon form and try
	// a few RFC3339-shaped layouts.
	fixed := tzFixRe.ReplaceAllString(value, `$1:$2`)
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000-07:00",
		"2006-01-02T15:04:05.000000-07:00",
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, fixed); err == nil {
			return &t
		}
	}
	return nil
}

func parseLegacyTimestamp(value string) *time.Time {
	if value == "" {
		return nil
	}
	return parseTimestamp(value)
}

func toMBPtr(value, unit string) *float64 {
	if value == "" || unit == "" {
		return nil
	}
	numeric, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
	if err != nil {
		return nil
	}
	u := strings.ToUpper(unit)
	switch u {
	case "B":
		v := round3(numeric / (1024.0 * 1024.0))
		return &v
	case "K":
		v := round3(numeric / 1024.0)
		return &v
	case "G":
		v := round3(numeric * 1024.0)
		return &v
	default:
		v := round3(numeric)
		return &v
	}
}

func round3(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return v
	}
	return math.Round(v*1000) / 1000
}

func safeAdd(a, b *float64) *float64 {
	if a == nil && b == nil {
		return nil
	}
	var av, bv float64
	if a != nil {
		av = *a
	}
	if b != nil {
		bv = *b
	}
	v := round3(av + bv)
	return &v
}

func isBlank(line string) bool {
	for _, r := range line {
		if r != ' ' && r != '\t' && r != '\r' && r != '\n' {
			return false
		}
	}
	return true
}

func captureGroups(re *regexp.Regexp, line string) map[string]string {
	match := re.FindStringSubmatch(line)
	if match == nil {
		return nil
	}
	out := make(map[string]string, len(re.SubexpNames()))
	for i, name := range re.SubexpNames() {
		if name == "" {
			continue
		}
		out[name] = match[i]
	}
	return out
}

// captureGroupsFromIndex extracts the named-group map from a single
// FindStringSubmatchIndex result so callers picking the Nth match
// (e.g. last legacy heap transition) can still get group access.
func captureGroupsFromIndex(re *regexp.Regexp, line string, idx []int) map[string]string {
	out := make(map[string]string, len(re.SubexpNames()))
	for i, name := range re.SubexpNames() {
		if name == "" {
			continue
		}
		start := idx[2*i]
		end := idx[2*i+1]
		if start < 0 || end < 0 {
			out[name] = ""
			continue
		}
		out[name] = line[start:end]
	}
	return out
}

// matchGroup returns the named capture from a `FindStringSubmatch`
// result without rebuilding the full map — used inside hot loops.
func matchGroup(re *regexp.Regexp, match []string, name string) string {
	for i, n := range re.SubexpNames() {
		if n == name {
			return match[i]
		}
	}
	return ""
}
