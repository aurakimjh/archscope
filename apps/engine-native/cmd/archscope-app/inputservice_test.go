package main

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/inputassist"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestT633BridgeDiscoveryDraftKitAndConversion(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	stateRoot := filepath.Join(root, "state")
	t.Setenv("ARCHSCOPE_STATE_ROOT", stateRoot)
	s := &EngineService{}
	defer s.closeEvidence()
	input := filepath.Join(root, "server.log")
	body, _ := catalog.Contracts.ReadFile("contracts/server.log")
	os.WriteFile(input, body, 0600)
	encoded, e := s.DiscoverEvidenceInput(ctx, input, "", "")
	if e != nil {
		t.Fatal(e)
	}
	var report inputassist.Report
	if json.Unmarshal([]byte(encoded), &report) != nil || report.Detection.Selected != "server.tomcat" {
		t.Fatal(encoded)
	}
	draft := filepath.Join(root, "capture.json")
	if e = s.ExportCaptureDraft(ctx, input, "", "", draft); e != nil {
		t.Fatal(e)
	}
	var capture cases.Capture
	b, _ := os.ReadFile(draft)
	if json.Unmarshal(b, &capture) != nil || capture.Validate() != nil {
		t.Fatal(string(b))
	}
	if _, e = s.DiscoverEvidenceInput(ctx, input, draft, ""); e != nil {
		t.Fatal(e)
	}
	if e = s.ExportCollectionKit(ctx, filepath.Join(root, "manual.zip")); e != nil {
		t.Fatal(e)
	}
	if e = s.ExportCollectionKit(ctx, filepath.Join(stateRoot, "forbidden.zip")); e == nil {
		t.Fatal("wrote inside managed state")
	}
	tool := filepath.Join(root, "tool-ok")
	if runtime.GOOS == "windows" {
		tool += ".exe"
	}
	if b, e := exec.Command("go", "build", "-o", tool, "../../internal/localconvert/testdata/tool.go").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	source := filepath.Join(root, "recording.jfr")
	os.WriteFile(source, []byte("FLR\x00synthetic"), 0600)
	output := filepath.Join(root, "converted.json")
	if _, e = s.ConvertEvidenceJFR(ctx, source, tool, output); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(output + ".conversion.json"); e != nil {
		t.Fatal(e)
	}
}
