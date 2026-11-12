package distribution_test

import (
	"context"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/aiinterpretation"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/proxy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/requestbuilder"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/session"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localconvert"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/netpolicy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/jfr"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOfflineGuardBeforeNetworkAndConverterAllocation(t *testing.T) {
	if os.Getenv("ARCHSCOPE_POLICY_TEST_CHILD") != "1" {
		exe, e := os.Executable()
		if e != nil {
			t.Fatal(e)
		}
		cmd := exec.Command(exe, "-test.run=^TestOfflineGuardBeforeNetworkAndConverterAllocation$")
		cmd.Env = append(os.Environ(), "ARCHSCOPE_POLICY_TEST_CHILD=1")
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("%s %v", out, e)
		}
		return
	}
	netpolicy.Enable()
	if len(netpolicy.Snapshot().Attempts) != 0 {
		t.Fatal("unexpected initial attempt")
	}
	if _, e := (&session.Manager{}).Start(context.Background(), capture.Config{}); e == nil {
		t.Fatal("capture accepted")
	}
	if _, e := (&requestbuilder.Client{}).Send(requestbuilder.Request{}); e == nil {
		t.Fatal("request accepted")
	}
	if _, e := (aiinterpretation.OllamaClient{}).Execute(context.Background(), aiinterpretation.PromptPayload{}, nil); e == nil {
		t.Fatal("AI accepted")
	}
	root := t.TempDir()
	if _, e := automation.Start(filepath.Join(root, "control.json"), nil); e == nil {
		t.Fatal("listener accepted")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("offline control allocated artifacts")
	}
	if _, e := (&proxy.Server{}).Start(context.Background(), capture.Config{}, nil); e == nil {
		t.Fatal("proxy accepted")
	}
	if _, e := jfr.ConvertJFRToJSON("unused.jfr", "must-not-execute"); e == nil || localconvert.Code(e) != "OFFLINE_OPERATION_DENIED" {
		t.Fatal("unqualified subprocess accepted")
	}
	if len(netpolicy.Snapshot().Attempts) != 6 {
		t.Fatal(netpolicy.Snapshot())
	}
}
