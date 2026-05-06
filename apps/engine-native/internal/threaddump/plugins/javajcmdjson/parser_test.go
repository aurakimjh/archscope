package javajcmdjson

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump"
)

// writeJSON serializes v to path and returns path. Helper for the
// tests below — all fixtures live in t.TempDir() so cleanup is free.
func writeJSON(t *testing.T, name string, v any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestPluginSatisfiesInterface(t *testing.T) {
	var _ threaddump.Plugin = New()
}

func TestFormatIDAndLanguage(t *testing.T) {
	p := New()
	if got := p.FormatID(); got != "java_jcmd_json" {
		t.Fatalf("FormatID = %q", got)
	}
	if got := p.Language(); got != "java" {
		t.Fatalf("Language = %q", got)
	}
}

func TestCanParseAcceptsJcmdJSONHead(t *testing.T) {
	head := `{"time":"2026-05-05T10:00:00Z","threadDump":{"threadContainers":[]}}`
	if !New().CanParse(head) {
		t.Fatalf("CanParse rejected valid jcmd head")
	}
}

func TestCanParseAcceptsLeadingWhitespace(t *testing.T) {
	head := "\n  \t" + `{"threadDump":{}}`
	if !New().CanParse(head) {
		t.Fatalf("CanParse should tolerate leading whitespace")
	}
}

func TestCanParseRejectsNonObject(t *testing.T) {
	if New().CanParse(`["threadDump"]`) {
		t.Fatalf("CanParse should reject non-object payload")
	}
}

func TestCanParseRejectsMissingMarker(t *testing.T) {
	if New().CanParse(`{"otherKey":"value"}`) {
		t.Fatalf("CanParse should require the threadDump marker")
	}
}

func TestParseHappyPathPromotesNetworkWait(t *testing.T) {
	// Mirrors Python test
	// `test_java_jcmd_json_parser_routes_and_promotes_network_wait`.
	payload := map[string]any{
		"time": "2026-05-05T10:00:00Z",
		"threadDump": map[string]any{
			"threadContainers": []any{
				map[string]any{
					"threads": []any{
						map[string]any{
							"name":  "http-client",
							"tid":   7,
							"state": "RUNNABLE",
							"stack": []any{
								"java.net.SocketInputStream.socketRead0(Native Method)",
								"com.acme.HttpClient.call(HttpClient.java:22)",
							},
						},
					},
				},
			},
		},
	}
	path := writeJSON(t, "jcmd.json", payload)

	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bundle.SourceFormat != "java_jcmd_json" {
		t.Fatalf("SourceFormat = %q", bundle.SourceFormat)
	}
	if bundle.Language != "java" {
		t.Fatalf("Language = %q", bundle.Language)
	}
	if bundle.CapturedAt == nil {
		t.Fatalf("CapturedAt should be parsed for ISO-8601 Z input")
	}
	if got, want := bundle.Metadata["raw_timestamp"], "2026-05-05T10:00:00Z"; got != want {
		t.Fatalf("raw_timestamp = %v, want %v", got, want)
	}
	if got, want := bundle.Metadata["thread_count"], len(bundle.Snapshots); got != want {
		t.Fatalf("thread_count = %v, want %v", got, want)
	}
	if len(bundle.Snapshots) != 1 {
		t.Fatalf("snapshots = %d, want 1", len(bundle.Snapshots))
	}
	snap := bundle.Snapshots[0]
	if snap.ThreadName != "http-client" {
		t.Fatalf("ThreadName = %q", snap.ThreadName)
	}
	if snap.State != models.ThreadStateNetworkWait {
		t.Fatalf("State = %q, want NETWORK_WAIT", snap.State)
	}
	if snap.ThreadID == nil || *snap.ThreadID != "7" {
		t.Fatalf("ThreadID = %v", snap.ThreadID)
	}
	if snap.SourceFormat == nil || *snap.SourceFormat != "java_jcmd_json" {
		t.Fatalf("SourceFormat = %v", snap.SourceFormat)
	}
	if snap.Language == nil || *snap.Language != "java" {
		t.Fatalf("Language = %v", snap.Language)
	}
	// First frame is the Native Method, so native_method should
	// have been recorded in the metadata.
	if _, ok := snap.Metadata["native_method"]; !ok {
		t.Fatalf("expected native_method metadata, got %#v", snap.Metadata)
	}
	// Snapshot id pattern: java_json::0::<index>::<name>.
	if !strings.HasPrefix(snap.SnapshotID, "java_json::0::0::http-client") {
		t.Fatalf("SnapshotID = %q", snap.SnapshotID)
	}
}

func TestParseRejectsMissingThreadDump(t *testing.T) {
	// Mirrors Python
	// `test_java_jcmd_json_parser_reports_missing_thread_dump`.
	path := writeJSON(t, "not-jcmd.json", map[string]any{"threads": []any{}})
	_, err := New().Parse(path)
	if err == nil {
		t.Fatalf("Parse should reject payload without threadDump")
	}
	if !strings.Contains(err.Error(), "missing required 'threadDump' object") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseRejectsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := New().Parse(path)
	if err == nil {
		t.Fatalf("Parse should fail on malformed JSON")
	}
	if !strings.Contains(err.Error(), "Malformed java_jcmd_json thread dump") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseRejectsEmptyThreadList(t *testing.T) {
	payload := map[string]any{
		"threadDump": map[string]any{
			"threadContainers": []any{},
		},
	}
	path := writeJSON(t, "empty.json", payload)
	_, err := New().Parse(path)
	if err == nil || !strings.Contains(err.Error(), "no thread objects were found") {
		t.Fatalf("expected empty-thread error, got %v", err)
	}
}

func TestParseRejectsTopLevelArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "array.json")
	if err := os.WriteFile(path, []byte(`["threadDump"]`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := New().Parse(path)
	if err == nil || !strings.Contains(err.Error(), "missing required 'threadDump' object") {
		t.Fatalf("expected missing-threadDump error, got %v", err)
	}
}

func TestParseLockedMonitorPromotesLockWait(t *testing.T) {
	// `lockInfo` with `lock_entry_wait` + an otherwise-RUNNABLE thread
	// must end up in LOCK_WAIT state.
	payload := map[string]any{
		"threadDump": map[string]any{
			"threads": []any{
				map[string]any{
					"name":  "blocked-worker",
					"state": "RUNNABLE",
					"stack": []any{
						"com.acme.Worker.run(Worker.java:10)",
					},
					"lockInfo": map[string]any{
						"className":        "java.util.concurrent.locks.ReentrantLock",
						"identityHashCode": 31806272840, // decimal -> hex coercion
					},
				},
			},
		},
	}
	path := writeJSON(t, "lockwait.json", payload)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(bundle.Snapshots) != 1 {
		t.Fatalf("snapshots = %d", len(bundle.Snapshots))
	}
	snap := bundle.Snapshots[0]
	if snap.State != models.ThreadStateLockWait {
		t.Fatalf("State = %q, want LOCK_WAIT", snap.State)
	}
	if snap.LockWaiting == nil {
		t.Fatalf("LockWaiting should be populated")
	}
	if !strings.HasPrefix(snap.LockWaiting.LockID, "0x") {
		t.Fatalf("decimal identityHashCode should be coerced to hex; got %q", snap.LockWaiting.LockID)
	}
	if got, ok := snap.Metadata["monitor_wait_mode"].(string); !ok || got != "lock_entry_wait" {
		t.Fatalf("monitor_wait_mode metadata = %v", snap.Metadata["monitor_wait_mode"])
	}
}

func TestParseLockHoldsFromLockedSynchronizers(t *testing.T) {
	payload := map[string]any{
		"threadDump": map[string]any{
			"threads": []any{
				map[string]any{
					"name":  "owner",
					"state": "RUNNABLE",
					"stack": []any{
						"com.acme.Worker.run(Worker.java:10)",
					},
					"lockedSynchronizers": []any{
						map[string]any{
							"className": "java.util.concurrent.locks.ReentrantLock$NonfairSync",
							"id":        "0xdeadbeef",
						},
					},
				},
			},
		},
	}
	path := writeJSON(t, "owner.json", payload)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	snap := bundle.Snapshots[0]
	if len(snap.LockHolds) != 1 || snap.LockHolds[0].LockID != "0xdeadbeef" {
		t.Fatalf("LockHolds = %+v", snap.LockHolds)
	}
	if snap.LockHolds[0].WaitMode != "locked_owner" {
		t.Fatalf("WaitMode = %q", snap.LockHolds[0].WaitMode)
	}
}

func TestParseFramesFromStructuredObjects(t *testing.T) {
	payload := map[string]any{
		"threadDump": map[string]any{
			"threads": []any{
				map[string]any{
					"name":  "structured",
					"state": "RUNNABLE",
					"stack": []any{
						map[string]any{
							"className":  "com.acme.Foo",
							"methodName": "bar",
							"fileName":   "Foo.java",
							"lineNumber": 42,
						},
					},
				},
			},
		},
	}
	path := writeJSON(t, "structured.json", payload)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	snap := bundle.Snapshots[0]
	if len(snap.StackFrames) != 1 {
		t.Fatalf("frames = %d", len(snap.StackFrames))
	}
	frame := snap.StackFrames[0]
	if frame.Function != "bar" {
		t.Fatalf("Function = %q", frame.Function)
	}
	if frame.Module == nil || *frame.Module != "com.acme.Foo" {
		t.Fatalf("Module = %v", frame.Module)
	}
	if frame.File == nil || *frame.File != "Foo.java" {
		t.Fatalf("File = %v", frame.File)
	}
	if frame.Line == nil || *frame.Line != 42 {
		t.Fatalf("Line = %v", frame.Line)
	}
}

func TestParseProxyFrameNormalization(t *testing.T) {
	payload := map[string]any{
		"threadDump": map[string]any{
			"threads": []any{
				map[string]any{
					"name":  "cglib",
					"state": "RUNNABLE",
					"stack": []any{
						"com.acme.Foo$$EnhancerByCGLIB$$abc12345.method(Foo.java:10)",
					},
				},
			},
		},
	}
	path := writeJSON(t, "proxy.json", payload)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	frame := bundle.Snapshots[0].StackFrames[0]
	rendered := frame.Render()
	if strings.Contains(rendered, "EnhancerByCGLIB") {
		t.Fatalf("CGLIB suffix should be stripped: %s", rendered)
	}
	if !strings.Contains(rendered, "com.acme.Foo") {
		t.Fatalf("expected normalized class name to remain: %s", rendered)
	}
}

func TestParseVirtualThreadMetadata(t *testing.T) {
	payload := map[string]any{
		"threadDump": map[string]any{
			"threads": []any{
				map[string]any{
					"name":    "vthread",
					"state":   "RUNNABLE",
					"virtual": true,
					"stack": []any{
						"com.acme.Foo.bar(Foo.java:1)",
					},
				},
			},
		},
	}
	path := writeJSON(t, "virtual.json", payload)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	snap := bundle.Snapshots[0]
	if got, ok := snap.Metadata["is_virtual_thread"].(bool); !ok || !got {
		t.Fatalf("is_virtual_thread metadata = %v", snap.Metadata["is_virtual_thread"])
	}
}

func TestParseTimestampSupportsRFC3339(t *testing.T) {
	payload := map[string]any{
		"timestamp": "2026-05-05T10:00:00Z",
		"threadDump": map[string]any{
			"threads": []any{
				map[string]any{"name": "t", "state": "RUNNABLE", "stack": []any{"a.B.c(D.java:1)"}},
			},
		},
	}
	path := writeJSON(t, "ts.json", payload)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bundle.CapturedAt == nil {
		t.Fatalf("CapturedAt should be populated from `timestamp` key")
	}
}

func TestRegistryRoutesViaCanParse(t *testing.T) {
	// Confirms the plugin works through the registry's CanParse path
	// — a regression net for the `{"threadDump"` substring contract.
	r := threaddump.NewRegistry()
	r.Register(New())

	payload := `{"threadDump":{"threads":[{"name":"x","state":"RUNNABLE","stack":["a.B.c(D.java:1)"]}]}}`
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	bundle, err := r.ParseOne(path, threaddump.ParseOptions{})
	if err != nil {
		t.Fatalf("ParseOne: %v", err)
	}
	if bundle.SourceFormat != "java_jcmd_json" {
		t.Fatalf("SourceFormat = %q", bundle.SourceFormat)
	}
}

func TestRegistryRejectsMixedHead(t *testing.T) {
	// Sanity: when the head doesn't contain the marker, the registry
	// must surface UnknownFormatError rather than dispatching to us.
	r := threaddump.NewRegistry()
	r.Register(New())
	path := filepath.Join(t.TempDir(), "no-marker.json")
	if err := os.WriteFile(path, []byte(`{"threads": []}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := r.ParseOne(path, threaddump.ParseOptions{})
	var ufe *threaddump.UnknownFormatError
	if !errors.As(err, &ufe) {
		t.Fatalf("expected UnknownFormatError, got %T: %v", err, err)
	}
}
