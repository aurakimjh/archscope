// Ports archscope_engine.parsers.dotnet_parser. The input is a mixed
// stream of .NET exception blocks (header + `at ...` frames) and IIS
// W3C access lines preceded by `#Fields: ...` directives. The parser
// dispatches per-line: exception headers start/flush blocks, IIS
// `#Fields:` resets state and captures the field schema, normal lines
// are decoded against the IIS schema if one is active, and stray
// content is reported as `UNSUPPORTED_DOTNET_OR_IIS_LINE`.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] dotnet parser — .NET 예외 + IIS W3C access 혼합 스트림.
//
// 입력 형태 (실세계 케이스)
//
//	IIS 환경의 .NET 애플리케이션 로그는 종종 access log 와 예외 dump
//	가 같은 파일에 섞여 있습니다. 운영자가 grep 등으로 한 케이스만
//	추출하기 어려운 경우가 많아, 본 파서는 한 입력으로 둘 다 처리.
//
// 라인 단위 dispatch 알고리즘
//  1. `<TypeName>Exception: message` 매칭 → 새 예외 블록 시작.
//     이전 블록이 있다면 flush 후 시작.
//  2. `#Fields: <col1> <col2> ...` 지시어 만나면 IIS 스키마 캡처
//     (필드 순서 저장). 진행중이던 예외 블록은 flush.
//  3. IIS 스키마가 활성 상태에서 정상 라인이 오면 IIS access record
//     로 디코드 (스페이스 split).
//  4. 예외 블록이 활성이면 `at <method> in <file>:line N` 같은 frame
//     라인을 stack 에 누적.
//  5. 어디에도 안 맞는 라인은 UNSUPPORTED_DOTNET_OR_IIS_LINE 사유로
//     diagnostics 에 기록 후 skip.
//
// dotnetExceptionRE
//
//	타입명은 반드시 "Exception" 으로 끝나야 함 (예: NullReference
//	Exception, InvalidOperationException). message 는 `:` 뒤의 임의
//	텍스트.
//
// async state machine 정리
//
//	.NET 의 async 메서드 stack frame 은 컴파일러가 `<MethodName>d__N`
//	형태로 mangle 하므로, 분석기 측에서 가독성 개선용 정리 처리.
//	파서 단계는 raw 보존.
package runtimestack

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

// dotnetExceptionRE mirrors Python's DOTNET_EXCEPTION_RE. The type
// must end in "Exception"; the message is everything after the
// optional ":" + run of spaces.
var dotnetExceptionRE = regexp.MustCompile(
	`^(?P<type>[A-Za-z_][\w.]*Exception)(?::\s*(?P<message>.*))?$`,
)

const iisFieldsPrefix = "#Fields:"

// Reasons surfaced through diagnostics.
const (
	ReasonUnsupportedDotnetOrIisLine = "UNSUPPORTED_DOTNET_OR_IIS_LINE"
)

// ParseDotnetFile mirrors `parse_dotnet_exception_and_iis`. The two
// return slices are independent — exception records and IIS access
// records share a source file but are surfaced separately so the
// downstream exception/runtime analyzers can join them.
func ParseDotnetFile(path string, opts Options) ([]RuntimeStackRecord, []IisAccessRecord, *diagnostics.ParserDiagnostics, error) {
	if opts.MaxLines < 0 {
		return nil, nil, nil, fmt.Errorf("max_lines must be a positive integer")
	}

	diags := diagnostics.New("dotnet_exception_iis")
	diags.SetSourceFile(path)

	exceptions := make([]RuntimeStackRecord, 0)
	iisRecords := make([]IisAccessRecord, 0)
	var current []string
	var fields []string

	flush := func() {
		if len(current) == 0 {
			return
		}
		if record, ok := parseDotnetExceptionBlock(current); ok {
			exceptions = append(exceptions, record)
			diags.ParsedRecords++
		}
		current = nil
	}

	err := textio.ForEachTextLine(path, "", func(lineNumber int, line string) error {
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			return errStopIteration
		}
		diags.TotalLines++

		stripped := strings.TrimSpace(line)
		if stripped == "" {
			flush()
			return nil
		}
		if strings.HasPrefix(stripped, iisFieldsPrefix) {
			flush()
			fieldsStr := strings.TrimSpace(stripped[len(iisFieldsPrefix):])
			fields = strings.Fields(fieldsStr)
			return nil
		}
		if strings.HasPrefix(stripped, "#") {
			return nil
		}
		if len(fields) > 0 {
			if record, ok := parseIisLine(stripped, fields); ok {
				iisRecords = append(iisRecords, record)
				diags.ParsedRecords++
				return nil
			}
		}
		if dotnetExceptionRE.MatchString(stripped) {
			flush()
			current = []string{stripped}
			return nil
		}
		if len(current) > 0 && strings.HasPrefix(stripped, "at ") {
			current = append(current, stripped)
			return nil
		}

		reason := ReasonUnsupportedDotnetOrIisLine
		message := "Line did not match .NET exception or IIS W3C access fields."
		diags.AddSkipped(lineNumber, reason, message, line)
		if opts.Strict {
			flush()
			return fmt.Errorf("%s:%d: %s: %s", path, lineNumber, reason, message)
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopIteration) {
		return exceptions, iisRecords, diags, err
	}

	flush()

	if diags.TotalLines == 0 {
		diags.AddWarning(0, "EMPTY_FILE", ".NET / IIS log file is empty.", "", false)
	}
	return exceptions, iisRecords, diags, nil
}

// parseDotnetExceptionBlock mirrors Python's `_parse_exception_block`.
// The dead-branch guard from Python (`if header is None: return None`)
// is preserved for parity even though our caller only feeds blocks
// whose first line already passed the regex.
func parseDotnetExceptionBlock(block []string) (RuntimeStackRecord, bool) {
	indices := dotnetExceptionRE.FindStringSubmatchIndex(block[0])
	if indices == nil {
		return RuntimeStackRecord{}, false
	}
	typeIdx := dotnetExceptionRE.SubexpIndex("type")
	msgIdx := dotnetExceptionRE.SubexpIndex("message")

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
		Runtime:    "dotnet",
		RecordType: errorType,
		Headline:   errorType,
		Message:    message,
		Stack:      stack,
		Signature:  fmt.Sprintf("%s|%s", errorType, topFrame),
		RawBlock:   strings.Join(block, "\n"),
	}, true
}

// parseIisLine mirrors `_parse_iis_line`. Requires len(values) >=
// len(fields) and presence of (cs-method, cs-uri-stem, sc-status as
// int). `time-taken` is best-effort.
func parseIisLine(line string, fields []string) (IisAccessRecord, bool) {
	values := strings.Fields(line)
	if len(values) < len(fields) {
		return IisAccessRecord{}, false
	}
	row := make(map[string]string, len(fields))
	for i, name := range fields {
		row[name] = values[i]
	}
	method, hasMethod := row["cs-method"]
	uri, hasURI := row["cs-uri-stem"]
	statusRaw, hasStatus := row["sc-status"]
	if !hasMethod || !hasURI || !hasStatus {
		return IisAccessRecord{}, false
	}
	status, ok := iisInt(statusRaw)
	if !ok {
		return IisAccessRecord{}, false
	}
	var timeTakenPtr *int
	if raw, ok := row["time-taken"]; ok {
		if t, ok2 := iisInt(raw); ok2 {
			timeTakenPtr = intPtr(t)
		}
	}
	return IisAccessRecord{
		Method:      method,
		URI:         uri,
		Status:      status,
		TimeTakenMS: timeTakenPtr,
		RawLine:     line,
	}, true
}

// iisInt mirrors Python's `_int` — `-` and unparseable values return
// (0, false); integer tokens return (n, true).
func iisInt(value string) (int, bool) {
	if value == "" || value == "-" {
		return 0, false
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return n, true
}
