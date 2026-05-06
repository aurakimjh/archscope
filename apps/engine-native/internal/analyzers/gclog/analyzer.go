// Package gclog ports archscope_engine.analyzers.gc_log_analyzer.
//
// The analyzer consumes events produced by
// internal/parsers/gclog and emits the AnalysisResult envelope the
// engine ↔ UI contract expects: a per-event pause / heap timeline,
// per-collector breakdown, JVM Info card sourced from the file
// header, and findings derived from pause / heap / throughput
// thresholds (LONG_GC_PAUSE / FULL_GC_PRESENT / LOW_GC_THROUGHPUT /
// HIGH_P99_PAUSE / HUMONGOUS_ALLOCATION / CONCURRENT_MODE_FAILURE /
// PROMOTION_FAILURE).
package gclog

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/gclog"
)

const (
	// SchemaVersion mirrors Python `metadata.schema_version`.
	SchemaVersion = "0.1.0"
	// DefaultTopN mirrors Python `top_n=20` — caps the per-event
	// `tables.events` list.
	DefaultTopN = 20
)

// parserNames mirrors Python `_PARSER_NAMES` — picks the
// `metadata.parser` literal based on the detected GC log format.
var parserNames = map[string]string{
	gclog.FormatUnified:  "hotspot_unified_gc_log",
	gclog.FormatG1Legacy: "hotspot_g1_legacy_gc_log",
	gclog.FormatLegacy:   "hotspot_legacy_gc_log",
	gclog.FormatUnknown:  "hotspot_unified_gc_log",
}

// histogramBucket is one row of the pause-time histogram. `Hi` is the
// open right bound; sentinel `+Inf` represents the unbounded top
// bucket (matched to `null` in JSON).
type histogramBucket struct {
	Label string
	Lo    float64
	Hi    float64
}

// histogramBuckets matches `_HISTOGRAM_BUCKETS_MS`.
var histogramBuckets = []histogramBucket{
	{"<1ms", 0.0, 1.0},
	{"1-5ms", 1.0, 5.0},
	{"5-10ms", 5.0, 10.0},
	{"10-50ms", 10.0, 50.0},
	{"50-100ms", 50.0, 100.0},
	{"100-500ms", 100.0, 500.0},
	{"500ms-1s", 500.0, 1_000.0},
	{"1-5s", 1_000.0, 5_000.0},
	{">=5s", 5_000.0, math.Inf(1)},
}

// Options carries analyzer-level knobs. Currently only `TopN` (matches
// Python's keyword arg) and the parser-level `MaxLines` / `Strict`
// passthroughs.
type Options struct {
	TopN     int
	MaxLines int
	Strict   bool
}

// Analyze parses `path`, extracts the JVM header, and returns the
// populated AnalysisResult.
func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	gcFormat := gclog.DetectFormat(path)
	jvmInfo := gclog.ExtractHeader(path)
	events, diags, err := gclog.ParseFile(path, gclog.Options{
		MaxLines: opts.MaxLines,
		Strict:   opts.Strict,
	})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return Build(events, path, gcFormat, jvmInfo.ToMap(), diags, opts), nil
}

// Build assembles the AnalysisResult from already-parsed events.
// Splitting Analyze and Build mirrors Python's two-tier API.
//
// `jvmInfo` may be nil/empty — only attached to metadata when
// non-empty (Python: `if jvm_info:`).
func Build(
	events []gclog.Event,
	sourceFile string,
	gcFormat string,
	jvmInfo map[string]any,
	diags *diagnostics.ParserDiagnostics,
	opts Options,
) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}

	var (
		totalEvents             int
		pauseCount              int
		totalPause              float64
		maxPause                float64
		pausesMS                = make([]float64, 0, len(events))
		typeCounts              = newOrderedCounter()
		causeCounts             = newOrderedCounter()
		pauseTimeline           = make([]map[string]any, 0, len(events))
		heapAfterMB             = make([]map[string]any, 0)
		heapBeforeMB            = make([]map[string]any, 0)
		heapCommittedMB         = make([]map[string]any, 0)
		youngBeforeMB           = make([]map[string]any, 0)
		youngAfterMB            = make([]map[string]any, 0)
		oldBeforeMB             = make([]map[string]any, 0)
		oldAfterMB              = make([]map[string]any, 0)
		metaspaceBeforeMB       = make([]map[string]any, 0)
		metaspaceAfterMB        = make([]map[string]any, 0)
		allocationRateTimeline  = make([]map[string]any, 0)
		promotionRateTimeline   = make([]map[string]any, 0)
		allocationRates         = make([]float64, 0)
		promotionRates          = make([]float64, 0)
		eventRows               = make([]map[string]any, 0, topN)
		firstUptimeSec          *float64
		lastUptimeSec           *float64
		humongousCount          int
		cmfCount                int
		promotionFailureCount   int
	)

	var prevEvent *gclog.Event

	for index, event := range events {
		event := event // capture for pointer
		totalEvents++
		typeCounts.add(strOrUnknown(event.GCType))
		causeCounts.add(strPtrOrUnknown(event.Cause))

		if event.PauseMS != nil {
			pauseCount++
			totalPause += *event.PauseMS
			if *event.PauseMS > maxPause {
				maxPause = *event.PauseMS
			}
			pausesMS = append(pausesMS, *event.PauseMS)
		}

		if event.UptimeSec != nil {
			if firstUptimeSec == nil {
				v := *event.UptimeSec
				firstUptimeSec = &v
			}
			v := *event.UptimeSec
			lastUptimeSec = &v
		}

		timeLabel := eventTime(event, index)

		var pauseValue float64
		if event.PauseMS != nil {
			pauseValue = *event.PauseMS
		}
		pauseTimeline = append(pauseTimeline, map[string]any{
			"time":    timeLabel,
			"value":   roundHalfEven(pauseValue, 3),
			"gc_type": strOrUnknown(event.GCType),
		})

		if event.HeapAfterMB != nil {
			heapAfterMB = append(heapAfterMB, map[string]any{"time": timeLabel, "value": *event.HeapAfterMB})
		}
		if event.HeapBeforeMB != nil {
			heapBeforeMB = append(heapBeforeMB, map[string]any{"time": timeLabel, "value": *event.HeapBeforeMB})
		}
		if event.HeapCommittedMB != nil {
			heapCommittedMB = append(heapCommittedMB, map[string]any{"time": timeLabel, "value": *event.HeapCommittedMB})
		}
		if event.YoungBeforeMB != nil {
			youngBeforeMB = append(youngBeforeMB, map[string]any{"time": timeLabel, "value": *event.YoungBeforeMB})
		}
		if event.YoungAfterMB != nil {
			youngAfterMB = append(youngAfterMB, map[string]any{"time": timeLabel, "value": *event.YoungAfterMB})
		}

		// Old gen synthesised from heap - young if no explicit value.
		var oldBefore *float64
		if event.OldBeforeMB != nil {
			v := *event.OldBeforeMB
			oldBefore = &v
		} else if event.HeapBeforeMB != nil && event.YoungBeforeMB != nil {
			v := math.Max(0.0, *event.HeapBeforeMB-*event.YoungBeforeMB)
			oldBefore = &v
		}
		if oldBefore != nil {
			oldBeforeMB = append(oldBeforeMB, map[string]any{"time": timeLabel, "value": *oldBefore})
		}

		var oldAfter *float64
		if event.OldAfterMB != nil {
			v := *event.OldAfterMB
			oldAfter = &v
		} else if event.HeapAfterMB != nil && event.YoungAfterMB != nil {
			v := math.Max(0.0, *event.HeapAfterMB-*event.YoungAfterMB)
			oldAfter = &v
		}
		if oldAfter != nil {
			oldAfterMB = append(oldAfterMB, map[string]any{"time": timeLabel, "value": *oldAfter})
		}

		if event.MetaspaceBeforeMB != nil {
			metaspaceBeforeMB = append(metaspaceBeforeMB, map[string]any{"time": timeLabel, "value": *event.MetaspaceBeforeMB})
		}
		if event.MetaspaceAfterMB != nil {
			metaspaceAfterMB = append(metaspaceAfterMB, map[string]any{"time": timeLabel, "value": *event.MetaspaceAfterMB})
		}

		if rate := allocationRateMBPerSec(prevEvent, &event); rate != nil {
			allocationRateTimeline = append(allocationRateTimeline, map[string]any{"time": timeLabel, "value": *rate})
			allocationRates = append(allocationRates, *rate)
		}
		if rate := promotionRateMBPerSec(prevEvent, &event); rate != nil {
			promotionRateTimeline = append(promotionRateTimeline, map[string]any{"time": timeLabel, "value": *rate})
			promotionRates = append(promotionRates, *rate)
		}

		if isHumongous(&event) {
			humongousCount++
		}
		if isConcurrentModeFailure(&event) {
			cmfCount++
		}
		if isPromotionFailure(&event) {
			promotionFailureCount++
		}

		if len(eventRows) < topN {
			eventRows = append(eventRows, eventToRow(&event))
		}
		prevEvent = &event
	}

	wallTimeSec := computeWallTimeSec(firstUptimeSec, lastUptimeSec)
	throughputPercent := computeThroughputPercent(totalPause, wallTimeSec)
	p50, p95, p99 := computePercentiles(pausesMS)

	avgPauseMS := 0.0
	maxPauseMS := 0.0
	if pauseCount > 0 {
		avgPauseMS = roundHalfEven(totalPause/float64(pauseCount), 3)
		maxPauseMS = roundHalfEven(maxPause, 3)
	}
	wallTimeOut := 0.0
	if wallTimeSec != nil {
		wallTimeOut = roundHalfEven(*wallTimeSec, 3)
	}

	youngGCCount := 0
	fullGCCount := 0
	for _, kv := range typeCounts.items() {
		if strings.Contains(kv.key, "Young") {
			youngGCCount += kv.count
		}
		if strings.Contains(kv.key, "Full") {
			fullGCCount += kv.count
		}
	}

	summary := map[string]any{
		"total_events":                  totalEvents,
		"total_pause_ms":                roundHalfEven(totalPause, 3),
		"avg_pause_ms":                  avgPauseMS,
		"max_pause_ms":                  maxPauseMS,
		"p50_pause_ms":                  p50,
		"p95_pause_ms":                  p95,
		"p99_pause_ms":                  p99,
		"throughput_percent":            throughputPercent,
		"wall_time_sec":                 wallTimeOut,
		"young_gc_count":                youngGCCount,
		"full_gc_count":                 fullGCCount,
		"avg_allocation_rate_mb_per_sec": avgFloat(allocationRates),
		"avg_promotion_rate_mb_per_sec":  avgFloat(promotionRates),
		"humongous_allocation_count":    humongousCount,
		"concurrent_mode_failure_count": cmfCount,
		"promotion_failure_count":       promotionFailureCount,
	}

	gcTypeBreakdown := make([]map[string]any, 0, typeCounts.len())
	for _, kv := range typeCounts.items() {
		gcTypeBreakdown = append(gcTypeBreakdown, map[string]any{"gc_type": kv.key, "count": kv.count})
	}
	causeBreakdown := make([]map[string]any, 0, causeCounts.len())
	for _, kv := range causeCounts.items() {
		causeBreakdown = append(causeBreakdown, map[string]any{"cause": kv.key, "count": kv.count})
	}

	series := map[string]any{
		"pause_timeline":             pauseTimeline,
		"heap_after_mb":              heapAfterMB,
		"heap_before_mb":             heapBeforeMB,
		"heap_committed_mb":          heapCommittedMB,
		"young_before_mb":            youngBeforeMB,
		"young_after_mb":             youngAfterMB,
		"old_before_mb":              oldBeforeMB,
		"old_after_mb":               oldAfterMB,
		"metaspace_before_mb":        metaspaceBeforeMB,
		"metaspace_after_mb":         metaspaceAfterMB,
		"pause_histogram":            buildPauseHistogram(pausesMS),
		"allocation_rate_mb_per_sec": allocationRateTimeline,
		"promotion_rate_mb_per_sec":  promotionRateTimeline,
		"gc_type_breakdown":          gcTypeBreakdown,
		"cause_breakdown":            causeBreakdown,
	}

	tables := map[string]any{
		"events": eventRows,
	}

	parser := parserNames[gcFormat]
	if parser == "" {
		parser = parserNames[gclog.FormatUnknown]
	}

	result := models.New("gc_log", parser)
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary
	result.Series = series
	result.Tables = tables
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = diags

	for _, finding := range buildFindings(summary) {
		result.Metadata.Findings = append(result.Metadata.Findings, finding)
	}

	if result.Metadata.Extra == nil {
		result.Metadata.Extra = map[string]any{}
	}
	result.Metadata.Extra["gc_format"] = gcFormat
	if len(jvmInfo) > 0 {
		result.Metadata.Extra["jvm_info"] = jvmInfo
	}
	result.Metadata.Extra["analysis_options"] = optionsToDict(opts)
	return result
}

// ─── Per-event helpers ───────────────────────────────────────────────

func eventTime(event gclog.Event, index int) string {
	if event.Timestamp != nil {
		// Python `datetime.isoformat()` emits microsecond precision and
		// no trailing 'Z' for naive datetimes; we mirror with
		// RFC3339Nano which carries whatever precision the parser
		// produced.
		return event.Timestamp.Format(time.RFC3339Nano)
	}
	if event.UptimeSec != nil {
		return formatFloat(*event.UptimeSec)
	}
	return strconv.Itoa(index)
}

// formatFloat mirrors Python `str(float)` — trims trailing zeros
// after the decimal but keeps `.0` to disambiguate integers.
func formatFloat(v float64) string {
	if v == math.Trunc(v) && !math.IsInf(v, 0) {
		return strconv.FormatFloat(v, 'f', 1, 64)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func eventToRow(event *gclog.Event) map[string]any {
	row := map[string]any{
		"timestamp":           nilOr(event.Timestamp, func(t time.Time) any { return t.Format(time.RFC3339Nano) }),
		"uptime_sec":          ptrOrNil(event.UptimeSec),
		"gc_type":             strOrNil(event.GCType),
		"cause":               strPtrOrNil(event.Cause),
		"pause_ms":            ptrOrNil(event.PauseMS),
		"heap_before_mb":      ptrOrNil(event.HeapBeforeMB),
		"heap_after_mb":       ptrOrNil(event.HeapAfterMB),
		"heap_committed_mb":   ptrOrNil(event.HeapCommittedMB),
		"young_before_mb":     ptrOrNil(event.YoungBeforeMB),
		"young_after_mb":      ptrOrNil(event.YoungAfterMB),
		"old_before_mb":       ptrOrNil(event.OldBeforeMB),
		"old_after_mb":        ptrOrNil(event.OldAfterMB),
		"metaspace_before_mb": ptrOrNil(event.MetaspaceBeforeMB),
		"metaspace_after_mb":  ptrOrNil(event.MetaspaceAfterMB),
		"raw_line":            event.RawLine,
	}
	return row
}

// ─── Wall time / throughput / percentiles ────────────────────────────

func computeWallTimeSec(first, last *float64) *float64 {
	if first == nil || last == nil {
		return nil
	}
	span := *last - *first
	if span <= 0 {
		return nil
	}
	return &span
}

func computeThroughputPercent(totalPauseMS float64, wallTimeSec *float64) float64 {
	if wallTimeSec == nil || *wallTimeSec <= 0 {
		return 0.0
	}
	pauseSec := totalPauseMS / 1000.0
	if pauseSec >= *wallTimeSec {
		return 0.0
	}
	return roundHalfEven((1.0-pauseSec/(*wallTimeSec))*100.0, 3)
}

func computePercentiles(pausesMS []float64) (float64, float64, float64) {
	if len(pausesMS) == 0 {
		return 0.0, 0.0, 0.0
	}
	sorted := append([]float64(nil), pausesMS...)
	sort.Float64s(sorted)
	return roundHalfEven(percentile(sorted, 50), 3),
		roundHalfEven(percentile(sorted, 95), 3),
		roundHalfEven(percentile(sorted, 99), 3)
}

// percentile mirrors Python's linear-interpolation percentile (the
// `numpy.percentile` default).
func percentile(sorted []float64, percent float64) float64 {
	if len(sorted) == 0 {
		return 0.0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := (percent / 100.0) * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := lo + 1
	if hi >= len(sorted) {
		hi = len(sorted) - 1
	}
	weight := rank - float64(lo)
	return sorted[lo]*(1.0-weight) + sorted[hi]*weight
}

func avgFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return roundHalfEven(sum/float64(len(values)), 3)
}

// ─── Allocation / promotion rates ────────────────────────────────────

func allocationRateMBPerSec(prev, curr *gclog.Event) *float64 {
	if prev == nil {
		return nil
	}
	if prev.UptimeSec == nil || curr.UptimeSec == nil {
		return nil
	}
	dt := *curr.UptimeSec - *prev.UptimeSec
	if dt <= 0 {
		return nil
	}
	if prev.HeapAfterMB == nil || curr.HeapBeforeMB == nil {
		return nil
	}
	allocated := *curr.HeapBeforeMB - *prev.HeapAfterMB
	if allocated < 0 {
		return nil
	}
	v := roundHalfEven(allocated/dt, 3)
	return &v
}

func promotionRateMBPerSec(prev, curr *gclog.Event) *float64 {
	if prev == nil {
		return nil
	}
	if prev.UptimeSec == nil || curr.UptimeSec == nil {
		return nil
	}
	dt := *curr.UptimeSec - *prev.UptimeSec
	if dt <= 0 {
		return nil
	}
	prevOld := oldGenAfterMB(prev)
	currOld := oldGenAfterMB(curr)
	if prevOld == nil || currOld == nil {
		return nil
	}
	promoted := *currOld - *prevOld
	if promoted < 0 {
		return nil
	}
	v := roundHalfEven(promoted/dt, 3)
	return &v
}

func oldGenAfterMB(event *gclog.Event) *float64 {
	if event.OldAfterMB != nil {
		v := *event.OldAfterMB
		return &v
	}
	if event.HeapAfterMB != nil && event.YoungAfterMB != nil {
		v := math.Max(0.0, *event.HeapAfterMB-*event.YoungAfterMB)
		return &v
	}
	return nil
}

// ─── Failure detectors ───────────────────────────────────────────────

func isHumongous(event *gclog.Event) bool {
	cause := strings.ToLower(strPtrOrEmpty(event.Cause))
	raw := strings.ToLower(event.RawLine)
	return strings.Contains(cause, "humongous") || strings.Contains(raw, "humongous")
}

func isConcurrentModeFailure(event *gclog.Event) bool {
	gcType := strings.ToLower(event.GCType)
	cause := strings.ToLower(strPtrOrEmpty(event.Cause))
	raw := strings.ToLower(event.RawLine)
	if strings.Contains(gcType, "to-space exhausted") || strings.Contains(gcType, "to-space overflow") {
		return true
	}
	if strings.Contains(cause, "concurrent mode failure") || strings.Contains(raw, "concurrent mode failure") {
		return true
	}
	return false
}

func isPromotionFailure(event *gclog.Event) bool {
	cause := strings.ToLower(strPtrOrEmpty(event.Cause))
	raw := strings.ToLower(event.RawLine)
	return strings.Contains(cause, "promotion failed") || strings.Contains(raw, "promotion failed")
}

// ─── Histogram / findings ────────────────────────────────────────────

func buildPauseHistogram(pausesMS []float64) []map[string]any {
	if len(pausesMS) == 0 {
		out := make([]map[string]any, 0, len(histogramBuckets))
		for _, b := range histogramBuckets {
			row := map[string]any{
				"bucket": b.Label,
				"min_ms": b.Lo,
				"max_ms": maxMSForJSON(b.Hi),
				"count":  0,
			}
			out = append(out, row)
		}
		return out
	}
	sorted := append([]float64(nil), pausesMS...)
	sort.Float64s(sorted)
	out := make([]map[string]any, 0, len(histogramBuckets))
	for _, b := range histogramBuckets {
		left := bisectLeft(sorted, b.Lo)
		var right int
		if math.IsInf(b.Hi, 1) {
			right = len(sorted)
		} else {
			right = bisectLeft(sorted, b.Hi)
		}
		count := right - left
		if count < 0 {
			count = 0
		}
		out = append(out, map[string]any{
			"bucket": b.Label,
			"min_ms": b.Lo,
			"max_ms": maxMSForJSON(b.Hi),
			"count":  count,
		})
	}
	return out
}

// maxMSForJSON yields `nil` for the unbounded top bucket, mirroring
// Python `hi if hi != float("inf") else None`.
func maxMSForJSON(hi float64) any {
	if math.IsInf(hi, 1) {
		return nil
	}
	return hi
}

// bisectLeft is the stdlib equivalent of Python's `bisect.bisect_left`
// — returns the leftmost insertion index for `value` in `sorted`.
func bisectLeft(sorted []float64, value float64) int {
	return sort.Search(len(sorted), func(i int) bool { return sorted[i] >= value })
}

// buildFindings ports `_build_findings`. Code names match Python
// verbatim.
func buildFindings(summary map[string]any) []map[string]any {
	findings := []map[string]any{}

	maxPause := asFloat(summary["max_pause_ms"])
	if maxPause >= 100 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "LONG_GC_PAUSE",
			"message":  "One or more GC pauses exceeded 100 ms.",
			"evidence": map[string]any{"max_pause_ms": summary["max_pause_ms"]},
		})
	}

	fullGC := asInt(summary["full_gc_count"])
	if fullGC > 0 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "FULL_GC_PRESENT",
			"message":  "Full GC activity was detected.",
			"evidence": map[string]any{"full_gc_count": summary["full_gc_count"]},
		})
	}

	throughput := asFloat(summary["throughput_percent"])
	wallTime := asFloat(summary["wall_time_sec"])
	if wallTime > 0 && throughput < 95.0 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "LOW_GC_THROUGHPUT",
			"message":  "GC throughput is below 95% — application is spending significant time in GC.",
			"evidence": map[string]any{"throughput_percent": throughput},
		})
	}

	p99 := asFloat(summary["p99_pause_ms"])
	if p99 >= 200.0 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "HIGH_P99_PAUSE",
			"message":  "p99 GC pause exceeds 200 ms — latency-sensitive workloads may be impacted.",
			"evidence": map[string]any{"p99_pause_ms": p99},
		})
	}

	humongous := asInt(summary["humongous_allocation_count"])
	if humongous > 0 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "HUMONGOUS_ALLOCATION",
			"message": "G1 humongous allocations detected. Consider increasing -XX:G1HeapRegionSize " +
				"or refactoring the allocation site to use smaller objects.",
			"evidence": map[string]any{"humongous_allocation_count": humongous},
		})
	}

	cmf := asInt(summary["concurrent_mode_failure_count"])
	if cmf > 0 {
		findings = append(findings, map[string]any{
			"severity": "critical",
			"code":     "CONCURRENT_MODE_FAILURE",
			"message": "Concurrent mode failure / to-space exhausted detected — " +
				"concurrent collector could not keep up with allocations, causing a Full GC.",
			"evidence": map[string]any{"concurrent_mode_failure_count": cmf},
		})
	}

	promotion := asInt(summary["promotion_failure_count"])
	if promotion > 0 {
		findings = append(findings, map[string]any{
			"severity": "critical",
			"code":     "PROMOTION_FAILURE",
			"message": "Promotion failure detected — old generation cannot accommodate promoted objects. " +
				"Increase old gen size or reduce allocation pressure.",
			"evidence": map[string]any{"promotion_failure_count": promotion},
		})
	}

	return findings
}

// ─── Counter (Python Counter.most_common) ────────────────────────────

// orderedCounter mirrors `collections.Counter.most_common()` —
// counts by insertion order so ties resolve to first-seen.
type orderedCounter struct {
	order []string
	seen  map[string]int // key → index in order
	count map[string]int
}

type counterEntry struct {
	key   string
	count int
}

func newOrderedCounter() *orderedCounter {
	return &orderedCounter{
		seen:  map[string]int{},
		count: map[string]int{},
	}
}

func (c *orderedCounter) add(key string) {
	if _, ok := c.seen[key]; !ok {
		c.seen[key] = len(c.order)
		c.order = append(c.order, key)
	}
	c.count[key]++
}

// items returns the entries sorted by count desc, breaking ties by
// insertion order. Mirrors Python's `Counter.most_common()` (which
// preserves insertion order for ties since CPython 3.7).
func (c *orderedCounter) items() []counterEntry {
	out := make([]counterEntry, 0, len(c.order))
	for _, key := range c.order {
		out = append(out, counterEntry{key: key, count: c.count[key]})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].count > out[j].count
	})
	return out
}

func (c *orderedCounter) len() int { return len(c.order) }

// ─── Misc helpers ────────────────────────────────────────────────────

func roundHalfEven(value float64, decimals int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	pow := math.Pow(10, float64(decimals))
	return math.RoundToEven(value*pow) / pow
}

func strOrUnknown(s string) string {
	if s == "" {
		return "UNKNOWN"
	}
	return s
}

func strPtrOrUnknown(p *string) string {
	if p == nil || *p == "" {
		return "UNKNOWN"
	}
	return *p
}

func strPtrOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func strOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func strPtrOrNil(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func ptrOrNil[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func nilOr[T any](p *T, fn func(T) any) any {
	if p == nil {
		return nil
	}
	return fn(*p)
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func optionsToDict(opts Options) map[string]any {
	out := map[string]any{
		"top_n":     DefaultTopN,
		"max_lines": nil,
		"strict":    opts.Strict,
	}
	if opts.TopN > 0 {
		out["top_n"] = opts.TopN
	}
	if opts.MaxLines > 0 {
		out["max_lines"] = opts.MaxLines
	}
	return out
}
