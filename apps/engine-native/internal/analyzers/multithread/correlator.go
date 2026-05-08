// [한글] correlator.go — multithread 분석기의 timeline 빌드와 연속
// run-length 계산 로직.
//
// 핵심 자료구조
//   threadObservation : 한 dump 에서 한 스레드를 관측한 결과 1개
//                       (dumpIndex, state, stackSignature).
//   threadTimeline    : 같은 thread name 으로 묶인 관측 list.
//
// Timeline 의미론
//   덤프가 시간순으로 입력되었다고 가정 — DumpIndex 가 작을수록 과거.
//   thread name 이 같으면 동일 스레드라고 본다(JVM 의 thread id 가
//   재사용될 수 있어 신뢰하지 않음; 5개 런타임 공통 안전책).
//
// stateRunLengths 알고리즘 (가장 긴 연속 run 찾기)
//   1) 관측을 dumpIndex ASC 로 정렬한 사본을 사용.
//   2) targetStates 에 속하지 않은 관측은 run 을 0으로 reset.
//   3) targetStates 에 속한 관측은:
//        • 직전 관측이 없거나 dumpIndex == prev+1 → current++
//        • dump 가 한 칸 건너뛰면 새 run 시작 (current=1)
//   4) 가장 큰 current 를 longest 로 보존하고 반환.
//
//   왜 "연속" 인가? 한 번 깬 run 을 합쳐서 N 만족시키는 것을 피하기
//   위함. 잠깐만 풀린 lock 을 만성 contention 으로 잘못 보고하지
//   않는 안전장치.
//
// 상태 카테고리 (findings 에서 어떤 state 가 어떤 finding 을 만드는지)
//   • runnableStates       → LONG_RUNNING_THREAD
//   • blockedStates        → PERSISTENT_BLOCKED_THREAD
//   • latencyWaitCategories → LATENCY_SECTION_DETECTED (각 카테고리별)
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
