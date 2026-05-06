// Package lockcontention ports archscope_engine.analyzers.lock_contention_analyzer.
//
// The analyzer consumes one or more ThreadDumpBundle values and emits an
// AnalysisResult of type "thread_dump_locks" describing:
//
//   - per-lock owner / waiter rows (lock_id, lock_class, owner thread,
//     waiter count, top waiter names, owner stack signature),
//   - a contention ranking sorted by waiter count
//     (LOCK_CONTENTION_HOTSPOT findings),
//   - a waits-for graph cycle scan with one DEADLOCK_DETECTED finding
//     per cycle (DFS-based, deterministic ordering).
//
// Java jstack today is the only parser that fills the
// lock_holds/lock_waiting fields, so this analyzer is JVM-driven in
// practice. Other runtimes simply yield empty fields and the contention
// table comes back empty.
package lockcontention

import (
	"errors"
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const (
	// DefaultTopN matches the Python `top_n=20` default. Caps every
	// table the renderer consumes.
	DefaultTopN = 20
	// SchemaVersion mirrors Python `schema_version="0.1.0"`.
	SchemaVersion = "0.1.0"
	// ParserName mirrors Python `metadata.parser="thread_dump_locks"`.
	ParserName = "thread_dump_locks"
	// ResultType mirrors Python `AnalysisResult.type="thread_dump_locks"`.
	ResultType = "thread_dump_locks"
)

// Wait modes that are treated as cooperative waits (Object.wait /
// Condition.await), not contention. Mirrors the Python set used in
// both the contention-candidate filter and the deadlock graph.
var cooperativeWaitModes = map[string]struct{}{
	"object_wait":            {},
	"parking_condition_wait": {},
}

// Options matches the Python keyword arguments of analyze_lock_contention.
// Zero values fall back to the canonical Python defaults.
type Options struct {
	// TopN caps every table emitted in the AnalysisResult. Defaults to
	// DefaultTopN when ≤0.
	TopN int
}

// ErrNoBundles mirrors Python `ValueError("analyze_lock_contention
// requires at least one bundle.")`.
var ErrNoBundles = errors.New("lockcontention: analyze requires at least one bundle")

// Analyze unions snapshots from `bundles` and returns the populated
// AnalysisResult. Mirrors `analyze_lock_contention`.
func Analyze(bundles []models.ThreadDumpBundle, opts Options) (models.AnalysisResult, error) {
	if len(bundles) == 0 {
		return models.AnalysisResult{}, ErrNoBundles
	}
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}

	snapshots := make([]models.ThreadSnapshot, 0)
	for _, b := range bundles {
		snapshots = append(snapshots, b.Snapshots...)
	}

	lockClassByID := map[string]*string{}
	ownersByLock := map[string][]string{}
	waitersByLock := map[string][]string{}
	waitModesByLock := map[string][]string{}
	threadsWithLocks := 0
	threadsWaiting := 0

	for _, snap := range snapshots {
		if len(snap.LockHolds) > 0 {
			threadsWithLocks++
		}
		if snap.LockWaiting != nil {
			threadsWaiting++
		}
		for _, hold := range snap.LockHolds {
			ownersByLock[hold.LockID] = append(ownersByLock[hold.LockID], snap.ThreadName)
			rememberClass(lockClassByID, hold)
		}
		if snap.LockWaiting != nil {
			waiting := *snap.LockWaiting
			waitersByLock[waiting.LockID] = append(waitersByLock[waiting.LockID], snap.ThreadName)
			mode := waiting.WaitMode
			if mode == "" {
				mode = "unknown_lock_wait"
			}
			waitModesByLock[waiting.LockID] = append(waitModesByLock[waiting.LockID], mode)
			rememberClass(lockClassByID, waiting)
		}
	}

	snapshotsByName := map[string]*models.ThreadSnapshot{}
	for i := range snapshots {
		// First-seen wins, matching `{s.thread_name: s for s in snapshots}`
		// dict-comprehension semantics where later keys overwrite earlier
		// ones — Python's behaviour is "last wins" but in practice the
		// Python code only relies on this for the owner stack signature
		// and the union case wouldn't disagree. Mirror the dict-comp.
		snapshotsByName[snapshots[i].ThreadName] = &snapshots[i]
	}

	contentionRows := contentionTable(
		ownersByLock,
		waitersByLock,
		waitModesByLock,
		lockClassByID,
		snapshotsByName,
	)

	hotspots := make([]map[string]any, 0, len(contentionRows))
	for _, row := range contentionRows {
		if asInt(row["waiter_count"]) > 0 && asBool(row["contention_candidate"]) {
			hotspots = append(hotspots, row)
		}
	}

	deadlocks := detectDeadlocks(snapshots)
	findings := buildFindings(limitRows(hotspots, topN), deadlocks)

	languages := uniqueSorted(bundles, func(b models.ThreadDumpBundle) string { return b.Language })
	formats := uniqueSorted(bundles, func(b models.ThreadDumpBundle) string { return b.SourceFormat })

	summary := map[string]any{
		"total_dumps":              len(bundles),
		"total_thread_snapshots":   len(snapshots),
		"threads_with_locks":       threadsWithLocks,
		"threads_waiting_on_lock":  threadsWaiting,
		"unique_locks":             len(lockClassByID),
		"contended_locks":          len(hotspots),
		"deadlocks_detected":       len(deadlocks),
		"languages_detected":       languages,
		"source_formats":           formats,
	}

	contentionRanking := make([]map[string]any, 0, len(hotspots))
	for _, row := range limitRows(hotspots, topN) {
		contentionRanking = append(contentionRanking, map[string]any{
			"lock_id":      row["lock_id"],
			"lock_class":   row["lock_class"],
			"waiter_count": row["waiter_count"],
		})
	}

	series := map[string]any{
		"contention_ranking": contentionRanking,
	}

	tables := map[string]any{
		"locks":            limitRows(contentionRows, topN),
		"deadlock_chains":  deadlocks,
		"dumps":            dumpsTable(bundles),
	}

	result := models.New(ResultType, ParserName)
	result.SourceFiles = sourceFiles(bundles)
	result.Summary = summary
	result.Series = series
	result.Tables = tables
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = buildDiagnostics(snapshots)
	result.Metadata.Findings = findings
	return result, nil
}

// rememberClass mirrors Python `_remember_class`: first non-empty class
// hint wins, but every lock_id ends up with a key (possibly nil).
func rememberClass(lockClassByID map[string]*string, lock models.LockHandle) {
	existing, present := lockClassByID[lock.LockID]
	if lock.LockClass != nil && *lock.LockClass != "" && (!present || existing == nil) {
		lockClassByID[lock.LockID] = lock.LockClass
		return
	}
	if !present {
		lockClassByID[lock.LockID] = lock.LockClass
	}
}

// contentionTable mirrors Python `_contention_table`. Returns the rows
// sorted by `(-waiter_count, -owner_count, lock_id)`.
func contentionTable(
	ownersByLock map[string][]string,
	waitersByLock map[string][]string,
	waitModesByLock map[string][]string,
	lockClassByID map[string]*string,
	snapshotsByName map[string]*models.ThreadSnapshot,
) []map[string]any {
	allLockIDs := map[string]struct{}{}
	for k := range ownersByLock {
		allLockIDs[k] = struct{}{}
	}
	for k := range waitersByLock {
		allLockIDs[k] = struct{}{}
	}
	for k := range lockClassByID {
		allLockIDs[k] = struct{}{}
	}

	rows := make([]map[string]any, 0, len(allLockIDs))
	for lockID := range allLockIDs {
		owners := ownersByLock[lockID]
		waiters := waitersByLock[lockID]
		waitModeCounts := counterStrings(waitModesByLock[lockID])
		contentionCandidate := false
		for mode := range waitModeCounts {
			if _, isCooperative := cooperativeWaitModes[mode]; !isCooperative {
				contentionCandidate = true
				break
			}
		}

		var ownerName *string
		if len(owners) > 0 {
			top := mostCommonName(owners)
			ownerName = &top
		}
		var ownerSignature any
		if ownerName != nil {
			if snap, ok := snapshotsByName[*ownerName]; ok {
				ownerSignature = snap.StackSignature(0)
			}
		}

		var ownerThreadValue any
		if ownerName != nil {
			ownerThreadValue = *ownerName
		}

		var lockClassValue any
		if cls, ok := lockClassByID[lockID]; ok && cls != nil {
			lockClassValue = *cls
		}

		// `wait_mode_counts` payload mirrors `dict(Counter(...))` in
		// Python — emit map[string]int so JSON shapes match.
		modeCountsOut := map[string]int{}
		for k, v := range waitModeCounts {
			modeCountsOut[k] = v
		}

		row := map[string]any{
			"lock_id":               lockID,
			"lock_class":            lockClassValue,
			"owner_thread":          ownerThreadValue,
			"owner_stack_signature": ownerSignature,
			"owner_count":           uniqueCount(owners),
			"waiter_count":          uniqueCount(waiters),
			"wait_mode_counts":      modeCountsOut,
			"contention_candidate":  contentionCandidate,
			"top_waiters":           topNNames(waiters, 5),
			"all_waiters":           sortedUnique(waiters),
		}
		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		wi, wj := asInt(rows[i]["waiter_count"]), asInt(rows[j]["waiter_count"])
		if wi != wj {
			return wi > wj
		}
		oi, oj := asInt(rows[i]["owner_count"]), asInt(rows[j]["owner_count"])
		if oi != oj {
			return oi > oj
		}
		return asString(rows[i]["lock_id"]) < asString(rows[j]["lock_id"])
	})
	return rows
}

// dumpsTable mirrors `tables["dumps"]` from the Python emitter.
func dumpsTable(bundles []models.ThreadDumpBundle) []map[string]any {
	out := make([]map[string]any, 0, len(bundles))
	for _, b := range bundles {
		out = append(out, map[string]any{
			"dump_index":    b.DumpIndex,
			"dump_label":    derefString(b.DumpLabel),
			"source_file":   b.SourceFile,
			"source_format": b.SourceFormat,
			"language":      b.Language,
			"thread_count":  len(b.Snapshots),
		})
	}
	return out
}

// buildDiagnostics mirrors Python's diagnostics block: the lock-
// contention analyzer doesn't re-parse, so it surfaces parsed_records
// = total snapshot count and zeroes the rest.
func buildDiagnostics(snapshots []models.ThreadSnapshot) *diagnostics.ParserDiagnostics {
	d := diagnostics.New(ParserName)
	d.ParsedRecords = len(snapshots)
	return d
}

func sourceFiles(bundles []models.ThreadDumpBundle) []string {
	out := make([]string, 0, len(bundles))
	for _, b := range bundles {
		out = append(out, b.SourceFile)
	}
	return out
}

// ─── small typed helpers ────────────────────────────────────────────────

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

func limitRows[T any](rows []T, n int) []T {
	if n <= 0 || len(rows) <= n {
		return rows
	}
	return rows[:n]
}

// counterStrings is the equivalent of `collections.Counter(items)`.
func counterStrings(items []string) map[string]int {
	out := map[string]int{}
	for _, item := range items {
		out[item]++
	}
	return out
}

// uniqueCount returns the count of distinct entries in `items`.
func uniqueCount(items []string) int {
	seen := map[string]struct{}{}
	for _, item := range items {
		seen[item] = struct{}{}
	}
	return len(seen)
}

func sortedUnique(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

// mostCommonName returns the most-frequent name. Ties are broken
// lexicographically (Python's `Counter.most_common(1)` falls back to
// insertion order; lex order is the deterministic Go-side equivalent).
func mostCommonName(names []string) string {
	counts := counterStrings(names)
	type kv struct {
		key   string
		count int
	}
	rows := make([]kv, 0, len(counts))
	for k, v := range counts {
		rows = append(rows, kv{k, v})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].key < rows[j].key
	})
	if len(rows) == 0 {
		return ""
	}
	return rows[0].key
}

// topNNames returns up to `limit` distinct names ordered by frequency
// then lexicographically. Mirrors Python `Counter.most_common(limit)`
// projected to just the names.
func topNNames(names []string, limit int) []string {
	counts := counterStrings(names)
	type kv struct {
		key   string
		count int
	}
	rows := make([]kv, 0, len(counts))
	for k, v := range counts {
		rows = append(rows, kv{k, v})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].key < rows[j].key
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.key)
	}
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

func asBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

