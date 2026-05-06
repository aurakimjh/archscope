// Smoke tests for the engine CLI subcommands. We build the binary
// once via `go build` and run each subcommand on its example fixture,
// asserting the JSON payload parses and the `type` field matches the
// expected string. This is the in-Go counterpart to the workflow
// parity step (.github/workflows/profiler-native.yml) — the workflow
// catches Python ↔ Go drift, this catches "Go subcommand silently
// stopped emitting JSON" without needing to run the full parity job
// locally.
//
// T-360 renamed every subcommand to mirror the typer surface (see
// cmd/archscope-engine/main.go); the cases below use the new Cobra
// names verbatim.
package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixturesRoot returns the absolute path to <repo>/examples by walking
// up from the test file location. The CLI subcommands consume relative
// paths fine, but anchoring to an absolute path keeps the test stable
// when `go test` runs from arbitrary working directories.
func fixturesRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate test source path")
	}
	// file == .../apps/engine-native/cmd/archscope-engine/main_test.go
	// repo  == file/../../../../..
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	return filepath.Join(repo, "examples")
}

// buildBinary compiles the CLI into a tempdir and returns the path.
// Doing this once per test run (via testMain helper) would shave a
// second or two; the suite is small enough that per-test build is
// fine, and the simpler shape avoids leaking shared state.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "archscope-engine")
	cmd := exec.Command("go", "build", "-o", out, ".")
	if buf, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, string(buf))
	}
	return out
}

// runJSON runs the binary and decodes stdout as a generic map. Errors
// surface the raw output so flaky JSON gets diagnosed without a
// re-run.
func runJSON(t *testing.T, bin string, args ...string) map[string]any {
	t.Helper()
	cmd := exec.Command(bin, args...)
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		t.Fatalf("run %v: %v\nstderr: %s", args, err, stderr)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		preview := string(out)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		t.Fatalf("decode %v: %v\npayload: %s", args, err, preview)
	}
	return payload
}

func TestCLISubcommands(t *testing.T) {
	bin := buildBinary(t)
	root := fixturesRoot(t)
	join := func(parts ...string) string {
		all := append([]string{root}, parts...)
		return filepath.Join(all...)
	}

	cases := []struct {
		name     string
		args     []string
		wantType string
	}{
		{
			name:     "access-log",
			args:     []string{"access-log", "analyze", "--in", join("access-logs", "sample-nginx-access.log"), "--format", "nginx"},
			wantType: "access_log",
		},
		{
			name:     "gc-log",
			args:     []string{"gc-log", "analyze", "--in", join("gc-logs", "sample-hotspot-gc.log")},
			wantType: "gc_log",
		},
		{
			name:     "jfr",
			args:     []string{"jfr", "analyze-json", "--in", join("jfr", "sample-jfr-print.json")},
			wantType: "jfr_recording",
		},
		{
			name:     "exception",
			args:     []string{"exception", "analyze", "--in", join("exceptions", "sample-java-exception.txt")},
			wantType: "exception_stack",
		},
		{
			name:     "otel",
			args:     []string{"otel", "analyze", "--in", join("otel", "sample-otel-logs.jsonl")},
			wantType: "otel_logs",
		},
		{
			name:     "thread-dump",
			args:     []string{"thread-dump", "analyze", "--in", join("thread-dumps", "sample-java-thread-dump.txt")},
			wantType: "thread_dump",
		},
		{
			name:     "nodejs",
			args:     []string{"nodejs", "analyze", "--in", join("runtime", "sample-nodejs-stack.txt")},
			wantType: "nodejs_stack",
		},
		{
			name:     "python-traceback",
			args:     []string{"python-traceback", "analyze", "--in", join("runtime", "sample-python-traceback.txt")},
			wantType: "python_traceback",
		},
		{
			name:     "go-panic",
			args:     []string{"go-panic", "analyze", "--in", join("runtime", "sample-go-panic.txt")},
			wantType: "go_panic",
		},
		{
			name:     "dotnet",
			args:     []string{"dotnet", "analyze", "--in", join("runtime", "sample-dotnet-iis.txt")},
			wantType: "dotnet_exception_iis",
		},
		{
			name:     "thread-dump-analyze-multi",
			args:     []string{"thread-dump", "analyze-multi", "--in", join("thread-dumps", "sample-java-thread-dump.txt")},
			wantType: "thread_dump_multi",
		},
		{
			name:     "thread-dump-analyze-locks",
			args:     []string{"thread-dump", "analyze-locks", "--in", join("thread-dumps", "sample-java-thread-dump.txt")},
			wantType: "thread_dump_locks",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := runJSON(t, bin, tc.args...)
			gotType, _ := payload["type"].(string)
			if gotType != tc.wantType {
				t.Fatalf("type mismatch: got %q want %q", gotType, tc.wantType)
			}
			// AnalysisResult envelope must carry `metadata` and `summary`
			// keys per the shared contract — assert their presence so a
			// regression in models.New surfaces here.
			if _, ok := payload["metadata"].(map[string]any); !ok {
				t.Fatalf("payload missing `metadata` map; keys=%v", mapKeys(payload))
			}
			if _, ok := payload["summary"].(map[string]any); !ok {
				t.Fatalf("payload missing `summary` map; keys=%v", mapKeys(payload))
			}
		})
	}
}

// TestCLIThreadDumpToCollapsed exercises the `thread-dump to-collapsed`
// subcommand, which emits `map[string]int` instead of an
// AnalysisResult envelope. We assert every key has the form
// `frame;...` and every value is a positive integer.
func TestCLIThreadDumpToCollapsed(t *testing.T) {
	bin := buildBinary(t)
	root := fixturesRoot(t)
	cmd := exec.Command(bin, "thread-dump", "to-collapsed", "--in",
		filepath.Join(root, "thread-dumps", "sample-java-thread-dump.txt"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("thread-dump to-collapsed: %v", err)
	}
	var payload map[string]int
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, string(out))
	}
	if len(payload) == 0 {
		t.Fatalf("expected at least one collapsed stack, got empty map")
	}
	for stack, count := range payload {
		if !strings.Contains(stack, ";") {
			t.Errorf("collapsed key %q has no `;` separator (expected `frame;...;leaf`)", stack)
		}
		if count <= 0 {
			t.Errorf("collapsed[%q] = %d (must be > 0)", stack, count)
		}
	}
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
