package dotnetclrstack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// dotnetDump mirrors the `_DOTNET_DUMP` fixture in
// engines/python/tests/test_dotnet_clrstack_parser.py byte-for-byte.
const dotnetDump = `OS Thread Id: 0x1a4 (1)
        Child SP               IP Call Site
000000A1B2C3D4E0 00007FF812345678 System.Threading.Monitor.Enter(System.Object)
000000A1B2C3D4F8 00007FF812345abc MyApp.Service.Process(MyApp.Request)
000000A1B2C3D510 00007FF812345def MyApp.Program.Main(System.String[])

OS Thread Id: 0x1a5 (2)
        Child SP               IP Call Site
000000A1B2C3D600 00007FF8DEAD1111 MyApp.Async.<DoWorkAsync>d__0.MoveNext()
000000A1B2C3D618 00007FF8DEAD2222 System.Net.Sockets.Socket.Receive(System.Byte[])

Sync Block Owner Info:
Index 0x123: SyncBlock 0x456 owned by Thread 0x1a4
`

// writeDump is a tiny helper that drops the fixture into a temp file
// and returns the absolute path.
func writeDump(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "dotnet.txt")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write dump: %v", err)
	}
	return path
}

// TestPluginInterfaceSurface confirms the FormatID / Language /
// CanParse / Parse contract surface lines up with the registry. We
// type-assert against the threaddump.Plugin interface in plugin_iface_test.go.
func TestFormatIDAndLanguage(t *testing.T) {
	p := New()
	if got := p.FormatID(); got != FormatID {
		t.Fatalf("FormatID() = %q, want %q", got, FormatID)
	}
	if got := p.Language(); got != Language {
		t.Fatalf("Language() = %q, want %q", got, Language)
	}
}

// TestCanParseRecognizesOSThreadHeader mirrors Python
// `test_can_parse_recognizes_os_thread_header`.
func TestCanParseRecognizesOSThreadHeader(t *testing.T) {
	p := New()
	if !p.CanParse("OS Thread Id: 0x123\n        Child SP\n") {
		t.Fatal("expected can_parse to claim OS Thread Id header")
	}
}

// TestCanParseRejectsJstackText mirrors Python
// `test_can_parse_rejects_jstack_text` — jstack-flavoured text must
// not steal the file from the JVM plugin.
func TestCanParseRejectsJstackText(t *testing.T) {
	p := New()
	if p.CanParse(`"worker" #1 nid=0x00ab` + "\n") {
		t.Fatal("expected can_parse to reject jstack-style header")
	}
}

// TestCanParseRequiresLineStart guards against a future regression
// where the regex would match `OS Thread Id: 0x` mid-line. The
// Python parser uses re.MULTILINE + ^, which the Go port mirrors via
// `(?m)^…`.
func TestCanParseRequiresLineStart(t *testing.T) {
	p := New()
	if p.CanParse("PrefixOS Thread Id: 0x1") {
		t.Fatal("expected can_parse to require start-of-line anchor")
	}
}

// TestParseExtractsThreadsWithManagedID mirrors Python
// `test_parse_extracts_threads_with_managed_id`.
func TestParseExtractsThreadsWithManagedID(t *testing.T) {
	path := writeDump(t, dotnetDump)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bundle.Language != Language {
		t.Fatalf("Language = %q, want %q", bundle.Language, Language)
	}
	if bundle.SourceFormat != FormatID {
		t.Fatalf("SourceFormat = %q, want %q", bundle.SourceFormat, FormatID)
	}
	if len(bundle.Snapshots) != 2 {
		t.Fatalf("len(Snapshots) = %d, want 2", len(bundle.Snapshots))
	}
	first := bundle.Snapshots[0]
	if first.ThreadID == nil || *first.ThreadID != "0x1a4" {
		var got string
		if first.ThreadID != nil {
			got = *first.ThreadID
		}
		t.Fatalf("first.ThreadID = %q, want 0x1a4", got)
	}
	if first.ThreadName != "1" {
		t.Fatalf("first.ThreadName = %q, want %q", first.ThreadName, "1")
	}
	// Top frame is Monitor.Enter → LOCK_WAIT.
	if first.State != models.ThreadStateLockWait {
		t.Fatalf("first.State = %q, want LOCK_WAIT", first.State)
	}
}

// TestParseFallsBackToTidWhenManagedIDMissing covers the branch
// where the optional `(N)` after the hex tid is absent — the thread
// name must fall back to the tid.
func TestParseFallsBackToTidWhenManagedIDMissing(t *testing.T) {
	dump := "OS Thread Id: 0x99\n" +
		"        Child SP               IP Call Site\n" +
		"0000 0001 MyApp.Foo.Bar(System.Object)\n"
	path := writeDump(t, dump)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(bundle.Snapshots) != 1 {
		t.Fatalf("snapshots = %d, want 1", len(bundle.Snapshots))
	}
	snap := bundle.Snapshots[0]
	if snap.ThreadName != "0x99" {
		t.Fatalf("ThreadName = %q, want 0x99 fallback", snap.ThreadName)
	}
	if v, ok := snap.Metadata["managed_id"]; !ok || v != nil {
		t.Fatalf("managed_id metadata = %v, want explicit nil", v)
	}
}

// TestParsePromotesSocketReceiveToNetworkWait mirrors Python
// `test_parse_promotes_socket_receive_to_network_wait` — the test
// only asserts that the Socket.Receive frame survives parsing, not
// that the state changed (because state inference uses the top
// frame which is MoveNext).
func TestParsePromotesSocketReceiveToNetworkWait(t *testing.T) {
	path := writeDump(t, dotnetDump)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	second := bundle.Snapshots[1]
	count := 0
	for _, f := range second.StackFrames {
		if f.Function == "Receive" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("Receive frames in second snapshot = %d, want 1", count)
	}
}

// TestAsyncStateMachineNormalized mirrors Python
// `test_async_state_machine_normalized` — `<DoWorkAsync>d__0` should
// flatten into the module identifier.
func TestAsyncStateMachineNormalized(t *testing.T) {
	path := writeDump(t, dotnetDump)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	second := bundle.Snapshots[1]
	if len(second.StackFrames) == 0 {
		t.Fatal("expected at least one frame")
	}
	top := second.StackFrames[0]
	module := ""
	if top.Module != nil {
		module = *top.Module
	}
	if !strings.Contains(module, "DoWorkAsync") {
		t.Fatalf("module = %q, want substring DoWorkAsync", module)
	}
	if strings.Contains(module, "<") {
		t.Fatalf("module = %q, expected no `<` after normalization", module)
	}
	if top.Function != "MoveNext" {
		t.Fatalf("top function = %q, want MoveNext", top.Function)
	}
}

// TestSyncBlockOwnerRecordedInMetadata mirrors Python
// `test_sync_block_owner_recorded_in_metadata`. The list is captured
// verbatim under bundle metadata.
func TestSyncBlockOwnerRecordedInMetadata(t *testing.T) {
	path := writeDump(t, dotnetDump)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	syncRaw, ok := bundle.Metadata["sync_block_owner_info"]
	if !ok {
		t.Fatal("expected sync_block_owner_info metadata key")
	}
	syncList, ok := syncRaw.([]string)
	if !ok {
		t.Fatalf("sync_block_owner_info type = %T, want []string", syncRaw)
	}
	found := false
	for _, line := range syncList {
		if strings.Contains(line, "SyncBlock") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no SyncBlock evidence captured: %v", syncList)
	}
}

// TestSyncBlockMissingProducesNoMetadata covers the negative branch
// where no sync-block table is present.
func TestSyncBlockMissingProducesNoMetadata(t *testing.T) {
	dump := "OS Thread Id: 0x1\n" +
		"        Child SP               IP Call Site\n" +
		"0 0 MyApp.Foo.Bar(System.Object)\n"
	path := writeDump(t, dump)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, ok := bundle.Metadata["sync_block_owner_info"]; ok {
		t.Fatal("did not expect sync_block_owner_info metadata key")
	}
}

// TestNormalizeLocalFunctionSynthesizedName mirrors Python
// `test_normalize_local_function_synthesized_name`.
func TestNormalizeLocalFunctionSynthesizedName(t *testing.T) {
	module := "MyApp.<Outer>g__Inner|3_0"
	frame := models.StackFrame{
		Module:   &module,
		Function: "Run",
		Language: strPtr(Language),
	}
	cleaned := normalizeDotnetFrame(frame)
	if cleaned.Module == nil || *cleaned.Module != "MyApp.Outer.Inner" {
		var got string
		if cleaned.Module != nil {
			got = *cleaned.Module
		}
		t.Fatalf("module = %q, want MyApp.Outer.Inner", got)
	}
}

// TestNormalizeIgnoresNonDotnetFrames protects parity with Python's
// guard `if frame.language != "dotnet": return frame`.
func TestNormalizeIgnoresNonDotnetFrames(t *testing.T) {
	module := "MyApp.<Outer>g__Inner|3_0"
	java := "java"
	frame := models.StackFrame{
		Module:   &module,
		Function: "Run",
		Language: &java,
	}
	cleaned := normalizeDotnetFrame(frame)
	if cleaned.Module == nil || *cleaned.Module != module {
		t.Fatal("non-dotnet frame should pass through unchanged")
	}
}

// TestStateInferenceKeepsUnrelatedRunnable mirrors Python
// `test_state_inference_keeps_unrelated_runnable`.
func TestStateInferenceKeepsUnrelatedRunnable(t *testing.T) {
	module := "MyApp.Service"
	frames := []models.StackFrame{{
		Module:   &module,
		Function: "Compute",
		Language: strPtr(Language),
	}}
	if got := inferDotnetState(models.ThreadStateRunnable, frames); got != models.ThreadStateRunnable {
		t.Fatalf("state = %q, want RUNNABLE", got)
	}
}

// TestStateInferenceIOWaitOnFileRead exercises the third branch of
// the state-inference cascade — File.Read should land on IO_WAIT.
func TestStateInferenceIOWaitOnFileRead(t *testing.T) {
	module := "System.IO.File"
	frames := []models.StackFrame{{
		Module:   &module,
		Function: "Read",
		Language: strPtr(Language),
	}}
	if got := inferDotnetState(models.ThreadStateRunnable, frames); got != models.ThreadStateIOWait {
		t.Fatalf("state = %q, want IO_WAIT", got)
	}
}

// TestParseCallSiteWithoutDot covers the branch where the qualified
// name has no dot — module stays nil, function is the whole token.
func TestParseCallSiteWithoutDot(t *testing.T) {
	frame := parseCallSite("UnscopedFunc(System.Object)")
	if frame.Module != nil {
		t.Fatalf("module = %v, want nil", *frame.Module)
	}
	if frame.Function != "UnscopedFunc" {
		t.Fatalf("function = %q, want UnscopedFunc", frame.Function)
	}
}

// TestParseCallSiteSplitsQualifiedName covers the standard case.
func TestParseCallSiteSplitsQualifiedName(t *testing.T) {
	frame := parseCallSite("MyApp.Service.Process(MyApp.Request)")
	if frame.Module == nil || *frame.Module != "MyApp.Service" {
		t.Fatalf("module = %v, want MyApp.Service", frame.Module)
	}
	if frame.Function != "Process" {
		t.Fatalf("function = %q, want Process", frame.Function)
	}
}

// TestSnapshotIDFormat protects the deterministic
// `dotnet::<index>::<tid>` format.
func TestSnapshotIDFormat(t *testing.T) {
	path := writeDump(t, dotnetDump)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bundle.Snapshots[0].SnapshotID != "dotnet::0::0x1a4" {
		t.Fatalf("first snapshot id = %q", bundle.Snapshots[0].SnapshotID)
	}
	if bundle.Snapshots[1].SnapshotID != "dotnet::1::0x1a5" {
		t.Fatalf("second snapshot id = %q", bundle.Snapshots[1].SnapshotID)
	}
}

// ---------------------------------------------------------------------------
// Environment.StackTrace plugin tests.
// ---------------------------------------------------------------------------

// envStackTraceFixture covers both heuristics: the literal token plus
// two CLR-flavoured frames so removing the token still leaves the
// fallback heuristic happy.
const envStackTraceFixture = `   at System.Environment.get_StackTrace()
   at MyApp.Service.Process(MyApp.Request) in /src/MyApp/Service.cs:line 42
   at MyApp.Program.Main(System.String[]) in /src/MyApp/Program.cs:line 7
`

func TestEnvironmentStackTraceCanParseExplicitToken(t *testing.T) {
	p := NewEnvironmentStackTracePlugin()
	if !p.CanParse(envStackTraceFixture) {
		t.Fatal("expected explicit Environment.get_StackTrace token to claim file")
	}
}

func TestEnvironmentStackTraceCanParseHeuristicNeedsTwoFrames(t *testing.T) {
	p := NewEnvironmentStackTracePlugin()
	single := "   at MyApp.Service.Process(System.Object)\n"
	if p.CanParse(single) {
		t.Fatal("single frame should not claim the file")
	}
	double := single + "   at MyApp.Program.Main(System.String[])\n"
	if !p.CanParse(double) {
		t.Fatal("two CLR-flavoured frames should claim the file")
	}
}

func TestEnvironmentStackTraceParseAttachesFileLine(t *testing.T) {
	p := NewEnvironmentStackTracePlugin()
	path := writeDump(t, envStackTraceFixture)
	bundle, err := p.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bundle.SourceFormat != EnvironmentStackTraceFormatID {
		t.Fatalf("SourceFormat = %q, want %q", bundle.SourceFormat, EnvironmentStackTraceFormatID)
	}
	if len(bundle.Snapshots) != 1 {
		t.Fatalf("snapshots = %d, want 1", len(bundle.Snapshots))
	}
	snap := bundle.Snapshots[0]
	if snap.ThreadName != "main" || snap.ThreadID == nil || *snap.ThreadID != "0" {
		t.Fatalf("snapshot identity = (%q,%v)", snap.ThreadName, snap.ThreadID)
	}
	if v, ok := snap.Metadata["format_variant"]; !ok || v != "environment_stacktrace" {
		t.Fatalf("format_variant = %v", v)
	}
	// The middle frame carries file/line info.
	var serviceFrame *models.StackFrame
	for i := range snap.StackFrames {
		if strings.HasPrefix(snap.StackFrames[i].Function, "MyApp.Service.Process") {
			serviceFrame = &snap.StackFrames[i]
			break
		}
	}
	if serviceFrame == nil {
		t.Fatal("expected a MyApp.Service.Process frame")
	}
	if serviceFrame.File == nil || *serviceFrame.File != "/src/MyApp/Service.cs" {
		t.Fatalf("file = %v, want /src/MyApp/Service.cs", serviceFrame.File)
	}
	if serviceFrame.Line == nil || *serviceFrame.Line != 42 {
		t.Fatalf("line = %v, want 42", serviceFrame.Line)
	}
}

func TestEnvironmentStackTraceParseEmptyFileEmitsNoSnapshots(t *testing.T) {
	p := NewEnvironmentStackTracePlugin()
	path := writeDump(t, "no relevant content here\n")
	bundle, err := p.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(bundle.Snapshots) != 0 {
		t.Fatalf("snapshots = %d, want 0 for unmatched body", len(bundle.Snapshots))
	}
}

// TestParseHandlesCRLFLineEndings ensures Windows-captured dumps
// don't smuggle stray `\r` into thread names or frame functions.
func TestParseHandlesCRLFLineEndings(t *testing.T) {
	body := strings.ReplaceAll(dotnetDump, "\n", "\r\n")
	path := writeDump(t, body)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(bundle.Snapshots) != 2 {
		t.Fatalf("snapshots = %d, want 2", len(bundle.Snapshots))
	}
	if got := bundle.Snapshots[0].ThreadName; got != "1" {
		t.Fatalf("thread name = %q, want %q (no CR)", got, "1")
	}
	for _, f := range bundle.Snapshots[0].StackFrames {
		if strings.ContainsAny(f.Function, "\r") {
			t.Fatalf("function carries CR: %q", f.Function)
		}
	}
}
