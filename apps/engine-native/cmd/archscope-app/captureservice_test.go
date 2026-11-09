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
			BodyCaptureDefault                bool
			BodyCaptureGate                   string
		}
		Renderer struct {
			SchemaVersion                      int
			RowCap                             int
			ResyncOnEventSkip                  bool
			RestoreCurrentSessionOnPageReentry bool
			FinalizedSessionUsesAnalysisResult bool
			BodyCaptureAvailable               bool
			BodyCaptureGate                    string
			BodyCaptureGateReason              string
			MaxBodyBytes                       int
			UpstreamTimingsAvailable           bool
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
	// Body capture remains unavailable until SEC-10 passes. The contract names
	// the pending gate while the default config continues to omit bodies.
	if fixture.Security.RetainUnattributedMetadataDefault != (capture.Config{}).RetainUnattributedMetadata ||
		fixture.Security.BodyStorage != proxy.BodyStorageOmitted ||
		!fixture.Security.CARemovedOnStop ||
		fixture.Security.BodyCaptureDefault != (capture.Config{}).CaptureBodies ||
		fixture.Security.BodyCaptureGate != capture.BodyGateUnavailable {
		t.Fatalf("security=%+v", fixture.Security)
	}
	contract := capture.DefaultLiveCaptureContract()
	if fixture.Renderer.SchemaVersion != contract.SchemaVersion ||
		fixture.Renderer.RowCap != contract.TransactionRowCap ||
		fixture.Renderer.ResyncOnEventSkip != contract.ResyncOnEventSkip ||
		fixture.Renderer.RestoreCurrentSessionOnPageReentry != contract.RestoreCurrentSessionOnPageReentry ||
		fixture.Renderer.FinalizedSessionUsesAnalysisResult != contract.FinalizedSessionUsesAnalysisResult ||
		fixture.Renderer.BodyCaptureAvailable != contract.BodyCaptureAvailable ||
		fixture.Renderer.BodyCaptureGate != contract.BodyCaptureGate ||
		fixture.Renderer.BodyCaptureGateReason != contract.BodyCaptureGateReason ||
		fixture.Renderer.MaxBodyBytes != contract.MaxBodyBytes ||
		fixture.Renderer.UpstreamTimingsAvailable != contract.UpstreamTimingsAvailable {
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
		!strings.Contains(script, "$fixtureTrafficOnly") ||
		!strings.Contains(script, "ArchScopeT581Pinning") ||
		!strings.Contains(script, "intentional T-581 pinning rejection") ||
		!strings.Contains(script, "setApplicationProtocols(new String[] {\"http/1.1\"})") {
		t.Fatal("PowerShell harness does not consume the versioned harness contract")
	}
	if !fixture.Store.StableSnapshotCursor ||
		!fixture.Store.SessionBoundCursor ||
		fixture.Store.MaxFetchLimit != store.MaxFetchLimit ||
		len(fixture.Store.Filters) != 5 {
		t.Fatalf("store=%+v", fixture.Store)
	}
}

// Reading selected detail must not stop recording or return an adjacent row.
func TestCaptureServiceSelectedExchangeDuringCapture(t *testing.T) {
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	started, err := service.StartCapture(capture.Config{ListenAddress: "127.0.0.1:0", RetainUnattributedMetadata: true})
	if err != nil {
		t.Fatal(err)
	}
	defer service.StopCapture(string(started.ID))
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Audit", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()
	u, _ := url.Parse("http://" + started.ListenAddress)
	tr := &http.Transport{Proxy: http.ProxyURL(u)}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr}
	for _, path := range []string{"/one", "/two"} {
		res, e := client.Get(origin.URL + path)
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
	}
	page, err := service.FetchCaptureTransactions(CaptureFetchRequest{SessionID: string(started.ID)})
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	want := page.Items[0].Transaction
	got, err := service.GetCaptureTransaction(string(started.ID), want.ID)
	if err != nil || got == nil || got.Path != "/one" {
		t.Fatalf("selected=%+v err=%v", got, err)
	}
	missing, err := service.GetCaptureTransaction(string(started.ID), "absent")
	if err != nil || missing != nil {
		t.Fatalf("missing=%+v err=%v", missing, err)
	}
	if service.GetCurrentCaptureSession().State != capture.StateRunning {
		t.Fatal("detail stopped recording")
	}
}

func TestCaptureServiceRecordingControls(t *testing.T) {
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	session, err := service.StartCapture(capture.Config{ListenAddress: "127.0.0.1:0", RetainUnattributedMetadata: true})
	if err != nil {
		t.Fatal(err)
	}
	defer service.StopCapture(string(session.ID))
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("X-Path", r.URL.Path) }))
	defer origin.Close()
	proxyURL, _ := url.Parse("http://" + session.ListenAddress)
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	get := func(path string) {
		t.Helper()
		r, e := client.Get(origin.URL + path)
		if e != nil {
			t.Fatal(e)
		}
		_, readErr := io.Copy(io.Discard, r.Body)
		closeErr := r.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read=%v close=%v", readErr, closeErr)
		}
	}
	// HTTP response delivery can precede the proxy's final persistence callback.
	// Wait on the store's acknowledgement instead of assuming client.Get is a
	// persistence barrier, especially under the Windows race scheduler.
	waitPersisted := func(want uint64) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			stats, err := service.GetCaptureStats(string(session.ID))
			if err != nil {
				t.Fatal(err)
			}
			if stats.Persisted == want {
				return
			}
			if stats.Persisted > want || time.Now().After(deadline) {
				t.Fatalf("persisted=%d, want %d: %+v", stats.Persisted, want, stats)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	get("/before")
	waitPersisted(1)
	if _, e := service.SetCapturePaused(string(session.ID), true); e != nil {
		t.Fatal(e)
	}
	get("/paused")
	stats, e := service.GetCaptureStats(string(session.ID))
	if e != nil || stats.Persisted != 1 {
		t.Fatalf("paused=%+v err=%v", stats, e)
	}
	cleared, e := service.ClearCapture(string(session.ID))
	if e != nil || cleared.Generation != 1 || !cleared.Paused {
		t.Fatalf("cleared=%+v err=%v", cleared, e)
	}
	if _, e := service.SetCapturePaused(string(session.ID), false); e != nil {
		t.Fatal(e)
	}
	get("/after")
	waitPersisted(1)
	page, e := service.FetchCaptureTransactions(CaptureFetchRequest{SessionID: string(session.ID)})
	if e != nil || len(page.Items) != 1 || page.Items[0].Transaction.Path != "/after" {
		t.Fatalf("after=%+v err=%v", page, e)
	}
	if _, e := service.StopCapture(string(session.ID)); e != nil {
		t.Fatal(e)
	}
	if _, e := service.ClearCapture(string(session.ID)); e != nil {
		t.Fatal(e)
	}
	stats, e = service.GetCaptureStats(string(session.ID))
	if e != nil || stats.Persisted != 0 || stats.Generation != 2 {
		t.Fatalf("stopped clear=%+v err=%v", stats, e)
	}
}

func TestStoredSearchFindsExchangesOutsideLiveWindow(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oldest" {
			w.Header().Set("X-Fixture", "oldest-marker")
		}
		w.WriteHeader(200)
	}))
	defer origin.Close()
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	started, err := service.StartCapture(capture.Config{ListenAddress: "127.0.0.1:0", LiveWindow: 2, RetainUnattributedMetadata: true})
	if err != nil {
		t.Fatal(err)
	}
	defer service.StopCapture(string(started.ID))
	proxyURL, _ := url.Parse("http://" + started.ListenAddress)
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	for _, path := range []string{"/oldest", "/next", "/latest"} {
		response, e := client.Get(origin.URL + path)
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
	}
	window, err := service.GetCaptureLiveWindow(string(started.ID))
	if err != nil || len(window) != 2 {
		t.Fatalf("window=%+v err=%v", window, err)
	}
	page, err := service.SearchCaptureTransactions(CaptureFetchRequest{SessionID: string(started.ID), Filter: store.Filter{Query: "oldest-marker", SearchFields: []string{"headers"}}})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Transaction.Path != "/oldest" {
		t.Fatalf("search=%+v err=%v", page, err)
	}
	for _, tx := range window {
		if tx.ID == page.Items[0].Transaction.ID {
			t.Fatal("test did not cross the live-window boundary")
		}
	}
	if page.Items[0].Transaction.Response.BodyPreview != "" {
		t.Fatal("search returned unsolicited payload data")
	}
	if _, err := service.SearchCaptureTransactions(CaptureFetchRequest{SessionID: string(started.ID), Filter: store.Filter{SearchFields: []string{"invalid"}}}); err == nil {
		t.Fatal("unknown search field accepted")
	}
}
