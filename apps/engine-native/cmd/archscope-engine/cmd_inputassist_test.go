package main

import (
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/inputassist"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localconvert"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestT633ActualCLIDiscoveryKitAndConversion(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ARCHSCOPE_STATE_ROOT", filepath.Join(root, "state"))
	bin := buildBinary(t)
	run := func(args ...string) []byte {
		t.Helper()
		b, e := exec.Command(bin, args...).CombinedOutput()
		if e != nil {
			t.Fatalf("%v: %v %s", args, e, b)
		}
		return b
	}
	input := filepath.Join(root, "server.log")
	body, _ := catalog.Contracts.ReadFile("contracts/server.log")
	os.WriteFile(input, body, 0600)
	var report inputassist.Report
	if e := json.Unmarshal(run("evidence", "discover", "--in", input), &report); e != nil || report.Detection.Selected != "server.tomcat" {
		t.Fatal(report, e)
	}
	kit := filepath.Join(root, "manual.zip")
	run("evidence", "collection-kit", "--out", kit)
	if i, e := os.Stat(kit); e != nil || i.Size() == 0 {
		t.Fatal(e)
	}
	source := filepath.Join(root, "recording.jfr")
	os.WriteFile(source, []byte("FLR\x00synthetic"), 0600)
	tool := filepath.Join(root, "tool-ok")
	if runtime.GOOS == "windows" {
		tool += ".exe"
	}
	if b, e := exec.Command("go", "build", "-o", tool, "../../internal/localconvert/testdata/tool.go").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	out := filepath.Join(root, "converted.json")
	var provenance localconvert.Provenance
	if e := json.Unmarshal(run("jfr", "convert", "--in", source, "--cli", tool, "--out", out), &provenance); e != nil || provenance.OutputSHA256 == "" {
		t.Fatal(provenance, e)
	}
	run("jfr", "analyze-json", "--in", out)
	if b, e := exec.Command(bin, "jfr", "convert", "--in", source, "--cli", tool, "--out", out).CombinedOutput(); e == nil {
		t.Fatal("replaced conversion", string(b))
	}
}
