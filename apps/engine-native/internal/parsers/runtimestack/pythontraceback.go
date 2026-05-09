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
//
// ─────────────────────────────────────────────────────────────────────
// [한글] python traceback parser — Python Traceback 블록 파서.
//
// 입력 패턴
//
//	Traceback (most recent call last):
//	  File "/app/main.py", line 42, in process
//	    raise ValueError("bad input") from e
//	ValueError: bad input
//
// 처리 흐름 (다른 런타임과 다른 점)
//  1. `Traceback (most recent call last):` 리터럴 헤더로 블록 시작.
//  2. 이후 `File "...", line N, in func` 형태 frame 라인을 누적.
//     들여쓰기(`  File "..."`) 가 의미가 있어 rstrip 만 적용 — strip
//     안 함.
//  3. 블록의 마지막 라인이 보통 `<TypeName>: <message>` — 단, Python
//     이 종종 traceback 사이에 context message 를 emit 하므로,
//     블록을 끝에서부터 거꾸로 스캔해 마지막 매칭(승자) 사용.
//
// frame 정규식
//
//	`^\s*File "(?P<file>[^"]+)", line (?P<line>\d+), in (?P<func>\S+)`
//	파이프 등에서 잘려도 정상 매칭되도록 file/line/func 만 캡처.
//
// 다중 traceback
//
//	같은 파일에 여러 traceback 이 있으면 헤더 등장마다 블록 분리.
//	각 블록이 1개 RuntimeStackRecord.
//
// chained exceptions ("During handling of the above exception, ...")
//
//	현재는 별도 record 로 처리하지 않고 같은 블록 안에 텍스트로 보존.
//	분석기 측이 필요시 raw_block 에서 추출 가능.
package runtimestack

import (
	"errors"
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

	type pending struct {
		startLine int
		body      []string
	}
	records := make([]RuntimeStackRecord, 0)
	var current pending
	currentActive := false

	flush := func() {
		if currentActive {
			record, ok := parsePythonTracebackBlock(current.body)
			if !ok {
				diags.AddSkipped(current.startLine, ReasonInvalidPythonTraceback,
					"Python traceback block was missing a terminal exception line.",
					strings.Join(current.body, "\n"))
			} else {
				records = append(records, record)
				diags.ParsedRecords++
			}
		}
		currentActive = false
		current = pending{}
	}

	err := textio.ForEachTextLine(path, "", func(lineNumber int, line string) error {
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			return errStopIteration
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
				return fmt.Errorf("%s:%d: %s: %s", path, lineNumber, reason, message)
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopIteration) {
		return records, diags, err
	}
	flush()

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
