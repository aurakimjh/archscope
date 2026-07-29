package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
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
	metadata := result.Metadata.Extra["http_capture"].(map[string]any)
	if metadata["capture_mode"] == "har_import" ||
		metadata["observation_point"] == "foreign_tool" ||
		metadata["fidelity"] == "semantic" {
		t.Fatalf("empty live session claimed imported semantic provenance: %+v", metadata)
	}
}

func TestCaptureServiceFinalizedAnalysisUsesLiveProvenanceAndPersistedRedaction(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "ok")
	}))
	defer origin.Close()
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	started, err := service.StartCapture(capture.Config{
		ListenAddress: "127.0.0.1:0", ReserveBytes: 0,
		RetainUnattributedMetadata: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	proxyAddress, err := url.Parse("http://" + started.ListenAddress)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyAddress)}}
	response, err := client.Get(origin.URL + "/capture?token=example-secret")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(response.Body)
	response.Body.Close()
	if _, err := service.StopCapture(string(started.ID)); err != nil {
		t.Fatal(err)
	}
	manifest, err := service.manager.Manifest(started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Redaction == nil || !manifest.Redaction.Applied ||
		manifest.Redaction.Counts["query_value"] == 0 {
		t.Fatalf("manifest redaction=%+v", manifest.Redaction)
	}
	result, err := service.AnalyzeCaptureSession(CaptureAnalyzeRequest{SessionID: string(started.ID)})
	if err != nil {
		t.Fatal(err)
	}
	metadata := result.Metadata.Extra["http_capture"].(map[string]any)
	if metadata["capture_mode"] != proxy.CaptureModeMITM ||
		metadata["observation_point"] != "proxy" ||
		metadata["fidelity"] != proxy.FidelityDecodedWire ||
		result.Summary["redaction_applied"] != true {
		t.Fatalf("finalized live result metadata=%+v summary=%+v", metadata, result.Summary)
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
			Visibility      string
			SemanticCapture bool
		}
		Security struct {
			RetainUnattributedMetadataDefault bool
			BodyStorage                       string
			CARemovedOnStop                   bool
		}
		Renderer struct {
			SchemaVersion                      int
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
		Harness acceptance.HarnessContract
		Store   struct {
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
		if scenario.Scenario == "quic" {
			if scenario.Fidelity != "not_applicable" ||
				scenario.Visibility != "not_observed" ||
				scenario.SemanticCapture {
				t.Fatalf("QUIC scenario=%+v", scenario)
			}
			continue
		}
		if scenario.Fidelity != proxy.FidelityUnsupported ||
			scenario.Visibility != "observed" ||
			scenario.SemanticCapture {
			t.Fatalf("unsupported scenario=%+v", scenario)
		}
	}
	if fixture.Security.RetainUnattributedMetadataDefault != (capture.Config{}).RetainUnattributedMetadata ||
		fixture.Security.BodyStorage != proxy.BodyStorageOmitted ||
		!fixture.Security.CARemovedOnStop {
		t.Fatalf("security=%+v", fixture.Security)
	}
	contract := capture.DefaultLiveCaptureContract()
	if fixture.Renderer.SchemaVersion != contract.SchemaVersion ||
		fixture.Renderer.RowCap != contract.TransactionRowCap ||
		fixture.Renderer.ResyncOnEventSkip != contract.ResyncOnEventSkip ||
		fixture.Renderer.RestoreCurrentSessionOnPageReentry != contract.RestoreCurrentSessionOnPageReentry ||
		fixture.Renderer.FinalizedSessionUsesAnalysisResult != contract.FinalizedSessionUsesAnalysisResult {
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
	harnessContract := acceptance.DefaultHarnessContract()
	if !reflect.DeepEqual(fixture.Harness, harnessContract) {
		t.Fatalf("harness=%+v contract=%+v", fixture.Harness, harnessContract)
	}
	contractData, err := os.ReadFile("../../../../scripts/t581-live-capture-harness-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var scriptContract acceptance.HarnessContract
	if err := json.Unmarshal(contractData, &scriptContract); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scriptContract, harnessContract) {
		t.Fatalf("script harness=%+v contract=%+v", scriptContract, harnessContract)
	}
	scriptData, err := os.ReadFile("../../../../scripts/verify-windows-live-capture.ps1")
	if err != nil {
		t.Fatal(err)
	}
	script := string(scriptData)
	if !strings.Contains(script, "t581-live-capture-harness-contract.json") ||
		!strings.Contains(script, "$harnessContract.schemaVersion") ||
		!strings.Contains(script, "$harnessContract.productEvidenceSchemaVersion") ||
		!strings.Contains(script, "$harnessContract.quicInvisibility") ||
		!strings.Contains(script, "Test-FixtureArtifactRow") ||
		!strings.Contains(script, "omittedNonFixtureRows") ||
		!strings.Contains(script, "$fixtureTrafficOnly") {
		t.Fatal("PowerShell harness does not consume the versioned harness contract")
	}
	if !fixture.Store.StableSnapshotCursor ||
		!fixture.Store.SessionBoundCursor ||
		fixture.Store.MaxFetchLimit != store.MaxFetchLimit ||
		len(fixture.Store.Filters) != 5 {
		t.Fatalf("store=%+v", fixture.Store)
	}
}
