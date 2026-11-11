package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	pprofprofile "github.com/google/pprof/profile"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func TestC4ActualCLIImportReopenExportsAndSelection(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "engine.exe")
	build := exec.Command("go", "build", "-o", executable, ".")
	if out, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build CLI: %v %s", e, out)
	}
	run := func(args ...string) ([]byte, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return exec.CommandContext(ctx, executable, args...).CombinedOutput()
	}
	root := t.TempDir()
	input := filepath.Join(root, "server.log")
	body, _ := catalog.Contracts.ReadFile("contracts/server.log")
	os.WriteFile(input, body, 0600)
	saved := filepath.Join(root, "saved.json")
	if out, e := run("evidence", "import", "--in", input, "--importer", "TOMCAT", "--out", saved); e != nil {
		t.Fatal(e, string(out))
	}
	raw, _ := os.ReadFile(saved)
	result, e := resultcontract.Decode(raw)
	if e != nil || result.Type != "server_log" || result.Summary["total_events"] != json.Number("3") {
		t.Fatal(result.Summary, e)
	}
	future, _ := catalog.Contracts.ReadFile("contracts/wire-cases.json")
	result.Metadata.Extra["future"] = json.RawMessage(future)
	raw, _ = json.Marshal(result)
	os.WriteFile(saved, raw, 0600)
	for _, format := range []string{"json", "csv", "html", "report-pack"} {
		outPath := filepath.Join(root, "export."+format)
		if out, e := run("evidence", "export", "--in", saved, "--export-format", format, "--out", outPath); e != nil {
			t.Fatal(format, e, string(out))
		}
		body, e := os.ReadFile(outPath)
		if e != nil {
			t.Fatal(e)
		}
		if format == "json" && string(body) != string(raw) {
			t.Fatal("CLI load/save lost exact unknown JSON")
		}
		if format == "csv" && !strings.Contains(string(body), "total_events,3") {
			t.Fatal("CLI CSV lost counts")
		}
		if format == "report-pack" {
			if _, e = zip.NewReader(bytes.NewReader(body), int64(len(body))); e != nil {
				t.Fatal(e)
			}
		}
	}
	hint := filepath.Join(root, "hint.log")
	os.WriteFile(hint, []byte("not a verified log record"), 0600)
	out, e := run("evidence", "import", "--in", hint)
	if e == nil || !strings.Contains(string(out), "IMPORT_SELECTION_REQUIRED") || !strings.Contains(string(out), "server.tomcat") {
		t.Fatal("hint auto-selected", e, string(out))
	}
	// Unknown payload snapshots are retainable raw data, never silently migrated.
	result = models.New("profile_evidence", "legacy")
	result.Metadata.SchemaVersion = "0.2.0"
	result.Summary["total_samples"] = 3
	raw, _ = json.Marshal(result)
	os.WriteFile(saved, raw, 0600)
	if _, e = run("evidence", "export", "--in", saved, "--out", filepath.Join(root, "blocked.json")); e == nil {
		t.Fatal("unsupported profile semantics exported as current")
	}
	if out, e = run("evidence", "export", "--in", saved, "--export-format", "raw-json"); e != nil || string(out) != string(raw) {
		t.Fatal("original unsupported snapshot not preserved", e)
	}
}

func TestReviewR01ActualCLIDeepProfilerReportsAndEvidenceExport(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "engine.exe")
	if out, e := exec.Command("go", "build", "-o", executable, ".").CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	run := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if out, e := exec.CommandContext(ctx, executable, args...).CombinedOutput(); e != nil {
			t.Fatal(args, e, string(out))
		}
	}
	root := t.TempDir()
	input := filepath.Join(root, "deep.collapsed")
	saved := filepath.Join(root, "deep.json")
	frames := make([]string, 256)
	for i := range frames {
		frames[i] = fmt.Sprintf("synthetic.Frame%d", i)
	}
	os.WriteFile(input, []byte(strings.Join(frames, ";")+" 3\n"), 0600)
	run("profiler", "analyze-collapsed", "--in", input, "--out", saved)
	for _, format := range []string{"json", "html", "csv", "pptx"} {
		output := filepath.Join(root, "report."+format)
		run("report", format, "--in", saved, "--out", output)
		if info, e := os.Stat(output); e != nil || info.Size() == 0 {
			t.Fatal(format, e)
		}
	}
	run("report", "diff", "--before", saved, "--after", saved, "--out", filepath.Join(root, "diff.json"))
	for _, format := range []string{"json", "html", "csv", "report-pack"} {
		run("evidence", "export", "--in", saved, "--export-format", format, "--out", filepath.Join(root, "evidence."+format))
	}
}

func TestT629ActualCLIProfileSetImportQueriesAndExports(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "engine.exe")
	if out, e := exec.Command("go", "build", "-o", executable, ".").CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	run := func(args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		body, e := exec.CommandContext(ctx, executable, args...).CombinedOutput()
		if e != nil {
			t.Fatal(args, e, string(body))
		}
		return body
	}
	directory := t.TempDir()
	input := filepath.Join(directory, "input.json")
	saved := filepath.Join(directory, "saved.json")
	fixture, _ := catalog.Contracts.ReadFile("contracts/profile-set.json")
	os.WriteFile(input, fixture, 0600)
	run("evidence", "import", "--in", input, "--importer", "profile-set", "--options", `{"profile_ids":["worker-a","worker-b"],"metric_id":"duration"}`, "--out", saved)
	tree := run("profile-set", "tree", "--in", saved)
	if !strings.Contains(string(tree), `"inclusive":"0.35"`) {
		t.Fatal(string(tree))
	}
	diff := run("profile-set", "diff", "--in", saved, "--after", saved)
	if !strings.Contains(string(diff), `"delta":"0"`) {
		t.Fatal(string(diff))
	}
	stages := run("profile-set", "drilldown", "--in", saved)
	if !strings.Contains(string(stages), `"inclusive":"0.35"`) {
		t.Fatal(string(stages))
	}
	for _, format := range []string{"json", "csv", "html", "report-pack"} {
		out := filepath.Join(directory, "out."+format)
		run("evidence", "export", "--in", saved, "--export-format", format, "--out", out)
		if info, e := os.Stat(out); e != nil || info.Size() == 0 {
			t.Fatal(format, e)
		}
	}
	pprof := filepath.Join(directory, "selected.pprof")
	run("profile-set", "pprof", "--in", saved, "--tick-scale", "0.01", "--out", pprof)
	raw, _ := os.ReadFile(pprof)
	profile, e := pprofprofile.ParseData(raw)
	if e != nil {
		t.Fatal(e)
	}
	sum := int64(0)
	for _, sample := range profile.Sample {
		sum += sample.Value[0]
	}
	if sum != 35 || len(profile.Sample) != 3 {
		t.Fatal(profile)
	}
	// Unsafe projection must not replace an existing artifact.
	target := filepath.Join(directory, "prior.pprof")
	os.WriteFile(target, []byte("prior"), 0600)
	if out, e := exec.Command(executable, "profile-set", "pprof", "--in", saved, "--tick-scale", "1", "--out", target).CombinedOutput(); e == nil || !strings.Contains(string(out), "precision") {
		t.Fatal(e, string(out))
	}
	prior, _ := os.ReadFile(target)
	if string(prior) != "prior" {
		t.Fatal("failed projection replaced prior artifact")
	}
}
