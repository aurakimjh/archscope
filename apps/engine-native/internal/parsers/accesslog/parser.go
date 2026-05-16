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
	"encoding/json"
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
	SourceFormat   string

	Host              string
	Protocol          string
	Scheme            string
	ServiceName       string
	UpstreamService   string
	UpstreamCluster   string
	Route             string
	BackendName       string
	BackendServer     string
	GatewayLatencyMS  float64
	UpstreamLatencyMS float64
	TLSVersion        string
	TLSCipher         string
	ResponseFlags     string
	TerminationState  string
	RetryCount        int
	TraceID           string
	RequestID         string
	Consumer          string
	CloudProvider     string
	EdgeLocation      string
}

// SupportedFormats matches Python's SUPPORTED_LOG_FORMATS — only used
// to validate the user-supplied format label, not to dispatch to
// different regexes. Keep alphabetised because diagnostics surfaces
// the sorted list when rejecting.
var SupportedFormats = map[string]struct{}{
	"apache":                 {},
	"auto":                   {},
	"aws-alb":                {},
	"aws-api-gateway-json":   {},
	"aws-cloudfront":         {},
	"aws-elb":                {},
	"azure-app-service-json": {},
	"azure-front-door-json":  {},
	"caddy-json":             {},
	"combined":               {},
	"common":                 {},
	"envoy-json":             {},
	"envoy-text":             {},
	"gcp-http-lb-json":       {},
	"haproxy":                {},
	"haproxy-http":           {},
	"iis-w3c":                {},
	"istio-json":             {},
	"istio-text":             {},
	"jetty":                  {},
	"kong-json":              {},
	"nginx":                  {},
	"ohs":                    {},
	"tomcat":                 {},
	"traefik-json":           {},
	"tyk-json":               {},
	"weblogic":               {},
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
	ReasonInvalidJSON      = "INVALID_JSON"
	ReasonMissingW3CFields = "MISSING_W3C_FIELDS"
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
	return ParseLineForFormat(line, "nginx")
}

// ParseLineForFormat parses a single line using a selected or auto-detected
// access/edge-log format. "nginx" keeps the legacy combined/common tolerant
// parser used by ParseLine.
func ParseLineForFormat(line, format string) (*Record, *parseError) {
	format = canonicalFormat(format)
	if format == "auto" {
		if detected := DetectLineFormat(line); detected != "" {
			return ParseLineForFormat(line, detected)
		}
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "Line does not match supported access or edge log formats."}
	}
	switch format {
	case "nginx", "apache", "combined", "common", "ohs", "tomcat", "weblogic", "jetty":
		return parseCombinedLikeLine(line, format)
	case "haproxy", "haproxy-http":
		return parseHAProxyLine(line, format)
	case "envoy-text", "istio-text":
		return parseEnvoyTextLine(line, format)
	case "envoy-json", "istio-json":
		return parseEnvoyJSONLine(line, format)
	case "aws-elb":
		return parseAWSELBLine(line, format)
	case "aws-alb":
		return parseAWSALBLine(line, format)
	case "aws-cloudfront":
		return parseCloudFrontLine(line, format)
	case "gcp-http-lb-json":
		return parseGCPHTTPLBJSONLine(line, format)
	case "azure-app-service-json", "azure-front-door-json":
		return parseAzureJSONLine(line, format)
	case "caddy-json":
		return parseCaddyJSONLine(line, format)
	case "traefik-json":
		return parseTraefikJSONLine(line, format)
	case "kong-json":
		return parseKongJSONLine(line, format)
	case "tyk-json":
		return parseTykJSONLine(line, format)
	case "aws-api-gateway-json":
		return parseAPIGatewayJSONLine(line, format)
	default:
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "Unsupported access log format selector."}
	}
}

func parseCombinedLikeLine(line, format string) (*Record, *parseError) {
	if record, perr, ok := fastParseNginxWithResponseTime(line); ok {
		if record != nil {
			record.SourceFormat = format
		}
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
		SourceFormat:   format,
		Protocol:       groups["protocol"],
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
		SourceFormat:   "nginx",
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

func DetectLineFormat(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "{") {
		payload, err := decodeJSONObject(trimmed)
		if err != nil {
			return ""
		}
		switch {
		case payloadHas(payload, "httpRequest"):
			return "gcp-http-lb-json"
		case payloadHas(payload, "DownstreamStatus", "OriginStatus", "ServiceName", "RouterName"):
			return "traefik-json"
		case payloadHas(payload, "latencies", "consumer", "service") || (payloadHas(payload, "request", "response") && payloadHas(payload, "route")):
			return "kong-json"
		case payloadHas(payload, "api_name", "org_id") || payloadHas(payload, "api_id"):
			return "tyk-json"
		case payloadHas(payload, "requestContext", "requestTimeEpoch", "integrationLatency", "responseLatency"):
			return "aws-api-gateway-json"
		case payloadHas(payload, "httpRequest", "resource"):
			return "gcp-http-lb-json"
		case payloadHas(payload, "timeTaken", "category", "operationName") || payloadHas(payload, "backendHostname", "trackingReference"):
			return "azure-front-door-json"
		case payloadHas(payload, "request", "duration") && payloadHas(payload, "status"):
			return "caddy-json"
		case payloadHas(payload, "response_code", "upstream_cluster") || payloadHas(payload, "response_flags", "upstream_cluster", "authority"):
			return "envoy-json"
		default:
			return ""
		}
	}
	if strings.HasPrefix(trimmed, "#Fields:") {
		return "iis-w3c"
	}
	if strings.Contains(trimmed, " haproxy[") || strings.Contains(trimmed, " haproxy:") {
		return "haproxy-http"
	}
	if strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, `] "`) {
		return "envoy-text"
	}
	fields := splitLogFields(trimmed)
	if len(fields) >= 12 && (fields[0] == "http" || fields[0] == "https" || fields[0] == "h2" || fields[0] == "ws" || fields[0] == "wss") {
		return "aws-alb"
	}
	if len(fields) >= 12 && strings.Contains(fields[0], "T") && strings.Contains(fields[1], "loadbalancer") {
		return "aws-elb"
	}
	if strings.Contains(trimmed, "\t") {
		parts := strings.Split(trimmed, "\t")
		if len(parts) >= 19 && looksLikeDate(parts[0]) {
			return "aws-cloudfront"
		}
	}
	if _, perr := parseCombinedLikeLine(trimmed, "nginx"); perr == nil {
		return "nginx"
	}
	return ""
}

func parseHAProxyLine(line, format string) (*Record, *parseError) {
	idx := strings.Index(line, ": ")
	if idx < 0 {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "HAProxy log line is missing syslog separator."}
	}
	payload := line[idx+2:]
	fields := splitLogFields(payload)
	if len(fields) < 11 {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "HAProxy HTTP log line has too few fields."}
	}
	client := strings.Split(fields[0], ":")[0]
	ts, err := parseHAProxyTimestamp(strings.Trim(fields[1], "[]"))
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidTimestamp, Message: "HAProxy timestamp could not be parsed."}
	}
	backendServer := fields[3]
	backendName := backendServer
	serverName := ""
	if parts := strings.SplitN(backendServer, "/", 2); len(parts) == 2 {
		backendName = parts[0]
		serverName = parts[1]
	}
	timings := strings.Split(fields[4], "/")
	if len(timings) < 5 {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "HAProxy timing fields could not be parsed."}
	}
	upstreamMS := parsePositiveFloat(timings[3])
	totalMS := parsePositiveFloat(timings[4])
	status, err := strconv.Atoi(fields[5])
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "HAProxy status could not be parsed."}
	}
	bytesSent, err := parseOptionalInt(fields[6])
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "HAProxy byte count could not be parsed."}
	}
	termination := ""
	retries := 0
	for i, field := range fields {
		if len(field) >= 4 && strings.ContainsAny(field, "-CSDPRLQIT") && strings.IndexByte(field, '/') < 0 && strings.IndexByte(field, '"') < 0 {
			termination = field
			if i+2 < len(fields) {
				retryParts := strings.Split(fields[i+2], "/")
				if len(retryParts) > 0 {
					retries, _ = strconv.Atoi(nonNegativeToken(retryParts[0]))
				}
			}
			break
		}
	}
	request := fields[len(fields)-1]
	method, uri, ok := splitRequestLine(request)
	if !ok {
		method = "UNKNOWN"
		uri = request
	}
	record := &Record{
		Timestamp:         ts,
		Method:            method,
		URI:               uri,
		Status:            status,
		ResponseTimeMS:    totalMS,
		BytesSent:         bytesSent,
		ClientIP:          client,
		RawLine:           line,
		SourceFormat:      format,
		BackendName:       backendName,
		BackendServer:     serverName,
		UpstreamService:   backendName,
		UpstreamLatencyMS: upstreamMS,
		GatewayLatencyMS:  math.Max(0, totalMS-upstreamMS),
		TerminationState:  termination,
		RetryCount:        retries,
		ServiceName:       "haproxy",
	}
	return validateRecord(record)
}

func parseEnvoyTextLine(line, format string) (*Record, *parseError) {
	groups := captureGroups(envoyTextLog, line)
	if groups == nil {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "Envoy/Istio text log line does not match the default text format."}
	}
	ts, err := time.Parse(time.RFC3339Nano, groups["timestamp"])
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidTimestamp, Message: "Envoy timestamp could not be parsed."}
	}
	status, err := strconv.Atoi(groups["status"])
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "Envoy status could not be parsed."}
	}
	bytesOut, _ := strconv.Atoi(nonNegativeToken(groups["bytes_out"]))
	durationMS := parsePositiveFloat(groups["duration_ms"])
	upstreamMS := parsePositiveFloat(groups["upstream_ms"])
	request := groups["request"]
	method, uri, ok := splitRequestLine(request)
	if !ok {
		method = "UNKNOWN"
		uri = request
	}
	record := &Record{
		Timestamp:         ts,
		Method:            method,
		URI:               uri,
		Status:            status,
		ResponseTimeMS:    durationMS,
		BytesSent:         bytesOut,
		UserAgent:         groups["user_agent"],
		RawLine:           line,
		SourceFormat:      format,
		Host:              groups["authority"],
		UpstreamService:   normalizeClusterService(groups["upstream_cluster"]),
		UpstreamCluster:   groups["upstream_cluster"],
		ResponseFlags:     groups["flags"],
		TraceID:           groups["request_id"],
		RequestID:         groups["request_id"],
		UpstreamLatencyMS: upstreamMS,
		GatewayLatencyMS:  math.Max(0, durationMS-upstreamMS),
		ServiceName:       "envoy",
	}
	return validateRecord(record)
}

func parseEnvoyJSONLine(line, format string) (*Record, *parseError) {
	payload, err := decodeJSONObject(line)
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidJSON, Message: err.Error()}
	}
	ts := firstJSONTime(payload, "start_time", "timestamp", "time", "@timestamp")
	status := intFromAny(firstAny(payload, "response_code", "status", "status_code"))
	durationMS := durationToMS(firstAny(payload, "duration", "duration_ms", "request_duration", "request_duration_ms"))
	upstreamMS := durationToMS(firstAny(payload, "upstream_service_time", "upstream_service_time_ms", "upstream_duration_ms"))
	request := stringValue(firstAny(payload, "request", "request_line"))
	method := stringValue(firstAny(payload, "method", "request_method"))
	uri := stringValue(firstAny(payload, "path", "uri", "request_path"))
	if request != "" {
		if m, u, ok := splitRequestLine(request); ok {
			method = firstNonEmpty(method, m)
			uri = firstNonEmpty(uri, u)
		}
	}
	if method == "" {
		method = "UNKNOWN"
	}
	record := &Record{
		Timestamp:         ts,
		Method:            method,
		URI:               firstNonEmpty(pathFromMaybeURL(uri), "/"),
		Status:            status,
		ResponseTimeMS:    durationMS,
		BytesSent:         intFromAny(firstAny(payload, "bytes_sent", "response_size", "response_bytes")),
		ClientIP:          firstNonEmpty(stringValue(firstAny(payload, "downstream_remote_address", "client_ip")), hostPart(stringValue(firstAny(payload, "x_forwarded_for")))),
		UserAgent:         stringValue(firstAny(payload, "user_agent")),
		RawLine:           line,
		SourceFormat:      format,
		Host:              stringValue(firstAny(payload, "authority", "host", "request_host")),
		Protocol:          stringValue(firstAny(payload, "protocol")),
		Route:             stringValue(firstAny(payload, "route_name", "route")),
		UpstreamCluster:   stringValue(firstAny(payload, "upstream_cluster")),
		UpstreamService:   firstNonEmpty(stringValue(firstAny(payload, "upstream_service", "destination_service")), normalizeClusterService(stringValue(firstAny(payload, "upstream_cluster")))),
		ResponseFlags:     stringValue(firstAny(payload, "response_flags")),
		TraceID:           firstNonEmpty(stringValue(firstAny(payload, "trace_id")), stringValue(firstAny(payload, "x_b3_traceid"))),
		RequestID:         firstNonEmpty(stringValue(firstAny(payload, "request_id")), stringValue(firstAny(payload, "x_request_id"))),
		UpstreamLatencyMS: upstreamMS,
		GatewayLatencyMS:  math.Max(0, durationMS-upstreamMS),
		ServiceName:       firstNonEmpty(stringValue(firstAny(payload, "service_name", "source_service")), "envoy"),
	}
	return validateRecord(record)
}

func parseAWSELBLine(line, format string) (*Record, *parseError) {
	fields := splitLogFields(line)
	if len(fields) < 12 {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "AWS ELB log line has too few fields."}
	}
	ts, err := parseAccessISOTime(fields[0])
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidTimestamp, Message: "AWS ELB timestamp could not be parsed."}
	}
	status, err := strconv.Atoi(fields[7])
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "AWS ELB status could not be parsed."}
	}
	bytesSent, _ := parseOptionalInt(fields[11])
	requestMS := parsePositiveFloat(fields[4]) * 1000
	targetMS := parsePositiveFloat(fields[5]) * 1000
	responseMS := parsePositiveFloat(fields[6]) * 1000
	method, uri, _ := splitRequestLine(fields[11])
	record := &Record{
		Timestamp:         ts,
		Method:            firstNonEmpty(method, "UNKNOWN"),
		URI:               firstNonEmpty(pathFromMaybeURL(uri), "/"),
		Status:            status,
		ResponseTimeMS:    requestMS + targetMS + responseMS,
		BytesSent:         bytesSent,
		ClientIP:          hostPart(fields[2]),
		UserAgent:         fieldAt(fields, 12),
		RawLine:           line,
		SourceFormat:      format,
		CloudProvider:     "aws",
		ServiceName:       "aws-elb",
		UpstreamService:   fields[3],
		UpstreamLatencyMS: targetMS,
		GatewayLatencyMS:  requestMS + responseMS,
	}
	return validateRecord(record)
}

func parseAWSALBLine(line, format string) (*Record, *parseError) {
	fields := splitLogFields(line)
	if len(fields) < 13 {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "AWS ALB log line has too few fields."}
	}
	ts, err := parseAccessISOTime(fields[1])
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidTimestamp, Message: "AWS ALB timestamp could not be parsed."}
	}
	status, err := strconv.Atoi(nonNegativeToken(fields[8]))
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "AWS ALB status could not be parsed."}
	}
	bytesSent, _ := parseOptionalInt(fields[11])
	requestMS := parsePositiveFloat(fields[5]) * 1000
	targetMS := parsePositiveFloat(fields[6]) * 1000
	responseMS := parsePositiveFloat(fields[7]) * 1000
	method, uri, _ := splitRequestLine(fields[12])
	record := &Record{
		Timestamp:         ts,
		Method:            firstNonEmpty(method, "UNKNOWN"),
		URI:               firstNonEmpty(pathFromMaybeURL(uri), "/"),
		Status:            status,
		ResponseTimeMS:    requestMS + targetMS + responseMS,
		BytesSent:         bytesSent,
		ClientIP:          hostPart(fields[3]),
		UserAgent:         fieldAt(fields, 13),
		RawLine:           line,
		SourceFormat:      format,
		CloudProvider:     "aws",
		ServiceName:       "aws-alb",
		UpstreamService:   fields[4],
		UpstreamLatencyMS: targetMS,
		GatewayLatencyMS:  requestMS + responseMS,
		TLSCipher:         fieldAt(fields, 14),
		TLSVersion:        fieldAt(fields, 15),
		TraceID:           cleanTraceID(fieldAt(fields, 16)),
		Host:              fieldAt(fields, 17),
	}
	return validateRecord(record)
}

func parseCloudFrontLine(line, format string) (*Record, *parseError) {
	fields := strings.Split(line, "\t")
	if len(fields) < 19 {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "CloudFront standard log line has too few tab-separated fields."}
	}
	ts, err := time.Parse("2006-01-02 15:04:05", fields[0]+" "+fields[1])
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidTimestamp, Message: "CloudFront timestamp could not be parsed."}
	}
	status, err := strconv.Atoi(nonNegativeToken(fields[8]))
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "CloudFront status could not be parsed."}
	}
	bytesSent, _ := parseOptionalInt(fields[3])
	responseMS := parsePositiveFloat(fields[18]) * 1000
	uri := fields[7]
	if fields[11] != "" && fields[11] != "-" {
		uri += "?" + fields[11]
	}
	record := &Record{
		Timestamp:        ts.UTC(),
		Method:           fields[5],
		URI:              firstNonEmpty(uri, "/"),
		Status:           status,
		ResponseTimeMS:   responseMS,
		BytesSent:        bytesSent,
		ClientIP:         fields[4],
		UserAgent:        fields[10],
		Referer:          fields[9],
		RawLine:          line,
		SourceFormat:     format,
		CloudProvider:    "aws",
		EdgeLocation:     fields[2],
		Host:             fields[6],
		ServiceName:      "cloudfront",
		GatewayLatencyMS: responseMS,
		TLSVersion:       fieldAt(fields, 20),
		TLSCipher:        fieldAt(fields, 21),
		ResponseFlags:    fieldAt(fields, 13),
		RequestID:        fieldAt(fields, 14),
		Protocol:         fieldAt(fields, 23),
	}
	return validateRecord(record)
}

func parseGCPHTTPLBJSONLine(line, format string) (*Record, *parseError) {
	payload, err := decodeJSONObject(line)
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidJSON, Message: err.Error()}
	}
	httpReq := mapAt(payload, "httpRequest")
	ts := firstJSONTime(payload, "timestamp", "receiveTimestamp")
	status := intFromAny(firstAny(httpReq, "status"))
	latencyMS := durationToMS(firstAny(httpReq, "latency"))
	method, uri := requestFromURL(stringValue(firstAny(httpReq, "requestMethod")), stringValue(firstAny(httpReq, "requestUrl")))
	resource := mapAt(payload, "resource")
	labels := mapAt(resource, "labels")
	record := &Record{
		Timestamp:        ts,
		Method:           firstNonEmpty(method, "UNKNOWN"),
		URI:              firstNonEmpty(uri, "/"),
		Status:           status,
		ResponseTimeMS:   latencyMS,
		BytesSent:        intFromAny(firstAny(httpReq, "responseSize")),
		ClientIP:         stringValue(firstAny(httpReq, "remoteIp")),
		UserAgent:        stringValue(firstAny(httpReq, "userAgent")),
		Referer:          stringValue(firstAny(httpReq, "referer")),
		RawLine:          line,
		SourceFormat:     format,
		CloudProvider:    "gcp",
		Host:             hostFromURL(stringValue(firstAny(httpReq, "requestUrl"))),
		ServiceName:      "gcp-http-lb",
		UpstreamService:  firstNonEmpty(stringValue(firstAny(labels, "backend_service_name")), stringValue(firstAny(labels, "url_map_name"))),
		GatewayLatencyMS: latencyMS,
		RequestID:        stringValue(firstAny(payload, "insertId")),
	}
	return validateRecord(record)
}

func parseAzureJSONLine(line, format string) (*Record, *parseError) {
	payload, err := decodeJSONObject(line)
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidJSON, Message: err.Error()}
	}
	props := mapAt(payload, "properties")
	if len(props) == 0 {
		props = payload
	}
	ts := firstJSONTime(payload, "time", "TimeGenerated")
	if ts.IsZero() {
		ts = firstJSONTime(props, "time", "timestamp")
	}
	method := firstNonEmpty(stringValue(firstAny(props, "httpMethod", "requestMethod", "method")), stringValue(firstAny(payload, "httpMethod")))
	uri := firstNonEmpty(stringValue(firstAny(props, "requestUri", "requestUri_s", "uri", "path", "url")), stringValue(firstAny(payload, "requestUri")))
	status := intFromAny(firstAny(props, "httpStatusCode", "statusCode", "status", "scStatus"))
	latencyMS := durationToMS(firstAny(props, "timeTaken", "timeTaken_d", "durationMs", "backendResponseLatency"))
	record := &Record{
		Timestamp:        ts,
		Method:           firstNonEmpty(method, "UNKNOWN"),
		URI:              firstNonEmpty(uri, "/"),
		Status:           status,
		ResponseTimeMS:   latencyMS,
		BytesSent:        intFromAny(firstAny(props, "responseBytes", "sentBytes")),
		ClientIP:         firstNonEmpty(stringValue(firstAny(props, "clientIp", "clientIP", "clientIP_s")), stringValue(firstAny(payload, "clientIp"))),
		UserAgent:        stringValue(firstAny(props, "userAgent")),
		RawLine:          line,
		SourceFormat:     format,
		CloudProvider:    "azure",
		Host:             firstNonEmpty(stringValue(firstAny(props, "hostName", "host", "requestHost")), hostFromURL(uri)),
		Route:            stringValue(firstAny(props, "routingRuleName", "routeName")),
		ServiceName:      azureServiceName(format),
		UpstreamService:  stringValue(firstAny(props, "backendHostname", "backendPoolName", "originName")),
		GatewayLatencyMS: latencyMS,
		RequestID:        firstNonEmpty(stringValue(firstAny(props, "trackingReference")), stringValue(firstAny(payload, "operation_Id"))),
	}
	return validateRecord(record)
}

func parseCaddyJSONLine(line, format string) (*Record, *parseError) {
	payload, err := decodeJSONObject(line)
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidJSON, Message: err.Error()}
	}
	req := mapAt(payload, "request")
	ts := firstJSONTime(payload, "ts", "time", "timestamp")
	status := intFromAny(firstAny(payload, "status", "resp_status"))
	latencyMS := durationToMS(firstAny(payload, "duration", "duration_ms"))
	record := &Record{
		Timestamp:        ts,
		Method:           firstNonEmpty(stringValue(firstAny(req, "method")), stringValue(firstAny(payload, "method")), "UNKNOWN"),
		URI:              firstNonEmpty(stringValue(firstAny(req, "uri")), stringValue(firstAny(payload, "uri")), "/"),
		Status:           status,
		ResponseTimeMS:   latencyMS,
		BytesSent:        intFromAny(firstAny(payload, "size", "resp_size")),
		ClientIP:         firstNonEmpty(stringValue(firstAny(req, "remote_ip", "client_ip")), hostPart(stringValue(firstAny(req, "remote_addr")))),
		UserAgent:        headerValue(req, "User-Agent"),
		RawLine:          line,
		SourceFormat:     format,
		Host:             firstNonEmpty(headerValue(req, "Host"), stringValue(firstAny(req, "host"))),
		ServiceName:      "caddy",
		GatewayLatencyMS: latencyMS,
		RequestID:        firstNonEmpty(headerValue(req, "X-Request-Id"), stringValue(firstAny(payload, "request_id"))),
	}
	return validateRecord(record)
}

func parseTraefikJSONLine(line, format string) (*Record, *parseError) {
	payload, err := decodeJSONObject(line)
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidJSON, Message: err.Error()}
	}
	ts := firstJSONTime(payload, "StartUTC", "time", "timestamp")
	durationMS := durationToMS(firstAny(payload, "Duration", "RequestDuration", "duration"))
	upstreamMS := durationToMS(firstAny(payload, "OriginDuration"))
	record := &Record{
		Timestamp:         ts,
		Method:            firstNonEmpty(stringValue(firstAny(payload, "RequestMethod")), "UNKNOWN"),
		URI:               firstNonEmpty(stringValue(firstAny(payload, "RequestPath", "RequestAddr")), "/"),
		Status:            intFromAny(firstAny(payload, "DownstreamStatus", "OriginStatus", "status")),
		ResponseTimeMS:    durationMS,
		BytesSent:         intFromAny(firstAny(payload, "DownstreamContentSize")),
		ClientIP:          hostPart(stringValue(firstAny(payload, "ClientAddr", "ClientHost"))),
		RawLine:           line,
		SourceFormat:      format,
		Host:              stringValue(firstAny(payload, "RequestHost")),
		Route:             stringValue(firstAny(payload, "RouterName")),
		ServiceName:       "traefik",
		UpstreamService:   stringValue(firstAny(payload, "ServiceName")),
		UpstreamLatencyMS: upstreamMS,
		GatewayLatencyMS:  math.Max(0, durationMS-upstreamMS),
		TLSVersion:        stringValue(firstAny(payload, "TLSVersion")),
		RequestID:         stringValue(firstAny(payload, "RequestID")),
	}
	return validateRecord(record)
}

func parseKongJSONLine(line, format string) (*Record, *parseError) {
	payload, err := decodeJSONObject(line)
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidJSON, Message: err.Error()}
	}
	req := mapAt(payload, "request")
	resp := mapAt(payload, "response")
	latencies := mapAt(payload, "latencies")
	route := mapAt(payload, "route")
	service := mapAt(payload, "service")
	consumer := mapAt(payload, "consumer")
	ts := firstJSONTime(payload, "started_at", "timestamp", "time")
	proxyMS := durationToMS(firstAny(latencies, "proxy"))
	requestMS := durationToMS(firstAny(latencies, "request"))
	kongMS := durationToMS(firstAny(latencies, "kong"))
	record := &Record{
		Timestamp:         ts,
		Method:            firstNonEmpty(stringValue(firstAny(req, "method")), "UNKNOWN"),
		URI:               firstNonEmpty(stringValue(firstAny(req, "uri", "url", "path")), "/"),
		Status:            intFromAny(firstAny(resp, "status", "status_code")),
		ResponseTimeMS:    firstPositive(requestMS, proxyMS+kongMS),
		BytesSent:         intFromAny(firstAny(resp, "size", "headers_size")),
		ClientIP:          firstNonEmpty(stringValue(firstAny(req, "remote_addr", "client_ip")), hostPart(stringValue(firstAny(req, "forwarded_for")))),
		UserAgent:         headerValue(req, "user-agent"),
		RawLine:           line,
		SourceFormat:      format,
		Host:              firstNonEmpty(headerValue(req, "host"), stringValue(firstAny(req, "host"))),
		Route:             stringValue(firstAny(route, "name", "id")),
		ServiceName:       "kong",
		UpstreamService:   firstNonEmpty(stringValue(firstAny(service, "name", "host")), stringValue(firstAny(payload, "upstream_uri"))),
		UpstreamLatencyMS: proxyMS,
		GatewayLatencyMS:  kongMS,
		Consumer:          firstNonEmpty(stringValue(firstAny(consumer, "username", "custom_id", "id")), stringValue(firstAny(payload, "consumer"))),
		RequestID:         firstNonEmpty(headerValue(req, "x-request-id"), stringValue(firstAny(payload, "request_id"))),
	}
	return validateRecord(record)
}

func parseTykJSONLine(line, format string) (*Record, *parseError) {
	payload, err := decodeJSONObject(line)
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidJSON, Message: err.Error()}
	}
	ts := firstJSONTime(payload, "timestamp", "time", "request_time")
	latencyMS := durationToMS(firstAny(payload, "latency", "total_time", "request_time_ms"))
	record := &Record{
		Timestamp:        ts,
		Method:           firstNonEmpty(stringValue(firstAny(payload, "method", "http_method")), "UNKNOWN"),
		URI:              firstNonEmpty(stringValue(firstAny(payload, "path", "uri", "request_path")), "/"),
		Status:           intFromAny(firstAny(payload, "status", "status_code", "response_code")),
		ResponseTimeMS:   latencyMS,
		BytesSent:        intFromAny(firstAny(payload, "content_length", "response_size")),
		ClientIP:         stringValue(firstAny(payload, "ip_address", "client_ip", "remote_addr")),
		UserAgent:        stringValue(firstAny(payload, "user_agent")),
		RawLine:          line,
		SourceFormat:     format,
		Route:            stringValue(firstAny(payload, "path", "route")),
		ServiceName:      "tyk",
		UpstreamService:  firstNonEmpty(stringValue(firstAny(payload, "api_name")), stringValue(firstAny(payload, "api_id"))),
		GatewayLatencyMS: latencyMS,
		Consumer:         firstNonEmpty(stringValue(firstAny(payload, "user_id", "oauth_id")), stringValue(firstAny(payload, "org_id"))),
		RequestID:        stringValue(firstAny(payload, "request_id")),
	}
	return validateRecord(record)
}

func parseAPIGatewayJSONLine(line, format string) (*Record, *parseError) {
	payload, err := decodeJSONObject(line)
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidJSON, Message: err.Error()}
	}
	context := mapAt(payload, "requestContext")
	ts := firstJSONTime(payload, "requestTimeEpoch", "requestTime", "time", "timestamp")
	if ts.IsZero() {
		ts = firstJSONTime(context, "requestTimeEpoch", "requestTime")
	}
	method := firstNonEmpty(stringValue(firstAny(payload, "httpMethod", "method")), stringValue(firstAny(context, "httpMethod")))
	uri := firstNonEmpty(stringValue(firstAny(payload, "path", "resourcePath", "routeKey")), stringValue(firstAny(context, "path", "resourcePath")))
	responseMS := durationToMS(firstAny(payload, "responseLatency", "latency", "duration"))
	integrationMS := durationToMS(firstAny(payload, "integrationLatency"))
	record := &Record{
		Timestamp:         ts,
		Method:            firstNonEmpty(method, "UNKNOWN"),
		URI:               firstNonEmpty(uri, "/"),
		Status:            intFromAny(firstAny(payload, "status", "statusCode")),
		ResponseTimeMS:    responseMS,
		BytesSent:         intFromAny(firstAny(payload, "responseLength")),
		ClientIP:          firstNonEmpty(stringValue(firstAny(payload, "sourceIp", "ip")), stringValue(firstAny(mapAt(context, "identity"), "sourceIp"))),
		UserAgent:         firstNonEmpty(stringValue(firstAny(payload, "userAgent")), stringValue(firstAny(mapAt(context, "identity"), "userAgent"))),
		RawLine:           line,
		SourceFormat:      format,
		CloudProvider:     "aws",
		Host:              stringValue(firstAny(payload, "domainName")),
		Route:             firstNonEmpty(stringValue(firstAny(payload, "routeKey")), stringValue(firstAny(context, "routeKey"))),
		ServiceName:       "aws-api-gateway",
		UpstreamService:   stringValue(firstAny(payload, "integrationService", "apiName")),
		UpstreamLatencyMS: integrationMS,
		GatewayLatencyMS:  math.Max(0, responseMS-integrationMS),
		RequestID:         firstNonEmpty(stringValue(firstAny(payload, "requestId")), stringValue(firstAny(context, "requestId"))),
	}
	return validateRecord(record)
}

var envoyTextLog = regexp.MustCompile(
	`^\[(?P<timestamp>[^\]]+)\] "(?P<request>[^"]*)" (?P<status>\S+) (?P<flags>\S+) (?P<response_code_details>\S+) (?P<connection_termination_details>\S+) "(?P<upstream_transport_failure_reason>[^"]*)" (?P<bytes_in>\S+) (?P<bytes_out>\S+) (?P<duration_ms>\S+) (?P<upstream_ms>\S+) "(?P<forwarded_for>[^"]*)" "(?P<user_agent>[^"]*)" "(?P<request_id>[^"]*)" "(?P<authority>[^"]*)" "(?P<upstream_host>[^"]*)" "(?P<upstream_cluster>[^"]*)".*$`,
)

func parseIISW3CLine(line string, fields []string) (*Record, *parseError) {
	if strings.HasPrefix(strings.TrimSpace(line), "#Fields:") {
		return nil, nil
	}
	values := strings.Fields(line)
	if len(fields) == 0 {
		fields = []string{"date", "time", "s-ip", "cs-method", "cs-uri-stem", "cs-uri-query", "s-port", "cs-username", "c-ip", "cs(User-Agent)", "sc-status", "sc-substatus", "sc-win32-status", "time-taken"}
	}
	if len(values) < len(fields) {
		return nil, &parseError{Reason: ReasonMissingW3CFields, Message: "IIS W3C line has fewer values than #Fields."}
	}
	row := map[string]string{}
	for i, field := range fields {
		row[field] = values[i]
	}
	ts, err := time.Parse("2006-01-02 15:04:05", row["date"]+" "+row["time"])
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidTimestamp, Message: "IIS W3C timestamp could not be parsed."}
	}
	status, err := strconv.Atoi(nonNegativeToken(row["sc-status"]))
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "IIS W3C status could not be parsed."}
	}
	uri := row["cs-uri-stem"]
	if query := row["cs-uri-query"]; query != "" && query != "-" {
		uri += "?" + query
	}
	record := &Record{
		Timestamp:        ts.UTC(),
		Method:           firstNonEmpty(row["cs-method"], "UNKNOWN"),
		URI:              firstNonEmpty(uri, "/"),
		Status:           status,
		ResponseTimeMS:   parsePositiveFloat(row["time-taken"]),
		ClientIP:         row["c-ip"],
		UserAgent:        row["cs(User-Agent)"],
		RawLine:          line,
		SourceFormat:     "iis-w3c",
		Host:             row["cs-host"],
		ServiceName:      "iis",
		GatewayLatencyMS: parsePositiveFloat(row["time-taken"]),
	}
	return validateRecord(record)
}

func canonicalFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		return "nginx"
	}
	format = strings.ReplaceAll(format, "_", "-")
	switch format {
	case "haproxy-default", "haproxy-http-log":
		return "haproxy-http"
	case "envoy", "envoy-default":
		return "envoy-text"
	case "istio", "istio-default":
		return "istio-text"
	case "alb", "aws-alb-v2":
		return "aws-alb"
	case "elb", "aws-elb-classic":
		return "aws-elb"
	case "cloudfront":
		return "aws-cloudfront"
	case "gcp-lb-json", "gcp-http-load-balancer-json":
		return "gcp-http-lb-json"
	case "azure-appservice-json":
		return "azure-app-service-json"
	case "azure-frontdoor-json":
		return "azure-front-door-json"
	case "iis", "iis-w3c-extended":
		return "iis-w3c"
	case "caddy":
		return "caddy-json"
	case "traefik":
		return "traefik-json"
	case "kong":
		return "kong-json"
	case "tyk":
		return "tyk-json"
	case "aws-api-gateway", "apigateway-json":
		return "aws-api-gateway-json"
	default:
		return format
	}
}

func validateRecord(record *Record) (*Record, *parseError) {
	if record == nil {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "No access log record was produced."}
	}
	if record.Status < 100 || record.Status > 999 ||
		record.BytesSent < 0 ||
		math.IsNaN(record.ResponseTimeMS) || math.IsInf(record.ResponseTimeMS, 0) || record.ResponseTimeMS < 0 {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: "Numeric field is outside the valid range."}
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Unix(0, 0).UTC()
	}
	if record.URI == "" {
		record.URI = "/"
	}
	if record.Method == "" {
		record.Method = "UNKNOWN"
	}
	if record.SourceFormat == "" {
		record.SourceFormat = "unknown"
	}
	return record, nil
}

func parseHAProxyTimestamp(value string) (time.Time, error) {
	layouts := []string{
		"02/Jan/2006:15:04:05.000",
		"02/Jan/2006:15:04:05",
		"02/Jan/2006:15:04:05.000 -0700",
		"02/Jan/2006:15:04:05 -0700",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid HAProxy timestamp %q", value)
}

func parseAccessISOTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000000Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05",
		"02/Jan/2006:15:04:05 -0700",
		"02/Jan/2006:15:04:05",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", value)
}

func firstJSONTime(payload map[string]any, keys ...string) time.Time {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		if ts := anyToTime(value); !ts.IsZero() {
			return ts
		}
	}
	return time.Time{}
}

func anyToTime(value any) time.Time {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" || v == "-" {
			return time.Time{}
		}
		if ts, err := parseAccessISOTime(v); err == nil {
			return ts
		}
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return epochToTime(n)
		}
	case json.Number:
		if n, err := v.Float64(); err == nil {
			return epochToTime(n)
		}
	case float64:
		return epochToTime(v)
	case int64:
		return epochToTime(float64(v))
	case int:
		return epochToTime(float64(v))
	}
	return time.Time{}
}

func epochToTime(value float64) time.Time {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return time.Time{}
	}
	switch {
	case value > 1e18:
		return time.Unix(0, int64(value)).UTC()
	case value > 1e14:
		return time.Unix(0, int64(value*1000)).UTC()
	case value > 1e11:
		return time.UnixMilli(int64(value)).UTC()
	default:
		sec, frac := math.Modf(value)
		return time.Unix(int64(sec), int64(frac*1e9)).UTC()
	}
}

func decodeJSONObject(line string) (map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(line))
	dec.UseNumber()
	var payload map[string]any
	if err := dec.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func payloadHas(payload map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := payload[key]; !ok {
			return false
		}
	}
	return true
}

func mapAt(payload map[string]any, key string) map[string]any {
	if payload == nil {
		return nil
	}
	if m, ok := payload[key].(map[string]any); ok {
		return m
	}
	return nil
}

func firstAny(payload map[string]any, keys ...string) any {
	if payload == nil {
		return nil
	}
	for _, key := range keys {
		if value, ok := payload[key]; ok && !isEmptyAny(value) {
			return value
		}
	}
	return nil
}

func isEmptyAny(value any) bool {
	if value == nil {
		return true
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s) == "" || s == "-"
	}
	return false
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		if v == "-" {
			return ""
		}
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := strconv.Atoi(nonNegativeToken(v.String()))
		return i
	case string:
		i, _ := strconv.Atoi(nonNegativeToken(v))
		return i
	default:
		return 0
	}
}

func durationToMS(value any) float64 {
	switch v := value.(type) {
	case nil:
		return 0
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return numericDurationToMS(f)
		}
	case float64:
		return numericDurationToMS(v)
	case int:
		return numericDurationToMS(float64(v))
	case int64:
		return numericDurationToMS(float64(v))
	case string:
		s := strings.TrimSpace(v)
		if s == "" || s == "-" {
			return 0
		}
		switch {
		case strings.HasSuffix(s, "ms"):
			return parsePositiveFloat(strings.TrimSuffix(s, "ms"))
		case strings.HasSuffix(s, "us"):
			return parsePositiveFloat(strings.TrimSuffix(s, "us")) / 1000
		case strings.HasSuffix(s, "ns"):
			return parsePositiveFloat(strings.TrimSuffix(s, "ns")) / 1_000_000
		case strings.HasSuffix(s, "s"):
			return parsePositiveFloat(strings.TrimSuffix(s, "s")) * 1000
		default:
			return numericDurationToMS(parsePositiveFloat(s))
		}
	}
	return 0
}

func numericDurationToMS(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value > 1_000_000 {
		return value / 1_000_000
	}
	if value < 10 {
		return value * 1000
	}
	return value
}

func parsePositiveFloat(value string) float64 {
	value = nonNegativeToken(value)
	if value == "" {
		return 0
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil || f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

func parseOptionalInt(value string) (int, error) {
	value = nonNegativeToken(value)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func nonNegativeToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "-" || value == "" {
		return ""
	}
	return value
}

func splitLogFields(line string) []string {
	fields := []string{}
	var b strings.Builder
	inQuote := false
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\' && inQuote:
			escaped = true
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t') && !inQuote:
			if b.Len() > 0 {
				fields = append(fields, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		fields = append(fields, b.String())
	}
	return fields
}

func fieldAt(fields []string, idx int) string {
	if idx >= 0 && idx < len(fields) {
		return fields[idx]
	}
	return ""
}

func hostPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return ""
	}
	if i := strings.IndexByte(value, ','); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	if strings.HasPrefix(value, "[") {
		if end := strings.Index(value, "]:"); end > 0 {
			return strings.Trim(value[:end+1], "[]")
		}
	}
	if idx := strings.LastIndex(value, ":"); idx > -1 && strings.Count(value, ":") == 1 {
		return value[:idx]
	}
	return value
}

func hostFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if i := strings.Index(raw, "://"); i >= 0 {
		rest := raw[i+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			rest = rest[:slash]
		}
		return hostPart(rest)
	}
	return ""
}

func pathFromMaybeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if i := strings.Index(raw, "://"); i >= 0 {
		rest := raw[i+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			return rest[slash:]
		}
		return "/"
	}
	return raw
}

func requestFromURL(method, raw string) (string, string) {
	if raw == "" {
		return method, ""
	}
	if i := strings.Index(raw, "://"); i >= 0 {
		rest := raw[i+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			return method, rest[slash:]
		}
		return method, "/"
	}
	return method, raw
}

func headerValue(payload map[string]any, name string) string {
	headers := mapAt(payload, "headers")
	if headers == nil {
		return ""
	}
	lowerName := strings.ToLower(name)
	for key, value := range headers {
		if strings.ToLower(key) == lowerName {
			switch v := value.(type) {
			case []any:
				if len(v) > 0 {
					return stringValue(v[0])
				}
			default:
				return stringValue(v)
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func normalizeClusterService(value string) string {
	value = strings.Trim(value, `" `)
	if value == "" || value == "-" {
		return ""
	}
	parts := strings.Split(value, "|")
	if len(parts) >= 4 && parts[3] != "" {
		return parts[3]
	}
	if idx := strings.LastIndex(value, "/"); idx >= 0 && idx < len(value)-1 {
		value = value[idx+1:]
	}
	return value
}

func cleanTraceID(value string) string {
	value = strings.Trim(value, `" `)
	if strings.HasPrefix(value, "Root=") {
		return strings.TrimPrefix(value, "Root=")
	}
	return value
}

func azureServiceName(format string) string {
	if format == "azure-front-door-json" {
		return "azure-front-door"
	}
	return "azure-app-service"
}

func looksLikeDate(value string) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
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
	format = canonicalFormat(format)
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
	var iisFields []string

	err := textio.ForEachTextLine(path, "", func(lineNumber int, line string) error {
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			return errStopIteration
		}
		diags.TotalLines++
		if isBlank(line) {
			return nil
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#Fields:") {
			iisFields = strings.Fields(strings.TrimSpace(strings.TrimPrefix(trimmed, "#Fields:")))
			return nil
		}
		var (
			record *Record
			perr   *parseError
		)
		if format == "iis-w3c" || (format == "auto" && len(iisFields) > 0) {
			record, perr = parseIISW3CLine(line, iisFields)
		} else {
			record, perr = ParseLineForFormat(line, format)
		}
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
