// Package nodejsreport ports archscope_engine.parsers.thread_dump.nodejs_report.
//
// Two plugins live here, mirroring the Python module:
//
//   - DiagnosticReportPlugin parses the JSON emitted by Node.js's
//     `process.report.writeReport()` (Node 12+). The report's main JS
//     thread becomes one snapshot tagged language="javascript"; the
//     native worker-pool stack — when present — becomes a second
//     snapshot tagged language="native" so JS-only enrichers can target
//     just the JS frames. Active libuv handles drive a state-inference
//     pass that promotes a plain RUNNABLE main thread to NETWORK_WAIT or
//     IO_WAIT depending on the dominant handle kind.
//
//   - SampleTracePlugin parses the simpler "Sample #N\nError\n  at ..."
//     text traces that Node emits via `--prof` style scripts and ad-hoc
//     diagnostic samplers. Each Sample block becomes one snapshot.
//
// Format-id parity (`nodejs_diagnostic_report`, `nodejs_sample_trace`)
// is preserved with the Python plugin so the registry detection +
// FormatOverride wiring keeps working unchanged across the two engines.
package nodejsreport

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// FormatID constants (kept exported so callers can reference them via
// the registry without reaching into Plugin instances).
const (
	FormatIDDiagnosticReport = "nodejs_diagnostic_report"
	FormatIDSampleTrace      = "nodejs_sample_trace"
	Language                 = "javascript"
	NativeLanguage           = "native"
)

// detectRe mirrors Python's `_DETECT_RE`. The 4 KB head almost always
// contains the opening brace + first few keys, so a single forgiving
// regex is enough to claim the file. (?s) enables dotall so the gap
// between "header" and "javascriptStack" can include newlines.
var detectRe = regexp.MustCompile(`(?s)"header"\s*:\s*\{.*?"javascriptStack"\s*:`)

// jsFrameRe mirrors `_JS_FRAME_RE`. Captures `func`, `file`, `line`,
// `col` for stack lines like `    at handler (/app/server.js:42:5)`.
var jsFrameRe = regexp.MustCompile(`^\s*at\s+(?P<func>[^()]+?)\s+\((?P<file>[^)]+):(?P<line>\d+):(?P<col>\d+)\)\s*$`)

// jsLocationOnlyRe is the anonymous-callback fallback `at /app/foo.js:1:2`.
var jsLocationOnlyRe = regexp.MustCompile(`^\s*at\s+(?P<file>[^:]+):(?P<line>\d+):\d+`)

// libuv handle classifications used by InferJSState. Mirrors the Python
// kind sets verbatim.
var (
	networkHandleKinds = map[string]struct{}{
		"tcp":  {},
		"udp":  {},
		"pipe": {},
	}
	fileHandleKinds = map[string]struct{}{
		"timer":   {},
		"fs_event": {},
		"fs_poll":  {},
	}
)

// DiagnosticReportPlugin parses `process.report.writeReport()` JSON.
// Implements threaddump.Plugin.
type DiagnosticReportPlugin struct{}

// FormatID returns "nodejs_diagnostic_report".
func (DiagnosticReportPlugin) FormatID() string { return FormatIDDiagnosticReport }

// Language returns "javascript".
func (DiagnosticReportPlugin) Language() string { return Language }

// CanParse uses a single forgiving regex sniff over the head bytes.
func (DiagnosticReportPlugin) CanParse(head string) bool {
	return detectRe.MatchString(head)
}

// Parse reads the file end-to-end and decodes it as JSON. On a JSON
// error we still return a structured (empty) bundle with a parse_error
// metadata flag — same forgiving behaviour as the Python plugin so the
// registry caller never sees an exception bubble.
func (p DiagnosticReportPlugin) Parse(path string) (models.ThreadDumpBundle, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}

	var payload map[string]any
	if jsonErr := json.Unmarshal(raw, &payload); jsonErr != nil {
		bundle := models.NewThreadDumpBundle(path, FormatIDDiagnosticReport, Language)
		bundle.Metadata["parse_error"] = "INVALID_NODEJS_REPORT_JSON"
		return bundle, nil
	}

	bundle := models.NewThreadDumpBundle(path, FormatIDDiagnosticReport, Language)
	bundle.Snapshots = append(bundle.Snapshots, jsMainSnapshot(payload))
	if native, ok := nativeSnapshot(payload); ok {
		bundle.Snapshots = append(bundle.Snapshots, native)
	}

	if libuv, ok := payload["libuv"].([]any); ok {
		// Python: `payload["libuv"][:50]`. Preserve the slice cap so
		// downstream consumers see the same size budget.
		limit := len(libuv)
		if limit > 50 {
			limit = 50
		}
		bundle.Metadata["libuv_handles"] = libuv[:limit]
	}
	if header, ok := payload["header"].(map[string]any); ok {
		filtered := map[string]any{}
		for _, key := range []string{"event", "trigger", "filename", "nodejsVersion"} {
			if v, present := header[key]; present {
				filtered[key] = v
			}
		}
		bundle.Metadata["header"] = filtered
	}

	return bundle, nil
}

// jsMainSnapshot extracts the JS main-thread snapshot from the payload.
// The frames go through normalizeJSFrame so `Layer.handle [as ...]`
// aliases get stripped consistently.
func jsMainSnapshot(payload map[string]any) models.ThreadSnapshot {
	jsSection, _ := payload["javascriptStack"].(map[string]any)
	if jsSection == nil {
		jsSection = map[string]any{}
	}

	frames := []models.StackFrame{}
	if rawFrames, ok := jsSection["stack"].([]any); ok {
		for _, raw := range rawFrames {
			s, ok := raw.(string)
			if !ok {
				continue
			}
			frame := parseJSFrame(s)
			frames = append(frames, normalizeJSFrame(frame))
		}
	}

	state := InferJSState(models.ThreadStateRunnable, payload)
	threadID := "main"
	category := string(state)
	lang := Language
	format := FormatIDDiagnosticReport

	snap := models.NewThreadSnapshot("javascript::0::main", "main", state)
	snap.ThreadID = &threadID
	snap.Category = &category
	snap.StackFrames = frames
	snap.Language = &lang
	snap.SourceFormat = &format
	// Python stores `{"javascript_message": js_section.get("message")}`
	// — `None` becomes JSON `null`. Use a Go nil interface for parity.
	snap.Metadata["javascript_message"] = jsSection["message"]
	return snap
}

// nativeSnapshot extracts the native worker-pool snapshot from the
// payload. Returns (zero, false) when nativeStack is missing or empty.
func nativeSnapshot(payload map[string]any) (models.ThreadSnapshot, bool) {
	raw, ok := payload["nativeStack"].([]any)
	if !ok || len(raw) == 0 {
		return models.ThreadSnapshot{}, false
	}

	frames := []models.StackFrame{}
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		symbol := pickString(entry, "symbol", "name")
		if symbol == "" {
			symbol = "<unknown>"
		}
		frame := models.StackFrame{Function: symbol}
		nativeLang := NativeLanguage
		frame.Language = &nativeLang
		if file, ok := entry["file"].(string); ok {
			f := file
			frame.File = &f
		}
		// Python: `int(entry["line"]) if isinstance(entry.get("line"), int) else None`.
		// JSON numbers decode as float64 in Go, but Python's `isinstance(x, int)`
		// rejects floats. Mirror that strictness.
		if lineVal, present := entry["line"]; present {
			if line, ok := asInt(lineVal); ok {
				frame.Line = &line
			}
		}
		frames = append(frames, frame)
	}

	threadID := "native"
	lang := NativeLanguage
	format := FormatIDDiagnosticReport
	snap := models.NewThreadSnapshot("native::0::worker-pool", "native", models.ThreadStateUnknown)
	snap.ThreadID = &threadID
	snap.StackFrames = frames
	snap.Language = &lang
	snap.SourceFormat = &format
	// Category stays nil — Python passes `category=None` explicitly.
	return snap, true
}

// parseJSFrame mirrors `_parse_js_frame`. The full match wins; the
// location-only fallback handles anonymous callbacks; everything else
// degrades to the raw text as the function name.
func parseJSFrame(text string) models.StackFrame {
	text = strings.TrimSpace(text)
	lang := Language

	if match := jsFrameRe.FindStringSubmatch(text); match != nil {
		groups := namedGroups(jsFrameRe, match)
		frame := models.StackFrame{
			Function: strings.TrimSpace(groups["func"]),
			Language: &lang,
		}
		file := groups["file"]
		frame.File = &file
		if line, err := strconv.Atoi(groups["line"]); err == nil {
			frame.Line = &line
		}
		return frame
	}

	if loc := jsLocationOnlyRe.FindStringSubmatch(text); loc != nil {
		groups := namedGroups(jsLocationOnlyRe, loc)
		if line, err := strconv.Atoi(groups["line"]); err == nil {
			file := groups["file"]
			return models.StackFrame{
				Function: "<anonymous>",
				File:     &file,
				Line:     &line,
				Language: &lang,
			}
		}
	}

	return models.StackFrame{Function: text, Language: &lang}
}

// normalizeJSFrame mirrors `_normalize_js_frame`. Strips `[as ...]`
// alias suffixes that Express adds to function names so framework
// boilerplate doesn't pollute stack signatures.
func normalizeJSFrame(frame models.StackFrame) models.StackFrame {
	if frame.Language == nil || *frame.Language != Language {
		return frame
	}
	if frame.Function == "" || !strings.Contains(frame.Function, "[as ") {
		return frame
	}
	stripped := strings.TrimSpace(strings.SplitN(frame.Function, "[as ", 2)[0])
	if stripped == "" {
		// Python falls back to the original name when the prefix is
		// empty (`frame_function or frame.function`).
		return frame
	}
	out := frame
	out.Function = stripped
	return out
}

// InferJSState mirrors `_infer_js_state`. The diagnostic report is
// captured *because* something interesting was happening on the main
// thread, so a plain RUNNABLE rarely tells the full story:
//
//   - Active TCP/UDP/pipe handles → NETWORK_WAIT (uv__io_poll).
//   - Active timer/fs_event/fs_poll handles → IO_WAIT.
//   - Otherwise the input state is preserved.
//
// Exported because the JS-enrichment plugin (T-201 follow-up) may want
// to re-run inference when handle data is supplied out-of-band.
func InferJSState(state models.ThreadState, payload map[string]any) models.ThreadState {
	libuv, ok := payload["libuv"].([]any)
	if !ok {
		return state
	}
	networkActive := 0
	fileActive := 0
	for _, item := range libuv {
		handle, ok := item.(map[string]any)
		if !ok {
			continue
		}
		// Python `if not handle.get("is_active"): continue` — falsy
		// covers missing / false / 0 / empty alike.
		if !truthy(handle["is_active"]) {
			continue
		}
		kind, _ := handle["type"].(string)
		kind = strings.ToLower(kind)
		if _, hit := networkHandleKinds[kind]; hit {
			networkActive++
		} else if _, hit := fileHandleKinds[kind]; hit {
			fileActive++
		}
	}
	if networkActive > 0 {
		return models.ThreadStateNetworkWait
	}
	if fileActive > 0 {
		return models.ThreadStateIOWait
	}
	return state
}

// ---------------------------------------------------------------------
// Sample-trace plugin
// ---------------------------------------------------------------------

// sampleHeaderRe mirrors `_NODEJS_SAMPLE_HEADER_RE`. The Python regex
// uses re.MULTILINE so `^Sample #N$` matches anywhere in the text;
// CanParse uses a contains-style search and Parse anchors per-line.
var (
	sampleHeaderRe       = regexp.MustCompile(`(?m)^Sample\s+#(?P<index>\d+)\s*$`)
	sampleHeaderLineOnly = regexp.MustCompile(`^Sample\s+#(?P<index>\d+)\s*$`)
	sampleFrameRe        = regexp.MustCompile(`^\s+at\s+(?P<func>[^()]+?)\s+\((?P<file>[^)]+):(?P<line>\d+):(?P<col>\d+)\)\s*$`)
)

// SampleTracePlugin parses the simpler "Sample #N" text trace format.
type SampleTracePlugin struct{}

// FormatID returns "nodejs_sample_trace".
func (SampleTracePlugin) FormatID() string { return FormatIDSampleTrace }

// Language returns "javascript".
func (SampleTracePlugin) Language() string { return Language }

// CanParse looks for at least one Sample-block header in the head bytes.
func (SampleTracePlugin) CanParse(head string) bool {
	return sampleHeaderRe.MatchString(head)
}

// Parse iterates the file line-by-line, accumulating frames inside each
// Sample block and flushing them on the next header (and at EOF).
func (SampleTracePlugin) Parse(path string) (models.ThreadDumpBundle, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}
	bundle := models.NewThreadDumpBundle(path, FormatIDSampleTrace, Language)

	currentIndex := -1
	currentFrames := []models.StackFrame{}
	sampleIdx := 0

	flush := func() {
		if currentIndex >= 0 {
			bundle.Snapshots = append(bundle.Snapshots, makeSampleSnapshot(currentIndex, currentFrames, sampleIdx))
			sampleIdx++
		}
	}

	// Splitting on \n then trimming \r mirrors `text.splitlines()` +
	// `raw_line.rstrip("\r")` — both treat \r\n / \n / lone-\r blocks
	// the same way for our purposes.
	for _, rawLine := range strings.Split(string(raw), "\n") {
		line := strings.TrimRight(rawLine, "\r")
		if header := sampleHeaderLineOnly.FindStringSubmatch(line); header != nil {
			flush()
			groups := namedGroups(sampleHeaderLineOnly, header)
			if idx, parseErr := strconv.Atoi(groups["index"]); parseErr == nil {
				currentIndex = idx
			} else {
				currentIndex = 0
			}
			currentFrames = []models.StackFrame{}
			continue
		}
		if currentIndex < 0 {
			continue
		}
		if frame := sampleFrameRe.FindStringSubmatch(line); frame != nil {
			groups := namedGroups(sampleFrameRe, frame)
			lang := Language
			f := models.StackFrame{
				Function: strings.TrimSpace(groups["func"]),
				Language: &lang,
			}
			file := groups["file"]
			f.File = &file
			if ln, lnErr := strconv.Atoi(groups["line"]); lnErr == nil {
				f.Line = &ln
			}
			currentFrames = append(currentFrames, f)
		}
	}
	flush()
	return bundle, nil
}

func makeSampleSnapshot(sampleID int, frames []models.StackFrame, snapshotIdx int) models.ThreadSnapshot {
	threadID := strconv.Itoa(sampleID)
	lang := Language
	format := FormatIDSampleTrace
	snap := models.NewThreadSnapshot(
		"nodejs::"+strconv.Itoa(snapshotIdx)+"::sample-"+strconv.Itoa(sampleID),
		"sample-"+strconv.Itoa(sampleID),
		models.ThreadStateRunnable,
	)
	snap.ThreadID = &threadID
	snap.StackFrames = frames
	snap.Language = &lang
	snap.SourceFormat = &format
	snap.Metadata["sample_index"] = sampleID
	return snap
}

// ---------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------

// namedGroups returns the named-capture map for one regex match. Goes
// through SubexpNames so we don't have to know offsets.
func namedGroups(re *regexp.Regexp, match []string) map[string]string {
	out := make(map[string]string, len(re.SubexpNames()))
	for i, name := range re.SubexpNames() {
		if name == "" {
			continue
		}
		out[name] = match[i]
	}
	return out
}

// pickString returns the first string value found among `keys`; "" if
// none of the keys are present or hold non-string values.
func pickString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}

// asInt mirrors Python's strict `isinstance(x, int)` filter for JSON
// numbers. JSON decoding gives us float64; we accept it only when the
// value is an integral float. (Python's `isinstance(True, int)` quirk
// is preserved by also accepting bool.)
func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		i := int(n)
		if float64(i) == n {
			return i, true
		}
		return 0, false
	case int:
		return n, true
	case int64:
		return int(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
		return 0, false
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// truthy mirrors Python's "falsy" check for libuv `is_active`. Treats
// missing / nil / false / 0 / empty-string / empty-collections as
// false, everything else as true.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case float64:
		return x != 0
	case int:
		return x != 0
	case int64:
		return x != 0
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}
