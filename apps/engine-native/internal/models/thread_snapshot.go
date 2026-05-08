// [한글] models/thread_snapshot.go — 단일 덤프 내의 스레드 1개를
// 표현하는 ThreadSnapshot 와, 한 덤프 파일 전체를 표현하는
// ThreadDumpBundle 의 정의.
//
// 데이터 흐름
//   파일 → 플러그인.Parse() → ThreadDumpBundle{Snapshots: []ThreadSnapshot}
//                          → multi/lock 분석기 입력
//
// 핵심 의도
//   • 모든 런타임의 dump 출력이 ThreadDumpBundle 한 형태로 동질화.
//   • 분석기는 더이상 "Java 인지 Go 인지" 분기하지 않음. 단지
//     ThreadState enum + StackFrame slice + LockHandle slice 만 본다.
//   • 외부에서 식별할 수 있는 메타 정보(SourceFormat, Language) 는
//     bundle 레벨로 빼두어 finding 메시지에 출처를 남길 수 있게 함.
//
// StackSignature 의 역할
//   상위 N개 프레임을 " | " 로 join 한 문자열은 멀티-덤프 분석에서
//   "같은 스레드의 같은 위치" 를 찾는 dedup 키로 사용됩니다.
//   PERSISTENT_BLOCKED_THREAD 등의 finding 이 이 시그니처로 그룹.
package models

import (
	"strings"
	"time"
)

// ThreadSnapshot is one thread captured in one dump. Mirrors
// `models/thread_snapshot.py:ThreadSnapshot`.
//
// Pointer fields (ThreadID / Category / LockInfo / Language /
// SourceFormat / LockWaiting) emit JSON `null` when unset to keep
// shape parity with Python's `Optional[...]` defaults.
type ThreadSnapshot struct {
	SnapshotID   string         `json:"snapshot_id"`
	ThreadName   string         `json:"thread_name"`
	ThreadID     *string        `json:"thread_id"`
	State        ThreadState    `json:"state"`
	Category     *string        `json:"category"`
	StackFrames  []StackFrame   `json:"stack_frames"`
	LockInfo     *string        `json:"lock_info"`
	Metadata     map[string]any `json:"metadata"`
	Language     *string        `json:"language"`
	SourceFormat *string        `json:"source_format"`
	LockHolds    []LockHandle   `json:"lock_holds"`
	LockWaiting  *LockHandle    `json:"lock_waiting"`
}

// NewThreadSnapshot builds a snapshot with non-nil slices/maps so JSON
// emits `[]` / `{}` instead of `null`. The caller still has to pass
// the required identity fields (snapshot id, name, state).
func NewThreadSnapshot(snapshotID, threadName string, state ThreadState) ThreadSnapshot {
	return ThreadSnapshot{
		SnapshotID:  snapshotID,
		ThreadName:  threadName,
		State:       state,
		StackFrames: []StackFrame{},
		Metadata:    map[string]any{},
		LockHolds:   []LockHandle{},
	}
}

// DefaultStackSignatureDepth matches Python's `depth=5` default.
const DefaultStackSignatureDepth = 5

// StackSignature returns a compact representation of the top `depth`
// frames joined with `" | "`. Matches the Python convention so legacy
// dashboards keep grouping the same threads after the multi-dump
// pipeline takes over.
//
// `depth <= 0` is treated like Python's `depth=5` default, which is
// also what `stack_signature()` uses when called without args.
//
// [한글] 알고리즘
//   1) depth <= 0 → DefaultStackSignatureDepth(=5).
//   2) 스택이 비어있으면 고정 문자열 "(no-stack)" 반환 — 통계 표에
//      "스택 없음" 도 한 그룹으로 묶이도록.
//   3) 상위 depth 개 프레임에 대해 frame.Render() 를 모아서 " | " 로
//      join. depth 가 실제 프레임 수보다 크면 자동 clamp.
//
// 시그니처가 짧은 이유
//   전체 스택을 키로 쓰면 그룹이 너무 많아져 finding 의 신호가 약해
//   집니다. 상위 5 프레임만 봐도 "같은 위치에서 멈춘 스레드" 는 거의
//   같은 그룹으로 묶이고, 그 아래 프레임 차이는 dashboard 의 detail
//   화면에서 보면 됩니다.
func (s ThreadSnapshot) StackSignature(depth int) string {
	if depth <= 0 {
		depth = DefaultStackSignatureDepth
	}
	if len(s.StackFrames) == 0 {
		return "(no-stack)"
	}
	limit := depth
	if limit > len(s.StackFrames) {
		limit = len(s.StackFrames)
	}
	rendered := make([]string, 0, limit)
	for _, frame := range s.StackFrames[:limit] {
		rendered = append(rendered, frame.Render())
	}
	return strings.Join(rendered, " | ")
}

// ThreadDumpBundle carries all snapshots emitted from a single dump
// file along with provenance (dump index, captured timestamp, source
// file, detected format). Mirrors `ThreadDumpBundle` in Python.
//
// CapturedAt is `*time.Time` so missing timestamps emit `null`.
// DumpLabel is `*string` for the same reason.
type ThreadDumpBundle struct {
	Snapshots    []ThreadSnapshot `json:"snapshots"`
	SourceFile   string           `json:"source_file"`
	SourceFormat string           `json:"source_format"`
	Language     string           `json:"language"`
	DumpIndex    int              `json:"dump_index"`
	DumpLabel    *string          `json:"dump_label"`
	CapturedAt   *time.Time       `json:"captured_at"`
	Metadata     map[string]any   `json:"metadata"`
}

// NewThreadDumpBundle constructs a bundle with non-nil snapshot/
// metadata defaults, matching Python's dataclass field factories.
func NewThreadDumpBundle(sourceFile, sourceFormat, language string) ThreadDumpBundle {
	return ThreadDumpBundle{
		Snapshots:    []ThreadSnapshot{},
		SourceFile:   sourceFile,
		SourceFormat: sourceFormat,
		Language:     language,
		Metadata:     map[string]any{},
	}
}
