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
