package pythondump

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Fixtures lifted byte-for-byte from
// engines/python/tests/test_python_dump_parsers.py so the Go port is
// validated against the same inputs the Python suite uses.
const pyspyDump = `Process 12345: python /app/server.py
Python v3.11.7 (/usr/bin/python3)

Thread 12345 (idle): "MainThread"
    sleep (time.py:42)
    serve (server.py:88)

Thread 12346 (active): "worker-1"
    recv (socket.py:660)
    handle (server.py:120)
`

const faulthandlerDump = `Fatal Python error: SIGTERM

Thread 0x00007f1234567890 (most recent call first):
  File "/app/server.py", line 42 in handler
  File "/app/main.py", line 10 in main

Thread 0x00007f1234560000 (most recent call first):
  File "/usr/lib/python3.11/threading.py", line 320 in wait
  File "/app/worker.py", line 55 in run
`

const tracebackDump = `Thread ID: 1
  File "/app/server.py", line 21, in main
  File "/app/server.py", line 5, in <module>

Thread ID: 2
  File "/app/worker.py", line 88, in run
`

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func ptrStr(s string) *string { return &s }

// ---------------------------------------------------------------------------
// py-spy
// ---------------------------------------------------------------------------

func TestPySpyCanParseRecognizesBanner(t *testing.T) {
	plugin := PySpyPlugin{}
	if !plugin.CanParse(pyspyDump[:200]) {
		t.Fatalf("CanParse should match py-spy banner")
	}
}

func TestPySpyRejectsOtherFormats(t *testing.T) {
	plugin := PySpyPlugin{}
	if plugin.CanParse(`"worker" #1 nid=0x123` + "\n") {
		t.Fatalf("CanParse should reject jstack lines")
	}
	if plugin.CanParse(faulthandlerDump) {
		t.Fatalf("CanParse should reject faulthandler dumps")
	}
}

func TestPySpyExtractsThreadsAndFrames(t *testing.T) {
	plugin := PySpyPlugin{}
	path := writeTemp(t, "pyspy.txt", pyspyDump)
	bundle, err := plugin.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bundle.Language != "python" {
		t.Fatalf("Language = %q", bundle.Language)
	}
	if bundle.SourceFormat != "python_pyspy" {
		t.Fatalf("SourceFormat = %q", bundle.SourceFormat)
	}
	if len(bundle.Snapshots) != 2 {
		t.Fatalf("snapshots = %d", len(bundle.Snapshots))
	}
	names := []string{bundle.Snapshots[0].ThreadName, bundle.Snapshots[1].ThreadName}
	if names[0] != "MainThread" || names[1] != "worker-1" {
		t.Fatalf("thread names = %+v", names)
	}
	main := bundle.Snapshots[0]
	if main.ThreadID == nil || *main.ThreadID != "12345" {
		t.Fatalf("MainThread tid = %v", main.ThreadID)
	}
	if main.Metadata["raw_state"] != "idle" {
		t.Fatalf("raw_state = %v", main.Metadata["raw_state"])
	}
	if len(main.StackFrames) != 2 {
		t.Fatalf("MainThread frames = %d", len(main.StackFrames))
	}
	top := main.StackFrames[0]
	if top.Function != "sleep" {
		t.Fatalf("top.Function = %q", top.Function)
	}
	if top.File == nil || *top.File != "time.py" {
		t.Fatalf("top.File = %v", top.File)
	}
	if top.Line == nil || *top.Line != 42 {
		t.Fatalf("top.Line = %v", top.Line)
	}
	if top.Language == nil || *top.Language != "python" {
		t.Fatalf("top.Language = %v", top.Language)
	}
}

func TestPySpyPromotesRecvToNetworkWait(t *testing.T) {
	plugin := PySpyPlugin{}
	path := writeTemp(t, "pyspy.txt", pyspyDump)
	bundle, _ := plugin.Parse(path)
	var worker *models.ThreadSnapshot
	for i := range bundle.Snapshots {
		if bundle.Snapshots[i].ThreadName == "worker-1" {
			worker = &bundle.Snapshots[i]
		}
	}
	if worker == nil {
		t.Fatalf("worker-1 snapshot not found")
	}
	if worker.State != models.ThreadStateNetworkWait {
		t.Fatalf("worker state = %q", worker.State)
	}
	if worker.Category == nil || *worker.Category != string(models.ThreadStateNetworkWait) {
		t.Fatalf("worker category = %v", worker.Category)
	}
}

func TestPySpyAnonymousNameFallsBackToTID(t *testing.T) {
	body := "Process 1: python\nPython v3.11\n\nThread 7777 (idle):\n    sleep (time.py:1)\n"
	path := writeTemp(t, "pyspy.txt", body)
	bundle, err := PySpyPlugin{}.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(bundle.Snapshots) != 1 {
		t.Fatalf("snapshots = %d", len(bundle.Snapshots))
	}
	if bundle.Snapshots[0].ThreadName != "thread-7777" {
		t.Fatalf("name fallback = %q", bundle.Snapshots[0].ThreadName)
	}
}

func TestPySpyEmptyInputProducesEmptyBundle(t *testing.T) {
	path := writeTemp(t, "empty.txt", "")
	bundle, err := PySpyPlugin{}.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(bundle.Snapshots) != 0 {
		t.Fatalf("snapshots = %d", len(bundle.Snapshots))
	}
}

// ---------------------------------------------------------------------------
// faulthandler
// ---------------------------------------------------------------------------

func TestFaulthandlerCanParseRecognizesThreadHeader(t *testing.T) {
	if !(FaulthandlerPlugin{}).CanParse(faulthandlerDump) {
		t.Fatalf("CanParse should match faulthandler dumps")
	}
}

func TestFaulthandlerRejectsPySpyText(t *testing.T) {
	if (FaulthandlerPlugin{}).CanParse(pyspyDump) {
		t.Fatalf("CanParse should reject py-spy dumps")
	}
}

func TestFaulthandlerExtractsThreadsAndFrames(t *testing.T) {
	plugin := FaulthandlerPlugin{}
	path := writeTemp(t, "fh.txt", faulthandlerDump)
	bundle, err := plugin.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bundle.SourceFormat != "python_faulthandler" {
		t.Fatalf("SourceFormat = %q", bundle.SourceFormat)
	}
	if bundle.Language != "python" {
		t.Fatalf("Language = %q", bundle.Language)
	}
	if len(bundle.Snapshots) != 2 {
		t.Fatalf("snapshots = %d", len(bundle.Snapshots))
	}
	first := bundle.Snapshots[0]
	if first.ThreadID == nil || *first.ThreadID != "0x00007f1234567890" {
		t.Fatalf("first tid = %v", first.ThreadID)
	}
	if first.Metadata["trace_order"] != "most_recent_first" {
		t.Fatalf("trace_order = %v", first.Metadata["trace_order"])
	}
	if len(first.StackFrames) != 2 {
		t.Fatalf("first frames = %d", len(first.StackFrames))
	}
	top := first.StackFrames[0]
	if top.Function != "handler" {
		t.Fatalf("top.Function = %q", top.Function)
	}
	if top.File == nil || *top.File != "/app/server.py" {
		t.Fatalf("top.File = %v", top.File)
	}
	if top.Line == nil || *top.Line != 42 {
		t.Fatalf("top.Line = %v", top.Line)
	}
}

func TestFaulthandlerThreadingWaitPromotesToLockWait(t *testing.T) {
	plugin := FaulthandlerPlugin{}
	path := writeTemp(t, "fh.txt", faulthandlerDump)
	bundle, _ := plugin.Parse(path)
	second := bundle.Snapshots[1]
	if second.State != models.ThreadStateLockWait {
		t.Fatalf("second state = %q", second.State)
	}
	if second.Category == nil || *second.Category != string(models.ThreadStateLockWait) {
		t.Fatalf("second category = %v", second.Category)
	}
}

func TestFaulthandlerCategoryReflectsInferredState(t *testing.T) {
	// First snapshot's top frame is `handler` in /app/server.py — no
	// promotion applies, state stays UNKNOWN, category = "UNKNOWN".
	plugin := FaulthandlerPlugin{}
	path := writeTemp(t, "fh.txt", faulthandlerDump)
	bundle, _ := plugin.Parse(path)
	first := bundle.Snapshots[0]
	if first.State != models.ThreadStateUnknown {
		t.Fatalf("first state = %q (want UNKNOWN)", first.State)
	}
	if first.Category == nil || *first.Category != string(models.ThreadStateUnknown) {
		t.Fatalf("first category = %v", first.Category)
	}
}

// ---------------------------------------------------------------------------
// traceback
// ---------------------------------------------------------------------------

func TestTracebackCanParseRecognizesHeader(t *testing.T) {
	if !(TracebackPlugin{}).CanParse(tracebackDump) {
		t.Fatalf("CanParse should match traceback dumps")
	}
}

func TestTracebackRejectsOtherFormats(t *testing.T) {
	if (TracebackPlugin{}).CanParse(pyspyDump) {
		t.Fatalf("CanParse should reject py-spy dumps")
	}
	if (TracebackPlugin{}).CanParse(faulthandlerDump) {
		t.Fatalf("CanParse should reject faulthandler dumps")
	}
}

func TestTracebackExtractsThreadsAndFrames(t *testing.T) {
	plugin := TracebackPlugin{}
	path := writeTemp(t, "tb.txt", tracebackDump)
	bundle, err := plugin.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bundle.SourceFormat != "python_traceback" {
		t.Fatalf("SourceFormat = %q", bundle.SourceFormat)
	}
	if len(bundle.Snapshots) != 2 {
		t.Fatalf("snapshots = %d", len(bundle.Snapshots))
	}
	first := bundle.Snapshots[0]
	if first.ThreadName != "thread-1" {
		t.Fatalf("first.ThreadName = %q", first.ThreadName)
	}
	if first.ThreadID == nil || *first.ThreadID != "1" {
		t.Fatalf("first tid = %v", first.ThreadID)
	}
	if first.Metadata["trace_order"] != "oldest_first" {
		t.Fatalf("trace_order = %v", first.Metadata["trace_order"])
	}
	if len(first.StackFrames) != 2 {
		t.Fatalf("frames = %d", len(first.StackFrames))
	}
	if first.StackFrames[0].Function != "main" {
		t.Fatalf("first.frames[0].Function = %q", first.StackFrames[0].Function)
	}
	if first.StackFrames[1].Function != "<module>" {
		t.Fatalf("first.frames[1].Function = %q", first.StackFrames[1].Function)
	}
}

// ---------------------------------------------------------------------------
// shared helpers (boilerplate strip + state inference)
// ---------------------------------------------------------------------------

func TestStripBoilerplateRemovesStarletteWrapper(t *testing.T) {
	frames := []models.StackFrame{
		{
			Function: "__call__",
			File:     ptrStr("/usr/lib/python3.11/site-packages/starlette/applications.py"),
			Line:     intPtr(120),
			Language: ptrStr("python"),
		},
		{
			Function: "route_handler",
			File:     ptrStr("/app/server.py"),
			Line:     intPtr(42),
			Language: ptrStr("python"),
		},
	}
	cleaned := stripPythonBoilerplate(frames)
	if len(cleaned) != 1 || cleaned[0].Function != "route_handler" {
		got := make([]string, 0, len(cleaned))
		for _, f := range cleaned {
			got = append(got, f.Function)
		}
		t.Fatalf("stripped frames = %v", got)
	}
}

func TestStripBoilerplateKeepsAllWhenStripWouldEmpty(t *testing.T) {
	// All frames are framework wrappers — the helper keeps the
	// originals so we never erase the stack entirely.
	frames := []models.StackFrame{
		{
			Function: "__call__",
			File:     ptrStr("/x/starlette/y.py"),
			Language: ptrStr("python"),
		},
	}
	cleaned := stripPythonBoilerplate(frames)
	if len(cleaned) != 1 {
		t.Fatalf("cleaned = %d, want 1", len(cleaned))
	}
}

func TestStripBoilerplateLeavesNonPythonFrames(t *testing.T) {
	frames := []models.StackFrame{
		{Function: "__call__", File: ptrStr("/x/starlette/y.py"), Language: ptrStr("c")},
	}
	cleaned := stripPythonBoilerplate(frames)
	if len(cleaned) != 1 || cleaned[0].Function != "__call__" {
		t.Fatalf("cleaned = %+v", cleaned)
	}
}

func TestInferPythonStatePromotesSelectToIOWait(t *testing.T) {
	frames := []models.StackFrame{
		{Function: "select", File: ptrStr("select.py"), Line: intPtr(88), Language: ptrStr("python")},
	}
	got := inferPythonState(models.ThreadStateRunnable, frames)
	if got != models.ThreadStateIOWait {
		t.Fatalf("state = %q", got)
	}
}

func TestInferPythonStatePromotesQueueGetToLockWait(t *testing.T) {
	frames := []models.StackFrame{
		{Function: "get", File: ptrStr("/usr/lib/python3.11/queue.py"), Language: ptrStr("python")},
	}
	got := inferPythonState(models.ThreadStateRunnable, frames)
	if got != models.ThreadStateLockWait {
		t.Fatalf("state = %q", got)
	}
}

func TestInferPythonStateUrllib3HintsAreNetwork(t *testing.T) {
	frames := []models.StackFrame{
		{Function: "do_request", File: ptrStr("/site-packages/urllib3/connection.py"), Language: ptrStr("python")},
	}
	got := inferPythonState(models.ThreadStateRunnable, frames)
	if got != models.ThreadStateNetworkWait {
		t.Fatalf("state = %q", got)
	}
}

func TestInferPythonStateAsyncioSleepIsIO(t *testing.T) {
	frames := []models.StackFrame{
		{Function: "sleep", File: ptrStr("/usr/lib/python3.11/asyncio/tasks.py"), Language: ptrStr("python")},
	}
	got := inferPythonState(models.ThreadStateRunnable, frames)
	if got != models.ThreadStateIOWait {
		t.Fatalf("state = %q", got)
	}
}

func TestInferPythonStateNoFramesKeepsState(t *testing.T) {
	got := inferPythonState(models.ThreadStateBlocked, nil)
	if got != models.ThreadStateBlocked {
		t.Fatalf("state = %q", got)
	}
}

func intPtr(n int) *int { return &n }
