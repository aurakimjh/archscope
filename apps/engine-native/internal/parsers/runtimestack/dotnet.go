// Ports archscope_engine.parsers.dotnet_parser. The input is a mixed
// stream of .NET exception blocks (header + `at ...` frames) and IIS
// W3C access lines preceded by `#Fields: ...` directives. The parser
// dispatches per-line: exception headers start/flush blocks, IIS
// `#Fields:` resets state and captures the field schema, normal lines
// are decoded against the IIS schema if one is active, and stray
// content is reported as `UNSUPPORTED_DOTNET_OR_IIS_LINE`.
package runtimestack

import (
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

	lines, err := textio.IterTextLines(path, "")
	if err != nil {
		return nil, nil, nil, err
	}

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

	for i, line := range lines {
		lineNumber := i + 1
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			break
		}
		diags.TotalLines++

		stripped := strings.TrimSpace(line)
		if stripped == "" {
			flush()
			continue
		}
		if strings.HasPrefix(stripped, iisFieldsPrefix) {
			flush()
			fieldsStr := strings.TrimSpace(stripped[len(iisFieldsPrefix):])
			fields = strings.Fields(fieldsStr)
			continue
		}
		if strings.HasPrefix(stripped, "#") {
			continue
		}
		if len(fields) > 0 {
			if record, ok := parseIisLine(stripped, fields); ok {
				iisRecords = append(iisRecords, record)
				diags.ParsedRecords++
				continue
			}
		}
		if dotnetExceptionRE.MatchString(stripped) {
			flush()
			current = []string{stripped}
			continue
		}
		if len(current) > 0 && strings.HasPrefix(stripped, "at ") {
			current = append(current, stripped)
			continue
		}

		reason := ReasonUnsupportedDotnetOrIisLine
		message := "Line did not match .NET exception or IIS W3C access fields."
		diags.AddSkipped(lineNumber, reason, message, line)
		if opts.Strict {
			flush()
			return exceptions, iisRecords, diags, fmt.Errorf("%s:%d: %s: %s", path, lineNumber, reason, message)
		}
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
