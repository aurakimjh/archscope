// Package multithread ports archscope_engine.analyzers.multi_thread_analyzer.
//
// The analyzer consumes an ordered list of ThreadDumpBundle values and
// emits an AnalysisResult of type "thread_dump_multi". It correlates
// per-thread behavior across dumps and produces ten cross-language
// findings:
//
//   - LONG_RUNNING_THREAD             (thread RUNNABLE on same stack ≥N dumps)
//   - PERSISTENT_BLOCKED_THREAD       (thread BLOCKED/LOCK_WAIT ≥N dumps)
//   - LATENCY_SECTION_DETECTED        (thread NETWORK/IO/CHANNEL_WAIT ≥N dumps)
//   - GROWING_LOCK_CONTENTION         (lock waiter count strictly grows)
//   - THREAD_CONGESTION_DETECTED      (>10% threads waiting per bundle)
//   - EXTERNAL_RESOURCE_WAIT_HIGH     (>25% threads timed-waiting per bundle)
//   - LIKELY_GC_PAUSE_DETECTED        (>50% threads blocked on unowned monitor)
//   - VIRTUAL_THREAD_CARRIER_PINNING  (JVM virtual-thread pinning marker)
//   - SMR_UNRESOLVED_THREAD           (JVM SMR diagnostics zombie/unresolved)
//   - INCOMPLETE_HISTOGRAM            (truncated class-histogram block)
//
// The analyzer never imports runtime-specific helpers; per-language
// enrichment (state coercion, lock metadata) is the parser plugin's job.
package multithread

import (
	"errors"
	"fmt"
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const (
	// ConsecutiveDumpsThreshold mirrors the Python
	// CONSECUTIVE_DUMPS_THRESHOLD default (3 dumps).
	ConsecutiveDumpsThreshold = 3
	// DefaultTopN matches the Python `top_n=20` default. Caps every
	// table the renderer consumes.
	DefaultTopN = 20
	// SchemaVersion mirrors Python `schema_version="0.1.0"`.
	SchemaVersion = "0.1.0"
	// ParserName mirrors Python `metadata.parser="thread_dump_multi"`.
	ParserName = "thread_dump_multi"
	// ResultType mirrors Python `AnalysisResult.type="thread_dump_multi"`.
	ResultType = "thread_dump_multi"
)

// Options matches the Python keyword arguments of
// analyze_multi_thread_dumps. Zero values fall back to the canonical
// Python defaults.
type Options struct {
	// Threshold is the minimum number of consecutive dumps required
	// for a persistence-style finding. Defaults to
	// ConsecutiveDumpsThreshold when ≤0.
	Threshold int
	// TopN caps every table emitted in the AnalysisResult. Defaults to
	// DefaultTopN when ≤0.
	TopN int
	// IncludeRawSnapshots is reserved for parity with Python's flag.
	// The Go port does not currently embed snapshot blobs in the
	// envelope (matching the Python default of False), so this field
	// is accepted but unused — it lets the same call sites flip the
	// flag once the renderer side is wired up.
	IncludeRawSnapshots bool
}

// ErrNoBundles mirrors Python `ValueError("analyze_multi_thread_dumps
// requires at least one bundle.")`.
var ErrNoBundles = errors.New("multithread: analyze requires at least one bundle")

// Analyze correlates threads across `bundles` and returns the populated
// AnalysisResult. Mirrors `analyze_multi_thread_dumps`.
func Analyze(bundles []models.ThreadDumpBundle, opts Options) (models.AnalysisResult, error) {
	if len(bundles) == 0 {
		return models.AnalysisResult{}, ErrNoBundles
	}
	threshold := opts.Threshold
	if threshold <= 0 {
		threshold = ConsecutiveDumpsThreshold
	}
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}

	timelines := buildTimelines(bundles)
	languages := uniqueSorted(bundles, func(b models.ThreadDumpBundle) string { return b.Language })
	formats := uniqueSorted(bundles, func(b models.ThreadDumpBundle) string { return b.SourceFormat })

	longRunning, persistentBlocked := buildPersistenceFindings(timelines, threshold)
	latencySections := buildLatencySections(timelines, threshold)
	growingLocks := buildGrowingLockFindings(bundles, threshold)

	// Sort tables to mirror Python's post-build sort step.
	sortByDumpsDesc(longRunning)
	sortByDumpsDesc(persistentBlocked)
	sortByDumpsDesc(latencySections)
	sortGrowingLocks(growingLocks)

	jvmTables := buildJVMMetadataTables(bundles, topN)
	jvmFindings := buildJVMMetadataFindings(jvmTables, topN)
	heuristicFindings := buildHeuristicFindings(bundles)

	findingsPayload := assembleFindings(
		heuristicFindings,
		longRunning,
		persistentBlocked,
		latencySections,
		growingLocks,
		jvmFindings,
		threshold,
	)

	totalObservations := 0
	for _, t := range timelines {
		totalObservations += len(t.observations)
	}

	summary := map[string]any{
		"total_dumps":                    len(bundles),
		"languages_detected":             languages,
		"source_formats":                 formats,
		"unique_threads":                 len(timelines),
		"total_thread_observations":      totalObservations,
		"long_running_threads":           len(longRunning),
		"persistent_blocked_threads":     len(persistentBlocked),
		"latency_sections":               len(latencySections),
		"growing_lock_contention":        len(growingLocks),
		"virtual_thread_carrier_pinning": len(jvmTables.carrierPinning),
		"smr_unresolved_threads":         len(jvmTables.smrUnresolved),
		"native_method_threads":          len(jvmTables.nativeMethods),
		"class_histogram_classes":        len(jvmTables.histogramRows),
		"class_histogram_incomplete":     len(jvmTables.histogramIncomplete),
		"consecutive_dump_threshold":     threshold,
	}

	series := map[string]any{
		"thread_persistence":          threadPersistenceSeries(timelines, topN),
		"state_distribution_per_dump": stateDistributionPerDump(bundles),
		"state_transition_timeline":   stateTransitionTimeline(timelines, topN),
	}

	tables := map[string]any{
		"long_running_stacks":             limitRows(longRunning, topN),
		"persistent_blocked_threads":      limitRows(persistentBlocked, topN),
		"latency_sections":                limitRows(latencySections, topN),
		"growing_lock_contention":         limitRows(growingLocks, topN),
		"dumps":                           dumpsTable(bundles),
		"virtual_thread_carrier_pinning":  jvmTables.carrierPinning,
		"smr_unresolved_threads":          jvmTables.smrUnresolved,
		"native_method_threads":           jvmTables.nativeMethods,
		"class_histogram_top_classes":     jvmTables.histogramRows,
		"class_histogram_incomplete":      jvmTables.histogramIncomplete,
	}

	result := models.New(ResultType, ParserName)
	result.SourceFiles = sourceFiles(bundles)
	result.Summary = summary
	result.Series = series
	result.Tables = tables
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = buildDiagnostics(bundles)
	result.Metadata.Findings = findingsPayload
	return result, nil
}

// buildDiagnostics mirrors the Python `metadata["diagnostics"]` block:
// the multi-dump analyzer doesn't re-parse, so it surfaces the union
// of parsed records across bundles and zeroes the rest. Format is
// left as the analyzer's parser name to match the Python literal.
func buildDiagnostics(bundles []models.ThreadDumpBundle) *diagnostics.ParserDiagnostics {
	d := diagnostics.New(ParserName)
	parsed := 0
	for _, b := range bundles {
		parsed += len(b.Snapshots)
	}
	d.ParsedRecords = parsed
	return d
}

func sourceFiles(bundles []models.ThreadDumpBundle) []string {
	out := make([]string, 0, len(bundles))
	for _, b := range bundles {
		out = append(out, b.SourceFile)
	}
	return out
}

// uniqueSorted returns the sorted, de-duplicated set of values produced
// by `pick(b)` across `bundles`.
func uniqueSorted(bundles []models.ThreadDumpBundle, pick func(models.ThreadDumpBundle) string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, b := range bundles {
		v := pick(b)
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// limitRows returns rows[:n] when n < len(rows), or rows itself when
// shorter. Matches Python `[:top_n]` slicing.
func limitRows[T any](rows []T, n int) []T {
	if n <= 0 || len(rows) <= n {
		return rows
	}
	return rows[:n]
}

// sortByDumpsDesc sorts persistence findings by their `dumps` integer
// descending. Matches Python `sort(key=lambda f: f["dumps"], reverse=True)`.
func sortByDumpsDesc(rows []map[string]any) {
	sort.SliceStable(rows, func(i, j int) bool {
		return asInt(rows[i]["dumps"]) > asInt(rows[j]["dumps"])
	})
}

// sortGrowingLocks sorts growing-lock entries: max_waiters DESC,
// consecutive_dumps DESC. Matches the Python tuple key
// `(-max_waiters, -consecutive_dumps)`.
func sortGrowingLocks(rows []map[string]any) {
	sort.SliceStable(rows, func(i, j int) bool {
		a := asInt(rows[i]["max_waiters"])
		b := asInt(rows[j]["max_waiters"])
		if a != b {
			return a > b
		}
		return asInt(rows[i]["consecutive_dumps"]) > asInt(rows[j]["consecutive_dumps"])
	})
}

// dumpsTable mirrors the per-bundle metadata table emitted under
// `tables["dumps"]`.
func dumpsTable(bundles []models.ThreadDumpBundle) []map[string]any {
	out := make([]map[string]any, 0, len(bundles))
	for _, b := range bundles {
		row := map[string]any{
			"dump_index":    b.DumpIndex,
			"dump_label":    derefString(b.DumpLabel),
			"source_file":   b.SourceFile,
			"source_format": b.SourceFormat,
			"language":      b.Language,
			"thread_count":  len(b.Snapshots),
			"start_line":    b.Metadata["start_line"],
			"end_line":      b.Metadata["end_line"],
			"raw_timestamp": b.Metadata["raw_timestamp"],
		}
		out = append(out, row)
	}
	return out
}

// stateDistributionPerDump mirrors Python `_state_distribution_per_dump`.
// Each row is `{dump_index, dump_label, counts: {state: count}}`.
func stateDistributionPerDump(bundles []models.ThreadDumpBundle) []map[string]any {
	out := make([]map[string]any, 0, len(bundles))
	for _, b := range bundles {
		counts := map[string]int{}
		for _, s := range b.Snapshots {
			counts[string(s.State)]++
		}
		out = append(out, map[string]any{
			"dump_index": b.DumpIndex,
			"dump_label": derefString(b.DumpLabel),
			"counts":     counts,
		})
	}
	return out
}

// threadPersistenceSeries mirrors Python `series["thread_persistence"]`.
// Sorted by observation count DESC, capped at topN. Ties are broken by
// thread name ASC for determinism — Python relies on insertion order
// which is also deterministic, but lex ordering is the safe Go-side
// equivalent that doesn't depend on map iteration.
func threadPersistenceSeries(timelines map[string]*threadTimeline, topN int) []map[string]any {
	tls := timelinesByObservationCount(timelines)
	limit := topN
	if limit <= 0 || limit > len(tls) {
		limit = len(tls)
	}
	out := make([]map[string]any, 0, limit)
	for _, t := range tls[:limit] {
		out = append(out, map[string]any{
			"thread_name":       t.threadName,
			"observed_in_dumps": len(t.observations),
		})
	}
	return out
}

// stateTransitionTimeline mirrors Python `_state_transition_timeline`.
// One row per (top-N) thread; transitions are the sorted observation
// list rendered as `{dump_index, state, stack_signature}`.
func stateTransitionTimeline(timelines map[string]*threadTimeline, topN int) []map[string]any {
	tls := timelinesByObservationCount(timelines)
	limit := topN
	if limit <= 0 || limit > len(tls) {
		limit = len(tls)
	}
	out := make([]map[string]any, 0, limit)
	for _, t := range tls[:limit] {
		obs := t.sortedObservations()
		transitions := make([]map[string]any, 0, len(obs))
		for _, o := range obs {
			transitions = append(transitions, map[string]any{
				"dump_index":      o.dumpIndex,
				"state":           string(o.state),
				"stack_signature": o.stackSignature,
			})
		}
		out = append(out, map[string]any{
			"thread_name": t.threadName,
			"transitions": transitions,
		})
	}
	return out
}

// timelinesByObservationCount returns timeline pointers sorted by
// `len(observations)` DESC then thread_name ASC.
func timelinesByObservationCount(timelines map[string]*threadTimeline) []*threadTimeline {
	out := make([]*threadTimeline, 0, len(timelines))
	for _, t := range timelines {
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := len(out[i].observations), len(out[j].observations)
		if ai != aj {
			return ai > aj
		}
		return out[i].threadName < out[j].threadName
	})
	return out
}

func derefString(p *string) any {
	if p == nil {
		return nil
	}
	return *p
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

// formatThreadName mirrors Python's `repr(name)` for typical thread
// names (no embedded single quotes / backslashes / control chars).
// Python's `f"... {name!r} ..."` produces a single-quoted form by
// default; we mimic that so messages line up with the Python emitter.
// Names with embedded single quotes or backslashes get escaped with
// `\` to keep round-tripping safe.
func formatThreadName(name string) string {
	// Python repr defaults to single quotes when the string contains
	// no single quote, otherwise it switches to double quotes when no
	// double quotes are present, otherwise it escapes single quotes.
	// We approximate the common case (single quotes) — the analyzer
	// only renders messages, no parser depends on the exact form.
	hasSingle := false
	hasDouble := false
	for _, r := range name {
		if r == '\'' {
			hasSingle = true
		}
		if r == '"' {
			hasDouble = true
		}
	}
	if hasSingle && !hasDouble {
		return fmt.Sprintf("\"%s\"", name)
	}
	if hasSingle {
		// Both quote kinds present — escape single quotes and wrap.
		escaped := ""
		for _, r := range name {
			if r == '\'' || r == '\\' {
				escaped += "\\"
			}
			escaped += string(r)
		}
		return "'" + escaped + "'"
	}
	return "'" + name + "'"
}
