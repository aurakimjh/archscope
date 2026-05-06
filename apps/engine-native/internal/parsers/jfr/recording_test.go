package jfr

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

func TestIsBinaryJFRDetectsMagic(t *testing.T) {
	tmp := t.TempDir()

	binPath := filepath.Join(tmp, "rec.jfr")
	if err := os.WriteFile(binPath, append([]byte{'F', 'L', 'R', 0x00}, []byte("…rest…")...), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !IsBinaryJFR(binPath) {
		t.Errorf("expected IsBinaryJFR to be true for FLR\\0 prefix")
	}

	jsonPath := filepath.Join(tmp, "rec.json")
	if err := os.WriteFile(jsonPath, []byte(`{"events": []}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if IsBinaryJFR(jsonPath) {
		t.Errorf("JSON file should not be flagged as binary JFR")
	}

	// Missing file returns false rather than panicking.
	if IsBinaryJFR(filepath.Join(tmp, "missing.jfr")) {
		t.Errorf("missing file should return false")
	}
}

func TestDiscoverCLIRespectsEnvOverride(t *testing.T) {
	tmp := t.TempDir()
	override := filepath.Join(tmp, "fake-jfr")
	if err := os.WriteFile(override, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("ARCHSCOPE_JFR_CLI", override)
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")

	got := DiscoverCLI()
	if got != override {
		t.Errorf("DiscoverCLI() = %q, want %q", got, override)
	}
}

func TestDiscoverCLIFallsBackToJavaHome(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	candidate := filepath.Join(bin, "jfr")
	if err := os.WriteFile(candidate, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("ARCHSCOPE_JFR_CLI", "")
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", tmp)

	got := DiscoverCLI()
	if got != candidate {
		t.Errorf("DiscoverCLI() = %q, want %q", got, candidate)
	}
}

func TestDiscoverCLIReturnsEmptyWhenNothingFound(t *testing.T) {
	t.Setenv("ARCHSCOPE_JFR_CLI", "")
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")
	if got := DiscoverCLI(); got != "" {
		t.Errorf("DiscoverCLI() = %q, want empty", got)
	}
}

func TestParseRecordingDispatchesToJSONForJSONInput(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rec.json")
	if err := os.WriteFile(path, []byte(`{"events": []}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	diags := diagnostics.New("jfr")
	events, info, err := ParseRecording(path, diags)
	if err != nil {
		t.Fatalf("ParseRecording: %v", err)
	}
	if info.SourceFormat != "json" {
		t.Errorf("SourceFormat = %q, want json", info.SourceFormat)
	}
	if len(events) != 0 {
		t.Errorf("events = %+v, want empty", events)
	}
}

func TestParseRecordingReturnsCLIMissingForBinaryWithoutCLI(t *testing.T) {
	t.Setenv("ARCHSCOPE_JFR_CLI", "")
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")

	tmp := t.TempDir()
	path := filepath.Join(tmp, "rec.jfr")
	if err := os.WriteFile(path, append([]byte{'F', 'L', 'R', 0x00}, make([]byte, 32)...), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, info, err := ParseRecording(path, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !IsCLIMissing(err) {
		t.Errorf("expected CLIMissingError, got %T: %v", err, err)
	}
	if info.SourceFormat != "binary_jfr" {
		t.Errorf("SourceFormat = %q, want binary_jfr", info.SourceFormat)
	}
}

func TestConvertJFRToJSONReturnsCLIMissingWithoutBinary(t *testing.T) {
	t.Setenv("ARCHSCOPE_JFR_CLI", "")
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")
	_, err := ConvertJFRToJSON("/nonexistent.jfr", "")
	if err == nil {
		t.Fatalf("expected error")
	}
	var missing *CLIMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("want *CLIMissingError, got %T: %v", err, err)
	}
}

// TestConvertJFRToJSONIntegration exercises the os/exec bridge end to
// end. Skipped when no `jfr` binary is on PATH (the common case in CI
// and on dev machines without a JDK).
func TestConvertJFRToJSONIntegration(t *testing.T) {
	if _, err := exec.LookPath("jfr"); err != nil {
		t.Skip("jfr binary not on PATH")
	}

	// We don't ship a real binary .jfr in the repo — the JDK CLI will
	// reject our placeholder, which is fine: we're verifying that we
	// invoked the CLI and surfaced its stderr, not that we round-tripped
	// a recording.
	tmp := t.TempDir()
	bogus := filepath.Join(tmp, "bogus.jfr")
	if err := os.WriteFile(bogus, append([]byte{'F', 'L', 'R', 0x00}, make([]byte, 32)...), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := ConvertJFRToJSON(bogus, "")
	if err == nil {
		t.Fatalf("expected error from jfr CLI on bogus input")
	}
	if IsCLIMissing(err) {
		t.Fatalf("CLI is on PATH but got CLIMissingError: %v", err)
	}
	if !strings.Contains(err.Error(), "jfr print --json failed") {
		t.Logf("note: integration error message = %v", err)
	}
}
