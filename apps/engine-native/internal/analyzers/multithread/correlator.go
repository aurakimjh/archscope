package multithread

import (
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// threadObservation is the per-(dump, thread) tuple recorded by the
// correlator. Mirrors Python's `ThreadObservation` dataclass.
type threadObservation struct {
	dumpIndex      int
	state          models.ThreadState
	stackSignature string
}

// threadTimeline is the per-thread observation history. Mirrors
// Python's `ThreadTimeline` dataclass.
type threadTimeline struct {
	threadName   string
	observations []threadObservation
}

// sortedObservations returns observations sorted by dumpIndex ASC. The
// underlying slice is left untouched so callers can call this on the
// same timeline multiple times without re-sorting in place.
func (t *threadTimeline) sortedObservations() []threadObservation {
	out := make([]threadObservation, len(t.observations))
	copy(out, t.observations)
	sort.SliceStable(out, func(i, j int) bool { return out[i].dumpIndex < out[j].dumpIndex })
	return out
}

// stateRunLengths returns the longest consecutive-dump run where every
// observed state belongs to `targetStates`. Matches Python's
// `ThreadTimeline.state_run_lengths`.
func (t *threadTimeline) stateRunLengths(targetStates map[models.ThreadState]struct{}) int {
	longest := 0
	current := 0
	prevIndex := -1
	hasPrev := false
	for _, obs := range t.sortedObservations() {
		if _, ok := targetStates[obs.state]; !ok {
			current = 0
			prevIndex = obs.dumpIndex
			hasPrev = true
			continue
		}
		if !hasPrev || obs.dumpIndex == prevIndex+1 {
			current++
		} else {
			current = 1
		}
		if current > longest {
			longest = current
		}
		prevIndex = obs.dumpIndex
		hasPrev = true
	}
	return longest
}

// buildTimelines walks every snapshot in every bundle and bucketizes
// observations by thread name. Mirrors Python's `_build_timelines`.
func buildTimelines(bundles []models.ThreadDumpBundle) map[string]*threadTimeline {
	timelines := map[string]*threadTimeline{}
	for _, bundle := range bundles {
		for _, snapshot := range bundle.Snapshots {
			tl, ok := timelines[snapshot.ThreadName]
			if !ok {
				tl = &threadTimeline{threadName: snapshot.ThreadName}
				timelines[snapshot.ThreadName] = tl
			}
			tl.observations = append(tl.observations, threadObservation{
				dumpIndex:      bundle.DumpIndex,
				state:          snapshot.State,
				stackSignature: snapshot.StackSignature(0),
			})
		}
	}
	return timelines
}

// runnableStates / blockedStates / latencyWaitCategories mirror the
// per-finding state filters declared at the top of the Python module.
var (
	runnableStates = map[models.ThreadState]struct{}{
		models.ThreadStateRunnable: {},
	}
	blockedStates = map[models.ThreadState]struct{}{
		models.ThreadStateBlocked:  {},
		models.ThreadStateLockWait: {},
	}
	// Order preserved from Python `_LATENCY_WAIT_CATEGORIES` so the
	// resulting findings list matches the reference order when
	// multiple categories trip on the same thread.
	latencyWaitCategories = []models.ThreadState{
		models.ThreadStateNetworkWait,
		models.ThreadStateIOWait,
		models.ThreadStateChannelWait,
	}
)
