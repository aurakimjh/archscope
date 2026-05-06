// Ports archscope_engine.parsers.nodejs_stack_parser. Node.js stack
// traces start with an optional ISO-ish timestamp followed by an
// `ErrorType: message` header, then a series of `at frame ...` lines
// (and optional `Caused by:` chain entries). Blocks are accreted by
// header detection — anything else inside a live block is appended,
// stray lines outside a block are reported.
package runtimestack

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

var nodeErrorRE = regexp.MustCompile(
	`^(?:(?P<timestamp>\d{4}-\d{2}-\d{2}[T ][^ ]+)\s+)?` +
		`(?P<type>(?:Error|Exception)|[A-Za-z_$][\w.$]*(?:Error|Exception))` +
		`(?::\s*(?P<message>.*))?$`,
)

const (
	ReasonNoNodejsErrorHeader = "NO_NODEJS_ERROR_HEADER"
	ReasonInvalidNodejsStack  = "INVALID_NODEJS_STACK"
)

// ParseNodejsFile mirrors `parse_nodejs_stack`.
func ParseNodejsFile(path string, opts Options) ([]RuntimeStackRecord, *diagnostics.ParserDiagnostics, error) {
	if opts.MaxLines < 0 {
		return nil, nil, fmt.Errorf("max_lines must be a positive integer")
	}

	diags := diagnostics.New("nodejs_stack")
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

		stripped := strings.TrimSpace(line)
		switch {
		case nodeErrorRE.MatchString(stripped):
			flush()
			current = pending{startLine: lineNumber, body: []string{stripped}}
			currentActive = true
		case currentActive && (strings.HasPrefix(stripped, "at ") || strings.HasPrefix(stripped, "Caused by:")):
			current.body = append(current.body, stripped)
		case currentActive && stripped != "":
			current.body = append(current.body, stripped)
		case stripped != "":
			reason := ReasonNoNodejsErrorHeader
			message := "Line did not start a supported Node.js stack block."
			diags.AddSkipped(lineNumber, reason, message, line)
			if opts.Strict {
				return nil, diags, fmt.Errorf("%s:%d: %s: %s", path, lineNumber, reason, message)
			}
		}
	}
	flush()

	records := make([]RuntimeStackRecord, 0, len(blocks))
	for _, b := range blocks {
		record, ok := parseNodejsBlock(b.body)
		if !ok {
			diags.AddSkipped(b.startLine, ReasonInvalidNodejsStack,
				"Node.js stack block was missing a supported header.",
				strings.Join(b.body, "\n"))
			continue
		}
		records = append(records, record)
		diags.ParsedRecords++
	}

	if diags.TotalLines == 0 {
		diags.AddWarning(0, "EMPTY_FILE", "Node.js stack file is empty.", "", false)
	}
	return records, diags, nil
}

// parseNodejsBlock mirrors Python's `_parse_block`.
func parseNodejsBlock(block []string) (RuntimeStackRecord, bool) {
	indices := nodeErrorRE.FindStringSubmatchIndex(block[0])
	if indices == nil {
		return RuntimeStackRecord{}, false
	}
	typeIdx := nodeErrorRE.SubexpIndex("type")
	msgIdx := nodeErrorRE.SubexpIndex("message")
	errorType := block[0][indices[2*typeIdx]:indices[2*typeIdx+1]]

	var message *string
	if msgStart := indices[2*msgIdx]; msgStart >= 0 {
		message = stringPtr(block[0][msgStart:indices[2*msgIdx+1]])
	}

	stack := make([]string, 0, len(block))
	for _, line := range block[1:] {
		if strings.HasPrefix(line, "at ") {
			stack = append(stack, line[3:])
		}
	}
	topFrame := "(no-frame)"
	if len(stack) > 0 {
		topFrame = stack[0]
	}
	return RuntimeStackRecord{
		Runtime:    "nodejs",
		RecordType: errorType,
		Headline:   errorType,
		Message:    message,
		Stack:      stack,
		Signature:  fmt.Sprintf("%s|%s", errorType, topFrame),
		RawBlock:   strings.Join(block, "\n"),
	}, true
}
