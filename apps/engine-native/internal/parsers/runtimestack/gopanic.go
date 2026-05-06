// Ports archscope_engine.parsers.go_panic_parser. The parser scans
// for `panic: ...` and `goroutine N [state]:` headers, accreting
// non-blank follow-up lines into the current block until either the
// next header or a blank line ends it. The frame regex is anchored on
// the unindented `func(args)` lines that follow each header (the
// trailing `\tfile.go:NN +0xNN` line is intentionally ignored — Python
// strips it via `not line.startswith("\\t")` after the strip()).
package runtimestack

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

var (
	goPanicRE     = regexp.MustCompile(`^panic:\s*(?P<message>.+)$`)
	goGoroutineRE = regexp.MustCompile(`^goroutine\s+(?P<id>\d+)\s+\[(?P<state>[^\]]+)\]:$`)
	goFuncRE      = regexp.MustCompile(`^(?P<func>[\w./*()$-]+)\(.*\)$`)
)

const (
	ReasonOutsideGoPanic = "OUTSIDE_GO_PANIC"
)

// ParseGoPanicFile mirrors `parse_go_panic`. Returns the parsed
// records plus diagnostics. Strict mode escalates the first
// outside-block line to a fatal error.
func ParseGoPanicFile(path string, opts Options) ([]RuntimeStackRecord, *diagnostics.ParserDiagnostics, error) {
	if opts.MaxLines < 0 {
		return nil, nil, fmt.Errorf("max_lines must be a positive integer")
	}

	diags := diagnostics.New("go_panic")
	diags.SetSourceFile(path)

	lines, err := textio.IterTextLines(path, "")
	if err != nil {
		return nil, nil, err
	}

	var blocks [][]string
	var current []string

	flush := func() {
		if len(current) == 0 {
			return
		}
		blocks = append(blocks, current)
		current = nil
	}

	for i, line := range lines {
		lineNumber := i + 1
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			break
		}
		diags.TotalLines++

		stripped := strings.TrimSpace(line)
		switch {
		case goPanicRE.MatchString(stripped) || goGoroutineRE.MatchString(stripped):
			flush()
			current = []string{stripped}
		case len(current) > 0 && stripped != "":
			current = append(current, stripped)
		case stripped != "":
			reason := ReasonOutsideGoPanic
			message := "Line was outside a supported Go panic or goroutine block."
			diags.AddSkipped(lineNumber, reason, message, line)
			if opts.Strict {
				return nil, diags, fmt.Errorf("%s:%d: %s: %s", path, lineNumber, reason, message)
			}
		}
	}
	flush()

	records := make([]RuntimeStackRecord, 0, len(blocks))
	for _, block := range blocks {
		if record, ok := parseGoPanicBlock(block); ok {
			records = append(records, record)
			diags.ParsedRecords++
		}
	}

	if diags.TotalLines == 0 {
		diags.AddWarning(0, "EMPTY_FILE", "Go panic file is empty.", "", false)
	}
	return records, diags, nil
}

// parseGoPanicBlock mirrors Python's `_parse_block`. Returns false
// when the header doesn't match either pattern (dead branch from the
// caller's perspective, kept for parity).
func parseGoPanicBlock(block []string) (RuntimeStackRecord, bool) {
	header := block[0]
	panicIdx := goPanicRE.FindStringSubmatchIndex(header)
	goroutineIdx := goGoroutineRE.FindStringSubmatchIndex(header)
	if panicIdx == nil && goroutineIdx == nil {
		return RuntimeStackRecord{}, false
	}

	stack := make([]string, 0, len(block))
	for _, line := range block[1:] {
		// Python guards `not line.startswith("\\t")`. Our `block`
		// elements are already stripped, so a leading tab is impossible
		// — but indented frame-detail lines like `/app/file.go:42 +0x45`
		// won't match goFuncRE anyway because they don't end in `)`.
		if strings.HasPrefix(line, "\t") {
			continue
		}
		match := goFuncRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		funcIdx := goFuncRE.SubexpIndex("func")
		stack = append(stack, match[funcIdx])
	}

	var recordType string
	var message string
	if panicIdx != nil {
		recordType = "panic"
		msgIdx := goPanicRE.SubexpIndex("message")
		message = header[panicIdx[2*msgIdx]:panicIdx[2*msgIdx+1]]
	} else {
		recordType = "goroutine"
		stateIdx := goGoroutineRE.SubexpIndex("state")
		message = header[goroutineIdx[2*stateIdx]:goroutineIdx[2*stateIdx+1]]
	}
	topFrame := "(no-frame)"
	if len(stack) > 0 {
		topFrame = stack[0]
	}
	msgPtr := stringPtr(message)
	return RuntimeStackRecord{
		Runtime:    "go",
		RecordType: recordType,
		Headline:   header,
		Message:    msgPtr,
		Stack:      stack,
		Signature:  fmt.Sprintf("%s|%s", recordType, topFrame),
		RawBlock:   strings.Join(block, "\n"),
	}, true
}
