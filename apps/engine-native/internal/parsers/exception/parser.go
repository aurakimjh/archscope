// Package exception ports archscope_engine.parsers.exception_parser.
//
// The Java exception-stack parser walks a text file line-by-line and
// groups each "Throwable header + at-frames + Caused by chain" into a
// single Record. Header detection is regex based; cause chains are
// folded onto the most recent open block. The first frame becomes part
// of the deduplication signature so analyzers can collapse repeated
// failures (e.g. "OrderController.create at line 18 throws X 200x in a
// row" -> a single REPEATED_EXCEPTION_SIGNATURE finding).
//
// See engines/python/archscope_engine/parsers/exception_parser.py for
// the reference implementation. We mirror its skip / warning reasons
// verbatim so the T-244 / T-390 cross-engine parity gate keeps passing.
package exception

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

// Record is the parsed exception block. Mirrors Python's
// `models.thread_dump.ExceptionRecord` field-for-field. Pointer types
// stand in for Python's `Optional[T]`.
type Record struct {
	Timestamp     *time.Time
	Language      string
	ExceptionType string
	Message       *string
	RootCause     *string
	Stack         []string
	Signature     string
	RawBlock      string
}

// Skip / warning reasons (mirror Python verbatim).
const (
	ReasonNoExceptionHeader    = "NO_EXCEPTION_HEADER"
	ReasonInvalidBlock         = "INVALID_EXCEPTION_BLOCK"
	ReasonEmptyFile            = "EMPTY_FILE"
	ReasonNoExceptionBlocks    = "NO_EXCEPTION_BLOCKS"
	FormatJavaExceptionStack   = "java_exception_stack"
	signatureNoFramePlaceholder = "(no-frame)"
)

// exceptionLineRE matches the Throwable header — optional ISO timestamp,
// optional "Caused by:" prefix, fully-qualified type ending in
// Exception/Error/Throwable, and an optional ": message" tail. Mirrors
// Python's EXCEPTION_LINE_RE byte-for-byte (Go RE2 has no lookbehinds
// or named-capture differences that affect this expression).
var exceptionLineRE = regexp.MustCompile(
	`^(?:(?P<timestamp>\d{4}-\d{2}-\d{2}[T ][^ ]+)\s+)?` +
		`(?:(?P<caused>Caused by:\s+))?` +
		`(?P<type>[A-Za-z_$][\w.$]*(?:Exception|Error|Throwable))` +
		`(?::\s*(?P<message>.*))?$`,
)

// parseError carries the (reason, message) pair Python uses to
// classify malformed blocks. The exception parser only emits
// block-level errors so it appears in ParseFile's diagnostics path.
type parseError struct {
	Reason  string
	Message string
}

// ParseLine is a thin wrapper around the header regex — kept so
// callers that want to peek at a single line (e.g. for triage tools or
// streaming analyzers) match the access-log parser's surface. Returns
// (record, nil) when the line is a complete one-line block (header
// only, no frames or cause chain). For real multi-line block parsing
// callers should use ParseFile or call ParseBlock with an explicit
// block of lines.
func ParseLine(line string) (*Record, *parseError) {
	stripped := strings.TrimSpace(line)
	if stripped == "" {
		return nil, &parseError{Reason: ReasonNoExceptionHeader, Message: "Line did not start a supported exception stack block."}
	}
	header := matchHeader(stripped)
	if header == nil || header["caused"] != "" {
		return nil, &parseError{Reason: ReasonNoExceptionHeader, Message: "Line did not start a supported exception stack block."}
	}
	rec := buildRecord([]string{stripped}, header)
	return rec, nil
}

// ParseBlock parses a contiguous block of stripped lines starting with
// a Throwable header. Returns nil + INVALID_EXCEPTION_BLOCK when the
// first line does not match the header regex.
func ParseBlock(block []string) (*Record, *parseError) {
	if len(block) == 0 {
		return nil, &parseError{Reason: ReasonInvalidBlock, Message: "Exception block was missing a supported exception header."}
	}
	header := matchHeader(block[0])
	if header == nil {
		return nil, &parseError{Reason: ReasonInvalidBlock, Message: "Exception block was missing a supported exception header."}
	}
	return buildRecord(block, header), nil
}

// Options carries the parser-level filters. The Python parser exposes
// no filter knobs (no max_lines, no time range, no strict), but the
// T-310 reference signature normalises Strict/MaxLines across all
// parsers so the engine driver can apply uniform limits without
// special-casing per-format. Zero-value Options yields the Python
// default behaviour exactly.
type Options struct {
	// MaxLines caps how many input lines are inspected. 0 means
	// unbounded. Mirrors the engine-driver knob — Python parser does
	// not expose this but accepting it here keeps parsers uniform.
	MaxLines int
	// Strict makes the first INVALID_EXCEPTION_BLOCK or
	// NO_EXCEPTION_HEADER row return an error after recording
	// diagnostics — same shape as accesslog.ParseFile.
	Strict bool
}

// ParseFile reads `path`, walks its lines, groups them into exception
// blocks and returns the parsed records plus a populated diagnostics
// builder. Mirrors `parse_exception_stack`. The `format` parameter is
// retained for caller symmetry with other parsers; only the literal
// "java_exception_stack" is recognised — anything else (including the
// empty string) defaults to that label.
func ParseFile(path, format string, opts Options) ([]Record, *diagnostics.ParserDiagnostics, error) {
	if format == "" {
		format = FormatJavaExceptionStack
	}
	if format != FormatJavaExceptionStack {
		return nil, nil, fmt.Errorf("Unsupported exception stack format: %s", format)
	}
	if opts.MaxLines < 0 {
		return nil, nil, fmt.Errorf("max_lines must be a positive integer")
	}

	diags := diagnostics.New(format)
	diags.SetSourceFile(path)

	lines, err := textio.IterTextLines(path, "")
	if err != nil {
		return nil, nil, err
	}

	records := make([]Record, 0)
	var current []string
	currentStart := 0

	flush := func(blockStart int) *parseError {
		if len(current) == 0 {
			return nil
		}
		rec, perr := ParseBlock(current)
		if perr != nil {
			diags.AddSkipped(blockStart, perr.Reason, perr.Message, strings.Join(current, "\n"))
			current = nil
			return perr
		}
		diags.ParsedRecords++
		records = append(records, *rec)
		current = nil
		return nil
	}

	for i, raw := range lines {
		lineNumber := i + 1
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			break
		}
		diags.TotalLines++
		stripped := strings.TrimSpace(raw)
		groups := matchHeader(stripped)

		// New block: header line that is NOT a "Caused by:" continuation.
		if groups != nil && !strings.HasPrefix(stripped, "Caused by:") {
			if perr := flush(currentStart); perr != nil && opts.Strict {
				return records, diags, fmt.Errorf("%s:%d: %s: %s", path, currentStart, perr.Reason, perr.Message)
			}
			current = append(current, stripped)
			currentStart = lineNumber
			continue
		}

		// Continuation: stack frame, "Caused by:" line, or any
		// non-blank trailing line while a block is still open.
		if len(current) > 0 && stripped != "" {
			current = append(current, stripped)
			continue
		}

		// Blank line outside a block: ignored.
		if stripped == "" {
			continue
		}

		// Non-blank line with no open block and no header match — Python
		// records this with NO_EXCEPTION_HEADER.
		diags.AddSkipped(lineNumber, ReasonNoExceptionHeader, "Line did not start a supported exception stack block.", raw)
		if opts.Strict {
			return records, diags, fmt.Errorf("%s:%d: %s: %s", path, lineNumber, ReasonNoExceptionHeader, "Line did not start a supported exception stack block.")
		}
	}

	// Flush trailing block.
	if perr := flush(currentStart); perr != nil && opts.Strict {
		return records, diags, fmt.Errorf("%s:%d: %s: %s", path, currentStart, perr.Reason, perr.Message)
	}

	if diags.TotalLines == 0 {
		diags.AddWarning(0, ReasonEmptyFile, "Exception stack file is empty.", "", false)
	} else if diags.ParsedRecords == 0 {
		diags.AddWarning(0, ReasonNoExceptionBlocks, "No supported Java exception blocks were parsed.", "", false)
	}

	return records, diags, nil
}

func matchHeader(line string) map[string]string {
	match := exceptionLineRE.FindStringSubmatch(line)
	if match == nil {
		return nil
	}
	groups := make(map[string]string, len(exceptionLineRE.SubexpNames()))
	for i, name := range exceptionLineRE.SubexpNames() {
		if name == "" {
			continue
		}
		groups[name] = match[i]
	}
	return groups
}

// buildRecord packs a parsed block into a Record. The first line is
// the root header (already matched into `header`); subsequent lines
// contribute frames and cause-chain entries. Python keeps only the
// last "Caused by:" type as root_cause — we preserve that behaviour
// exactly so analyzer findings line up across engines.
func buildRecord(block []string, header map[string]string) *Record {
	rootType := header["type"]
	rootMessageRaw := header["message"]

	type causedEntry struct {
		typ string
		msg string
	}
	var causedBy []causedEntry
	stack := make([]string, 0, len(block))

	for _, line := range block[1:] {
		if strings.HasPrefix(line, "at ") {
			stack = append(stack, line[len("at "):])
			continue
		}
		causedMatch := matchHeader(line)
		if causedMatch != nil && causedMatch["caused"] != "" {
			causedBy = append(causedBy, causedEntry{
				typ: causedMatch["type"],
				msg: causedMatch["message"],
			})
		}
	}

	var rootCause *string
	if len(causedBy) > 0 {
		v := causedBy[len(causedBy)-1].typ
		rootCause = &v
	}

	signatureFrame := signatureNoFramePlaceholder
	if len(stack) > 0 {
		signatureFrame = stack[0]
	}

	rec := &Record{
		Language:      "java",
		ExceptionType: rootType,
		RootCause:     rootCause,
		Stack:         stack,
		Signature:     fmt.Sprintf("%s|%s", rootType, signatureFrame),
		RawBlock:      strings.Join(block, "\n"),
	}

	// Python uses re.match's group("message") which returns None when
	// the optional capture didn't fire. Go's regexp returns "" for
	// missing groups — we have to look at the second-pass parse to
	// know whether the suffix was actually present. The cleanest
	// proxy is: if the original header line has no ":" after the type,
	// the message is absent. We reconstruct that by checking whether
	// the regex-supplied `message` group is empty AND the original line
	// has no ": " after the type — but Python's behaviour is simpler:
	// if the optional ": (.*)" did not match, group is None; if it
	// matched but captured the empty string, group is "". In Go we
	// can't distinguish those two from the slice form alone, but
	// exceptionLineRE is greedy — if the type+colon prefix is present,
	// the trailing message group always captures (possibly empty).
	// So: empty string means "explicit empty message", absent colon
	// means no group fired. We detect the latter via the raw header.
	if hasMessageDelimiter(block[0], rootType) {
		m := rootMessageRaw
		rec.Message = &m
	}

	// Timestamp parsing. Python uses `datetime.fromisoformat` which
	// accepts both `2026-04-27T10:00:00+09:00` (RFC-3339) and bare
	// `2026-04-27T10:00:00` (no offset). We mirror that envelope.
	if ts := parseTimestamp(header["timestamp"]); ts != nil {
		rec.Timestamp = ts
	}

	return rec
}

// hasMessageDelimiter returns true when the original header line carries
// a `<type>:` suffix — the only way Python's optional `:\s*(.*)` group
// fires. Used to tell apart "no message" from "explicit empty message".
func hasMessageDelimiter(headerLine, exceptionType string) bool {
	idx := strings.Index(headerLine, exceptionType)
	if idx < 0 {
		return false
	}
	tail := headerLine[idx+len(exceptionType):]
	return strings.HasPrefix(tail, ":")
}

// parseTimestamp mirrors Python's `datetime.fromisoformat`. It accepts
// the same envelope of layouts the regex can produce — RFC-3339 with
// or without a tz offset, optionally with fractional seconds, and the
// space-separated `YYYY-MM-DD HH:MM:SS[.frac][offset]` form.
func parseTimestamp(value string) *time.Time {
	if value == "" {
		return nil
	}
	candidates := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range candidates {
		if t, err := time.Parse(layout, value); err == nil {
			return &t
		}
	}
	return nil
}
