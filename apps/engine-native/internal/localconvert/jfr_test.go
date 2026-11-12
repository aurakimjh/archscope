package localconvert

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func buildTool(t *testing.T, mode string) string {
	t.Helper()
	name := "tool-" + mode
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	if out, e := exec.Command("go", "build", "-o", path, "./testdata/tool.go").CombinedOutput(); e != nil {
		t.Fatalf("tool: %v %s", e, out)
	}
	return path
}
func inputFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input ;$(no-shell).jfr")
	if e := os.WriteFile(path, []byte("FLR\x00synthetic"), 0600); e != nil {
		t.Fatal(e)
	}
	return path
}
func TestT633ConversionSuccessProvenanceAndNoReplacement(t *testing.T) {
	t.Setenv("JAVA_TOOL_OPTIONS", "private-canary")
	ctx := context.Background()
	source := inputFile(t)
	tool := buildTool(t, "ok")
	result, e := JFR(ctx, source, tool)
	if e != nil {
		t.Fatal(e)
	}
	defer os.Remove(result.Path)
	p := result.Provenance
	if p.Version != "21.0.1" || p.InputSHA256 == "" || p.OutputSHA256 == "" || p.ExecutableSHA256 == "" || len(p.Arguments) != 3 {
		t.Fatal(p)
	}
	out := filepath.Join(t.TempDir(), "converted.json")
	if e = SaveNew(ctx, result, out); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(out + ".conversion.json")
	if e != nil {
		t.Fatal(e)
	}
	var saved Provenance
	if json.Unmarshal(b, &saved) != nil || saved.OutputSHA256 != p.OutputSHA256 {
		t.Fatal(string(b))
	}
	if e = SaveNew(ctx, result, out); e == nil {
		t.Fatal("overwrote existing output")
	}
	if e = SaveNew(ctx, result, source); e == nil {
		t.Fatal("overwrote input")
	}
	if _, e = JFR(ctx, source, "jfr"); Code(e) != "CONVERTER_EXPLICIT_TOOL_REQUIRED" {
		t.Fatal(e)
	}
}
func TestT633ConverterFailuresNeverPublishOrLeakStderr(t *testing.T) {
	for _, mode := range []string{"fail", "invalid", "flood", "stderr"} {
		t.Run(mode, func(t *testing.T) {
			result, e := JFR(context.Background(), inputFile(t), buildTool(t, mode))
			if e == nil || result.Path != "" {
				t.Fatal(result, e)
			}
			if Code(e) == "CONVERTER_FAILED" {
				t.Fatal("lost structured code", e)
			}
			if errors.Is(e, context.DeadlineExceeded) {
				t.Fatal("budget failed to kill promptly")
			}
		})
	}
}
func TestT633CancellationTerminatesProcessTree(t *testing.T) {
	tool := buildTool(t, "hang")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type outcome struct {
		path string
		err  error
	}
	done := make(chan outcome, 1)
	source := inputFile(t)
	go func() {
		result, err := JFR(ctx, source, tool)
		done <- outcome{result.Path, err}
	}()
	// Tool verification and Windows process startup can exceed one second on
	// a busy host. Prove a child exists before measuring cancellation latency.
	readyDeadline := time.Now().Add(15 * time.Second)
	for {
		if marker, err := os.ReadFile(tool + ".pid"); err == nil && len(marker) > 0 {
			break
		}
		select {
		case result := <-done:
			t.Fatalf("converter exited before child readiness: %v", result.err)
		default:
		}
		if time.Now().After(readyDeadline) {
			t.Fatal("converter child did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	start := time.Now()
	cancel()
	select {
	case result := <-done:
		if !errors.Is(result.err, context.Canceled) || result.path != "" || time.Since(start) > 5*time.Second {
			t.Fatal(result, time.Since(start))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("converter did not return promptly after cancellation")
	}
	assertChildStopped(t, tool+".pid")
}
