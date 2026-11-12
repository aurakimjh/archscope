package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestT637ActualCLISourcesProfilesAndExistingExports(t *testing.T) {
	engine := filepath.Join(t.TempDir(), "engine")
	if out, e := exec.Command("go", "build", "-o", engine, ".").CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	run := func(args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, e := exec.CommandContext(ctx, engine, args...).CombinedOutput()
		if e != nil {
			t.Fatal(args, e, string(out))
		}
		return out
	}
	dir := t.TempDir()
	base := "../../internal/parsers/systemtrace/testdata/"
	for _, tc := range []struct{ file, format, capture, metric string }{{"perf-script.txt", "perf-script", "perf-capture.json", "period/cpu-clock:HG"}, {"events.jsonl", "events-jsonl", "events-capture.json", "off_cpu"}, {"bpftrace-hist.jsonl", "bpftrace-jsonl", "bpftrace-capture.json", ""}} {
		result := filepath.Join(dir, tc.format+".json")
		model := filepath.Join(dir, tc.format+"-model.json")
		out := run("system", "inspect-source", "--in", base+tc.file, "--format", tc.format, "--capture-file", base+tc.capture)
		if !strings.Contains(string(out), `"source_sha256"`) {
			t.Fatal(string(out))
		}
		run("system", "normalize-source", "--in", base+tc.file, "--format", tc.format, "--capture-file", base+tc.capture, "--out", model)
		run("evidence", "import", "--in", model, "--importer", "system.evidence-json", "--options", "{}", "--out", result)
		for _, format := range []string{"json", "csv", "html", "report-pack"} {
			run("evidence", "export", "--in", result, "--export-format", format, "--out", filepath.Join(dir, tc.format+"-report."+format))
		}
		if tc.metric != "" {
			profile := filepath.Join(dir, tc.format+"-profile.json")
			run("system", "project-profile", "--in", result, "--metric-id", tc.metric, "--out", profile)
			tree := run("profile-set", "tree", "--in", profile)
			if !strings.Contains(string(tree), `"inclusive"`) {
				t.Fatal(string(tree))
			}
			run("visual", "create-spec", "--in", profile, "--query", `{"kind":"profile_top","offset":0,"limit":25}`, "--out", filepath.Join(dir, tc.format+"-visual.json"))
		} else {
			out, e := exec.Command(engine, "system", "project-profile", "--in", result, "--metric-id", "histogram").CombinedOutput()
			if e == nil || !strings.Contains(string(out), "SYSTEM_PROJECTION_UNSUPPORTED") {
				t.Fatal("histogram produced event profile", e)
			}
		}
		body, _ := os.ReadFile(result)
		var doc map[string]any
		json.Unmarshal(body, &doc)
		if doc["type"] != "system_evidence" {
			t.Fatal("envelope changed")
		}
		if out, e := exec.Command(engine, "system", "normalize-source", "--in", base+tc.file, "--format", tc.format, "--capture-file", base+tc.capture, "--out", model).CombinedOutput(); e == nil || !strings.Contains(string(out), "EXISTS") {
			t.Fatal("overwrite permitted", e)
		}
	}
}
