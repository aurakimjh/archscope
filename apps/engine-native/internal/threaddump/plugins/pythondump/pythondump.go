// Package pythondump ports
// archscope_engine.parsers.thread_dump.python_dump.
//
// Three flavors of Python dump are common in production and each one
// is exposed as a separate threaddump.Plugin so format detection is
// unambiguous:
//
//   - py-spy: ``py-spy dump --pid <pid>`` produces ``Process 12345:``
//     followed by per-thread blocks ``Thread <id> (<state>): "name"``
//     and indented ``func (file:line)`` frames.
//   - faulthandler: ``faulthandler.dump_traceback`` (and SIGSEGV crash
//     dumps) produces ``Thread 0x... (most recent call first):`` and
//     ``  File "...", line N in func`` frames.
//   - traceback: ``traceback.format_stack`` text prefixed with
//     ``Thread ID: <n>`` and frames like ``  File "...", line N, in func``.
//
// All three share a single enrichment pass (T-199 in Python) that
// strips Django/FastAPI/Flask middleware wrappers and promotes
// ``select.*`` / ``socket.recv`` / async sleep frames to the proper
// ThreadState.
package pythondump

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

// ---------------------------------------------------------------------------
// py-spy
// ---------------------------------------------------------------------------

// pyspyBannerRE matches the two-line banner emitted by `py-spy dump`.
// Python uses re.MULTILINE; in Go we encode the same intent with `(?m)`.
var pyspyBannerRE = regexp.MustCompile(`(?m)^Process\s+\d+:.*\n.*Python\s+v?\d+\.\d+`)

// pyspyThreadRE matches ``Thread <tid> (<state>): "name"``. The name
// portion is optional and is captured greedy through end-of-line.
var pyspyThreadRE = regexp.MustCompile(`^Thread\s+(\d+)\s+\(([^)]+)\):\s*(.*)$`)

// pyspyFrameRE matches ``    func (file:line)``. The function token
// disallows whitespace and ``(`` so qualified names like ``Class.method``
// still match.
var pyspyFrameRE = regexp.MustCompile(`^\s+([^\s(]+)\s+\(([^:]+):(\d+)\)\s*$`)

// PySpyPlugin parses ``py-spy dump`` text output.
type PySpyPlugin struct{}

// FormatID returns the stable identifier used by ParseOptions.FormatOverride
// and stamped onto ThreadDumpBundle.SourceFormat.
func (PySpyPlugin) FormatID() string { return "python_pyspy" }

// Language returns the runtime label.
func (PySpyPlugin) Language() string { return "python" }

// CanParse sniffs the head for the py-spy banner.
func (PySpyPlugin) CanParse(head string) bool { return pyspyBannerRE.MatchString(head) }

// Parse reads `path`, walks thread blocks, and returns one bundle.
func (p PySpyPlugin) Parse(path string) (models.ThreadDumpBundle, error) {
	bundle := models.NewThreadDumpBundle(path, p.FormatID(), p.Language())
	lines, err := readPythonDumpLines(path)
	if err != nil {
		return bundle, err
	}

	var current *models.ThreadSnapshot
	threadIndex := 0
	flush := func() {
		if current != nil {
			bundle.Snapshots = append(bundle.Snapshots, finalizePythonSnapshot(*current))
			current = nil
		}
	}
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if m := pyspyThreadRE.FindStringSubmatch(line); m != nil {
			flush()
			tid, rawState, rawName := m[1], m[2], m[3]
			name := strings.Trim(strings.TrimSpace(rawName), `"`)
			if name == "" {
				name = "thread-" + tid
			}
			state := models.CoerceThreadState(rawState)
			category := string(state)
			tidCopy := tid
			lang := "python"
			source := p.FormatID()
			snap := models.NewThreadSnapshot(
				"python::"+strconv.Itoa(threadIndex)+"::"+name,
				name,
				state,
			)
			snap.ThreadID = &tidCopy
			snap.Category = &category
			snap.Language = &lang
			snap.SourceFormat = &source
			snap.Metadata["raw_state"] = rawState
			current = &snap
			threadIndex++
			continue
		}
		if current == nil {
			continue
		}
		if m := pyspyFrameRE.FindStringSubmatch(line); m != nil {
			current.StackFrames = append(current.StackFrames, makePythonFrame(m[1], m[2], m[3]))
		}
	}
	flush()
	return bundle, nil
}

// ---------------------------------------------------------------------------
// faulthandler
// ---------------------------------------------------------------------------

// faulthandlerThreadRE matches ``Thread 0x<hex> (most recent call first):``.
var faulthandlerThreadRE = regexp.MustCompile(`(?m)^Thread\s+0x[0-9a-fA-F]+\s+\(most recent call first\):\s*$`)

// faulthandlerThreadLineRE matches the same header at line start when
// scanning line-by-line (no MULTILINE flag needed at parse time).
var faulthandlerThreadLineRE = regexp.MustCompile(`^Thread\s+0x[0-9a-fA-F]+\s+\(most recent call first\):\s*$`)

// faulthandlerHexRE pulls the hex tid for use as snapshot id.
var faulthandlerHexRE = regexp.MustCompile(`0x[0-9a-fA-F]+`)

// faulthandlerFrameRE matches ``  File "...", line N in func``.
var faulthandlerFrameRE = regexp.MustCompile(`^\s+File\s+"([^"]+)",\s+line\s+(\d+)\s+in\s+(\S+)\s*$`)

// FaulthandlerPlugin parses ``faulthandler.dump_traceback`` output.
type FaulthandlerPlugin struct{}

// FormatID returns the stable identifier used by ParseOptions.FormatOverride.
func (FaulthandlerPlugin) FormatID() string { return "python_faulthandler" }

// Language returns the runtime label.
func (FaulthandlerPlugin) Language() string { return "python" }

// CanParse sniffs the head for ``Thread 0x... (most recent call first):``.
func (FaulthandlerPlugin) CanParse(head string) bool { return faulthandlerThreadRE.MatchString(head) }

// Parse reads `path` and returns one bundle of thread snapshots.
func (p FaulthandlerPlugin) Parse(path string) (models.ThreadDumpBundle, error) {
	bundle := models.NewThreadDumpBundle(path, p.FormatID(), p.Language())
	lines, err := readPythonDumpLines(path)
	if err != nil {
		return bundle, err
	}

	var current *models.ThreadSnapshot
	threadIndex := 0
	flush := func() {
		if current != nil {
			bundle.Snapshots = append(bundle.Snapshots, finalizePythonSnapshot(*current))
			current = nil
		}
	}
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if faulthandlerThreadLineRE.MatchString(line) {
			flush()
			tid := faulthandlerHexRE.FindString(line)
			if tid == "" {
				tid = "thread-" + strconv.Itoa(threadIndex)
			}
			tidCopy := tid
			lang := "python"
			source := p.FormatID()
			snap := models.NewThreadSnapshot(
				"python::"+strconv.Itoa(threadIndex)+"::"+tid,
				tid,
				models.ThreadStateUnknown,
			)
			snap.ThreadID = &tidCopy
			snap.Language = &lang
			snap.SourceFormat = &source
			snap.Metadata["trace_order"] = "most_recent_first"
			// category remains nil — Python explicitly sets category=None.
			current = &snap
			threadIndex++
			continue
		}
		if current == nil {
			continue
		}
		if m := faulthandlerFrameRE.FindStringSubmatch(line); m != nil {
			current.StackFrames = append(current.StackFrames, makePythonFrame(m[3], m[1], m[2]))
		}
	}
	flush()
	return bundle, nil
}

// ---------------------------------------------------------------------------
// plain traceback (``Thread ID: <n>``)
// ---------------------------------------------------------------------------

// tracebackHeaderRE matches ``Thread ID: <n>`` headers. Python uses
// MULTILINE for can_parse — we mirror with `(?m)`.
var tracebackHeaderRE = regexp.MustCompile(`(?m)^Thread\s+ID:\s*(\d+)\s*$`)

// tracebackHeaderLineRE matches the same header line-by-line.
var tracebackHeaderLineRE = regexp.MustCompile(`^Thread\s+ID:\s*(\d+)\s*$`)

// tracebackFrameRE matches ``  File "...", line N, in func`` (note
// the trailing comma + the function being any non-greedy text — Python
// allows ``in <module>`` etc.).
var tracebackFrameRE = regexp.MustCompile(`^\s+File\s+"([^"]+)",\s+line\s+(\d+),\s+in\s+(.+?)\s*$`)

// TracebackPlugin parses ``traceback.format_stack()``-style dumps
// prefixed with ``Thread ID: <n>`` headers. Used by simple test
// fixtures that don't depend on py-spy or faulthandler.
type TracebackPlugin struct{}

// FormatID returns the stable identifier.
func (TracebackPlugin) FormatID() string { return "python_traceback" }

// Language returns the runtime label.
func (TracebackPlugin) Language() string { return "python" }

// CanParse sniffs the head for ``Thread ID: <n>``.
func (TracebackPlugin) CanParse(head string) bool { return tracebackHeaderRE.MatchString(head) }

// Parse reads `path` and returns one bundle.
func (p TracebackPlugin) Parse(path string) (models.ThreadDumpBundle, error) {
	bundle := models.NewThreadDumpBundle(path, p.FormatID(), p.Language())
	lines, err := readPythonDumpLines(path)
	if err != nil {
		return bundle, err
	}

	var current *models.ThreadSnapshot
	threadIndex := 0
	flush := func() {
		if current != nil {
			bundle.Snapshots = append(bundle.Snapshots, finalizePythonSnapshot(*current))
			current = nil
		}
	}
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if m := tracebackHeaderLineRE.FindStringSubmatch(line); m != nil {
			flush()
			tid := m[1]
			tidCopy := tid
			lang := "python"
			source := p.FormatID()
			name := "thread-" + tid
			snap := models.NewThreadSnapshot(
				"python::"+strconv.Itoa(threadIndex)+"::"+tid,
				name,
				models.ThreadStateUnknown,
			)
			snap.ThreadID = &tidCopy
			snap.Language = &lang
			snap.SourceFormat = &source
			snap.Metadata["trace_order"] = "oldest_first"
			current = &snap
			threadIndex++
			continue
		}
		if current == nil {
			continue
		}
		if m := tracebackFrameRE.FindStringSubmatch(line); m != nil {
			current.StackFrames = append(current.StackFrames, makePythonFrame(m[3], m[1], m[2]))
		}
	}
	flush()
	return bundle, nil
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// readPythonDumpLines reads the file with the project's encoding-fallback
// chain (utf-8 first, latin-1 last). Falls back to a raw os.ReadFile on
// the rare path where textio's strict utf-8 mode would refuse — Python
// uses ``errors="replace"`` and never raises here.
func readPythonDumpLines(path string) ([]string, error) {
	if lines, err := textio.IterTextLines(path, ""); err == nil {
		return lines, nil
	}
	// Last-ditch fallback so a malformed encoding never blocks parsing.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(raw), "\n"), nil
}

// makePythonFrame builds a StackFrame with language="python" and a
// best-effort line number. Mirrors the Python plugin's `int()` on
// the line group, falling back to nil on parse error.
func makePythonFrame(function, file, lineStr string) models.StackFrame {
	fileCopy := file
	lang := "python"
	frame := models.StackFrame{
		Function: function,
		File:     &fileCopy,
		Language: &lang,
	}
	if n, err := strconv.Atoi(lineStr); err == nil {
		frame.Line = &n
	}
	return frame
}

// pythonBoilerplateFunctions lists generic ASGI/WSGI / framework wrapper
// names. Matches Python `_PYTHON_BOILERPLATE_FUNCTIONS`.
var pythonBoilerplateFunctions = map[string]struct{}{
	"MiddlewareMixin.__call__": {},
	"__call__":                 {}, // generic wrappers, scoped further by file path
	"solve_dependencies":       {},
	"run_endpoint_function":    {},
	"view_func":                {},
	"wraps":                    {},
	"wrapper":                  {},
	"_wrapper":                 {},
	"decorator":                {},
	"dispatch_request":         {},
	"dispatch":                 {},
}

// pythonBoilerplateFiles matches paths inside known framework packages.
// Mirrors the Python `_PYTHON_BOILERPLATE_FILES` regex (the trailing
// `\b` anchors each name on a word boundary so `asgi` matches inside
// ``my_asgi_pkg`` but not inside ``asgir``).
var pythonBoilerplateFiles = regexp.MustCompile(`(starlette|fastapi|django|flask|gunicorn|uvicorn|werkzeug|asgi)\b`)

// stripPythonBoilerplate drops framework wrappers that obscure the
// user's view function. If every frame is boilerplate we keep the
// originals so we never empty the stack — Python parity (`cleaned or list(frames)`).
func stripPythonBoilerplate(frames []models.StackFrame) []models.StackFrame {
	cleaned := make([]models.StackFrame, 0, len(frames))
	for _, frame := range frames {
		if frame.Language == nil || *frame.Language != "python" {
			cleaned = append(cleaned, frame)
			continue
		}
		_, isWrapper := pythonBoilerplateFunctions[frame.Function]
		inFrameworkFile := false
		if frame.File != nil && *frame.File != "" {
			inFrameworkFile = pythonBoilerplateFiles.MatchString(strings.ToLower(*frame.File))
		}
		if isWrapper && inFrameworkFile {
			continue
		}
		cleaned = append(cleaned, frame)
	}
	if len(cleaned) == 0 {
		// Same-slice copy so the caller can mutate without touching input.
		out := make([]models.StackFrame, len(frames))
		copy(out, frames)
		return out
	}
	return cleaned
}

// pyNetworkFunctions / pyIOFunctions / pyLockFunctions mirror the
// equivalent Python frozensets.
var (
	pyNetworkFunctions = map[string]struct{}{
		"recv":     {},
		"recvfrom": {},
		"recvmsg":  {},
		"send":     {},
		"sendto":   {},
		"sendmsg":  {},
		"accept":   {},
		"connect":  {},
	}
	pyNetworkFileHints = []string{"urllib3", "requests/", "httpx/", "http/client.py"}
	pyIOFunctions      = map[string]struct{}{
		"select": {},
		"poll":   {},
		"epoll":  {},
		"kqueue": {},
	}
	pyLockFunctions = map[string]struct{}{
		"acquire": {},
		"wait":    {},
	}
	pyAsyncioFunctions = map[string]struct{}{
		"sleep":       {},
		"run_forever": {},
		"_run_once":   {},
	}
)

// inferPythonState promotes a ThreadState based on the (file, function)
// pair of the deepest (top-of-stack) frame. Mirrors the Python helper:
// network / lock / io heuristics applied in priority order.
func inferPythonState(state models.ThreadState, frames []models.StackFrame) models.ThreadState {
	if len(frames) == 0 {
		return state
	}
	top := frames[0]
	fn := top.Function
	fileLower := ""
	if top.File != nil {
		fileLower = strings.ToLower(*top.File)
	}

	if _, ok := pyNetworkFunctions[fn]; ok && strings.Contains(fileLower, "socket") {
		return models.ThreadStateNetworkWait
	}
	for _, hint := range pyNetworkFileHints {
		if strings.Contains(fileLower, hint) {
			return models.ThreadStateNetworkWait
		}
	}

	if _, ok := pyLockFunctions[fn]; ok && strings.Contains(fileLower, "threading.py") {
		return models.ThreadStateLockWait
	}
	if fn == "get" && strings.Contains(fileLower, "queue.py") {
		return models.ThreadStateLockWait
	}

	if _, ok := pyIOFunctions[fn]; ok && strings.Contains(fileLower, "select") {
		return models.ThreadStateIOWait
	}
	if strings.Contains(fileLower, "asyncio") {
		if _, ok := pyAsyncioFunctions[fn]; ok {
			return models.ThreadStateIOWait
		}
	}
	if strings.Contains(fileLower, "gevent") {
		return models.ThreadStateIOWait
	}
	if fn == "read" && (strings.Contains(fileLower, "os.py") ||
		strings.Contains(fileLower, "file") ||
		strings.Contains(fileLower, "io.py")) {
		return models.ThreadStateIOWait
	}

	return state
}

// finalizePythonSnapshot strips framework boilerplate, infers a more
// specific state from the top frame, and updates the category to match
// the new state. Mirrors `_finalize_python_snapshot` in Python.
func finalizePythonSnapshot(snap models.ThreadSnapshot) models.ThreadSnapshot {
	cleaned := stripPythonBoilerplate(snap.StackFrames)
	newState := inferPythonState(snap.State, cleaned)
	snap.StackFrames = cleaned
	snap.State = newState
	cat := string(newState)
	snap.Category = &cat
	return snap
}
