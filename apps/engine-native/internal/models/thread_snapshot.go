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
