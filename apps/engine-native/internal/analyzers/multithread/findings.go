package multithread

import (
	"fmt"
	"math"
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Per-bundle ratio thresholds for the heuristic findings. Mirror the
// Python module-level constants verbatim.
const (
	congestionWaitingRatio        = 0.10
	externalResourceSleepingRatio = 0.25
	gcPauseUnownedBlockRatio      = 0.50
)

// roundToEven rounds to `decimals` digits using banker's rounding.
// Matches Python's built-in `round()` (and `math.RoundToEven` on the
// Go side) so the emitted JSON ratios stay identical across engines.
func roundToEven(value float64, decimals int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	pow := math.Pow(10, float64(decimals))
	return math.RoundToEven(value*pow) / pow
}

// buildPersistenceFindings emits the LONG_RUNNING_THREAD evidence
// rows and PERSISTENT_BLOCKED_THREAD evidence rows. Mirrors Python's
// `_build_findings`.
func buildPersistenceFindings(
	timelines map[string]*threadTimeline,
	threshold int,
) (longRunning []map[string]any, persistentBlocked []map[string]any) {
	// Sort timeline names so the output order is deterministic when
	// multiple threads tie. Python relies on dict insertion order;
	// we lex-sort for portability.
	names := make([]string, 0, len(timelines))
	for name := range timelines {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		timeline := timelines[name]

		// LONG_RUNNING — same RUNNABLE stack across `threshold`
		// consecutive dumps. Pythonemits at most one finding per thread
		// (`break` after the first matching signature).
		runnable := filterObservations(timeline, runnableStates)
		if len(runnable) > 0 {
			signatures, runLengths, firstIndex := runnableRunLengths(runnable)
			for _, sig := range signatures {
				run := runLengths[sig]
				if run >= threshold {
					longRunning = append(longRunning, map[string]any{
						"thread_name":      timeline.threadName,
						"stack_signature":  sig,
						"dumps":            run,
						"first_dump_index": firstIndex[sig],
					})
					break
				}
			}
		}

		// PERSISTENT_BLOCKED — ≥threshold consecutive BLOCKED/LOCK_WAIT.
		blockedRun := timeline.stateRunLengths(blockedStates)
		if blockedRun >= threshold {
			blocked := filterObservations(timeline, blockedStates)
			persistentBlocked = append(persistentBlocked, map[string]any{
				"thread_name":      timeline.threadName,
				"dumps":            blockedRun,
				"first_dump_index": minDumpIndex(blocked),
				"stack_signatures": uniqueSortedSignatures(blocked),
			})
		}
	}
	return
}

// filterObservations returns the timeline's sorted observations
// restricted to the subset whose state belongs to `states`.
func filterObservations(timeline *threadTimeline, states map[models.ThreadState]struct{}) []threadObservation {
	sorted := timeline.sortedObservations()
	out := sorted[:0:0]
	for _, obs := range sorted {
		if _, ok := states[obs.state]; ok {
			out = append(out, obs)
		}
	}
	return out
}

// runnableRunLengths walks the runnable observation list once,
// recording (a) the longest run-length per stack signature, (b) the
// signatures in the order they first hit their max run, and (c) the
// minimum dump_index per signature. Mirrors the Python loop in
// `_build_findings`.
//
// Returning an ordered signature slice + the per-signature first-index
// map preserves Python semantics: the first signature whose run hits
// the threshold wins. Without preserving order the resulting finding
// would depend on map iteration.
func runnableRunLengths(runnable []threadObservation) (
	orderedSignatures []string,
	runLengths map[string]int,
	firstIndex map[string]int,
) {
	runLengths = map[string]int{}
	firstIndex = map[string]int{}
	prevIndex := -1
	hasPrev := false
	currentSig := ""
	currentRun := 0
	seenOrder := map[string]struct{}{}

	for _, obs := range runnable {
		consecutive := hasPrev && obs.dumpIndex == prevIndex+1 && obs.stackSignature == currentSig
		if consecutive {
			currentRun++
		} else {
			currentSig = obs.stackSignature
			currentRun = 1
		}
		if existing, ok := runLengths[currentSig]; !ok || currentRun > existing {
			runLengths[currentSig] = currentRun
		}
		if _, ok := seenOrder[currentSig]; !ok {
			seenOrder[currentSig] = struct{}{}
			orderedSignatures = append(orderedSignatures, currentSig)
		}
		// First-index per signature: track the earliest dump_index
		// across all matching observations (Python uses
		// `min(obs.dump_index for obs in runnable_obs if sig matches)`).
		if cur, ok := firstIndex[currentSig]; !ok || obs.dumpIndex < cur {
			firstIndex[currentSig] = obs.dumpIndex
		}
		prevIndex = obs.dumpIndex
		hasPrev = true
	}
	return
}

func minDumpIndex(obs []threadObservation) int {
	if len(obs) == 0 {
		return 0
	}
	min := obs[0].dumpIndex
	for _, o := range obs[1:] {
		if o.dumpIndex < min {
			min = o.dumpIndex
		}
	}
	return min
}

func uniqueSortedSignatures(obs []threadObservation) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, o := range obs {
		if _, ok := seen[o.stackSignature]; ok {
			continue
		}
		seen[o.stackSignature] = struct{}{}
		out = append(out, o.stackSignature)
	}
	sort.Strings(out)
	return out
}

// buildLatencySections detects threads that linger in NETWORK_WAIT,
// IO_WAIT, or CHANNEL_WAIT for `threshold` consecutive dumps. Mirrors
// Python's `_build_latency_sections`.
func buildLatencySections(timelines map[string]*threadTimeline, threshold int) []map[string]any {
	out := []map[string]any{}
	// Walk threads in lex order so output is deterministic.
	names := make([]string, 0, len(timelines))
	for name := range timelines {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		timeline := timelines[name]
		sortedObs := timeline.sortedObservations()
		for _, category := range latencyWaitCategories {
			longestRun := 0
			currentRun := 0
			currentSignatures := []string{}
			bestSignatures := []string{}
			bestFirstIndex := -1
			currentFirstIndex := -1
			prevIndex := -1
			hasPrev := false

			for _, obs := range sortedObs {
				inCategory := obs.state == category
				consecutive := inCategory && hasPrev && obs.dumpIndex == prevIndex+1
				switch {
				case inCategory && !consecutive:
					currentRun = 1
					currentSignatures = []string{obs.stackSignature}
					currentFirstIndex = obs.dumpIndex
				case inCategory && consecutive:
					currentRun++
					currentSignatures = append(currentSignatures, obs.stackSignature)
				default:
					currentRun = 0
					currentSignatures = nil
					currentFirstIndex = -1
				}
				if currentRun > longestRun {
					longestRun = currentRun
					bestSignatures = append(bestSignatures[:0:0], currentSignatures...)
					bestFirstIndex = currentFirstIndex
				}
				prevIndex = obs.dumpIndex
				hasPrev = true
			}

			if longestRun >= threshold && bestFirstIndex >= 0 {
				out = append(out, map[string]any{
					"thread_name":      timeline.threadName,
					"wait_category":    string(category),
					"dumps":            longestRun,
					"first_dump_index": bestFirstIndex,
					"stack_signatures": dedupSorted(bestSignatures),
				})
			}
		}
	}
	return out
}

func dedupSorted(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// buildGrowingLockFindings detects locks whose waiter count strictly
// increases across consecutive dumps. Mirrors Python's
// `_build_growing_lock_findings`.
//
// Skips wait_mode in {object_wait, parking_condition_wait} — those are
// `Object.wait()` / parked-on-condition waits, not lock contention.
func buildGrowingLockFindings(bundles []models.ThreadDumpBundle, threshold int) []map[string]any {
	type perDumpEntry struct {
		index int
		count int
	}
	waitersByLock := map[string]map[int]int{}
	classesByLock := map[string]*string{}

	for _, bundle := range bundles {
		perDumpCounts := map[string]map[string]struct{}{}
		perDumpClasses := map[string]*string{}
		for _, snapshot := range bundle.Snapshots {
			waiting := snapshot.LockWaiting
			if waiting == nil {
				continue
			}
			if waiting.WaitMode == "object_wait" || waiting.WaitMode == "parking_condition_wait" {
				continue
			}
			set, ok := perDumpCounts[waiting.LockID]
			if !ok {
				set = map[string]struct{}{}
				perDumpCounts[waiting.LockID] = set
			}
			set[snapshot.ThreadName] = struct{}{}
			if _, ok := perDumpClasses[waiting.LockID]; !ok {
				perDumpClasses[waiting.LockID] = waiting.LockClass
			}
		}
		for lockID, names := range perDumpCounts {
			if _, ok := waitersByLock[lockID]; !ok {
				waitersByLock[lockID] = map[int]int{}
			}
			waitersByLock[lockID][bundle.DumpIndex] = len(names)
			if _, ok := classesByLock[lockID]; !ok {
				classesByLock[lockID] = perDumpClasses[lockID]
			}
		}
	}

	// Walk locks in deterministic lex order.
	lockIDs := make([]string, 0, len(waitersByLock))
	for id := range waitersByLock {
		lockIDs = append(lockIDs, id)
	}
	sort.Strings(lockIDs)

	out := []map[string]any{}
	for _, lockID := range lockIDs {
		perDump := waitersByLock[lockID]
		ordered := make([]perDumpEntry, 0, len(perDump))
		for idx, count := range perDump {
			ordered = append(ordered, perDumpEntry{idx, count})
		}
		sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].index < ordered[j].index })
		if len(ordered) < threshold {
			continue
		}

		longestRun, longestMax := 0, 0
		currentRun, currentMax := 0, 0
		prevIndex, prevCount := -1, -1
		hasPrev := false
		for _, entry := range ordered {
			if hasPrev && entry.index == prevIndex+1 && entry.count > prevCount {
				currentRun++
			} else {
				currentRun = 1
			}
			if entry.count > currentMax {
				currentMax = entry.count
			}
			if currentRun > longestRun {
				longestRun = currentRun
				longestMax = currentMax
			}
			prevIndex = entry.index
			prevCount = entry.count
			hasPrev = true
		}

		if longestRun >= threshold {
			row := map[string]any{
				"lock_id":           lockID,
				"lock_class":        derefString(classesByLock[lockID]),
				"consecutive_dumps": longestRun,
				"max_waiters":       longestMax,
			}
			out = append(out, row)
		}
	}
	return out
}

// buildHeuristicFindings emits the per-bundle ratio findings:
// THREAD_CONGESTION_DETECTED, EXTERNAL_RESOURCE_WAIT_HIGH,
// LIKELY_GC_PAUSE_DETECTED. Mirrors Python's
// `_build_heuristic_findings`.
//
// The Python code iterates `enumerate(bundles)` so `index` is the
// position in the bundle list, not the bundle's DumpIndex. We mirror
// that exactly (Python: `for index, bundle in enumerate(bundles)`).
func buildHeuristicFindings(bundles []models.ThreadDumpBundle) []map[string]any {
	out := []map[string]any{}
	for index, bundle := range bundles {
		total := len(bundle.Snapshots)
		if total == 0 {
			continue
		}

		waiting := 0
		sleeping := 0
		heldLocks := map[string]struct{}{}
		for _, s := range bundle.Snapshots {
			switch s.State {
			case models.ThreadStateWaiting, models.ThreadStateLockWait, models.ThreadStateBlocked:
				waiting++
			}
			if s.State == models.ThreadStateTimedWaiting {
				sleeping++
			}
			for _, handle := range s.LockHolds {
				if handle.LockID != "" {
					heldLocks[handle.LockID] = struct{}{}
				}
			}
		}

		unownedBlocked := 0
		for _, s := range bundle.Snapshots {
			if s.State != models.ThreadStateBlocked && s.State != models.ThreadStateLockWait {
				continue
			}
			target := s.LockWaiting
			if target == nil {
				continue
			}
			if target.LockID == "" {
				continue
			}
			if _, owned := heldLocks[target.LockID]; !owned {
				unownedBlocked++
			}
		}

		waitingRatio := float64(waiting) / float64(total)
		sleepingRatio := float64(sleeping) / float64(total)
		unownedRatio := float64(unownedBlocked) / float64(total)

		if waitingRatio > congestionWaitingRatio {
			out = append(out, map[string]any{
				"severity": "warning",
				"code":     "THREAD_CONGESTION_DETECTED",
				"message": fmt.Sprintf(
					"Dump #%d: %.1f%% of threads are waiting for a monitor — "+
						"possible congestion or upstream deadlock.",
					index, waitingRatio*100,
				),
				"evidence": map[string]any{
					"dump_index":      index,
					"source_file":     bundle.SourceFile,
					"waiting_threads": waiting,
					"total_threads":   total,
					"waiting_ratio":   roundToEven(waitingRatio, 4),
				},
				"thresholds": map[string]any{"waiting_ratio": congestionWaitingRatio},
			})
		}
		if sleepingRatio > externalResourceSleepingRatio {
			out = append(out, map[string]any{
				"severity": "info",
				"code":     "EXTERNAL_RESOURCE_WAIT_HIGH",
				"message": fmt.Sprintf(
					"Dump #%d: %.1f%% of threads are sleeping on a monitor — "+
						"likely waiting on an external resource (DB, network, "+
						"or idle worker pool).",
					index, sleepingRatio*100,
				),
				"evidence": map[string]any{
					"dump_index":       index,
					"source_file":      bundle.SourceFile,
					"sleeping_threads": sleeping,
					"total_threads":    total,
					"sleeping_ratio":   roundToEven(sleepingRatio, 4),
				},
				"thresholds": map[string]any{"sleeping_ratio": externalResourceSleepingRatio},
			})
		}
		if unownedRatio > gcPauseUnownedBlockRatio {
			out = append(out, map[string]any{
				"severity": "warning",
				"code":     "LIKELY_GC_PAUSE_DETECTED",
				"message": fmt.Sprintf(
					"Dump #%d: %.1f%% of threads are blocked on monitors with "+
						"no application owner — strongly suggests a GC pause.",
					index, unownedRatio*100,
				),
				"evidence": map[string]any{
					"dump_index":              index,
					"source_file":             bundle.SourceFile,
					"unowned_blocked_threads": unownedBlocked,
					"total_threads":           total,
					"unowned_block_ratio":     roundToEven(unownedRatio, 4),
				},
				"thresholds": map[string]any{"unowned_block_ratio": gcPauseUnownedBlockRatio},
			})
		}
	}
	return out
}

// assembleFindings joins the per-finding lists in the canonical order
// (heuristics → long_running → persistent_blocked → latency →
// growing_locks → JVM metadata) and wraps each entry in the
// {severity, code, message, evidence, thresholds?} envelope. Mirrors
// Python's `_findings_payload`.
func assembleFindings(
	heuristic []map[string]any,
	longRunning []map[string]any,
	persistentBlocked []map[string]any,
	latency []map[string]any,
	growingLocks []map[string]any,
	jvm []map[string]any,
	threshold int,
) []map[string]any {
	out := []map[string]any{}
	out = append(out, heuristic...)

	for _, entry := range longRunning {
		out = append(out, map[string]any{
			"severity": "warning",
			"code":     "LONG_RUNNING_THREAD",
			"message": fmt.Sprintf(
				"Thread %s stayed RUNNABLE on the same stack for %d consecutive dumps.",
				formatThreadName(asString(entry["thread_name"])),
				asInt(entry["dumps"]),
			),
			"evidence":   entry,
			"thresholds": map[string]any{"consecutive_dumps": threshold},
		})
	}

	for _, entry := range persistentBlocked {
		out = append(out, map[string]any{
			"severity": "critical",
			"code":     "PERSISTENT_BLOCKED_THREAD",
			"message": fmt.Sprintf(
				"Thread %s stayed BLOCKED for %d consecutive dumps.",
				formatThreadName(asString(entry["thread_name"])),
				asInt(entry["dumps"]),
			),
			"evidence":   entry,
			"thresholds": map[string]any{"consecutive_dumps": threshold},
		})
	}

	for _, entry := range latency {
		out = append(out, map[string]any{
			"severity": "warning",
			"code":     "LATENCY_SECTION_DETECTED",
			"message": fmt.Sprintf(
				"Thread %s stayed in %s for %d consecutive dumps.",
				formatThreadName(asString(entry["thread_name"])),
				asString(entry["wait_category"]),
				asInt(entry["dumps"]),
			),
			"evidence":   entry,
			"thresholds": map[string]any{"consecutive_dumps": threshold},
		})
	}

	for _, entry := range growingLocks {
		out = append(out, map[string]any{
			"severity": "warning",
			"code":     "GROWING_LOCK_CONTENTION",
			"message": fmt.Sprintf(
				"Lock %s waiter count grew strictly across %d consecutive dumps "+
					"(max waiters: %d).",
				asString(entry["lock_id"]),
				asInt(entry["consecutive_dumps"]),
				asInt(entry["max_waiters"]),
			),
			"evidence":   entry,
			"thresholds": map[string]any{"consecutive_dumps": threshold},
		})
	}

	out = append(out, jvm...)
	return out
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
