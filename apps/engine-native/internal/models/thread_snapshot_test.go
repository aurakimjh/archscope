package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCoerceThreadStateAliases(t *testing.T) {
	cases := []struct {
		input string
		want  ThreadState
	}{
		{"RUNNING", ThreadStateRunnable},
		{"Sleeping", ThreadStateTimedWaiting},
		{"chan receive", ThreadStateChannelWait},
		{"chan-send", ThreadStateChannelWait},
		{"select", ThreadStateChannelWait},
		{"PARKED", ThreadStateWaiting},
		{"TERMINATED", ThreadStateDead},
		{"INITIALIZING", ThreadStateNew},
		{"IO", ThreadStateIOWait},
		{"", ThreadStateUnknown},
		{"nonsense", ThreadStateUnknown},
	}
	for _, tc := range cases {
		got := CoerceThreadState(tc.input)
		if got != tc.want {
			t.Errorf("CoerceThreadState(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCoerceThreadStateCanonicalIdentity(t *testing.T) {
	for _, canonical := range []ThreadState{
		ThreadStateRunnable,
		ThreadStateBlocked,
		ThreadStateWaiting,
		ThreadStateTimedWaiting,
		ThreadStateNetworkWait,
		ThreadStateIOWait,
		ThreadStateLockWait,
		ThreadStateChannelWait,
		ThreadStateDead,
		ThreadStateNew,
		ThreadStateUnknown,
	} {
		got := CoerceThreadState(string(canonical))
		if got != canonical {
			t.Errorf("CoerceThreadState(%q) = %q, want identity", canonical, got)
		}
	}
}

func strPtr(s string) *string { return &s }
func intPtr(n int) *int       { return &n }

func TestStackFrameRender(t *testing.T) {
	cases := []struct {
		name  string
		frame StackFrame
		want  string
	}{
		{
			name:  "function only",
			frame: StackFrame{Function: "run"},
			want:  "run",
		},
		{
			name:  "module + function",
			frame: StackFrame{Function: "run", Module: strPtr("com.example.Worker")},
			want:  "com.example.Worker.run",
		},
		{
			name:  "function + file",
			frame: StackFrame{Function: "run", File: strPtr("Worker.java")},
			want:  "run (Worker.java)",
		},
		{
			name:  "module + function + file + line",
			frame: StackFrame{Function: "run", Module: strPtr("com.example.Worker"), File: strPtr("Worker.java"), Line: intPtr(42)},
			want:  "com.example.Worker.run (Worker.java:42)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.frame.Render(); got != tc.want {
				t.Errorf("Render() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestThreadSnapshotStackSignaturePythonParity(t *testing.T) {
	snapshot := NewThreadSnapshot("snap-1", "worker-1", ThreadStateRunnable)
	snapshot.StackFrames = []StackFrame{
		{Function: "run", Module: strPtr("com.example.Worker"), Language: strPtr("java")},
		{Function: "exec", Module: strPtr("com.example.Pool"), Language: strPtr("java")},
	}
	want := "com.example.Worker.run | com.example.Pool.exec"
	if got := snapshot.StackSignature(0); got != want {
		t.Fatalf("StackSignature(default) = %q, want %q", got, want)
	}
}

func TestThreadSnapshotStackSignatureDepthLimit(t *testing.T) {
	snapshot := NewThreadSnapshot("snap", "t", ThreadStateRunnable)
	snapshot.StackFrames = []StackFrame{
		{Function: "a"}, {Function: "b"}, {Function: "c"}, {Function: "d"},
	}
	if got := snapshot.StackSignature(2); got != "a | b" {
		t.Fatalf("depth=2 signature = %q, want %q", got, "a | b")
	}
	if got := snapshot.StackSignature(0); got != "a | b | c | d" {
		t.Fatalf("default-depth signature = %q, want %q", got, "a | b | c | d")
	}
}

func TestThreadSnapshotEmptyStackSignature(t *testing.T) {
	snapshot := NewThreadSnapshot("snap", "t", ThreadStateRunnable)
	if got := snapshot.StackSignature(0); got != "(no-stack)" {
		t.Fatalf("empty stack signature = %q, want %q", got, "(no-stack)")
	}
}

func TestThreadSnapshotJSONShape(t *testing.T) {
	snapshot := NewThreadSnapshot("snap-1", "worker-1", ThreadStateRunnable)
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`"snapshot_id":"snap-1"`,
		`"thread_name":"worker-1"`,
		`"state":"RUNNABLE"`,
		`"stack_frames":[]`,
		`"metadata":{}`,
		`"lock_holds":[]`,
		`"thread_id":null`,
		`"category":null`,
		`"lock_info":null`,
		`"language":null`,
		`"source_format":null`,
		`"lock_waiting":null`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("missing %q in %s", want, body)
		}
	}
}

func TestThreadDumpBundleJSONShape(t *testing.T) {
	bundle := NewThreadDumpBundle("/tmp/jstack.txt", "java_jstack", "java")
	bundle.DumpIndex = 0
	body, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`"snapshots":[]`,
		`"source_file":"/tmp/jstack.txt"`,
		`"source_format":"java_jstack"`,
		`"language":"java"`,
		`"dump_index":0`,
		`"dump_label":null`,
		`"captured_at":null`,
		`"metadata":{}`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("missing %q in %s", want, body)
		}
	}
}

func TestLockHandleJSONOmitsWaitMode(t *testing.T) {
	cls := "java.util.concurrent.locks.ReentrantLock"
	handle := LockHandle{LockID: "<0x76ab62208>", LockClass: &cls}
	body, err := json.Marshal(handle)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(body), "wait_mode") {
		t.Fatalf("wait_mode must be omitted when empty; got %s", body)
	}
	handle.WaitMode = "to_lock"
	body, err = json.Marshal(handle)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), `"wait_mode":"to_lock"`) {
		t.Fatalf("wait_mode missing when set: %s", body)
	}
}
