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
//
// ─────────────────────────────────────────────────────────────────────
// [한글] jfr 분석기 — JDK Flight Recorder JSON 출력 분석기.
//
// 입력
//
//	parsers/jfr.ParseFile() 가 만든 []Event. 입력 형식은
//	`jfr print --json recording.jfr` 의 출력. 바이너리 .jfr 자체는
//	Go 측에서 직접 파싱하지 않음 — JDK 의존을 끊기 위함.
//
// 출력 (analyze)
//
//	AnalysisResult{type: "jfr_recording"}
//	  • summary: total_events / by_mode 카운트 / 시간 범위.
//	  • series: events_over_time / pause_events (GC/safepoint) /
//	            events_by_type (top-N) / heatmap_strip (wall-clock).
//	  • tables: notable_events (최장 지속 / 최대 alloc 등 상위 N).
//	  • metadata: selected_mode / available_modes / filter_window.
//
// mode 필터 (--mode)
//
//	각 mode 는 Python MODE_EVENT_TYPES 와 byte 동일한 event-type
//	집합:
//	  cpu        : ExecutionSample / NativeMethodSample / MethodTrace ...
//	  wall       : (cpu 와 동일 — 캡처 시 sampling rate 만 다름)
//	  alloc      : ObjectAllocationInNewTLAB / OutsideTLAB / SampledObjectAllocation
//	  lock       : JavaMonitorEnter / Wait / ThreadPark
//	  gc         : GarbageCollection / Pause / Phase
//	  exception  : ThrowableConstructor / ExceptionStatistics
//	  io         : FileRead / FileWrite / SocketRead / SocketWrite
//	  nativemem  : NativeMemoryAllocation / NativeMemoryFree
//	  all        : 모든 mode 합집합.
//
// 시간 필터 (--from / --to)
//
//	parseTimeFilter 가 ISO 8601 / HH:MM[:SS] / 상대표기(+30s/-2m/500ms)
//	를 모두 처리. 상대 표기는 레코딩 시작/끝을 anchor 로 변환.
//
// 알고리즘 흐름 (Analyze)
//  1. parsers/jfr.ParseFile → []Event.
//  2. 시간 윈도우 + mode 필터 적용 — 통과한 이벤트만 스트림.
//  3. 한 번 순회로 events_over_time(분당 카운트) +
//     events_by_type(top-N) + pause_events(GC) + notable_events
//     (지속시간 또는 alloc bytes 상위 N) + heatmap_strip 동시 채움.
//  4. heatmap_strip : wall-clock 0~100% 구간을 분당 또는 1초 bucket
//     으로 stride. UI 의 "drag-to-set" 히트맵에 직결.
//  5. summary / metadata 채워서 반환.
//
// 결정론
//
//	sortedKeys / topN sort 모두 명시적 정렬. event-type 동률은
//	사전순.
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
const SchemaVersion = "0.3.0"

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

var asyncProfileEventTypes = setOf(
	"jdk.CPUTimeSample",
	"jdk.ExecutionSample",
	"jdk.NativeMethodSample",
	"jdk.MethodTrace",
	"jdk.ObjectAllocationInNewTLAB",
	"jdk.ObjectAllocationOutsideTLAB",
	"jdk.ObjectAllocationSample",
	"jdk.OldObjectSample",
	"jdk.JavaMonitorEnter",
	"jdk.JavaMonitorWait",
	"jdk.ThreadPark",
	"jdk.ThreadSleep",
)

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
	asyncProfile := buildAsyncProfile(filtered, opts.TopN)

	series := map[string]any{
		"events_over_time":      eventsOverTime(filtered),
		"pause_events":          pauseRows,
		"events_by_type":        eventsByType(filtered),
		"heatmap_strip":         buildHeatmapStrip(filtered),
		"sample_events_by_type": asyncProfile.SampleEventsByType,
	}

	tables := map[string]any{
		"notable_events": notableRows,
		"top_methods":    asyncProfile.TopMethods,
		"top_packages":   asyncProfile.TopPackages,
		"top_threads":    asyncProfile.TopThreads,
		"sample_stacks":  asyncProfile.SampleStacks,
	}
	charts := map[string]any{}
	if asyncProfile.Flamegraph != nil {
		charts["async_profile_flamegraph"] = asyncProfile.Flamegraph
	}

	summary["sample_event_count"] = asyncProfile.SampleEventCount
	summary["stack_sample_count"] = asyncProfile.StackSampleCount
	summary["unique_sample_stacks"] = asyncProfile.UniqueSampleStacks

	result := models.New(ResultType, ParserName)
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary
	result.Series = series
	result.Tables = tables
	result.Charts = charts
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = diags
	for _, finding := range buildRecordingFindings(events, filtered, asyncProfile) {
		result.AddFinding(finding.Severity, finding.Code, finding.Message, finding.Evidence)
	}

	if result.Metadata.Extra == nil {
		result.Metadata.Extra = map[string]any{}
	}
	result.Metadata.Extra["jfr_contract"] = map[string]any{
		"binary_boundary": "Binary .jfr files are converted by the JDK jfr CLI using `jfr print --json`; ArchScope analyzes the JSON output.",
		"desktop_scope":   "ArchScope surfaces report-oriented event summaries, stack aggregation, hints, and evidence rows. It does not attempt to replace the full JDK `jfr view` or `jfr summary` UI.",
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
	result.Metadata.Extra["async_profile"] = map[string]any{
		"frame_source":           "stackTrace.frames",
		"sample_event_count":     asyncProfile.SampleEventCount,
		"stack_sample_count":     asyncProfile.StackSampleCount,
		"unique_sample_stacks":   asyncProfile.UniqueSampleStacks,
		"dominant_sample_family": asyncProfile.DominantSampleFamily,
	}
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

// ── Async-profiler stack aggregation ───────────────────────────────────────

type asyncProfileData struct {
	SampleEventCount     int
	StackSampleCount     int
	UniqueSampleStacks   int
	DominantSampleFamily string
	SampleEventsByType   []map[string]any
	TopMethods           []map[string]any
	TopPackages          []map[string]any
	TopThreads           []map[string]any
	SampleStacks         []map[string]any
	Flamegraph           *asyncFlameNode
}

type asyncCounter struct {
	Name   string
	Count  int
	Bytes  int64
	Extra  map[string]int
	Frames []string
}

type asyncMutableNode struct {
	Name     string
	Samples  int
	Children map[string]*asyncMutableNode
}

type asyncFlameNode struct {
	ID       string           `json:"id"`
	ParentID *string          `json:"parentId"`
	Name     string           `json:"name"`
	Samples  int              `json:"samples"`
	Ratio    float64          `json:"ratio"`
	Category *string          `json:"category"`
	Color    *string          `json:"color"`
	Path     []string         `json:"path"`
	Children []asyncFlameNode `json:"children"`
}

type recordingFinding struct {
	Severity string
	Code     string
	Message  string
	Evidence map[string]any
}

func buildAsyncProfile(events []jfrparser.Event, topN int) asyncProfileData {
	if topN <= 0 {
		topN = DefaultTopN
	}
	methods := map[string]*asyncCounter{}
	packages := map[string]*asyncCounter{}
	threads := map[string]*asyncCounter{}
	stacks := map[string]*asyncCounter{}
	eventTypes := map[string]*asyncCounter{}
	familyCounts := map[string]int{}

	root := &asyncMutableNode{Name: "All", Children: map[string]*asyncMutableNode{}}
	out := asyncProfileData{}

	for _, event := range events {
		if !isAsyncProfileEvent(event) {
			continue
		}
		out.SampleEventCount++
		eventTypeCounter := ensureAsyncCounter(eventTypes, event.EventType)
		eventTypeCounter.Count++
		familyCounts[sampleFamily(event.EventType)]++

		if len(event.Frames) == 0 {
			continue
		}
		out.StackSampleCount++
		bytes := int64(0)
		if event.Size != nil && *event.Size > 0 {
			bytes = *event.Size
		}
		thread := "unknown"
		if event.Thread != nil && strings.TrimSpace(*event.Thread) != "" {
			thread = *event.Thread
		}
		topMethod := event.Frames[0]
		pkg := packageForFrame(topMethod)
		methodCounter := ensureAsyncCounter(methods, topMethod)
		methodCounter.Count++
		methodCounter.Bytes += bytes
		methodCounter.Extra[thread]++

		packageCounter := ensureAsyncCounter(packages, pkg)
		packageCounter.Count++
		packageCounter.Bytes += bytes
		packageCounter.Extra[topMethod]++

		threadCounter := ensureAsyncCounter(threads, thread)
		threadCounter.Count++
		threadCounter.Bytes += bytes
		threadCounter.Extra[topMethod]++

		flameFrames := reverseFrames(event.Frames)
		stackKey := strings.Join(flameFrames, ";")
		stackCounter := ensureAsyncCounter(stacks, stackKey)
		stackCounter.Count++
		stackCounter.Bytes += bytes
		stackCounter.Frames = append([]string(nil), event.Frames...)
		stackCounter.Extra[event.EventType]++
		addAsyncStack(root, flameFrames)
	}

	out.UniqueSampleStacks = len(stacks)
	out.DominantSampleFamily = dominantFamily(familyCounts)
	out.SampleEventsByType = asyncCountersToRows(eventTypes, topN, out.SampleEventCount, func(c *asyncCounter) map[string]any {
		return map[string]any{
			"event_type": c.Name,
			"count":      c.Count,
			"ratio":      ratio(c.Count, out.SampleEventCount, 4),
		}
	})
	out.TopMethods = asyncCountersToRows(methods, topN, out.StackSampleCount, func(c *asyncCounter) map[string]any {
		return map[string]any{
			"method":           c.Name,
			"package":          packageForFrame(c.Name),
			"samples":          c.Count,
			"sample_ratio":     ratio(c.Count, out.StackSampleCount, 4),
			"allocation_bytes": c.Bytes,
			"threads":          topNames(c.Extra, 5),
		}
	})
	out.TopPackages = asyncCountersToRows(packages, topN, out.StackSampleCount, func(c *asyncCounter) map[string]any {
		return map[string]any{
			"package":          c.Name,
			"samples":          c.Count,
			"sample_ratio":     ratio(c.Count, out.StackSampleCount, 4),
			"allocation_bytes": c.Bytes,
			"top_methods":      topNames(c.Extra, 5),
		}
	})
	out.TopThreads = asyncCountersToRows(threads, topN, out.StackSampleCount, func(c *asyncCounter) map[string]any {
		return map[string]any{
			"thread":           c.Name,
			"samples":          c.Count,
			"sample_ratio":     ratio(c.Count, out.StackSampleCount, 4),
			"allocation_bytes": c.Bytes,
			"top_methods":      topNames(c.Extra, 5),
		}
	})
	out.SampleStacks = asyncCountersToRows(stacks, topN, out.StackSampleCount, func(c *asyncCounter) map[string]any {
		return map[string]any{
			"collapsed_stack":   c.Name,
			"frames":            c.Frames,
			"samples":           c.Count,
			"sample_ratio":      ratio(c.Count, out.StackSampleCount, 4),
			"allocation_bytes":  c.Bytes,
			"event_types":       topNames(c.Extra, 5),
			"flamegraph_frames": strings.Split(c.Name, ";"),
		}
	})
	if out.StackSampleCount > 0 {
		out.Flamegraph = freezeAsyncNode(root, out.StackSampleCount, nil, []string{}, 0)
	}
	return out
}

func isAsyncProfileEvent(event jfrparser.Event) bool {
	if _, ok := asyncProfileEventTypes[event.EventType]; ok {
		return true
	}
	return len(event.Frames) > 0 && strings.Contains(event.EventType, "Sample")
}

func sampleFamily(eventType string) string {
	switch eventType {
	case "jdk.CPUTimeSample", "jdk.ExecutionSample", "jdk.NativeMethodSample", "jdk.MethodTrace":
		return "cpu_wall"
	case "jdk.ObjectAllocationInNewTLAB", "jdk.ObjectAllocationOutsideTLAB", "jdk.ObjectAllocationSample", "jdk.OldObjectSample":
		return "allocation"
	case "jdk.JavaMonitorEnter", "jdk.JavaMonitorWait", "jdk.ThreadPark", "jdk.ThreadSleep":
		return "lock_wait"
	default:
		return "other"
	}
}

func dominantFamily(counts map[string]int) string {
	bestName := ""
	bestCount := 0
	for name, count := range counts {
		if count > bestCount || (count == bestCount && name < bestName) {
			bestName = name
			bestCount = count
		}
	}
	return bestName
}

func ensureAsyncCounter(counters map[string]*asyncCounter, name string) *asyncCounter {
	counter := counters[name]
	if counter == nil {
		counter = &asyncCounter{Name: name, Extra: map[string]int{}}
		counters[name] = counter
	}
	return counter
}

func asyncCountersToRows(counters map[string]*asyncCounter, limit int, total int, row func(*asyncCounter) map[string]any) []map[string]any {
	if limit <= 0 {
		limit = DefaultTopN
	}
	items := make([]*asyncCounter, 0, len(counters))
	for _, counter := range counters {
		items = append(items, counter)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
	if limit > len(items) {
		limit = len(items)
	}
	rows := make([]map[string]any, 0, limit)
	for _, item := range items[:limit] {
		r := row(item)
		if _, ok := r["sample_ratio"]; !ok {
			r["sample_ratio"] = ratio(item.Count, total, 4)
		}
		rows = append(rows, r)
	}
	return rows
}

func topNames(counts map[string]int, limit int) []string {
	type item struct {
		name  string
		count int
	}
	items := make([]item, 0, len(counts))
	for name, count := range counts {
		items = append(items, item{name: name, count: count})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].name < items[j].name
	})
	if limit > len(items) {
		limit = len(items)
	}
	out := make([]string, 0, limit)
	for _, item := range items[:limit] {
		out = append(out, item.name)
	}
	return out
}

func reverseFrames(frames []string) []string {
	out := make([]string, 0, len(frames))
	for i := len(frames) - 1; i >= 0; i-- {
		frame := strings.TrimSpace(frames[i])
		if frame != "" {
			out = append(out, frame)
		}
	}
	return out
}

func addAsyncStack(root *asyncMutableNode, frames []string) {
	current := root
	current.Samples++
	for _, frame := range frames {
		child := current.Children[frame]
		if child == nil {
			child = &asyncMutableNode{Name: frame, Children: map[string]*asyncMutableNode{}}
			current.Children[frame] = child
		}
		child.Samples++
		current = child
	}
}

func freezeAsyncNode(node *asyncMutableNode, total int, parentID *string, path []string, ordinal int) *asyncFlameNode {
	id := "root"
	if parentID != nil {
		id = *parentID + "." + strconv.Itoa(ordinal)
	}
	currentPath := path
	if parentID != nil {
		currentPath = append(append([]string(nil), path...), node.Name)
	}
	children := make([]*asyncMutableNode, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, child)
	}
	sort.SliceStable(children, func(i, j int) bool {
		if children[i].Samples != children[j].Samples {
			return children[i].Samples > children[j].Samples
		}
		return children[i].Name < children[j].Name
	})
	out := &asyncFlameNode{
		ID:       id,
		ParentID: parentID,
		Name:     node.Name,
		Samples:  node.Samples,
		Ratio:    ratio(node.Samples, total, 4),
		Path:     currentPath,
		Children: make([]asyncFlameNode, 0, len(children)),
	}
	if pkg := packageForFrame(node.Name); parentID != nil && pkg != "" && pkg != "unknown" {
		out.Category = &pkg
	}
	for idx, child := range children {
		childParent := id
		out.Children = append(out.Children, *freezeAsyncNode(child, total, &childParent, currentPath, idx))
	}
	return out
}

func packageForFrame(frame string) string {
	text := strings.TrimSpace(frame)
	if text == "" || text == "unknown" {
		return "unknown"
	}
	parts := strings.Split(text, ".")
	if len(parts) >= 3 {
		return strings.Join(parts[:len(parts)-2], ".")
	}
	if len(parts) == 2 {
		return parts[0]
	}
	return "unknown"
}

func buildRecordingFindings(allEvents, filtered []jfrparser.Event, profile asyncProfileData) []recordingFinding {
	findings := []recordingFinding{}
	if len(filtered) == 0 && len(allEvents) > 0 {
		findings = append(findings, recordingFinding{
			Severity: "info",
			Code:     "JFR_FILTER_EMPTY",
			Message:  "The selected JFR filters matched no events. Clear mode, state, duration, or time-window filters before concluding the recording is empty.",
			Evidence: map[string]any{"total_events": len(allEvents)},
		})
		return findings
	}
	if len(filtered) > 0 && profile.StackSampleCount == 0 {
		findings = append(findings, recordingFinding{
			Severity: "warning",
			Code:     "JFR_NO_STACK_SAMPLES",
			Message:  "The filtered JFR events do not include stackTrace.frames, so ArchScope can show event summaries but cannot build a method/package stack profile.",
			Evidence: map[string]any{
				"filtered_events":    len(filtered),
				"sample_event_count": profile.SampleEventCount,
			},
		})
	}
	if profile.SampleEventCount > 0 {
		sampleRatio := float64(profile.SampleEventCount) / float64(max(len(filtered), 1))
		if sampleRatio >= 0.70 {
			findings = append(findings, recordingFinding{
				Severity: "info",
				Code:     "JFR_ASYNC_PROFILER_STYLE_RECORDING",
				Message:  "Most filtered JFR events are sample-style CPU, allocation, or lock events. ArchScope is treating this as an async-profiler-style recording and aggregating stackTrace.frames into report-ready method evidence.",
				Evidence: map[string]any{
					"filtered_events":        len(filtered),
					"sample_event_count":     profile.SampleEventCount,
					"dominant_sample_family": profile.DominantSampleFamily,
				},
			})
		}
	}
	if profile.StackSampleCount > 0 && profile.StackSampleCount < 5 {
		findings = append(findings, recordingFinding{
			Severity: "warning",
			Code:     "JFR_SPARSE_STACK_SAMPLES",
			Message:  "Only a few JFR events include stack frames. Treat top method and flamegraph rankings as directional, not statistically stable.",
			Evidence: map[string]any{
				"stack_sample_count":   profile.StackSampleCount,
				"unique_sample_stacks": profile.UniqueSampleStacks,
			},
		})
	}
	return findings
}

func ratio(value, total, decimals int) float64 {
	if total <= 0 {
		return 0
	}
	return roundHalfEven(float64(value)/float64(total)*100, decimals)
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
