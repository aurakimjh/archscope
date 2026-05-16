// Package otel ports archscope_engine.parsers.otel_parser — the
// JSONL-per-line OTel log/span parser feeding the trace analyzer.
//
// Each input line is a self-contained JSON object describing a single
// log or span emission. The parser is alias-tolerant: snake_case,
// camelCase, and dotted forms (`service.name`) are all accepted, and
// service name / parent span id may appear either at the top level or
// nested under `resource.attributes` / `attributes`. Body values may
// arrive as strings, primitives (rendered with Python `str(...)`
// semantics — `True`/`False` capitalised, integers without decimals),
// or nested dicts carrying `stringValue` / `str` / `value` / `text`.
//
// Skip reasons mirror Python verbatim so cross-engine diagnostics
// remain diff-able: INVALID_OTEL_JSON, INVALID_OTEL_RECORD.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] otel parser — OpenTelemetry JSONL 로그/스팬 파서.
//
// 입력 형식
//
//	라인 단위 JSON. 각 라인이 독립된 JSON 객체이며 한 로그/스팬을
//	표현. OTel exporter 마다 키 이름이 다양하게 emit 되므로 alias
//	허용 정책을 채택 — snake_case / camelCase / 도트 표기 모두 인식.
//
// alias 매핑 예
//
//	service name : `service.name` / `serviceName` / `resource.service.name`
//	               / `resource.attributes.service.name`.
//	parent span  : `parent_span_id` / `parentSpanId` / `parent_id`.
//	trace/span   : `trace_id` / `traceId`, `span_id` / `spanId`.
//
// body 값 형태
//
//	문자열, 원시 타입(bool/int/float), 또는 dict (stringValue / str /
//	value / text 키 안에 실 값) 모두 허용. dict 가 들어오면 위 키들을
//	순서대로 시도해 첫 매치를 사용.
//
// Python str() semantics
//
//	bool → "True"/"False" (대문자), int → "123" (소수점 없음), float
//	→ "1.5". Python 의 str() 결과를 byte 단위로 일치 — parity gate
//	가 stderr/JSON 비교 시 안전.
//
// skip 사유
//
//	ReasonInvalidOTelJSON   : 라인이 JSON 으로 파싱 불가.
//	ReasonInvalidOTelRecord : JSON 은 OK 인데 trace/span_id 등 필수
//	                          필드 누락.
//	둘 다 라인 단위 skip → diagnostics 에 카운트, 분석 진행.
package otel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

var errStopIteration = errors.New("stop otel iteration")

// Record mirrors `models.otel.OTelLogRecord`. Pointer types double as
// Python's `Optional[str]` — nil means "field absent from payload".
// `Severity` and `Body` are non-pointer because Python stores empty
// string / "UNSPECIFIED" defaults rather than None.
type Record struct {
	Timestamp    *string
	TraceID      *string
	SpanID       *string
	ParentSpanID *string
	ServiceName  *string
	Severity     string
	Body         string
	Attributes   map[string]string
	Resource     map[string]string
	RawLine      string
}

// Skip reasons (Python-verbatim).
const (
	ReasonInvalidOTelJSON   = "INVALID_OTEL_JSON"
	ReasonInvalidOTelRecord = "INVALID_OTEL_RECORD"
)

// FailedPattern is the marker emitted in debug logs (parity with
// Python's `failed_pattern="OTEL_JSONL_LOG_RECORD"`).
const FailedPattern = "OTEL_JSONL_LOG_RECORD"

// parseError carries the (reason, message) pair Python uses to
// classify a malformed line. ParseLine returns this alongside a nil
// record so callers can differentiate JSON errors from missing-field
// rejections without re-decoding.
type parseError struct {
	Reason  string
	Message string
}

// ParseLine parses a single non-empty JSONL line. Returns
// (record, nil) on success, (nil, *parseError) on a classified
// rejection. Mirrors the inner loop of Python `parse_otel_jsonl`.
func ParseLine(line string) (*Record, *parseError) {
	var payload any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return nil, &parseError{Reason: ReasonInvalidOTelJSON, Message: err.Error()}
	}
	rec := recordFromPayload(payload, line)
	if rec == nil {
		return nil, &parseError{
			Reason:  ReasonInvalidOTelRecord,
			Message: "JSON object did not contain OTel log fields.",
		}
	}
	return rec, nil
}

// recordFromPayload mirrors Python `_record_from_payload`. The
// payload must be a JSON object and must carry at least one of
// trace_id / span_id / body — otherwise the row is rejected with
// INVALID_OTEL_RECORD.
func recordFromPayload(payload any, rawLine string) *Record {
	obj, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	traceID := stringValue(obj, "trace_id", "traceId", "traceid")
	spanID := stringValue(obj, "span_id", "spanId", "spanid")
	parentSpanID := parentSpanID(obj)
	bodyRaw := firstNonNilTruthy(obj, "body", "message")
	body := bodyValue(bodyRaw)
	if traceID == nil && spanID == nil && body == nil {
		return nil
	}

	severity := stringValue(obj, "severity_text", "severityText", "level")
	severityStr := "UNSPECIFIED"
	if severity != nil {
		severityStr = *severity
	}
	bodyStr := ""
	if body != nil {
		bodyStr = *body
	}

	return &Record{
		Timestamp:    stringValue(obj, "timestamp", "time", "observed_time"),
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentSpanID,
		ServiceName:  serviceName(obj),
		Severity:     severityStr,
		Body:         bodyStr,
		Attributes:   attributeMap(mapValue(obj, "attributes")),
		Resource:     resourceAttributes(obj),
		RawLine:      rawLine,
	}
}

func mapValue(obj map[string]any, key string) map[string]any {
	if obj == nil {
		return nil
	}
	if m, ok := obj[key].(map[string]any); ok {
		return m
	}
	return nil
}

func resourceAttributes(obj map[string]any) map[string]string {
	resource, ok := obj["resource"].(map[string]any)
	if !ok {
		return map[string]string{}
	}
	return attributeMap(mapValue(resource, "attributes"))
}

func attributeMap(obj map[string]any) map[string]string {
	if len(obj) == 0 {
		return map[string]string{}
	}
	out := map[string]string{}
	for key, value := range obj {
		if s := bodyValue(value); s != nil {
			out[key] = *s
		}
	}
	return out
}

// serviceName mirrors Python `_service_name` — top-level alias →
// resource.attributes → top-level attributes.
func serviceName(obj map[string]any) *string {
	if v := stringValue(obj, "service_name", "service.name"); v != nil {
		return v
	}
	if resource, ok := obj["resource"].(map[string]any); ok {
		if attrs, ok := resource["attributes"].(map[string]any); ok {
			if v := stringValue(attrs, "service.name", "service_name"); v != nil {
				return v
			}
		}
	}
	if attrs, ok := obj["attributes"].(map[string]any); ok {
		if v := stringValue(attrs, "service.name", "service_name"); v != nil {
			return v
		}
	}
	return nil
}

// parentSpanID mirrors Python `_parent_span_id` — top-level aliases
// take precedence over the four `attributes`-nested aliases.
func parentSpanID(obj map[string]any) *string {
	if v := stringValue(obj,
		"parent_span_id",
		"parentSpanId",
		"parent_spanid",
		"parentSpanID",
	); v != nil {
		return v
	}
	if attrs, ok := obj["attributes"].(map[string]any); ok {
		if v := stringValue(attrs,
			"parent_span_id",
			"parentSpanId",
			"parent.span_id",
			"parentSpanID",
		); v != nil {
			return v
		}
	}
	return nil
}

// stringValue mirrors Python `_string_value`. Iterates the alias keys
// in order; first hit wins. Strings pass through; numbers are
// rendered with Python `str(...)` semantics; nested dicts are
// resolved through `bodyValue` so things like
// `{"trace_id": {"stringValue": "..."}}` still work.
func stringValue(obj map[string]any, keys ...string) *string {
	for _, k := range keys {
		v, ok := obj[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case string:
			s := t
			return &s
		case float64:
			s := formatNumber(t)
			return &s
		case bool:
			// Python's `_string_value` does not accept booleans —
			// only strings, ints, floats, and dicts. Skip the alias
			// and try the next one. This is intentionally NOT a
			// catch-all; mirrors the isinstance chain literally.
			continue
		case map[string]any:
			if nested := bodyValue(t); nested != nil {
				return nested
			}
		}
	}
	return nil
}

// bodyValue mirrors Python `_body_value`. Strings pass through;
// primitives render via `str(...)`; nested dicts probe the four
// well-known keys in order.
func bodyValue(v any) *string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		s := t
		return &s
	case bool:
		s := pythonBool(t)
		return &s
	case float64:
		s := formatNumber(t)
		return &s
	case map[string]any:
		for _, k := range []string{"stringValue", "str", "value", "text"} {
			if nested, ok := t[k]; ok {
				if s, ok := nested.(string); ok {
					out := s
					return &out
				}
			}
		}
	}
	return nil
}

// firstNonNilTruthy mirrors Python `payload.get("body") or
// payload.get("message")` — returns the first key whose raw value is
// not None and not falsy. Falsy here matches Python's truth-table for
// the types we care about: empty string, 0, 0.0, False, empty dict,
// empty list.
func firstNonNilTruthy(obj map[string]any, keys ...string) any {
	for _, k := range keys {
		v, ok := obj[k]
		if !ok || v == nil {
			continue
		}
		if isPythonFalsy(v) {
			continue
		}
		return v
	}
	return nil
}

func isPythonFalsy(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case bool:
		return !t
	case float64:
		return t == 0
	case map[string]any:
		return len(t) == 0
	case []any:
		return len(t) == 0
	}
	return false
}

// formatNumber renders a JSON number the way Python's `str(x)` would.
// Integral values lose the trailing `.0`; fractional values keep the
// shortest round-trip representation. encoding/json gives us float64
// for every JSON number, so the integer check is "is this exactly
// representable as int64 with no fractional part".
func formatNumber(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// pythonBool returns "True"/"False" — Python's `str(bool)` is
// capitalised, and the Python test fixture asserts exactly that.
func pythonBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// Options carries the parser-level filters surfaced by ParseFile.
// MaxLines / Strict are Go-side conveniences for parity with the
// accesslog parser. The Python implementation does not expose these;
// when set they layer on top of the same Python-verbatim diagnostics.
type Options struct {
	MaxLines int
	Strict   bool
}

// ParseFile reads `path` end-to-end and returns the parsed records
// alongside a populated diagnostics builder. Mirrors Python
// `parse_otel_jsonl`.
//
// On `Strict=true`, the first malformed line is returned as a
// non-nil error after the diagnostics row has been recorded.
func ParseFile(path string, opts Options) ([]Record, *diagnostics.ParserDiagnostics, error) {
	records := make([]Record, 0, 1024)
	diags, err := ForEachRecord(path, opts, func(record Record) error {
		records = append(records, record)
		return nil
	})
	if err != nil {
		return records, diags, err
	}
	return records, diags, nil
}

// ForEachRecord streams parsed OTel JSONL records without retaining the full
// record set in parser memory.
func ForEachRecord(path string, opts Options, fn func(Record) error) (*diagnostics.ParserDiagnostics, error) {
	if opts.MaxLines < 0 {
		return nil, fmt.Errorf("max_lines must be a positive integer")
	}

	diags := diagnostics.New("otel_jsonl")
	diags.SetSourceFile(path)

	if handled, err := forEachOTLPRecord(path, diags, opts, fn); handled || err != nil {
		return diags, err
	}

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
			diags.ParsedRecords++
			return fn(*record)
		}
		if perr == nil {
			return fmt.Errorf("otel parser returned neither record nor error")
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
		diags.AddWarning(0, "EMPTY_FILE", "OTel log file is empty.", "", false)
	}
	return diags, nil
}

func forEachOTLPRecord(path string, diags *diagnostics.ParserDiagnostics, opts Options, fn func(Record) error) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if !strings.Contains(string(data[:min(len(data), 4096)]), "resourceLogs") {
		return false, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		diags.TotalLines = 1
		diags.AddSkipped(1, ReasonInvalidOTelJSON, err.Error(), string(data[:min(len(data), diagnostics.RawPreviewLimit)]))
		if opts.Strict {
			return true, err
		}
		return true, nil
	}
	diags.Format = "otlp_logs_json"
	lineNumber := 0
	for _, resourceLog := range arrayValue(payload["resourceLogs"]) {
		resourceObj, _ := resourceLog.(map[string]any)
		resource := attributeListMap(nestedArray(resourceObj, "resource", "attributes"))
		service := resource["service.name"]
		for _, scopeLog := range arrayValue(resourceObj["scopeLogs"]) {
			scopeObj, _ := scopeLog.(map[string]any)
			for _, item := range arrayValue(scopeObj["logRecords"]) {
				lineNumber++
				if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
					return true, nil
				}
				diags.TotalLines++
				obj, ok := item.(map[string]any)
				if !ok {
					diags.AddSkipped(lineNumber, ReasonInvalidOTelRecord, "OTLP logRecord is not an object.", "")
					continue
				}
				attrs := attributeListMap(arrayValue(obj["attributes"]))
				body := bodyValue(obj["body"])
				bodyStr := ""
				if body != nil {
					bodyStr = *body
				}
				ts := otlpTime(obj["timeUnixNano"], obj["observedTimeUnixNano"])
				rec := Record{
					Timestamp:    ts,
					TraceID:      stringValue(obj, "traceId", "trace_id"),
					SpanID:       stringValue(obj, "spanId", "span_id"),
					ParentSpanID: stringValue(obj, "parentSpanId", "parent_span_id"),
					ServiceName:  stringPtr(service),
					Severity:     derefString(stringValue(obj, "severityText", "severity_text", "level"), "UNSPECIFIED"),
					Body:         bodyStr,
					Attributes:   attrs,
					Resource:     resource,
					RawLine:      "",
				}
				if rec.ServiceName == nil {
					rec.ServiceName = stringPtr(attrs["service.name"])
				}
				if rec.TraceID == nil && rec.SpanID == nil && rec.Body == "" {
					diags.AddSkipped(lineNumber, ReasonInvalidOTelRecord, "OTLP logRecord did not contain OTel log fields.", "")
					continue
				}
				diags.ParsedRecords++
				if err := fn(rec); err != nil {
					return true, err
				}
			}
		}
	}
	return true, nil
}

func isBlank(line string) bool {
	for _, r := range line {
		if r != ' ' && r != '\t' && r != '\r' && r != '\n' {
			return false
		}
	}
	return true
}

func arrayValue(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}
	return nil
}

func nestedArray(obj map[string]any, parent, child string) []any {
	if obj == nil {
		return nil
	}
	if parentObj, ok := obj[parent].(map[string]any); ok {
		return arrayValue(parentObj[child])
	}
	return nil
}

func attributeListMap(items []any) map[string]string {
	out := map[string]string{}
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		key := derefString(stringValue(obj, "key"), "")
		value, ok := obj["value"]
		if key == "" || !ok {
			continue
		}
		if s := otlpAnyValue(value); s != "" {
			out[key] = s
		}
	}
	return out
}

func otlpAnyValue(value any) string {
	if obj, ok := value.(map[string]any); ok {
		for _, key := range []string{"stringValue", "intValue", "doubleValue", "boolValue"} {
			if v, exists := obj[key]; exists {
				if s := bodyValue(v); s != nil {
					return *s
				}
			}
		}
	}
	if s := bodyValue(value); s != nil {
		return *s
	}
	return ""
}

func otlpTime(values ...any) *string {
	for _, value := range values {
		raw := ""
		switch v := value.(type) {
		case string:
			raw = v
		case float64:
			raw = strconv.FormatInt(int64(v), 10)
		}
		if raw == "" || raw == "0" {
			continue
		}
		ns, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			continue
		}
		ts := time.Unix(0, ns).UTC().Format(time.RFC3339Nano)
		return &ts
	}
	return nil
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func derefString(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
