package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopProgressLogsStayOutsideExecutableBundle(t *testing.T) {
	service := &ProfilerService{}
	options, err := service.optionsFromRequest(AnalyzeRequest{Path: "sample.collapsed"})
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if options.ProgressLogDir == "" || strings.HasPrefix(options.ProgressLogDir, filepath.Dir(exe)+string(os.PathSeparator)) {
		t.Fatalf("desktop progress log can modify the executable bundle: %q", options.ProgressLogDir)
	}
	requested := filepath.Join(t.TempDir(), "explicit-logs")
	options, err = service.optionsFromRequest(AnalyzeRequest{Path: "sample.collapsed", ProgressLogDir: requested})
	if err != nil || options.ProgressLogDir != requested {
		t.Fatalf("explicit log directory not retained: %+v %v", options, err)
	}
}
