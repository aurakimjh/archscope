package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestT631ActualCLIProviderCommands(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "engine")
	if out, e := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	run := func(args ...string) []byte {
		t.Helper()
		out, e := exec.Command(binary, args...).CombinedOutput()
		if e != nil {
			t.Fatal(args, e, string(out))
		}
		return out
	}
	directory := t.TempDir()
	log := filepath.Join(directory, "server.log")
	saved := filepath.Join(directory, "result.json")
	target := filepath.Join(directory, "target.json")
	os.WriteFile(log, []byte("11-Nov-2026 10:00:03.000 ERROR [worker] org.apache.catalina.core.ContainerBase Synthetic failure\n"), 0600)
	run("evidence", "analyze", "--in", log, "--importer", "tomcat", "--out", saved)
	run("evidence", "analyze", "--in", log, "--importer", "tomcat", "--options", `{"top_n":1}`, "--out", target)
	description := run("evidence", "describe", "--in", saved)
	if !strings.Contains(string(description), `"identity_basis": "declared"`) || !strings.Contains(string(description), "server:events") {
		t.Fatal(string(description))
	}
	page := run("evidence", "query", "--in", saved, "--provider", "server:events", "--limit", "1")
	var display struct {
		Rows  [][]any `json:"rows"`
		Total int     `json:"total"`
	}
	if e := json.Unmarshal(page, &display); e != nil || len(display.Rows) != 1 || display.Total != 1 {
		t.Fatal(string(page), e)
	}
	equal := run("evidence", "compare", "--in", saved, "--target", saved, "--provider", "compare:/summary/total_events")
	if !strings.Contains(string(equal), `"value": "0"`) {
		t.Fatal(string(equal))
	}
	refused := run("evidence", "compare", "--in", saved, "--target", target, "--provider", "compare:/summary/total_events")
	if !strings.Contains(string(refused), `"code": "selection_mismatch"`) || strings.Contains(string(refused), `"delta"`) {
		t.Fatal(string(refused))
	}
	if out, e := exec.Command(binary, "evidence", "query", "--in", saved, "--limit", "101").CombinedOutput(); e == nil {
		t.Fatal(string(out))
	}
}
