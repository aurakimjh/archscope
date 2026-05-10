// Package accesslog ports archscope_engine.parsers.access_log_parser.
//
// Three regex variants are tried in order — nginx (with trailing
// response time), combined (referer + user-agent, no response time),
// common (no referer / user-agent / response time). All seven Python
// "supported formats" share the same parser; the format string is a
// label only, surfaced via diagnostics so renderers can group reports.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] accesslog parser — HTTP 액세스 로그 라인을 Record 구조체로 변환.
//
// 지원 포맷 (3종 regex 우선순위)
//
//  1. nginx-combined-with-response-time: combined + 끝에 응답시간(초).
//
//  2. combined: referer + user-agent + 응답시간 없음.
//
//  3. common: client IP + status + bytes 만(제일 단순).
//
//     라인을 위에서부터 시도해 첫 매칭이 채택. 정확히 어떤 포맷인지는
//     분석기의 `format` 라벨로 사용자에게 노출되지만, 파싱 자체는 위
//     3개로 dispatch — 실제 파일이 mixed 라도 라인 단위로 가능한 한
//     많이 살림.
//
// 시간 파싱
//
//	nginx 형식: `27/Apr/2026:10:00:01 +0900`. 월 약어를 numeric 으로
//	변환해 time.Parse 의 reference layout 으로 처리.
//
// 라인 손상 정책 (T-013)
//   - 정상 미매칭        : skip + diagnostics.SkippedRecords++
//   - 매칭됐으나 status 가 숫자 아님 : skip + 별도 reason 카운트.
//   - 응답시간 음수 / NaN: 0 으로 강제 + warning.
//   - Strict=true        : 첫 skip 이 fatal error 로 즉시 반환.
//
// MaxLines / 시간 윈도우 필터
//
//	사용자가 큰 로그를 빠르게 sampling 하거나(MaxLines), 특정 분 단위
//	구간만 분석(StartTime/EndTime)할 수 있도록 옵션 제공.
package accesslog

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/timeutil"
)

var errStopIteration = errors.New("stop access log iteration")

// Record is the parsed access-log row. Mirrors Python's
// `models.access_log.AccessLogRecord` field-for-field; `RawLine` is
// kept so analyzers / debug logs can re-emit the source verbatim.
type Record struct {
	Timestamp      time.Time
	Method         string
	URI            string
	Status         int
	ResponseTimeMS float64
	BytesSent      int
	ClientIP       string
	UserAgent      string
	Referer        string
	RawLine        string
}

// SupportedFormats matches Python's SUPPORTED_LOG_FORMATS — only used
// to validate the user-supplied format label, not to dispatch to
// different regexes. Keep alphabetised because diagnostics surfaces
// the sorted list when rejecting.
var SupportedFormats = map[string]struct{}{
	"apache":   {},
	"combined": {},
	"common":   {},
	"nginx":    {},
	"ohs":      {},
	"tomcat":   {},
	"weblogic": {},
}

// SortedSupportedFormats returns the format keys in deterministic
// order — used to produce stable error messages.
func SortedSupportedFormats() []string {
	out := make([]string, 0, len(SupportedFormats))
	for k := range SupportedFormats {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Skip reasons (mirror Python).
const (
	ReasonNoFormatMatch    = "NO_FORMAT_MATCH"
	ReasonInvalidTimestamp = "INVALID_TIMESTAMP"
	ReasonInvalidNumber    = "INVALID_NUMBER"
)

var (
	// `^(client_ip) (\S+) (\S+) [(timestamp)] "(method) (uri) (protocol)" (status) (bytes) "(referer)" "(user_agent)" (response_time_sec)$`
	nginxWithResponseTime = regexp.MustCompile(
		`^(?P<client_ip>\S+) \S+ \S+ \[(?P<timestamp>[^\]]+)\] ` +
			`"(?P<method>\S+) (?P<uri>\S+) (?P<protocol>[^"]+)" ` +
			`(?P<status>\S+) (?P<bytes_sent>\S+) ` +
			`"(?P<referer>[^"]*)" "(?P<user_agent>[^"]*)" ` +
			`(?P<response_time_sec>\S+)$`,
	)
	combinedAccessLog = regexp.MustCompile(
		`^(?P<client_ip>\S+) \S+ \S+ \[(?P<timestamp>[^\]]+)\] ` +
			`"(?P<method>\S+) (?P<uri>\S+) (?P<protocol>[^"]+)" ` +
			`(?P<status>\S+) (?P<bytes_sent>\S+) ` +
			`"(?P<referer>[^"]*)" "(?P<user_agent>[^"]*)"$`,
	)
	commonAccessLog = regexp.MustCompile(
		`^(?P<client_ip>\S+) \S+ \S+ \[(?P<timestamp>[^\]]+)\] ` +
			`"(?P<method>\S+) (?P<uri>\S+) (?P<protocol>[^"]+)" ` +
			`(?P<status>\S+) (?P<bytes_sent>\S+)$`,
	)
)

// parseError carries the (reason, message) pair Python uses to
// classify malformed lines. Returned alongside a nil record from
// ParseLine when the line is rejectable.
type parseError struct {
	Reason  string
	Message string
}

// ParseLine attempts to parse a single non-empty access-log line.
// Returns (record, nil) on success, (nil, *parseError) on a
// classified rejection. Mirrors Python `_parse_nginx_access_line`.
func ParseLine(line string) (*Record, *parseError) {
	if record, perr, ok := fastParseNginxWithResponseTime(line); ok {
		return record, perr
	}

	groups, hasResponseTime := matchAny(line)
	if groups == nil {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "Line does not match supported access log formats."}
	}

	ts, err := timeutil.ParseNginxTimestamp(groups["timestamp"])
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidTimestamp, Message: "Timestamp does not match nginx format."}
	}

	status, err := strconv.Atoi(groups["status"])
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "Numeric field could not be parsed."}
	}
	bytesSent := 0
	if groups["bytes_sent"] != "-" {
		bytesSent, err = strconv.Atoi(groups["bytes_sent"])
		if err != nil {
			return nil, &parseError{Reason: ReasonInvalidNumber, Message: "Numeric field could not be parsed."}
		}
	}
	var responseTimeMS float64
	if hasResponseTime {
		raw := groups["response_time_sec"]
		sec, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, &parseError{Reason: ReasonInvalidNumber, Message: "Numeric field could not be parsed."}
		}
		responseTimeMS = sec * 1000
	}

	if status < 100 || status > 999 || bytesSent < 0 || math.IsNaN(responseTimeMS) || math.IsInf(responseTimeMS, 0) || responseTimeMS < 0 {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "Numeric field is outside the valid range."}
	}

	return &Record{
		Timestamp:      ts,
		Method:         groups["method"],
		URI:            groups["uri"],
		Status:         status,
		ResponseTimeMS: responseTimeMS,
		BytesSent:      bytesSent,
		ClientIP:       groups["client_ip"],
		UserAgent:      groups["user_agent"],
		Referer:        groups["referer"],
		RawLine:        line,
	}, nil
}

func fastParseNginxWithResponseTime(line string) (*Record, *parseError, bool) {
	clientEnd := strings.IndexByte(line, ' ')
	if clientEnd <= 0 {
		return nil, nil, false
	}
	clientIP := line[:clientEnd]

	openTS := strings.IndexByte(line[clientEnd+1:], '[')
	if openTS < 0 {
		return nil, nil, false
	}
	openTS += clientEnd + 1
	closeTS := strings.IndexByte(line[openTS+1:], ']')
	if closeTS < 0 {
		return nil, nil, false
	}
	closeTS += openTS + 1

	pos := skipSpaces(line, closeTS+1)
	request, pos, ok := nextQuotedField(line, pos)
	if !ok {
		return nil, nil, false
	}
	method, uri, ok := splitRequestLine(request)
	if !ok {
		return nil, nil, false
	}

	pos = skipSpaces(line, pos)
	statusText, pos, ok := nextToken(line, pos)
	if !ok {
		return nil, nil, false
	}
	bytesText, pos, ok := nextToken(line, pos)
	if !ok {
		return nil, nil, false
	}
	pos = skipSpaces(line, pos)
	referer, pos, ok := nextQuotedField(line, pos)
	if !ok {
		return nil, nil, false
	}
	pos = skipSpaces(line, pos)
	userAgent, pos, ok := nextQuotedField(line, pos)
	if !ok {
		return nil, nil, false
	}
	pos = skipSpaces(line, pos)
	responseTimeText := line[pos:]
	if responseTimeText == "" || strings.IndexFunc(responseTimeText, unicode.IsSpace) >= 0 {
		return nil, nil, false
	}

	ts, err := timeutil.ParseNginxTimestamp(line[openTS+1 : closeTS])
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidTimestamp, Message: "Timestamp does not match nginx format."}, true
	}
	status, err := strconv.Atoi(statusText)
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "Numeric field could not be parsed."}, true
	}
	bytesSent := 0
	if bytesText != "-" {
		bytesSent, err = strconv.Atoi(bytesText)
		if err != nil {
			return nil, &parseError{Reason: ReasonInvalidNumber, Message: "Numeric field could not be parsed."}, true
		}
	}
	sec, err := strconv.ParseFloat(responseTimeText, 64)
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "Numeric field could not be parsed."}, true
	}
	responseTimeMS := sec * 1000
	if status < 100 || status > 999 || bytesSent < 0 || math.IsNaN(responseTimeMS) || math.IsInf(responseTimeMS, 0) || responseTimeMS < 0 {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "Numeric field is outside the valid range."}, true
	}

	return &Record{
		Timestamp:      ts,
		Method:         method,
		URI:            uri,
		Status:         status,
		ResponseTimeMS: responseTimeMS,
		BytesSent:      bytesSent,
		ClientIP:       clientIP,
		UserAgent:      userAgent,
		Referer:        referer,
		RawLine:        line,
	}, nil, true
}

func skipSpaces(line string, pos int) int {
	for pos < len(line) && line[pos] == ' ' {
		pos++
	}
	return pos
}

func nextToken(line string, pos int) (string, int, bool) {
	if pos >= len(line) || line[pos] == ' ' {
		return "", pos, false
	}
	end := strings.IndexByte(line[pos:], ' ')
	if end < 0 {
		return line[pos:], len(line), true
	}
	end += pos
	return line[pos:end], skipSpaces(line, end), true
}

func nextQuotedField(line string, pos int) (string, int, bool) {
	if pos >= len(line) || line[pos] != '"' {
		return "", pos, false
	}
	end := strings.IndexByte(line[pos+1:], '"')
	if end < 0 {
		return "", pos, false
	}
	end += pos + 1
	return line[pos+1 : end], end + 1, true
}

func splitRequestLine(request string) (method, uri string, ok bool) {
	first := strings.IndexByte(request, ' ')
	if first <= 0 {
		return "", "", false
	}
	rest := request[first+1:]
	second := strings.IndexByte(rest, ' ')
	if second <= 0 || second == len(rest)-1 {
		return "", "", false
	}
	return request[:first], rest[:second], true
}

// matchAny tries the three regex variants in order, returning the
// captured group dict and whether the variant carried a trailing
// response_time field.
func matchAny(line string) (map[string]string, bool) {
	if g := captureGroups(nginxWithResponseTime, line); g != nil {
		return g, true
	}
	if g := captureGroups(combinedAccessLog, line); g != nil {
		return g, false
	}
	if g := captureGroups(commonAccessLog, line); g != nil {
		return g, false
	}
	return nil, false
}

func captureGroups(re *regexp.Regexp, line string) map[string]string {
	match := re.FindStringSubmatch(line)
	if match == nil {
		return nil
	}
	groups := make(map[string]string, len(re.SubexpNames()))
	for i, name := range re.SubexpNames() {
		if name == "" {
			continue
		}
		groups[name] = match[i]
	}
	return groups
}

// Options carries the parser-level filters Python exposes — same names
// (max_lines, start_time, end_time, strict). Time pointers double as
// "filter unset" sentinels.
type Options struct {
	MaxLines  int
	StartTime *time.Time
	EndTime   *time.Time
	Strict    bool
}

// ParseFile reads `path` end-to-end and returns the parsed records
// alongside a populated diagnostics builder. Mirrors Python
// `parse_access_log_with_diagnostics`. Format defaults to "nginx".
//
// On `Strict=true`, the first malformed line is returned as a
// non-nil error after the diagnostics row has been recorded — same
// behaviour as Python's `ValueError` from inside the iterator.
func ParseFile(path, format string, opts Options) ([]Record, *diagnostics.ParserDiagnostics, error) {
	records := make([]Record, 0, 1024)
	diags, err := ForEachRecord(path, format, opts, func(record Record) error {
		records = append(records, record)
		return nil
	})
	if err != nil {
		return records, diags, err
	}
	return records, diags, nil
}

// ForEachRecord streams parsed records from `path` into fn without retaining
// the whole record set. Diagnostics are returned after iteration completes or
// stops with a parse/callback error.
func ForEachRecord(path, format string, opts Options, fn func(Record) error) (*diagnostics.ParserDiagnostics, error) {
	if format == "" {
		format = "nginx"
	}
	if _, ok := SupportedFormats[format]; !ok {
		formats := SortedSupportedFormats()
		joined := ""
		for i, f := range formats {
			if i > 0 {
				joined += ", "
			}
			joined += f
		}
		return nil, fmt.Errorf("Unsupported access log format. Supported formats: %s", joined)
	}
	if opts.MaxLines < 0 {
		return nil, fmt.Errorf("max_lines must be a positive integer")
	}

	diags := diagnostics.New(format)
	diags.SetSourceFile(path)

	err := textio.ForEachTextLine(path, "", func(lineNumber int, line string) error {
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			return errStopIteration
		}
		diags.TotalLines++
		if isBlank(line) {
			return nil
		}
		record, perr := ParseLine(line)
		if record != nil {
			if !inTimeRange(record.Timestamp, opts.StartTime, opts.EndTime) {
				return nil
			}
			diags.ParsedRecords++
			return fn(*record)
		}
		if perr == nil {
			return fmt.Errorf("access log parser returned neither record nor error")
		}
		diags.AddSkipped(lineNumber, perr.Reason, perr.Message, line)
		if opts.Strict {
			return fmt.Errorf("%s:%d: %s: %s", path, lineNumber, perr.Reason, perr.Message)
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopIteration) {
		return diags, err
	}

	if diags.TotalLines == 0 {
		diags.AddWarning(0, "EMPTY_FILE", "Access log file is empty.", "", false)
	}
	return diags, nil
}

func isBlank(line string) bool {
	for _, r := range line {
		if r != ' ' && r != '\t' && r != '\r' && r != '\n' {
			return false
		}
	}
	return true
}

// inTimeRange mirrors `_is_in_time_range`. Go time comparisons are
// absolute (timezone-aware by construction) so the Python tz-coerce
// branch is unnecessary here.
func inTimeRange(value time.Time, start, end *time.Time) bool {
	if start != nil && value.Before(*start) {
		return false
	}
	if end != nil && value.After(*end) {
		return false
	}
	return true
}
