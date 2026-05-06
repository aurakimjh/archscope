// Tests for the Go goroutine-dump parser plugin (T-323). Mirrors the
// Python suite at engines/python/tests/test_go_goroutine_parser.py.
package gogoroutine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const goDumpFixture = "goroutine 1 [running]:\n" +
	"main.main()\n" +
	"\t/app/main.go:42 +0x1a\n" +
	"created by main.start\n" +
	"\t/app/main.go:10 +0x33\n" +
	"\n" +
	"goroutine 5 [chan receive, 5 minutes]:\n" +
	"main.worker(0xc0000a8000)\n" +
	"\t/app/main.go:88 +0x55\n" +
	"\n" +
	"goroutine 17 [select]:\n" +
	"runtime.selectgo(0xc0000b0000, 0xc0000a0000)\n" +
	"\t/usr/local/go/src/runtime/select.go:328 +0x49b\n" +
	"main.background()\n" +
	"\t/app/main.go:120 +0xab\n" +
	"\n" +
	"goroutine 24 [semacquire, 1 minutes]:\n" +
	"sync.runtime_Semacquire(0xc0000c0000)\n" +
	"\t/usr/local/go/src/runtime/sema.go:62 +0x42\n" +
	"sync.(*Mutex).Lock(0xc0000c0000)\n" +
	"\t/usr/local/go/src/sync/mutex.go:81 +0x90\n"

func writeFixture(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func goLang() *string {
	s := Language
	return &s
}

func TestCanParseRecognizesGoroutineHeader(t *testing.T) {
	p := New()
	if !p.CanParse("goroutine 1 [running]:\n") {
		t.Errorf("CanParse should match `goroutine 1 [running]:`")
	}
	if !p.CanParse("goroutine 99 [chan receive, 5 minutes]:") {
		t.Errorf("CanParse should match goroutine 99 header")
	}
}

func TestCanParseRejectsNonGoroutineText(t *testing.T) {
	p := New()
	if p.CanParse("\"worker\" #1 nid=0x00ab\n") {
		t.Errorf("CanParse should reject jstack-style header")
	}
	if p.CanParse("just a regular log line\n") {
		t.Errorf("CanParse should reject random text")
	}
}

func TestParseExtractsThreadCountAndStates(t *testing.T) {
	path := writeFixture(t, "go.dump", goDumpFixture)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bundle.Language != Language {
		t.Errorf("bundle.Language = %q, want %q", bundle.Language, Language)
	}
	if bundle.SourceFormat != FormatID {
		t.Errorf("bundle.SourceFormat = %q, want %q", bundle.SourceFormat, FormatID)
	}
	want := []string{"goroutine-1", "goroutine-5", "goroutine-17", "goroutine-24"}
	if len(bundle.Snapshots) != len(want) {
		t.Fatalf("got %d snapshots, want %d", len(bundle.Snapshots), len(want))
	}
	for i, w := range want {
		if bundle.Snapshots[i].ThreadName != w {
			t.Errorf("snapshots[%d].ThreadName = %q, want %q", i, bundle.Snapshots[i].ThreadName, w)
		}
	}
	// goroutine 5 → CHANNEL_WAIT via ThreadState alias for "chan receive".
	if got := bundle.Snapshots[1].State; got != models.ThreadStateChannelWait {
		t.Errorf("snapshots[1].State = %q, want %q", got, models.ThreadStateChannelWait)
	}
	if dur, _ := bundle.Snapshots[1].Metadata["duration"].(string); dur != "5 minutes" {
		t.Errorf(`metadata["duration"] = %v, want "5 minutes"`, bundle.Snapshots[1].Metadata["duration"])
	}
	// raw_status preserved verbatim, including the duration suffix.
	if rs, _ := bundle.Snapshots[1].Metadata["raw_status"].(string); rs != "chan receive, 5 minutes" {
		t.Errorf(`metadata["raw_status"] = %v`, bundle.Snapshots[1].Metadata["raw_status"])
	}
}

func TestParseExtractsFramesWithFileAndLine(t *testing.T) {
	path := writeFixture(t, "go.dump", goDumpFixture)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(bundle.Snapshots) == 0 || len(bundle.Snapshots[0].StackFrames) == 0 {
		t.Fatalf("snapshots[0] is missing frames")
	}
	mainFrame := bundle.Snapshots[0].StackFrames[0]
	if mainFrame.Module == nil || *mainFrame.Module != "main" {
		t.Errorf("Module = %v, want main", mainFrame.Module)
	}
	if mainFrame.Function != "main" {
		t.Errorf("Function = %q, want main", mainFrame.Function)
	}
	if mainFrame.File == nil || *mainFrame.File != "/app/main.go" {
		t.Errorf("File = %v, want /app/main.go", mainFrame.File)
	}
	if mainFrame.Line == nil || *mainFrame.Line != 42 {
		t.Errorf("Line = %v, want 42", mainFrame.Line)
	}
	if mainFrame.Language == nil || *mainFrame.Language != Language {
		t.Errorf("Language = %v, want go", mainFrame.Language)
	}
}

func TestParseAppendsCreatedByFrame(t *testing.T) {
	// goroutine 1 has a `created by main.start` parent frame. After
	// the `main.main` entry, the parent is appended as the bottom of
	// the stack.
	path := writeFixture(t, "go.dump", goDumpFixture)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	frames := bundle.Snapshots[0].StackFrames
	if len(frames) != 2 {
		t.Fatalf("frames len = %d, want 2 (main.main + created-by main.start)", len(frames))
	}
	parent := frames[1]
	if parent.Module == nil || *parent.Module != "main" {
		t.Errorf("parent.Module = %v, want main", parent.Module)
	}
	if parent.Function != "start" {
		t.Errorf("parent.Function = %q, want start", parent.Function)
	}
	if parent.File == nil || *parent.File != "/app/main.go" {
		t.Errorf("parent.File = %v, want /app/main.go", parent.File)
	}
	if parent.Line == nil || *parent.Line != 10 {
		t.Errorf("parent.Line = %v, want 10", parent.Line)
	}
}

func TestStateInferencePromotesGoparkedToChannelWait(t *testing.T) {
	mod := "runtime"
	frames := []models.StackFrame{{Module: &mod, Function: "gopark", Language: goLang()}}
	if got := inferGoState(models.ThreadStateRunnable, frames); got != models.ThreadStateChannelWait {
		t.Errorf("got %q, want CHANNEL_WAIT", got)
	}
}

func TestStateInferencePromotesNetpollToNetworkWait(t *testing.T) {
	mod := "runtime"
	frames := []models.StackFrame{{Module: &mod, Function: "netpollblock", Language: goLang()}}
	if got := inferGoState(models.ThreadStateRunnable, frames); got != models.ThreadStateNetworkWait {
		t.Errorf("got %q, want NETWORK_WAIT", got)
	}
}

func TestStateInferencePromotesMutexLockToLockWait(t *testing.T) {
	mod := "sync.(*Mutex)"
	frames := []models.StackFrame{{Module: &mod, Function: "Lock", Language: goLang()}}
	if got := inferGoState(models.ThreadStateRunnable, frames); got != models.ThreadStateLockWait {
		t.Errorf("got %q, want LOCK_WAIT", got)
	}
}

func TestStateInferenceLeavesUnknownFramesUntouched(t *testing.T) {
	mod := "main"
	frames := []models.StackFrame{{Module: &mod, Function: "doWork", Language: goLang()}}
	if got := inferGoState(models.ThreadStateRunnable, frames); got != models.ThreadStateRunnable {
		t.Errorf("got %q, want RUNNABLE (unchanged)", got)
	}
}

func TestStateInferenceEmptyFramesPreservesState(t *testing.T) {
	if got := inferGoState(models.ThreadStateRunnable, nil); got != models.ThreadStateRunnable {
		t.Errorf("got %q, want RUNNABLE", got)
	}
}

func TestNormalizeCollapsesAnonymousClosureChain(t *testing.T) {
	mod := "github.com/myapp/handler"
	frame := models.StackFrame{
		Module:   &mod,
		Function: "ServeHTTP.func1.func2",
		Language: goLang(),
	}
	cleaned := normalizeGoFrame(frame)
	if cleaned.Function != "ServeHTTP" {
		t.Errorf("Function = %q, want ServeHTTP", cleaned.Function)
	}
	if cleaned.Module == nil || *cleaned.Module != "github.com/myapp/handler" {
		t.Errorf("Module = %v, want github.com/myapp/handler", cleaned.Module)
	}
}

func TestNormalizeCollapsesGinHandlerFunc(t *testing.T) {
	mod := "github.com/gin-gonic/gin"
	frame := models.StackFrame{
		Module:   &mod,
		Function: "HandlerFunc.func1",
		Language: goLang(),
	}
	cleaned := normalizeGoFrame(frame)
	if cleaned.Function != "HandlerFunc" {
		t.Errorf("Function = %q, want HandlerFunc", cleaned.Function)
	}
}

func TestNormalizeLeavesNonGoFramesUntouched(t *testing.T) {
	mod := "java.util.concurrent"
	java := "java"
	frame := models.StackFrame{
		Module:   &mod,
		Function: "ThreadPoolExecutor.runWorker",
		Language: &java,
	}
	cleaned := normalizeGoFrame(frame)
	if cleaned.Function != "ThreadPoolExecutor.runWorker" {
		t.Errorf("non-go frame should be untouched, got Function=%q", cleaned.Function)
	}
}

func TestFullParseRecordsSelectAndMutexStates(t *testing.T) {
	path := writeFixture(t, "go.dump", goDumpFixture)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	states := map[string]models.ThreadState{}
	for _, snap := range bundle.Snapshots {
		states[snap.ThreadName] = snap.State
	}
	if states["goroutine-17"] != models.ThreadStateChannelWait {
		t.Errorf("goroutine-17 = %q, want CHANNEL_WAIT (top frame runtime.selectgo)", states["goroutine-17"])
	}
	if states["goroutine-24"] != models.ThreadStateLockWait {
		t.Errorf("goroutine-24 = %q, want LOCK_WAIT (sync.(*Mutex).Lock)", states["goroutine-24"])
	}
}

func TestSnapshotIDEncodesIndexAndID(t *testing.T) {
	path := writeFixture(t, "go.dump", goDumpFixture)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []string{
		"go::0::goroutine-1",
		"go::1::goroutine-5",
		"go::2::goroutine-17",
		"go::3::goroutine-24",
	}
	for i, w := range want {
		if bundle.Snapshots[i].SnapshotID != w {
			t.Errorf("snapshots[%d].SnapshotID = %q, want %q", i, bundle.Snapshots[i].SnapshotID, w)
		}
	}
}

func TestSnapshotCarriesLanguageAndFormat(t *testing.T) {
	path := writeFixture(t, "go.dump", goDumpFixture)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for i, snap := range bundle.Snapshots {
		if snap.Language == nil || *snap.Language != Language {
			t.Errorf("snapshots[%d].Language = %v, want go", i, snap.Language)
		}
		if snap.SourceFormat == nil || *snap.SourceFormat != FormatID {
			t.Errorf("snapshots[%d].SourceFormat = %v, want go_goroutine", i, snap.SourceFormat)
		}
		if snap.Category == nil || *snap.Category != string(snap.State) {
			t.Errorf("snapshots[%d].Category = %v, want %q", i, snap.Category, snap.State)
		}
	}
}

func TestPluginIdentifiers(t *testing.T) {
	p := New()
	if p.FormatID() != "go_goroutine" {
		t.Errorf("FormatID = %q", p.FormatID())
	}
	if p.Language() != "go" {
		t.Errorf("Language = %q", p.Language())
	}
}
