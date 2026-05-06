// Package jfr ports archscope_engine.analyzers.jfr_analyzer and
// archscope_engine.analyzers.native_memory_analyzer.
//
// Two analyzers live here:
//
//   - analyzer.go (this file) — `analyze_jfr_print_json` port.
//     Filters events by mode (cpu/wall/alloc/lock/gc/exception/io/
//     nativemem) and time window, then emits the AnalysisResult
//     envelope: events_over_time, pause_events, events_by_type, a
//     wall-clock heatmap_strip, plus notable_events and the metadata
//     companion fields (selected_mode, available_modes, filter_window).
//   - native_memory.go — `analyze_native_memory` port. Pairs
//     NativeMemoryAllocation/NativeMemoryFree events to surface
//     unfreed call sites, emits a flame tree under charts.flamegraph.
//
// The native memory analyzer is a separate file (rather than inlined)
// because its output type (`native_memory`), its parser metadata
// (`jdk_jfr_native_memory`), and its flame-tree emission are all
// independent of the main JFR analyzer. They share the JFR parser,
// the time helpers in this file, and the diagnostics envelope, which
// is exactly what the same package gives us.
package jfr

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	jfrparser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/jfr"
)

// SchemaVersion mirrors archscope_engine.analyzers.jfr_analyzer.SCHEMA_VERSION.
const SchemaVersion = "0.2.0"

// ResultType mirrors Python `AnalysisResult(type="jfr_recording", ...)`.
const ResultType = "jfr_recording"

// ParserName mirrors the Python `metadata.parser` literal.
const ParserName = "jdk_jfr_recording"

// DefaultTopN mirrors the Python `top_n` default for notable_events.
const DefaultTopN = 20

// modeEventTypes mirrors Python `MODE_EVENT_TYPES`. `wall` is
// intentionally identical to `cpu` (same event types — only sampling
// rate differs at capture time).
var modeEventTypes = map[string]map[string]struct{}{
	"cpu": setOf(
		"jdk.CPUTimeSample",
		"jdk.ExecutionSample",
		"jdk.NativeMethodSample",
		"jdk.MethodTrace",
	),
	"wall": setOf(
		"jdk.CPUTimeSample",
		"jdk.ExecutionSample",
		"jdk.NativeMethodSample",
		"jdk.MethodTrace",
	),
	"alloc": setOf(
		"jdk.ObjectAllocationInNewTLAB",
		"jdk.ObjectAllocationOutsideTLAB",
		"jdk.ObjectAllocationSample",
		"jdk.OldObjectSample",
		"jdk.PromoteObjectInNewPLAB",
		"jdk.PromoteObjectOutsidePLAB",
	),
	"lock": setOf(
		"jdk.JavaMonitorEnter",
		"jdk.JavaMonitorWait",
		"jdk.ThreadPark",
		"jdk.ThreadSleep",
	),
	"gc": setOf(
		"jdk.GarbageCollection",
		"jdk.GCPhasePause",
		"jdk.GCPhaseParallel",
		"jdk.GCPhaseConcurrent",
		"jdk.GCHeapSummary",
		"jdk.G1HeapSummary",
		"jdk.MetaspaceSummary",
		"jdk.SafepointBegin",
		"jdk.SafepointEnd",
	),
	"exception": setOf(
		"jdk.JavaExceptionThrow",
		"jdk.JavaErrorThrow",
	),
	"io": setOf(
		"jdk.SocketRead",
		"jdk.SocketWrite",
		"jdk.FileRead",
		"jdk.FileWrite",
	),
	"nativemem": setOf(
		"jdk.NativeMemoryAllocation",
		"jdk.NativeMemoryFree",
		"jdk.NativeAllocation",
		"jdk.NativeFree",
	),
}

// SupportedModes mirrors Python `supported_modes()` — `["all", ...sorted]`.
func SupportedModes() []string {
	modes := make([]string, 0, len(modeEventTypes))
	for mode := range modeEventTypes {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	return append([]string{"all"}, modes...)
}

// Options captures the analyze_jfr_print_json kwargs. A nil pointer is
// the "filter unset" sentinel matching Python's `None`.
type Options struct {
	TopN          int
	Mode          string
	FromTime      string
	ToTime        string
	State         string
	MinDurationMS *float64
}

// Analyze parses `path` (binary .jfr or jfr-print JSON) and returns the
// populated AnalysisResult. Mirrors `analyze_jfr_print_json`.
func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	diags := diagnostics.New("jfr")
	events, info, err := jfrparser.ParseRecording(path, diags)
	if err != nil {
		// Surface CLI-missing as a regular error — the caller (CLI /
		// API) decides how to render. The Python analyzer wraps it in
		// ValueError; the Go layer leaves the typed error for callers
		// to inspect.
		return models.AnalysisResult{}, err
	}
	return Build(events, path, info, diags, opts), nil
}

// Build assembles the AnalysisResult from already-parsed events.
// Splitting Analyze and Build matches Python's two-tier shape and lets
// tests feed events directly without going through the parser.
func Build(events []jfrparser.Event, sourceFile string, info jfrparser.SourceInfo, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	if opts.TopN <= 0 {
		opts.TopN = DefaultTopN
	}
	selectedMode := normalizeMode(opts.Mode)
	availableModes := detectAvailableModes(events)
	availableStates := detectAvailableStates(events)

	modeEvents := filterByMode(events, selectedMode)
	window := resolveWindow(events, opts.FromTime, opts.ToTime)
	filtered := filterByWindow(modeEvents, window)
	if opts.State != "" {
		stateUpper := strings.ToUpper(opts.State)
		filtered = filterFunc(filtered, func(e jfrparser.Event) bool {
			return e.State != nil && strings.ToUpper(*e.State) == stateUpper
		})
	}
	if opts.MinDurationMS != nil && *opts.MinDurationMS > 0 {
		minDur := *opts.MinDurationMS
		filtered = filterFunc(filtered, func(e jfrparser.Event) bool {
			d := durationOrZero(e)
			return d >= minDur
		})
	}

	pauseEvents := filterFunc(filtered, func(e jfrparser.Event) bool {
		return e.DurationMS != nil && *e.DurationMS > 0
	})

	// notable_events — sort by duration desc (None == 0). Python uses
	// the standard `sorted(..., reverse=True)`, which is stable; Go's
	// sort.SliceStable preserves the original ordering for ties.
	notableSorted := append([]jfrparser.Event(nil), filtered...)
	sort.SliceStable(notableSorted, func(i, j int) bool {
		return durationOrZero(notableSorted[i]) > durationOrZero(notableSorted[j])
	})
	notableLimit := opts.TopN
	if notableLimit > len(notableSorted) {
		notableLimit = len(notableSorted)
	}
	notable := notableSorted[:notableLimit]

	gcEvents := filterFunc(filtered, isGCEvent)
	gcPauseTotal := durationSumMS(gcEvents)
	blockedThreadEvents := 0
	for _, e := range filtered {
		if e.EventType == "jdk.ThreadPark" || e.EventType == "jdk.JavaMonitorEnter" {
			blockedThreadEvents++
		}
	}

	summary := map[string]any{
		"event_count":           len(filtered),
		"event_count_total":     len(events),
		"duration_ms":           roundHalfEven(durationSumMS(filtered), 3),
		"gc_pause_total_ms":     roundHalfEven(gcPauseTotal, 3),
		"blocked_thread_events": blockedThreadEvents,
		"selected_mode":         selectedMode,
	}

	pauseRows := make([]map[string]any, 0, len(pauseEvents))
	for _, e := range pauseEvents {
		pauseRows = append(pauseRows, toPauseEvent(e))
	}
	notableRows := make([]map[string]any, 0, len(notable))
	for i, e := range notable {
		notableRows = append(notableRows, toNotableEvent(e, i+1))
	}

	series := map[string]any{
		"events_over_time": eventsOverTime(filtered),
		"pause_events":     pauseRows,
		"events_by_type":   eventsByType(filtered),
		"heatmap_strip":    buildHeatmapStrip(filtered),
	}

	tables := map[string]any{
		"notable_events": notableRows,
	}

	result := models.New(ResultType, ParserName)
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary
	result.Series = series
	result.Tables = tables
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = diags

	if result.Metadata.Extra == nil {
		result.Metadata.Extra = map[string]any{}
	}
	result.Metadata.Extra["jfr_command_version"] = "external"
	result.Metadata.Extra["selected_mode"] = selectedMode
	result.Metadata.Extra["available_modes"] = availableModes
	result.Metadata.Extra["supported_modes"] = SupportedModes()
	result.Metadata.Extra["available_states"] = availableStates
	if opts.State == "" {
		result.Metadata.Extra["selected_state"] = nil
	} else {
		result.Metadata.Extra["selected_state"] = opts.State
	}
	if opts.MinDurationMS == nil {
		result.Metadata.Extra["min_duration_ms"] = nil
	} else {
		result.Metadata.Extra["min_duration_ms"] = *opts.MinDurationMS
	}
	result.Metadata.Extra["filter_window"] = windowToDict(window)
	result.Metadata.Extra["source_format"] = info.SourceFormat
	if info.JFRCli != nil {
		result.Metadata.Extra["jfr_cli"] = *info.JFRCli
	} else {
		result.Metadata.Extra["jfr_cli"] = nil
	}
	return result
}

// ── Mode filter ─────────────────────────────────────────────────────────────

func normalizeMode(mode string) string {
	if mode == "" {
		return "all"
	}
	if mode == "all" {
		return "all"
	}
	if _, ok := modeEventTypes[mode]; ok {
		return mode
	}
	return "all"
}

func filterByMode(events []jfrparser.Event, mode string) []jfrparser.Event {
	if mode == "all" {
		return events
	}
	allowed, ok := modeEventTypes[mode]
	if !ok {
		return events
	}
	out := make([]jfrparser.Event, 0, len(events))
	for _, e := range events {
		if _, ok := allowed[e.EventType]; ok {
			out = append(out, e)
		}
	}
	return out
}

func detectAvailableModes(events []jfrparser.Event) []string {
	seen := map[string]struct{}{}
	for _, e := range events {
		seen[e.EventType] = struct{}{}
	}
	available := make([]string, 0, len(modeEventTypes)+1)
	if len(seen) > 0 {
		available = append(available, "all")
	}
	// Python iterates MODE_EVENT_TYPES in insertion order. We need a
	// deterministic Go equivalent: iterate the keys in the same order
	// Python defines them in the source.
	for _, mode := range modeOrder {
		types := modeEventTypes[mode]
		if hasIntersection(seen, types) {
			available = append(available, mode)
		}
	}
	return available
}

// modeOrder mirrors the dict insertion order used by the Python source
// so detect_available_modes returns the same sequence on parity tests.
var modeOrder = []string{"cpu", "wall", "alloc", "lock", "gc", "exception", "io", "nativemem"}

func detectAvailableStates(events []jfrparser.Event) []string {
	seen := map[string]struct{}{}
	for _, e := range events {
		if e.State != nil && *e.State != "" {
			seen[*e.State] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func hasIntersection(seen map[string]struct{}, types map[string]struct{}) bool {
	for t := range types {
		if _, ok := seen[t]; ok {
			return true
		}
	}
	return false
}

// ── Time-range filter ───────────────────────────────────────────────────────

var (
	relativeRE = regexp.MustCompile(`^([+-]?)(\d+(?:\.\d+)?)(ms|s|m|h)$`)
	hmsRE      = regexp.MustCompile(`^(\d{1,2}):(\d{2})(?::(\d{2}(?:\.\d+)?))?$`)
)

// filterWindow holds resolved start/end + the original raw inputs so
// metadata.filter_window can echo them back verbatim. Mirrors Python's
// `_Window`.
type filterWindow struct {
	Start     *time.Time
	End       *time.Time
	FromInput string
	ToInput   string
}

func resolveWindow(events []jfrparser.Event, from, to string) filterWindow {
	if from == "" && to == "" {
		return filterWindow{}
	}
	start, end := recordingBounds(events)
	parsedStart := parseTimeInput(from, start, end)
	parsedEnd := parseTimeInput(to, start, end)
	return filterWindow{
		Start:     parsedStart,
		End:       parsedEnd,
		FromInput: from,
		ToInput:   to,
	}
}

func windowToDict(w filterWindow) map[string]any {
	out := map[string]any{
		"from":            nullable(w.FromInput),
		"to":              nullable(w.ToInput),
		"effective_start": nil,
		"effective_end":   nil,
	}
	if w.Start != nil {
		out["effective_start"] = formatPythonISO(*w.Start)
	}
	if w.End != nil {
		out["effective_end"] = formatPythonISO(*w.End)
	}
	return out
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func recordingBounds(events []jfrparser.Event) (*time.Time, *time.Time) {
	var first, last *time.Time
	for _, e := range events {
		ts := parseEventTime(e.Time)
		if ts == nil {
			continue
		}
		if first == nil || ts.Before(*first) {
			t := *ts
			first = &t
		}
		if last == nil || ts.After(*last) {
			t := *ts
			last = &t
		}
	}
	return first, last
}

func parseTimeInput(raw string, recordingStart, recordingEnd *time.Time) *time.Time {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}

	// Relative offsets (e.g. +30s, -2m, 500ms).
	if m := relativeRE.FindStringSubmatch(text); m != nil {
		sign := m[1]
		value, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return nil
		}
		unit := m[3]
		multiplier := map[string]float64{
			"ms": 1,
			"s":  1_000,
			"m":  60_000,
			"h":  3_600_000,
		}[unit]
		deltaMS := value * multiplier
		delta := time.Duration(deltaMS * float64(time.Millisecond))
		if sign == "-" {
			anchor := recordingEnd
			if anchor == nil {
				now := time.Now().UTC()
				anchor = &now
			}
			out := anchor.Add(-delta)
			return &out
		}
		anchor := recordingStart
		if anchor == nil {
			now := time.Now().UTC()
			anchor = &now
		}
		out := anchor.Add(delta)
		return &out
	}

	// ISO 8601 first.
	if t, ok := parseISOLike(text); ok {
		return &t
	}

	// HH:MM[:SS[.fff]] anchored to the recording's date.
	if hm := hmsRE.FindStringSubmatch(text); hm != nil && recordingStart != nil {
		hour, err := strconv.Atoi(hm[1])
		if err != nil {
			return nil
		}
		minute, err := strconv.Atoi(hm[2])
		if err != nil {
			return nil
		}
		secText := hm[3]
		if secText == "" {
			secText = "0"
		}
		secFloat, err := strconv.ParseFloat(secText, 64)
		if err != nil {
			return nil
		}
		secInt := int(secFloat)
		micro := int(math.Round((secFloat - float64(secInt)) * 1_000_000))
		nanosec := micro * 1_000
		anchor := *recordingStart
		out := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), hour, minute, secInt, nanosec, anchor.Location())
		return &out
	}

	return nil
}

// parseISOLike mirrors Python's `datetime.fromisoformat(text.replace("Z","+00:00"))`.
// Python 3.9 only accepts microsecond precision (1-6 fractional
// digits); strings carrying nanoseconds (7+ fractional digits) raise
// ValueError, which the analyzer treats as "no parseable timestamp".
// We reproduce that contract here.
func parseISOLike(text string) (time.Time, bool) {
	swapped := strings.Replace(text, "Z", "+00:00", 1)
	// Reject strings whose fractional second has >6 digits to match
	// Python 3.9's stricter `fromisoformat`.
	if idx := strings.Index(swapped, "."); idx >= 0 {
		end := idx + 1
		for end < len(swapped) {
			if swapped[end] < '0' || swapped[end] > '9' {
				break
			}
			end++
		}
		if end-idx-1 > 6 {
			return time.Time{}, false
		}
	}
	// Try a sequence of layouts roughly matching what
	// `datetime.fromisoformat` accepts.
	layouts := []string{
		"2006-01-02T15:04:05.999999-07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, swapped); err == nil {
			// time.Parse defaults naive layouts to UTC, which matches
			// Python's `_ensure_aware` (None tzinfo → UTC).
			return t, true
		}
	}
	return time.Time{}, false
}

// parseEventTime mirrors Python `_parse_event_time`. Returns nil on
// any parse failure (matching Python's `except ValueError: return None`).
func parseEventTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	if t, ok := parseISOLike(value); ok {
		return &t
	}
	return nil
}

func filterByWindow(events []jfrparser.Event, w filterWindow) []jfrparser.Event {
	if w.Start == nil && w.End == nil {
		return events
	}
	out := make([]jfrparser.Event, 0, len(events))
	for _, e := range events {
		ts := parseEventTime(e.Time)
		if ts == nil {
			out = append(out, e)
			continue
		}
		if w.Start != nil && ts.Before(*w.Start) {
			continue
		}
		if w.End != nil && ts.After(*w.End) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ── Series helpers ──────────────────────────────────────────────────────────

func durationOrZero(e jfrparser.Event) float64 {
	if e.DurationMS == nil {
		return 0
	}
	return *e.DurationMS
}

func durationSumMS(events []jfrparser.Event) float64 {
	total := 0.0
	for _, e := range events {
		total += durationOrZero(e)
	}
	return total
}

func isGCEvent(e jfrparser.Event) bool {
	return strings.Contains(e.EventType, "GC") || strings.Contains(e.EventType, "Garbage")
}

// buildHeatmapStrip mirrors `_build_heatmap_strip`.
func buildHeatmapStrip(events []jfrparser.Event) map[string]any {
	times := make([]time.Time, 0, len(events))
	for _, e := range events {
		ts := parseEventTime(e.Time)
		if ts != nil {
			times = append(times, *ts)
		}
	}
	if len(times) == 0 {
		return map[string]any{
			"bucket_seconds": 0.0,
			"start_time":     nil,
			"end_time":       nil,
			"max_count":      0,
			"buckets":        []map[string]any{},
		}
	}

	start := times[0]
	end := times[0]
	for _, t := range times[1:] {
		if t.Before(start) {
			start = t
		}
		if t.After(end) {
			end = t
		}
	}
	durationS := end.Sub(start).Seconds()
	if durationS < 0.001 {
		durationS = 0.001
	}

	var bucketS float64
	switch {
	case durationS <= 30:
		bucketS = 0.5
	case durationS <= 120:
		bucketS = 1.0
	case durationS <= 600:
		bucketS = 5.0
	case durationS <= 3600:
		bucketS = 30.0
	case durationS <= 86_400:
		bucketS = 300.0
	default:
		bucketS = 1_800.0
	}

	bucketCount := int(durationS/bucketS) + 1
	if bucketCount < 1 {
		bucketCount = 1
	}
	counts := make([]int, bucketCount)
	startTS := float64(start.Unix()) + float64(start.Nanosecond())/1e9
	for _, t := range times {
		ts := float64(t.Unix()) + float64(t.Nanosecond())/1e9
		idx := int((ts - startTS) / bucketS)
		if idx < 0 {
			idx = 0
		}
		if idx >= bucketCount {
			idx = bucketCount - 1
		}
		counts[idx]++
	}

	buckets := make([]map[string]any, 0, bucketCount)
	maxCount := 0
	for i, count := range counts {
		offsetNS := int64(bucketS * float64(i) * float64(time.Second))
		bucketStart := start.Add(time.Duration(offsetNS))
		buckets = append(buckets, map[string]any{
			"index": i,
			"time":  formatPythonISO(bucketStart),
			"count": count,
		})
		if count > maxCount {
			maxCount = count
		}
	}

	return map[string]any{
		"bucket_seconds": bucketS,
		"start_time":     formatPythonISO(start),
		"end_time":       formatPythonISO(end),
		"max_count":      maxCount,
		"buckets":        buckets,
	}
}

// eventsOverTime mirrors `_events_over_time`. The Python implementation
// keys a Counter by (time, event_type), sorts the items lexically (the
// default tuple comparison), and emits rows in that order.
func eventsOverTime(events []jfrparser.Event) []map[string]any {
	type key struct {
		time      string
		eventType string
	}
	counts := map[key]int{}
	for _, e := range events {
		counts[key{e.Time, e.EventType}]++
	}
	keys := make([]key, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].time != keys[j].time {
			return keys[i].time < keys[j].time
		}
		return keys[i].eventType < keys[j].eventType
	})
	rows := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, map[string]any{
			"time":       k.time,
			"event_type": k.eventType,
			"count":      counts[k],
		})
	}
	return rows
}

// eventsByType mirrors `_events_by_type` — Counter.most_common emits
// rows in count-desc order, ties broken by insertion order. Go has no
// insertion order for maps; we tie-break by event_type ascending so
// the output is deterministic.
func eventsByType(events []jfrparser.Event) []map[string]any {
	counts := map[string]int{}
	for _, e := range events {
		counts[e.EventType]++
	}
	type kv struct {
		eventType string
		count     int
	}
	rows := make([]kv, 0, len(counts))
	for k, v := range counts {
		rows = append(rows, kv{k, v})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].eventType < rows[j].eventType
	})
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"event_type": r.eventType,
			"count":      r.count,
		})
	}
	return out
}

func toPauseEvent(e jfrparser.Event) map[string]any {
	return map[string]any{
		"time":          e.Time,
		"duration_ms":   ptrToAny(e.DurationMS),
		"event_type":    e.EventType,
		"thread":        ptrStringToAny(e.Thread),
		"sampling_type": samplingType(e.EventType),
	}
}

func toNotableEvent(e jfrparser.Event, index int) map[string]any {
	frames := e.Frames
	if frames == nil {
		frames = []string{}
	}
	return map[string]any{
		"time":          e.Time,
		"event_type":    e.EventType,
		"duration_ms":   ptrToAny(e.DurationMS),
		"thread":        ptrStringToAny(e.Thread),
		"message":       e.Message,
		"frames":        frames,
		"sampling_type": samplingType(e.EventType),
		"evidence_ref":  fmt.Sprintf("jfr:event:%d", index),
		"raw_preview":   e.RawPreview,
	}
}

func samplingType(eventType string) string {
	if eventType == "jdk.CPUTimeSample" || eventType == "jdk.ExecutionSample" {
		return "sampled"
	}
	return "exact"
}

// ── small utilities ─────────────────────────────────────────────────────────

func setOf(items ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		out[item] = struct{}{}
	}
	return out
}

func filterFunc[T any](in []T, keep func(T) bool) []T {
	out := make([]T, 0, len(in))
	for _, item := range in {
		if keep(item) {
			out = append(out, item)
		}
	}
	return out
}

func ptrToAny(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func ptrStringToAny(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func roundHalfEven(value float64, decimals int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	pow := math.Pow(10, float64(decimals))
	return math.RoundToEven(value*pow) / pow
}

// formatPythonISO returns the same string Python's
// `datetime.isoformat()` emits for a timezone-aware datetime —
// fractional seconds at microsecond resolution, "+00:00" for UTC.
func formatPythonISO(t time.Time) string {
	t = t.UTC()
	usec := t.Nanosecond() / 1000
	base := t.Format("2006-01-02T15:04:05")
	if usec != 0 {
		base += "." + padLeftZero(usec, 6)
	}
	return base + "+00:00"
}

func padLeftZero(value, width int) string {
	s := strconv.Itoa(value)
	for len(s) < width {
		s = "0" + s
	}
	return s
}
