package models

import "strconv"

// LockHandle identifies a single monitor / lock object. JVM jstack
// exposes `<0x000000076ab62208>` plus a class hint; other runtimes
// usually leave LockID empty and only carry LockClass. Mirrors the
// Python `LockHandle` dataclass.
//
// LockClass is `*string` so the JSON shape can emit `null` (matching
// Python `str | None`). WaitMode is omitted entirely when empty to
// match Python's `if self.wait_mode: payload["wait_mode"] = ...`.
type LockHandle struct {
	LockID    string  `json:"lock_id"`
	LockClass *string `json:"lock_class"`
	WaitMode  string  `json:"wait_mode,omitempty"`
}

// StackFrame is one normalized stack frame. Function is the only
// required field; everything else is best-effort capture from the
// source format. Mirrors `models/thread_snapshot.py:StackFrame`.
//
// Module / File / Line / Language are all pointer types so unset
// fields emit JSON `null` (Python parity).
type StackFrame struct {
	Function string  `json:"function"`
	Module   *string `json:"module"`
	File     *string `json:"file"`
	Line     *int    `json:"line"`
	Language *string `json:"language"`
}

// Render is the human-readable single-line rendering used for stack
// signatures. Matches `StackFrame.render()` in Python so signatures
// stay byte-identical across the two implementations.
func (f StackFrame) Render() string {
	head := f.Function
	if f.Module != nil && *f.Module != "" {
		head = *f.Module + "." + f.Function
	}
	if f.File == nil || *f.File == "" {
		return head
	}
	location := *f.File
	if f.Line != nil {
		location = *f.File + ":" + strconv.Itoa(*f.Line)
	}
	return head + " (" + location + ")"
}
