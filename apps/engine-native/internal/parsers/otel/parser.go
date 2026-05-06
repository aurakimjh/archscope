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
package otel

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

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
		RawLine:      rawLine,
	}
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
	if opts.MaxLines < 0 {
		return nil, nil, fmt.Errorf("max_lines must be a positive integer")
	}

	diags := diagnostics.New("otel_jsonl")
	diags.SetSourceFile(path)

	lines, err := textio.IterTextLines(path, "")
	if err != nil {
		return nil, nil, err
	}

	records := make([]Record, 0, len(lines))
	for i, line := range lines {
		lineNumber := i + 1
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			break
		}
		diags.TotalLines++
		if isBlank(line) {
			continue
		}
		record, perr := ParseLine(line)
		if record != nil {
			diags.ParsedRecords++
			records = append(records, *record)
			continue
		}
		if perr == nil {
			return nil, nil, fmt.Errorf("otel parser returned neither record nor error")
		}
		diags.AddSkipped(lineNumber, perr.Reason, perr.Message, line)
		if opts.Strict {
			return records, diags, fmt.Errorf("%s:%d: %s: %s", path, lineNumber, perr.Reason, perr.Message)
		}
	}

	if diags.TotalLines == 0 {
		diags.AddWarning(0, "EMPTY_FILE", "OTel log file is empty.", "", false)
	}
	return records, diags, nil
}

func isBlank(line string) bool {
	for _, r := range line {
		if r != ' ' && r != '\t' && r != '\r' && r != '\n' {
			return false
		}
	}
	return true
}

