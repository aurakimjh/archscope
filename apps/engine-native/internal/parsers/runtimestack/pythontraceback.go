// Ports archscope_engine.parsers.python_traceback_parser. Tracebacks
// start with the literal `Traceback (most recent call last):` header
// and run until the next traceback or end of file. The terminating
// exception line is found by scanning the block from the bottom up
// (Python sometimes emits a context-message before the actual
// exception type, so the last regex match wins). Frames are extracted
// via `File "...", line N, in func`.
//
// Note: unlike the other runtimes, Python uses `line.rstrip()` so the
// `File ...` indentation survives into the block and the file regex
// can anchor on `^\s*File`.
package runtimestack

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

var (
	pyExceptionRE = regexp.MustCompile(
		`^(?P<type>[A-Za-z_][\w.]*Error|[A-Za-z_][\w.]*Exception)` +
			`:?\s*(?P<message>.*)$`,
	)
	pyFileRE = regexp.MustCompile(
		`^\s*File "(?P<file>[^"]+)", line (?P<line>\d+), in (?P<func>.+)$`,
	)
)

const (
	ReasonOutsidePythonTraceback = "OUTSIDE_PYTHON_TRACEBACK"
	ReasonInvalidPythonTraceback = "INVALID_PYTHON_TRACEBACK"
)

const tracebackHeader = "Traceback (most recent call last):"

// ParsePythonTracebackFile mirrors `parse_python_traceback`.
func ParsePythonTracebackFile(path string, opts Options) ([]RuntimeStackRecord, *diagnostics.ParserDiagnostics, error) {
	if opts.MaxLines < 0 {
		return nil, nil, fmt.Errorf("max_lines must be a positive integer")
	}

	diags := diagnostics.New("python_traceback")
	diags.SetSourceFile(path)

	lines, err := textio.IterTextLines(path, "")
	if err != nil {
		return nil, nil, err
	}

	type pending struct {
		startLine int
		body      []string
	}
	var blocks []pending
	var current pending
	currentActive := false

	flush := func() {
		if currentActive {
			blocks = append(blocks, current)
		}
		currentActive = false
		current = pending{}
	}

	for i, line := range lines {
		lineNumber := i + 1
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			break
		}
		diags.TotalLines++

		stripped := rstrip(line)
		switch {
		case strings.HasPrefix(stripped, tracebackHeader):
			flush()
			current = pending{startLine: lineNumber, body: []string{stripped}}
			currentActive = true
		case currentActive:
			current.body = append(current.body, stripped)
		case strings.TrimSpace(stripped) != "":
			reason := ReasonOutsidePythonTraceback
			message := "Line was outside a Python traceback block."
			diags.AddSkipped(lineNumber, reason, message, line)
			if opts.Strict {
				return nil, diags, fmt.Errorf("%s:%d: %s: %s", path, lineNumber, reason, message)
			}
		}
	}
	flush()

	records := make([]RuntimeStackRecord, 0, len(blocks))
	for _, b := range blocks {
		record, ok := parsePythonTracebackBlock(b.body)
		if !ok {
			diags.AddSkipped(b.startLine, ReasonInvalidPythonTraceback,
				"Python traceback block was missing a terminal exception line.",
				strings.Join(b.body, "\n"))
			continue
		}
		records = append(records, record)
		diags.ParsedRecords++
	}

	if diags.TotalLines == 0 {
		diags.AddWarning(0, "EMPTY_FILE", "Python traceback file is empty.", "", false)
	}
	return records, diags, nil
}

// parsePythonTracebackBlock mirrors Python's `_parse_block`.
func parsePythonTracebackBlock(block []string) (RuntimeStackRecord, bool) {
	var (
		excIndices []int
		excSrc     string
	)
	for i := len(block) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(block[i])
		idx := pyExceptionRE.FindStringSubmatchIndex(candidate)
		if idx != nil {
			excIndices = idx
			excSrc = candidate
			break
		}
	}
	if excIndices == nil {
		return RuntimeStackRecord{}, false
	}
	typeIdx := pyExceptionRE.SubexpIndex("type")
	msgIdx := pyExceptionRE.SubexpIndex("message")
	errorType := excSrc[excIndices[2*typeIdx]:excIndices[2*typeIdx+1]]

	var message *string
	if msgStart := excIndices[2*msgIdx]; msgStart >= 0 {
		raw := excSrc[msgStart:excIndices[2*msgIdx+1]]
		// Python folds empty match to None via `or None`.
		if raw != "" {
			message = stringPtr(raw)
		}
	}

	stack := make([]string, 0, len(block))
	for _, line := range block {
		match := pyFileRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		fileIdx := pyFileRE.SubexpIndex("file")
		lineIdx := pyFileRE.SubexpIndex("line")
		funcIdx := pyFileRE.SubexpIndex("func")
		stack = append(stack, fmt.Sprintf("%s:%s in %s",
			match[fileIdx], match[lineIdx], match[funcIdx]))
	}

	topFrame := "(no-frame)"
	if len(stack) > 0 {
		topFrame = stack[len(stack)-1]
	}
	return RuntimeStackRecord{
		Runtime:    "python",
		RecordType: errorType,
		Headline:   errorType,
		Message:    message,
		Stack:      stack,
		Signature:  fmt.Sprintf("%s|%s", errorType, topFrame),
		RawBlock:   strings.Join(block, "\n"),
	}, true
}

// rstrip mirrors Python's `str.rstrip()` — removes trailing
// whitespace runes only.
func rstrip(s string) string {
	end := len(s)
	for end > 0 {
		r := rune(s[end-1])
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			end--
			continue
		}
		break
	}
	return s[:end]
}
