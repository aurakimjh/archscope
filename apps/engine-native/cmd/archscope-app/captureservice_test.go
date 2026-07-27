package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
)

type memoryTrustBackend struct{}

func (memoryTrustBackend) Install(string, []byte) error { return nil }
func (memoryTrustBackend) Remove(string, []byte) error  { return nil }

func TestCaptureServiceSessionCanBeAnalyzedAfterStop(t *testing.T) {
	service := newCaptureService(t.TempDir(), certstore.New(memoryTrustBackend{}, nil))
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
	if fixture.SchemaVersion != 1 || fixture.Platform != "windows" {
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
			scenario.CaptureMode != "proxy_mitm" ||
			scenario.Fidelity != "decoded_wire" ||
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
		if scenario.Fidelity != "unsupported" || scenario.SemanticCapture {
			t.Fatalf("unsupported scenario=%+v", scenario)
		}
	}
	if fixture.Security.RetainUnattributedMetadataDefault ||
		fixture.Security.BodyStorage != "omitted" ||
		!fixture.Security.CARemovedOnStop {
		t.Fatalf("security=%+v", fixture.Security)
	}
	if fixture.Renderer.RowCap != 500 ||
		!fixture.Renderer.ResyncOnEventSkip ||
		!fixture.Renderer.RestoreCurrentSessionOnPageReentry ||
		!fixture.Renderer.FinalizedSessionUsesAnalysisResult {
		t.Fatalf("renderer=%+v", fixture.Renderer)
	}
	if !fixture.Store.StableSnapshotCursor ||
		!fixture.Store.SessionBoundCursor ||
		fixture.Store.MaxFetchLimit != 2000 ||
		len(fixture.Store.Filters) != 5 {
		t.Fatalf("store=%+v", fixture.Store)
	}
}
