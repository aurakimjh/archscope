package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProfilerCLIExplicitLockUnit(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "lock.collapsed")
	out := filepath.Join(dir, "result.json")
	if err := os.WriteFile(input, []byte("A;Monitor 1000000\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newProfilerAnalyzeCollapsedCommand()
	cmd.SetArgs([]string{"--in", input, "--out", out, "--profile-kind", "lock", "--value-unit", "nanoseconds"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Summary struct {
			Seconds *float64 `json:"estimated_seconds"`
			Unit    string   `json:"value_unit"`
		}
	}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatal(err)
	}
	if result.Summary.Seconds == nil || *result.Summary.Seconds != 0.001 || result.Summary.Unit != "nanoseconds" {
		t.Fatalf("wrong CLI unit contract: %s", b)
	}
	bad := newProfilerAnalyzeCollapsedCommand()
	bad.SetArgs([]string{"--in", input, "--out", out, "--value-unit", "fortnights"})
	if err := bad.Execute(); err == nil {
		t.Fatal("unsupported unit accepted")
	}
}
