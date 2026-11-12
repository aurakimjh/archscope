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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	result, e := JFR(ctx, inputFile(t), tool)
	if !errors.Is(e, context.DeadlineExceeded) || result.Path != "" || time.Since(start) > 5*time.Second {
		t.Fatal(result, e, time.Since(start))
	}
	assertChildStopped(t, tool+".pid")
}
