package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/proxy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestBuildReadsFinalizedProductRowsAndStats(t *testing.T) {
	st, err := store.New(store.Config{
		Root: t.TempDir(), SessionID: "cap-evidence", ReserveBytes: -1,
		FreeBytes: func(string) (uint64, error) { return ^uint64(0), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	tx := models.CaptureTransaction{
		ID: "tx-1", Method: "GET",
		URL:    "https://example.test/check?archscope_t581_client=curl-https",
		Scheme: "https", Host: "example.test", Path: "/check",
		StatusCode: 200, State: models.TxComplete,
		CaptureMode: proxy.CaptureModeMITM,
		Fidelity:    proxy.FidelityDecodedWire,
		Coverage:    "confirmed",
		Process:     &models.ProcessInstance{Name: "curl.exe", Attribution: "confirmed"},
		Request: models.HTTPMessage{
			BodyStorage: "omitted",
			Headers:     []models.HeaderField{{Name: "Authorization", Value: "[REDACTED]", Redacted: true}},
		},
		Response: models.HTTPMessage{BodyStorage: "omitted"},
	}
	if _, err := st.Append(tx); err != nil {
		t.Fatal(err)
	}
	stats := capture.Stats{
		SessionID: "cap-evidence", State: capture.StateFinalized,
		Observed: 1, Captured: 1, Persisted: 1,
	}
	if err := st.SetCaptureStats(stats); err != nil {
		t.Fatal(err)
	}
	if err := st.SetRedactionSummary(redact.Summary{
		Applied: true, Version: redact.PolicyVersion,
		Rules: []string{"query_value"}, Counts: map[string]int{"query_value": 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Finalize(capture.StateFinalized); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(st.Path(), "manifest.json")
	before, err := os.Stat(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	evidence, err := Build(st.Path())
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("read-only evidence export modified manifest: before=%s after=%s", before.ModTime(), after.ModTime())
	}
	if evidence.SchemaVersion != SchemaVersion ||
		evidence.Session.TotalRows != 1 ||
		evidence.Stats.Observed != 1 ||
		!evidence.Redaction.Known ||
		!evidence.Redaction.Applied ||
		evidence.LiveContract.TransactionRowCap != capture.DefaultLiveTransactionRowCap ||
		evidence.Counts["mode:"+proxy.CaptureModeMITM] != 1 ||
		evidence.Counts["fidelity:"+proxy.FidelityDecodedWire] != 1 {
		t.Fatalf("evidence=%+v", evidence)
	}
	if len(evidence.Rows) != 1 ||
		evidence.Rows[0].ProcessName != "curl.exe" ||
		evidence.Rows[0].RequestBodyStorage != "omitted" {
		t.Fatalf("rows=%+v", evidence.Rows)
	}
}

func TestBuildUsesStoredRowsAsConservativeFallbackForLegacyRecovery(t *testing.T) {
	st, err := store.New(store.Config{
		Root: t.TempDir(), SessionID: "legacy-recovery", ReserveBytes: -1,
		FreeBytes: func(string) (uint64, error) { return ^uint64(0), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Append(models.CaptureTransaction{
		ID: "tx-recovered", Method: "GET", URL: "http://127.0.0.1/recovered",
		State: models.TxComplete, CaptureMode: proxy.CaptureModeMITM,
		Fidelity: proxy.FidelityDecodedWire,
		Request:  models.HTTPMessage{BodyStorage: "omitted"},
		Response: models.HTTPMessage{BodyStorage: "omitted"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Finalize(capture.StateRecoverable); err != nil {
		t.Fatal(err)
	}

	evidence, err := Build(st.Path())
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Stats.Observed != 1 ||
		evidence.Stats.Captured != 1 ||
		evidence.Stats.Persisted != 1 {
		t.Fatalf("recovery fallback stats=%+v", evidence.Stats)
	}
	if evidence.Redaction.Known || evidence.Redaction.Applied {
		t.Fatalf("missing legacy redaction checkpoint must remain unknown: %+v", evidence.Redaction)
	}
}
