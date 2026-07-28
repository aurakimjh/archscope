package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/acceptance"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/proxy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
)

type memoryTrustBackend struct{}

func (memoryTrustBackend) Install(string, []byte) error { return nil }
func (memoryTrustBackend) Remove(string, []byte) error  { return nil }

func TestCaptureServiceSessionCanBeAnalyzedAfterStop(t *testing.T) {
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	started, err := service.StartCapture(capture.Config{
		ListenAddress: "127.0.0.1:0", ReserveBytes: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if current := service.GetCurrentCaptureSession(); current.ID != started.ID {
		t.Fatalf("current=%+v started=%+v", current, started)
	}
	if window, windowErr := service.GetCaptureLiveWindow(string(started.ID)); windowErr != nil || len(window) != 0 {
		t.Fatalf("live window=%+v err=%v", window, windowErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ctx
	stopped, err := service.StopCapture(string(started.ID))
	if err != nil {
		t.Fatal(err)
	}
	if stopped.State != capture.StateFinalized {
		t.Fatalf("stopped=%+v", stopped)
	}
	evidence, err := service.GetCaptureAcceptanceEvidence(string(started.ID))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Session.SessionID != string(started.ID) ||
		evidence.Stats.State != capture.StateFinalized {
		t.Fatalf("evidence=%+v", evidence)
	}
	result, err := service.AnalyzeCaptureSession(CaptureAnalyzeRequest{SessionID: string(started.ID)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "http_capture" || result.Summary["total_transactions"] != 0 {
		t.Fatalf("result type=%q summary=%+v", result.Type, result.Summary)
	}
}

func TestT581LiveCaptureAcceptanceFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/t581_live_capture_acceptance.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		SchemaVersion int `json:"schemaVersion"`
		Platform      string
		SupportedTier []struct {
			Client      string
			Transport   string
			CaptureMode string
			Fidelity    string
			Coverage    string
		}
		UnsupportedTier []struct {
			Scenario        string
			Fidelity        string
			SemanticCapture bool
		}
		Security struct {
			RetainUnattributedMetadataDefault bool
			BodyStorage                       string
			CARemovedOnStop                   bool
		}
		Renderer struct {
			RowCap                             int
			ResyncOnEventSkip                  bool
			RestoreCurrentSessionOnPageReentry bool
			FinalizedSessionUsesAnalysisResult bool
		}
		AcceptanceEvidence struct {
			SchemaVersion           int
			MaxRows                 int
			RequiresTerminalSession bool
			ProductReadback         bool
			FailsOnMissingClient    bool
			Transports              []string
		}
		Store struct {
			StableSnapshotCursor bool
			SessionBoundCursor   bool
			MaxFetchLimit        int
			Filters              []string
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion != acceptance.SchemaVersion || fixture.Platform != "windows" {
		t.Fatalf("fixture header=%+v", fixture)
	}
	expectedClients := map[string]bool{
		"browser": false, "curl": false, "jvm": false, "electron": false,
	}
	for _, scenario := range fixture.SupportedTier {
		if _, ok := expectedClients[scenario.Client]; !ok {
			t.Fatalf("unexpected client %q", scenario.Client)
		}
		expectedClients[scenario.Client] = true
		if scenario.Transport != "http/1.1" ||
			scenario.CaptureMode != proxy.CaptureModeMITM ||
			scenario.Fidelity != proxy.FidelityDecodedWire ||
			scenario.Coverage != "confirmed" {
			t.Fatalf("supported scenario=%+v", scenario)
		}
	}
	for client, present := range expectedClients {
		if !present {
			t.Fatalf("missing supported client %q", client)
		}
	}
	for _, scenario := range fixture.UnsupportedTier {
		if scenario.Fidelity != proxy.FidelityUnsupported || scenario.SemanticCapture {
			t.Fatalf("unsupported scenario=%+v", scenario)
		}
	}
	if fixture.Security.RetainUnattributedMetadataDefault != (capture.Config{}).RetainUnattributedMetadata ||
		fixture.Security.BodyStorage != proxy.BodyStorageOmitted ||
		!fixture.Security.CARemovedOnStop {
		t.Fatalf("security=%+v", fixture.Security)
	}
	if fixture.Renderer.RowCap != 500 ||
		!fixture.Renderer.ResyncOnEventSkip ||
		!fixture.Renderer.RestoreCurrentSessionOnPageReentry ||
		!fixture.Renderer.FinalizedSessionUsesAnalysisResult {
		t.Fatalf("renderer=%+v", fixture.Renderer)
	}
	if fixture.AcceptanceEvidence.SchemaVersion != acceptance.SchemaVersion ||
		fixture.AcceptanceEvidence.MaxRows != acceptance.MaxEvidenceRows ||
		!fixture.AcceptanceEvidence.RequiresTerminalSession ||
		!fixture.AcceptanceEvidence.ProductReadback ||
		!fixture.AcceptanceEvidence.FailsOnMissingClient ||
		len(fixture.AcceptanceEvidence.Transports) != 2 ||
		fixture.AcceptanceEvidence.Transports[0] != "http" ||
		fixture.AcceptanceEvidence.Transports[1] != "https" {
		t.Fatalf("acceptance evidence=%+v", fixture.AcceptanceEvidence)
	}
	if !fixture.Store.StableSnapshotCursor ||
		!fixture.Store.SessionBoundCursor ||
		fixture.Store.MaxFetchLimit != store.MaxFetchLimit ||
		len(fixture.Store.Filters) != 5 {
		t.Fatalf("store=%+v", fixture.Store)
	}
}
