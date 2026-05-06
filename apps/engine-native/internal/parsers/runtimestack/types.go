// Package runtimestack ports the four Python runtime stack-trace
// parsers (`dotnet_parser`, `go_panic_parser`, `nodejs_stack_parser`,
// `python_traceback_parser`) to Go. They all emit the same
// RuntimeStackRecord shape so they live in one package with one file
// per language and a shared types.go for the records and Options.
//
// Each language exposes a `Parse<Lang>File` entrypoint mirroring
// Python's `parse_<lang>(...)`. The entrypoints share the Strict /
// MaxLines knobs the access-log reference parser (T-310) uses so
// analyzers can dispatch through a uniform Options struct.
package runtimestack

import "time"

// RuntimeStackRecord mirrors `models.runtime_stack.RuntimeStackRecord`
// — the cross-runtime aggregation key the exception/runtime analyzers
// (T-333) consume. `Message` is a pointer because Python distinguishes
// "no header message" (None) from "header message was the empty string"
// for python_traceback; the other runtimes only ever produce non-empty
// messages but use the same field for parity.
type RuntimeStackRecord struct {
	Runtime    string
	RecordType string
	Headline   string
	Message    *string
	Stack      []string
	Signature  string
	RawBlock   string
}

// IisAccessRecord mirrors `models.runtime_stack.IisAccessRecord`. Only
// the .NET parser produces these (the IIS W3C log block is interleaved
// with .NET exception text in the same input file).
type IisAccessRecord struct {
	Method      string
	URI         string
	Status      int
	TimeTakenMS *int
	RawLine     string
}

// Options mirrors the per-parser knobs the access-log reference
// surfaces. `MaxLines` is honoured before line-level dispatch so
// strict-mode rejections from beyond the cap don't fire. `Strict`
// turns the first per-line skip into a fatal error.
type Options struct {
	MaxLines int
	Strict   bool

	// StartTime / EndTime are accepted for API symmetry with
	// access-log; the runtime stack inputs aren't time-stamped, so
	// the fields are ignored.
	StartTime *time.Time
	EndTime   *time.Time
}

// stringPtr / intPtr build the optional pointer fields on
// RuntimeStackRecord and IisAccessRecord. Python uses None for
// "absent"; we map that to nil and reserve an actual pointer for
// "present, possibly empty".
func stringPtr(s string) *string { return &s }

func intPtr(n int) *int { return &n }
