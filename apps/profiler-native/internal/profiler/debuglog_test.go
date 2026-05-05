package profiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugLogVerdictAndPortableFilename(t *testing.T) {
	log := NewDebugLog("collapsed", "/tmp/sample.collapsed")
	if log.Verdict() != "CLEAN" {
		t.Fatalf("verdict should start CLEAN; got %q", log.Verdict())
	}
	log.AddParseError(10, "BAD", "bad row", "PATTERN", map[string]string{"target": "Authorization: Bearer abc"}, nil)
	if log.Verdict() != "PARSE_ISSUES" {
		t.Fatalf("verdict should be PARSE_ISSUES after error; got %q", log.Verdict())
	}
	log.AddException("analyze", "boom", "")
	if log.Verdict() != "FATAL_ERROR" {
		t.Fatalf("verdict should escalate to FATAL_ERROR; got %q", log.Verdict())
	}

	name := log.PortableFilename()
	if !strings.HasPrefix(name, "archscope-debug-collapsed-") {
		t.Fatalf("filename prefix; got %q", name)
	}
	if !strings.HasSuffix(name, ".json") {
		t.Fatalf("filename should be JSON; got %q", name)
	}
}

func TestDebugLogWriteRedactsContextAndCapsSamples(t *testing.T) {
	log := NewDebugLog("collapsed", "/tmp/sample.collapsed")
	for i := 0; i < 12; i++ {
		log.AddParseError(
			i,
			"INVALID",
			"bad",
			"PATTERN",
			map[string]string{"target": "Authorization: Bearer redact-me"},
			nil,
		)
	}
	dir := t.TempDir()
	path, err := log.WriteJSON(dir)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(body), "redact-me") {
		t.Fatalf("debug log must not contain unredacted token: %s", body)
	}
	if !strings.Contains(string(body), "TOKEN len=") {
		t.Fatalf("expected token placeholder in payload; got %s", body)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs, ok := parsed["parse_errors"].([]any)
	if !ok {
		t.Fatalf("parse_errors should be array; got %T", parsed["parse_errors"])
	}
	if len(errs) > log.maxSamplesByType {
		t.Fatalf("sample cap exceeded: got %d, max %d", len(errs), log.maxSamplesByType)
	}
}

func TestDebugLogHasContent(t *testing.T) {
	log := NewDebugLog("collapsed", "/tmp/sample.collapsed")
	if log.HasContent() {
		t.Fatalf("fresh log should be empty")
	}
	log.AddParseError(1, "X", "msg", "P", nil, nil)
	if !log.HasContent() {
		t.Fatalf("log should report content after AddParseError")
	}
}

func TestDebugLogWriteCreatesDir(t *testing.T) {
	log := NewDebugLog("svg", "/tmp/sample.svg")
	log.AddParseError(0, "INVALID", "bad", "PAT", nil, nil)
	dir := filepath.Join(t.TempDir(), "subdir", "nested")
	path, err := log.WriteJSON(dir)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.HasPrefix(path, dir) {
		t.Fatalf("path %q should live under %q", path, dir)
	}
}
