// Package javajcmdjson ports archscope_engine.parsers.thread_dump.java_jcmd_json.
//
// `jcmd <pid> Thread.dump_to_file -format=json` emits a structured JSON
// thread dump with an explicit thread state, a stack array per thread,
// and (on JDK 21+) virtual-thread / carrier markers. This plugin keeps
// it separate from the text jstack parser so Java-specific JSON shape
// assumptions do not leak into the multi-language registry.
//
// The JSON schema is intentionally loose — JDK versions disagree about
// key names (`name` vs `threadName`, `tid` vs `nid` vs `threadId`,
// `stack` vs `stackTrace` vs `frames`, ...) so the plugin walks the
// payload as `map[string]any` and probes a small set of candidate keys
// for each field.
package javajcmdjson

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

// FormatID is the registry identifier surfaced via FormatOverride and
// stamped onto every emitted bundle's SourceFormat.
const FormatID = "java_jcmd_json"

// Language is the runtime tag set on every emitted bundle / snapshot.
const Language = "java"

// Plugin implements threaddump.Plugin for jcmd JSON dumps. Stateless;
// the zero value is usable, but callers typically use New() so future
// fields can be added without breaking call sites.
type Plugin struct{}

// New constructs a ready-to-register plugin instance.
func New() *Plugin { return &Plugin{} }

// FormatID returns the stable registry identifier.
func (Plugin) FormatID() string { return FormatID }

// Language returns the runtime tag.
func (Plugin) Language() string { return Language }

// CanParse mirrors the Python `can_parse`: a leading `{` plus the
// literal `"threadDump"` somewhere in the head sample. The substring
// check is deliberately strict — the registry hands us at most 4 KiB,
// and the threadDump key always sits inside the first object.
func (Plugin) CanParse(head string) bool {
	stripped := strings.TrimLeft(head, " \t\r\n")
	if !strings.HasPrefix(stripped, "{") {
		return false
	}
	return strings.Contains(head, `"threadDump"`)
}

// Parse reads the entire file, decodes the JSON payload, and returns a
// single bundle with one snapshot per discovered thread object. Errors
// mirror Python's ValueError messages so end-to-end tests stay aligned.
func (p Plugin) Parse(path string) (models.ThreadDumpBundle, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}
	encoding, err := textio.DetectFromBytes(raw, nil)
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}
	decoded, err := textio.DecodeBytes(raw, encoding)
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}

	var payload any
	dec := json.NewDecoder(strings.NewReader(decoded))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		// json.Decoder doesn't expose line/col like Python's
		// JSONDecodeError, so we surface the underlying message and
		// the file path. Tests should match on the prefix.
		return models.ThreadDumpBundle{}, fmt.Errorf(
			"Malformed java_jcmd_json thread dump in %s: %s.", path, err,
		)
	}

	root, ok := payload.(map[string]any)
	if !ok {
		return models.ThreadDumpBundle{}, fmt.Errorf(
			"Unsupported java_jcmd_json thread dump: missing required "+
				"'threadDump' object. Found: %q.", typeName(payload),
		)
	}
	if _, present := root["threadDump"]; !present {
		keys := sortedKeys(root)
		return models.ThreadDumpBundle{}, fmt.Errorf(
			"Unsupported java_jcmd_json thread dump: missing required "+
				"'threadDump' object. Found: %v.", keys,
		)
	}

	threads := iterThreadObjects(root)
	snapshots := make([]models.ThreadSnapshot, 0, len(threads))
	for index, thread := range threads {
		snapshots = append(snapshots, snapshotFromThread(thread, index))
	}
	if len(snapshots) == 0 {
		return models.ThreadDumpBundle{}, fmt.Errorf(
			"Unsupported java_jcmd_json thread dump: no thread objects were " +
				"found under 'threadDump'.",
		)
	}

	bundle := models.NewThreadDumpBundle(path, FormatID, Language)
	bundle.Snapshots = snapshots
	rawTimestamp := firstString(root, "time", "timestamp")
	bundle.CapturedAt = parseTimestamp(rawTimestamp)
	bundle.Metadata = map[string]any{
		"source":        "jcmd_json",
		"thread_count":  len(snapshots),
		"raw_timestamp": rawTimestamp,
	}
	return bundle, nil
}

// snapshotFromThread builds one ThreadSnapshot from a thread map. The
// shape mirrors Python `_snapshot_from_thread` byte-for-byte, including
// the `java_json::0::<index>::<name>` snapshot id (the leading `0` is
// the section index — jcmd JSON only has one section per file, but the
// id stays parallel to the jstack one for easy cross-referencing).
func snapshotFromThread(thread map[string]any, index int) models.ThreadSnapshot {
	name := firstString(thread, "name", "threadName")
	if name == "" {
		name = fmt.Sprintf("thread-%d", index)
	}
	threadID := firstStringPtr(thread, "tid", "nid", "threadId", "id")
	rawState := firstString(thread, "threadState", "state")

	frames := framesFromThread(thread)
	for i := range frames {
		frames[i] = normalizeProxyFrame(frames[i])
	}

	state := inferJavaState(models.CoerceThreadState(rawState), frames)
	rawText := compactJSON(thread)
	lockHolds, lockWaiting := extractJSONLocks(thread, rawText)
	if lockWaiting != nil && lockWaiting.WaitMode == "lock_entry_wait" &&
		(state == models.ThreadStateRunnable || state == models.ThreadStateUnknown) {
		state = models.ThreadStateLockWait
	} else if lockWaiting != nil && state == models.ThreadStateUnknown {
		state = models.ThreadStateWaiting
	}

	metadata := map[string]any{
		"source_fields": sourceFields(thread),
	}
	if isVirtualJSONThread(thread, rawText) {
		metadata["is_virtual_thread"] = true
	}
	if native := nativeMethodFromFrames(frames); native != "" {
		metadata["native_method"] = native
	}
	if pinning := carrierPinning(rawText, frames); pinning != nil {
		metadata["carrier_pinning"] = pinning
	}
	if lockWaiting != nil && lockWaiting.WaitMode != "" {
		metadata["monitor_wait_mode"] = lockWaiting.WaitMode
	}

	snap := models.NewThreadSnapshot(
		fmt.Sprintf("java_json::0::%d::%s", index, name),
		name,
		state,
	)
	snap.ThreadID = threadID
	category := string(state)
	snap.Category = &category
	snap.StackFrames = frames
	snap.Metadata = metadata
	lang := Language
	snap.Language = &lang
	format := FormatID
	snap.SourceFormat = &format
	snap.LockHolds = lockHolds
	snap.LockWaiting = lockWaiting
	return snap
}

// iterThreadObjects walks the payload depth-first and yields every
// dict that looks like a thread object. Mirrors Python
// `_iter_thread_objects` — the explicit stack + reversed-iteration
// trick keeps result order stable across Go map iteration.
//
// We process maps in *sorted-key order* (reversed onto the stack) so
// the output is deterministic — Python relied on dict insertion order,
// but Go's map iteration is randomized. Sorted keys are a defensible
// stand-in: the tests fix on thread *names*, not positions, and the
// snapshot id includes the index anyway.
func iterThreadObjects(root any) []map[string]any {
	var out []map[string]any
	stack := []any{root}
	for len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch v := item.(type) {
		case map[string]any:
			if looksLikeThread(v) {
				out = append(out, v)
				continue
			}
			keys := sortedKeys(v)
			// reverse so popping yields ascending order
			for i := len(keys) - 1; i >= 0; i-- {
				stack = append(stack, v[keys[i]])
			}
		case []any:
			for i := len(v) - 1; i >= 0; i-- {
				stack = append(stack, v[i])
			}
		}
	}
	return out
}

// looksLikeThread mirrors Python `_looks_like_thread`. A thread object
// has at least one name-y key plus either a stack-y key or a state-y
// key; that's loose enough to catch every JDK 17/21/22 variant.
func looksLikeThread(item map[string]any) bool {
	hasName := hasAny(item, "name", "threadName")
	hasStack := hasAny(item, "stack", "stackTrace", "frames")
	hasState := hasAny(item, "state", "threadState")
	return hasName && (hasStack || hasState)
}

// framesFromThread returns the stack-frame list for one thread. It
// tries the known stack-key aliases in priority order, matching
// Python's `or`-chain.
func framesFromThread(thread map[string]any) []models.StackFrame {
	var raw []any
	for _, key := range []string{"stack", "stackTrace", "frames"} {
		if value, ok := thread[key]; ok && value != nil {
			if list, ok := value.([]any); ok {
				raw = list
				break
			}
		}
	}
	frames := make([]models.StackFrame, 0, len(raw))
	for _, item := range raw {
		if frame, ok := frameFromJSONItem(item); ok {
			frames = append(frames, frame)
		}
	}
	return frames
}

// frameFromJSONItem normalizes one stack item. Items can be plain
// strings (`at foo.Bar.method(File.java:42)`) or structured dicts.
func frameFromJSONItem(item any) (models.StackFrame, bool) {
	switch v := item.(type) {
	case string:
		text := strings.TrimSpace(v)
		text = strings.TrimPrefix(text, "at ")
		return frameFromJstack(text), true
	case map[string]any:
		className := firstString(v, "className", "class", "declaringClass")
		methodName := firstString(v, "methodName", "method", "function")
		fileName := firstString(v, "fileName", "file")
		line := lineFromAny(v["lineNumber"])
		if line == nil {
			line = lineFromAny(v["line"])
		}
		if methodName == "" && className == "" {
			rendered := firstString(v, "frame", "text")
			if rendered == "" {
				return models.StackFrame{}, false
			}
			return frameFromJstack(rendered), true
		}
		function := methodName
		if function == "" {
			function = className
		}
		if function == "" {
			function = "(unknown)"
		}
		var module *string
		if methodName != "" && className != "" {
			c := className
			module = &c
		}
		var file *string
		if fileName != "" {
			f := fileName
			file = &f
		}
		lang := Language
		return models.StackFrame{
			Function: function,
			Module:   module,
			File:     file,
			Line:     line,
			Language: &lang,
		}, true
	default:
		return models.StackFrame{}, false
	}
}

// lineFromAny coerces a JSON line-number value (which can be a json.Number,
// a stringified int, a float64 from older Decode paths, or nil) into
// `*int` for the stack frame. Returns nil for any unparseable input —
// matching Python's `try: int(...) except ValueError: None` shape.
func lineFromAny(value any) *int {
	switch v := value.(type) {
	case nil:
		return nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return nil
		}
		out := int(n)
		return &out
	case float64:
		out := int(v)
		return &out
	case int:
		out := v
		return &out
	case int64:
		out := int(v)
		return &out
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return nil
		}
		var n int
		if _, err := fmt.Sscanf(text, "%d", &n); err != nil {
			return nil
		}
		return &n
	default:
		return nil
	}
}

// firstString returns the first non-empty stringified value among the
// candidate keys, mirroring Python `_first_text`. Numeric values are
// rendered via fmt to keep parity with `str(value)`.
func firstString(thread map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := thread[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			return v
		case json.Number:
			return v.String()
		case bool:
			if v {
				return "True"
			}
			return "False"
		case float64:
			return fmt.Sprintf("%g", v)
		default:
			return fmt.Sprint(v)
		}
	}
	return ""
}

// firstStringPtr is firstString that returns *string so the result can
// flow into a snapshot's optional ThreadID slot. Empty result -> nil.
func firstStringPtr(thread map[string]any, keys ...string) *string {
	if s := firstString(thread, keys...); s != "" {
		return &s
	}
	return nil
}

// hasAny reports whether the map has at least one of the candidate
// keys present (regardless of value).
func hasAny(item map[string]any, keys ...string) bool {
	for _, k := range keys {
		if _, ok := item[k]; ok {
			return true
		}
	}
	return false
}

// sortedKeys returns the map keys in lexical order — used for
// deterministic traversal and the `source_fields` metadata slice.
func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sourceFields returns the first 80 sorted keys of a thread object,
// matching Python `sorted(thread.keys())[:80]`. The cap shields the UI
// from pathological dumps that smuggle JSON garbage into the thread.
func sourceFields(thread map[string]any) []string {
	keys := sortedKeys(thread)
	if len(keys) > 80 {
		keys = keys[:80]
	}
	return keys
}

// compactJSON renders a thread map as compact JSON to feed the
// text-mode lock-handle / virtual-thread heuristics. Errors fall back
// to an empty string — heuristics simply skip when the text is empty.
func compactJSON(thread map[string]any) string {
	buf, err := json.Marshal(thread)
	if err != nil {
		return ""
	}
	return string(buf)
}

// typeName returns a Python-ish name for the top-level payload type,
// used in the "missing threadDump" error message when the payload is
// not even a JSON object.
func typeName(payload any) string {
	switch payload.(type) {
	case nil:
		return "NoneType"
	case []any:
		return "list"
	case string:
		return "str"
	case bool:
		return "bool"
	case json.Number:
		return "number"
	case float64:
		return "float"
	default:
		return fmt.Sprintf("%T", payload)
	}
}

// parseTimestamp accepts the `time` / `timestamp` field. JDK uses ISO
// 8601 with a trailing `Z`; Go's time.Parse needs RFC3339 for that.
// Returns nil for empty / unparseable values.
func parseTimestamp(value string) *time.Time {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.999999999",
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, text); err == nil {
			return &t
		}
	}
	return nil
}
