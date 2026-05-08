// [한글] models/thread_state.go — 5개 런타임의 스레드 상태를 단일
// enum 으로 정규화.
//
// 왜 정규화가 필요한가?
//   • Java jstack       : "RUNNABLE", "WAITING", "TIMED_WAITING", ...
//   • Go goroutine      : "running", "IO wait", "chan receive", "select"
//   • Python py-spy     : "running", "sleeping"
//   • Node.js report    : libuv state — "active", "idle"
//   • .NET clrstack     : "RUNNING" (대부분 추론으로 IO_WAIT 등 부여)
//   각 런타임이 사용하는 raw 문자열을 그대로 두면 multi-thread 분석기가
//   각 케이스를 따로 분기해야 하므로 코드가 폭발적으로 늘어납니다.
//   따라서 파서/플러그인 단계에서 이 ThreadState 로 강제 정규화하고,
//   분석기는 이 enum 만 본다는 것이 핵심 설계 원칙입니다.
//
// 정규화 규칙 (CoerceThreadState)
//   1) 공백 trim → upper-case → "-"/" " 를 "_" 로 치환.
//   2) threadStateAliases (실세계에서 발견된 별칭) 우선 매칭.
//        RUNNING/ACTIVE       → RUNNABLE
//        WAIT/PARKED          → WAITING
//        TERMINATED           → DEAD
//        SLEEP/SLEEPING       → TIMED_WAITING
//        IO                   → IO_WAIT
//        CHAN_RECEIVE / CHAN_SEND / SELECT → CHANNEL_WAIT
//   3) canonicalThreadStates (정식 enum 11개) 일치하면 그대로 사용.
//   4) 모두 미스이면 UNKNOWN.
//
// 주의: 별칭 표는 Python coerce 와 verbatim 동일해야 합니다 — parity
// gate 가 동일 입력 → 동일 enum 이어야 finding 결과가 같아집니다.
package models

import "strings"

// ThreadState is the normalized thread state across runtimes. Mirrors
// engines/python/archscope_engine/models/thread_snapshot.py:ThreadState.
//
// The string form is what serializes to JSON, matching the Python
// `str` enum behaviour (`"RUNNABLE"`, `"BLOCKED"`, ...).
type ThreadState string

const (
	ThreadStateRunnable     ThreadState = "RUNNABLE"
	ThreadStateBlocked      ThreadState = "BLOCKED"
	ThreadStateWaiting      ThreadState = "WAITING"
	ThreadStateTimedWaiting ThreadState = "TIMED_WAITING"
	ThreadStateNetworkWait  ThreadState = "NETWORK_WAIT"
	ThreadStateIOWait       ThreadState = "IO_WAIT"
	ThreadStateLockWait     ThreadState = "LOCK_WAIT"
	ThreadStateChannelWait  ThreadState = "CHANNEL_WAIT"
	ThreadStateDead         ThreadState = "DEAD"
	ThreadStateNew          ThreadState = "NEW"
	ThreadStateUnknown      ThreadState = "UNKNOWN"
)

// canonical states that are accepted as-is when no alias applies.
var canonicalThreadStates = map[string]ThreadState{
	"RUNNABLE":      ThreadStateRunnable,
	"BLOCKED":       ThreadStateBlocked,
	"WAITING":       ThreadStateWaiting,
	"TIMED_WAITING": ThreadStateTimedWaiting,
	"NETWORK_WAIT":  ThreadStateNetworkWait,
	"IO_WAIT":       ThreadStateIOWait,
	"LOCK_WAIT":     ThreadStateLockWait,
	"CHANNEL_WAIT":  ThreadStateChannelWait,
	"DEAD":          ThreadStateDead,
	"NEW":           ThreadStateNew,
	"UNKNOWN":       ThreadStateUnknown,
}

// Aliases observed in the wild — the Python coerce table verbatim.
var threadStateAliases = map[string]ThreadState{
	"RUNNING":       ThreadStateRunnable,
	"ACTIVE":        ThreadStateRunnable,
	"WAIT":          ThreadStateWaiting,
	"PARKED":        ThreadStateWaiting,
	"TERMINATED":    ThreadStateDead,
	"INITIALIZING": ThreadStateNew,
	"SLEEP":         ThreadStateTimedWaiting,
	"SLEEPING":      ThreadStateTimedWaiting,
	"IO":            ThreadStateIOWait,
	"CHAN_RECEIVE":  ThreadStateChannelWait,
	"CHAN_SEND":     ThreadStateChannelWait,
	"SELECT":        ThreadStateChannelWait,
}

// CoerceThreadState is the Go counterpart of `ThreadState.coerce`. It
// best-effort maps a raw runtime state string to the enum.
//
// Rules (Python parity):
//  1. nil-equivalent (`""`) → UNKNOWN.
//  2. uppercase + replace `-` and ` ` with `_`.
//  3. apply alias table; canonical states are accepted as-is.
//  4. anything else → UNKNOWN.
func CoerceThreadState(value string) ThreadState {
	if value == "" {
		return ThreadStateUnknown
	}
	upper := strings.ReplaceAll(strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(value)), "-", "_"), " ", "_")
	if alias, ok := threadStateAliases[upper]; ok {
		return alias
	}
	if canon, ok := canonicalThreadStates[upper]; ok {
		return canon
	}
	return ThreadStateUnknown
}
