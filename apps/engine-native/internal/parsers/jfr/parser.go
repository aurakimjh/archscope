// Package jfr ports archscope_engine.parsers.jfr_parser and
// archscope_engine.parsers.jfr_recording.
//
// Two layers live here:
//
//   - parser.go (this file) — JSON ingestion of the output produced
//     by `jfr print --json`. Mirrors the Python `parse_jfr_print_json`
//     behaviour: tolerant of either the `{"recording": {"events":[…]}}`
//     wrapper, a top-level `{"events":[…]}` object, or a raw `[…]`
//     events array. Per-event field extraction matches the Python
//     helpers (`_to_jfr_event`, `_extract_thread`, `_format_frame`,
//     `_duration_to_ms`, `_to_int`, `_string_value`).
//   - recording.go — discovery + os/exec bridge to the JDK `jfr` CLI
//     so binary `.jfr` recordings can be converted to JSON before
//     parsing.
//
// Reasons surfaced through diagnostics are kept verbatim from Python so
// cross-engine parity holds (T-244 / T-390 gate).
package jfr

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

// Skip / error reasons mirror Python verbatim.
const (
	ReasonInvalidJFRJSON  = "INVALID_JFR_JSON"
	ReasonInvalidJFRShape = "INVALID_JFR_SHAPE"
	ReasonInvalidJFREvent = "INVALID_JFR_EVENT"
)

// rawPreviewLimit caps the JSON preview attached to a diagnostic
// sample. Matches Python's `[:500]` slice.
const rawPreviewLimit = 500

// Event mirrors Python's `JfrEvent` TypedDict. Pointer fields encode
// the Python `... | None` optional shape — the JSON tag carries
// `omitempty` only for slices/strings; numeric optionals stay
// pointer-typed so a missing value renders as `null`.
type Event struct {
	EventType  string   `json:"event_type"`
	Time       string   `json:"time"`
	DurationMS *float64 `json:"duration_ms"`
	Thread     *string  `json:"thread"`
	State      *string  `json:"state"`
	Address    *int64   `json:"address"`
	Size       *int64   `json:"size"`
	Message    string   `json:"message"`
	Frames     []string `json:"frames"`
	RawPreview string   `json:"raw_preview"`
}

// ShapeError is returned when the JSON payload is well-formed but does
// not contain an events array. Mirrors the Python `ValueError`
// surfaced by `_extract_events`.
type ShapeError struct {
	Message string
}

func (e *ShapeError) Error() string { return e.Message }

// JSONDecodeError wraps a JSON syntax/type error along with the
// approximate (line, column) the byte offset maps to. Mirrors the
// Python `json.JSONDecodeError` shape so callers can preserve the
// `line=N col=N` trail diagnostics expects.
type JSONDecodeError struct {
	Message string
	Line    int
	Column  int
	Offset  int64
}

func (e *JSONDecodeError) Error() string {
	return fmt.Sprintf("%s: line %d col %d", e.Message, e.Line, e.Column)
}

// ParseJSONFile reads `path` and parses it as `jfr print --json`
// output. Mirrors `parse_jfr_print_json`. When `diags` is nil a fresh
// builder is allocated internally so this function can be called
// without a pre-existing diagnostics scope.
//
// On a JSON syntax error: diags is populated with a single
// INVALID_JFR_JSON sample and a *JSONDecodeError is returned. On a
// shape error (well-formed JSON, no events array): diags gets an
// INVALID_JFR_SHAPE sample and *ShapeError is returned. Per-event
// failures are recorded as INVALID_JFR_EVENT and skipped without
// halting the parse.
func ParseJSONFile(path string, diags *diagnostics.ParserDiagnostics) ([]Event, error) {
	if diags == nil {
		diags = diagnostics.New("jfr")
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		decodeErr := jsonDecodeErrorFrom(body, err)
		diags.TotalLines = 1
		diags.AddSkipped(
			decodeErr.Line,
			ReasonInvalidJFRJSON,
			decodeErr.Message,
			fmt.Sprintf("line=%d col=%d", decodeErr.Line, decodeErr.Column),
		)
		return nil, decodeErr
	}

	events, shapeErr := extractEvents(payload)
	if shapeErr != nil {
		diags.TotalLines = 1
		diags.AddSkipped(
			1,
			ReasonInvalidJFRShape,
			shapeErr.Message,
			previewJSON(payload),
		)
		return nil, shapeErr
	}

	parsed := make([]Event, 0, len(events))
	diags.TotalLines = len(events)
	for index, raw := range events {
		lineNumber := index + 1
		obj, ok := raw.(map[string]any)
		if !ok {
			diags.AddSkipped(
				lineNumber,
				ReasonInvalidJFREvent,
				"JFR event entry must be an object.",
				previewJSON(raw),
			)
			continue
		}
		parsed = append(parsed, toEvent(obj))
		diags.ParsedRecords++
	}
	return parsed, nil
}

// extractEvents mirrors `_extract_events` — accepts a top-level array,
// an object with `recording.events`, or an object with `events`.
func extractEvents(payload any) ([]any, *ShapeError) {
	if list, ok := payload.([]any); ok {
		return list, nil
	}
	obj, ok := payload.(map[string]any)
	if !ok {
		return nil, &ShapeError{Message: "JFR JSON payload must be an object or array."}
	}
	if recording, ok := obj["recording"].(map[string]any); ok {
		if events, ok := recording["events"].([]any); ok {
			return events, nil
		}
	}
	if events, ok := obj["events"].([]any); ok {
		return events, nil
	}
	return nil, &ShapeError{Message: "JFR JSON payload does not contain an events array."}
}

// toEvent mirrors `_to_jfr_event`. Field-by-field translation of the
// Python helpers. The `raw_preview` is a deterministic JSON
// representation (sorted keys, no extra whitespace) capped at 500
// chars.
func toEvent(event map[string]any) Event {
	values, _ := event["values"].(map[string]any)
	if values == nil {
		values = map[string]any{}
	}

	eventType := stringValuePtr(event["type"])
	if eventType == nil {
		eventType = stringValuePtr(event["eventType"])
	}

	timeStr := stringValuePtr(values["startTime"])
	if timeStr == nil {
		timeStr = stringValuePtr(event["startTime"])
	}

	durationRaw, hasValuesDuration := values["duration"]
	if !hasValuesDuration || durationRaw == nil {
		durationRaw = event["duration"]
	}
	durationMS := durationToMS(durationRaw)

	threadRaw, hasEventThread := values["eventThread"]
	if !hasEventThread || threadRaw == nil {
		threadRaw = values["thread"]
	}
	thread := extractThread(threadRaw)

	frames := extractFrames(values["stackTrace"])

	message := stringValuePtr(values["message"])
	if message == nil {
		message = stringValuePtr(values["name"])
	}

	var state *string
	if stateMap, ok := values["state"].(map[string]any); ok {
		state = stringValuePtr(stateMap["name"])
	} else {
		state = stringValuePtr(values["state"])
	}

	address := toInt(values["address"])

	sizeRaw, hasSize := values["size"]
	if !hasSize || sizeRaw == nil {
		if alloc, ok := values["allocationSize"]; ok {
			sizeRaw = alloc
		} else {
			sizeRaw = values["bytes"]
		}
	}
	size := toInt(sizeRaw)

	out := Event{
		EventType:  derefOr(eventType, "unknown"),
		Time:       derefOr(timeStr, ""),
		DurationMS: durationMS,
		Thread:     thread,
		State:      state,
		Address:    address,
		Size:       size,
		Message:    derefOr(message, ""),
		Frames:     frames,
		RawPreview: previewJSON(event),
	}
	return out
}

// toInt mirrors `_to_int`. Booleans (Python `bool` subclass of `int`)
// have no Go equivalent, but `json.Unmarshal` decodes JSON booleans
// into Go `bool` so we reject those explicitly. Hex prefixes accepted
// only on string inputs, matching Python.
func toInt(value any) *int64 {
	switch v := value.(type) {
	case nil:
		return nil
	case bool:
		return nil
	case float64:
		// json.Unmarshal returns all JSON numbers as float64.
		i := int64(v)
		return &i
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return &i
		}
		if f, err := v.Float64(); err == nil {
			i := int64(f)
			return &i
		}
		return nil
	case int:
		i := int64(v)
		return &i
	case int64:
		i := v
		return &i
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return nil
		}
		if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
			if i, err := strconv.ParseInt(text[2:], 16, 64); err == nil {
				return &i
			}
			return nil
		}
		if i, err := strconv.ParseInt(text, 10, 64); err == nil {
			return &i
		}
		return nil
	}
	return nil
}

// extractThread mirrors `_extract_thread`. Strings pass through; dicts
// are searched in (javaName, osName, name) order, returning the first
// non-empty string.
func extractThread(value any) *string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil
		}
		s := v
		return &s
	case map[string]any:
		for _, key := range []string{"javaName", "osName", "name"} {
			if s := stringValuePtr(v[key]); s != nil {
				return s
			}
		}
	}
	return nil
}

// extractFrames mirrors `_extract_frames`. Returns an empty slice
// when the stack trace is missing or malformed — Python returns `[]`.
func extractFrames(value any) []string {
	obj, ok := value.(map[string]any)
	if !ok {
		return []string{}
	}
	frames, ok := obj["frames"].([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(frames))
	for _, frame := range frames {
		if frameMap, ok := frame.(map[string]any); ok {
			out = append(out, formatFrame(frameMap))
		}
	}
	return out
}

// formatFrame mirrors `_format_frame`.
func formatFrame(frame map[string]any) string {
	if method, ok := frame["method"].(map[string]any); ok {
		methodName := stringValuePtr(method["name"])
		var typeName *string
		if typeValue, ok := method["type"].(map[string]any); ok {
			typeName = stringValuePtr(typeValue["name"])
		}
		if typeName != nil && methodName != nil {
			return *typeName + "." + *methodName
		}
		if methodName != nil {
			return *methodName
		}
	}
	if name := stringValuePtr(frame["name"]); name != nil {
		return *name
	}
	return "unknown"
}

// durationToMS mirrors `_duration_to_ms`. Numeric values pass through
// as float64; strings carry an optional unit suffix.
func durationToMS(value any) *float64 {
	switch v := value.(type) {
	case nil:
		return nil
	case float64:
		f := v
		return &f
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return &f
		}
		return nil
	case int:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return nil
		}
		// Match Python's order: " ms" first (substring of " s"), then
		// " ns", " us", " s", finally bare float.
		if num, ok := tryStripSuffix(text, " ms"); ok {
			if f, err := strconv.ParseFloat(num, 64); err == nil {
				return &f
			}
			return nil
		}
		if num, ok := tryStripSuffix(text, " ns"); ok {
			if f, err := strconv.ParseFloat(num, 64); err == nil {
				out := f / 1_000_000
				return &out
			}
			return nil
		}
		if num, ok := tryStripSuffix(text, " us"); ok {
			if f, err := strconv.ParseFloat(num, 64); err == nil {
				out := f / 1_000
				return &out
			}
			return nil
		}
		if num, ok := tryStripSuffix(text, " s"); ok {
			if f, err := strconv.ParseFloat(num, 64); err == nil {
				out := f * 1_000
				return &out
			}
			return nil
		}
		if f, err := strconv.ParseFloat(text, 64); err == nil {
			return &f
		}
		return nil
	}
	return nil
}

func tryStripSuffix(text, suffix string) (string, bool) {
	if strings.HasSuffix(text, suffix) {
		return strings.TrimSuffix(text, suffix), true
	}
	return "", false
}

// stringValuePtr mirrors `_string_value` — non-empty strings only,
// returns nil for the empty string or non-string input.
func stringValuePtr(value any) *string {
	if s, ok := value.(string); ok && s != "" {
		out := s
		return &out
	}
	return nil
}

func derefOr(p *string, fallback string) string {
	if p == nil {
		return fallback
	}
	return *p
}

// previewJSON mirrors `_preview_json` — emits a deterministic
// (sorted-key) JSON string truncated to 500 chars. On any
// marshalling failure it falls back to a Go formatted preview, which
// is the closest equivalent to Python's `repr(value)[:500]`.
func previewJSON(value any) string {
	body, err := marshalSorted(value)
	if err != nil {
		fallback := fmt.Sprintf("%#v", value)
		if len(fallback) > rawPreviewLimit {
			fallback = fallback[:rawPreviewLimit]
		}
		return fallback
	}
	if len(body) > rawPreviewLimit {
		body = body[:rawPreviewLimit]
	}
	return body
}

// marshalSorted produces JSON with map keys in sorted order. Go's
// `encoding/json` already sorts `map[string]any` keys, so this is
// mostly a thin wrapper — but we go through a normalisation pass so
// nested structures we built ourselves come out deterministic too.
func marshalSorted(value any) (string, error) {
	normalised := normaliseForSort(value)
	body, err := json.Marshal(normalised)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func normaliseForSort(value any) any {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(v))
		for _, k := range keys {
			out[k] = normaliseForSort(v[k])
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normaliseForSort(item)
		}
		return out
	default:
		return v
	}
}

// jsonDecodeErrorFrom converts an `encoding/json` error into a
// JSONDecodeError carrying line/column. The Go syntax/type error
// surfaces the byte offset; we walk `body` once to translate. Mirrors
// the (lineno, colno, msg) trio Python's `json.JSONDecodeError`
// exposes.
func jsonDecodeErrorFrom(body []byte, err error) *JSONDecodeError {
	out := &JSONDecodeError{Message: err.Error(), Line: 1, Column: 1}
	switch e := err.(type) {
	case *json.SyntaxError:
		out.Offset = e.Offset
		out.Message = e.Error()
	case *json.UnmarshalTypeError:
		out.Offset = e.Offset
		out.Message = e.Error()
	default:
		return out
	}
	line, col := offsetToLineCol(body, out.Offset)
	out.Line = line
	out.Column = col
	return out
}

// offsetToLineCol converts a byte offset into 1-indexed (line, column)
// the way Python's json module does. An offset past EOF clamps to the
// last position rather than panicking.
func offsetToLineCol(body []byte, offset int64) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(body)) {
		offset = int64(len(body))
	}
	line := 1
	col := 1
	for i := int64(0); i < offset; i++ {
		if body[i] == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return line, col
}
