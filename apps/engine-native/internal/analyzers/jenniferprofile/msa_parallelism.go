package jenniferprofile

import (
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// interval is a half-open [start, end) window in ms-since-body-start.
// All §16/§25 algorithms operate on these.
type interval struct {
	start int
	end   int
}

// extractExternalCallIntervals walks one profile's body events,
// pulls every EXTERNAL_CALL with a usable startOffset/elapsed, and
// returns its [start, end) interval list. Events missing offsets
// are skipped (rare — happens when body START_TIME failed to parse).
func extractExternalCallIntervals(p *models.JenniferTransactionProfile) []interval {
	out := make([]interval, 0, len(p.Body.Events))
	for i := range p.Body.Events {
		ev := &p.Body.Events[i]
		if ev.EventType != models.JenniferEventExternalCall {
			continue
		}
		if ev.StartOffsetMs == nil || ev.ElapsedMs == nil {
			continue
		}
		out = append(out, interval{
			start: *ev.StartOffsetMs,
			end:   *ev.StartOffsetMs + *ev.ElapsedMs,
		})
	}
	return out
}

// computeProfileParallelism is the §16.5-§16.7 per-profile envelope.
// Returns NONE/SEQUENTIAL/PARALLEL/MIXED + cumulative/wall-clock
// split + max concurrency + parallelism ratio.
func computeProfileParallelism(p *models.JenniferTransactionProfile) models.JenniferProfileParallelism {
	out := models.JenniferProfileParallelism{
		TXID:          p.Header.TXID,
		ExecutionMode: models.JenniferExecNone,
	}
	intervals := extractExternalCallIntervals(p)
	out.ExternalCallCount = len(intervals)
	if len(intervals) == 0 {
		return out
	}

	cumMs := 0
	for _, iv := range intervals {
		cumMs += iv.end - iv.start
	}
	out.ExternalCallCumulativeMs = cumMs
	out.ExternalCallWallTimeMs = unionDuration(intervals)
	if out.ExternalCallWallTimeMs > 0 {
		out.ParallelismRatio = float64(out.ExternalCallCumulativeMs) / float64(out.ExternalCallWallTimeMs)
	} else {
		out.ParallelismRatio = 1
	}
	out.MaxExternalCallConcurrency = maxConcurrency(intervals)
	out.ExecutionMode = detectExecutionMode(intervals)
	return out
}

// detectExecutionMode implements §16.5 strict-strict overlap rule:
//
//	a.start < b.end && b.start < a.end
//
// SEQUENTIAL = no pair overlaps.
// PARALLEL   = every interval overlaps at least one other.
// MIXED      = some overlap, some don't.
func detectExecutionMode(intervals []interval) models.JenniferExecutionMode {
	if len(intervals) == 0 {
		return models.JenniferExecNone
	}
	if len(intervals) == 1 {
		return models.JenniferExecSequential
	}
	overlaps := make([]int, len(intervals))
	hasAny := false
	for i := range intervals {
		for j := range intervals {
			if i == j {
				continue
			}
			if intervals[i].start < intervals[j].end && intervals[j].start < intervals[i].end {
				overlaps[i]++
				hasAny = true
			}
		}
	}
	if !hasAny {
		return models.JenniferExecSequential
	}
	allOverlap := true
	for _, c := range overlaps {
		if c == 0 {
			allOverlap = false
			break
		}
	}
	if allOverlap {
		return models.JenniferExecParallel
	}
	return models.JenniferExecMixed
}

// unionDuration sums the merged-interval lengths per §25.5. Sorts
// by start, walks merging adjacent overlapping ranges, then sums
// each merged segment's length.
func unionDuration(intervals []interval) int {
	if len(intervals) == 0 {
		return 0
	}
	sorted := make([]interval, len(intervals))
	copy(sorted, intervals)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].start != sorted[j].start {
			return sorted[i].start < sorted[j].start
		}
		return sorted[i].end < sorted[j].end
	})
	total := 0
	current := sorted[0]
	for _, iv := range sorted[1:] {
		if iv.start <= current.end {
			if iv.end > current.end {
				current.end = iv.end
			}
			continue
		}
		total += current.end - current.start
		current = iv
	}
	total += current.end - current.start
	return total
}

// maxConcurrency returns the peak number of overlapping intervals
// at any instant. Sweep-line: for each unique starting boundary,
// count how many intervals contain it. Strict-strict overlap rule
// matches detectExecutionMode.
func maxConcurrency(intervals []interval) int {
	if len(intervals) == 0 {
		return 0
	}
	// Snapshot at each interval's start. Active = intervals i where
	// i.start <= start && start < i.end.
	max := 0
	for i := range intervals {
		s := intervals[i].start
		count := 0
		for j := range intervals {
			if intervals[j].start <= s && s < intervals[j].end {
				count++
			}
		}
		if count > max {
			max = count
		}
	}
	return max
}

// computeGroupParallelism rolls per-profile parallelism into the
// GUID-group envelope. Group-level wall-clock is the union of all
// matched-edge intervals across all profiles in the group; group
// cumulative is just the sum of profile cumulatives. Group execution
// mode is the worst-case across profiles (PARALLEL > MIXED >
// SEQUENTIAL > NONE).
func computeGroupParallelism(profiles []models.JenniferTransactionProfile) (
	totalCum int,
	totalWall int,
	maxConc int,
	ratio float64,
	mode models.JenniferExecutionMode,
	perProfile []models.JenniferProfileParallelism,
) {
	mode = models.JenniferExecNone
	for _, p := range profiles {
		pp := computeProfileParallelism(&p)
		perProfile = append(perProfile, pp)
		totalCum += pp.ExternalCallCumulativeMs
		totalWall += pp.ExternalCallWallTimeMs
		if pp.MaxExternalCallConcurrency > maxConc {
			maxConc = pp.MaxExternalCallConcurrency
		}
		mode = mergeExecutionMode(mode, pp.ExecutionMode)
	}
	if totalWall > 0 {
		ratio = float64(totalCum) / float64(totalWall)
	} else {
		ratio = 1
	}
	return
}

// mergeExecutionMode picks the "worst" of two modes. The ordering
// reflects how the renderer surfaces them: PARALLEL/MIXED imply
// some calls overlapped, which is what the user wants to spot.
func mergeExecutionMode(a, b models.JenniferExecutionMode) models.JenniferExecutionMode {
	rank := map[models.JenniferExecutionMode]int{
		models.JenniferExecNone:       0,
		models.JenniferExecSequential: 1,
		models.JenniferExecMixed:      2,
		models.JenniferExecParallel:   3,
	}
	if rank[a] >= rank[b] {
		return a
	}
	return b
}
