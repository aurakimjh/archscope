package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func TestT631GenericBridgeProvidersJobsComparisonAndReopen(t *testing.T) {
	svc := evidenceBridge(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fixture, _ := catalog.Contracts.ReadFile("contracts/server.log")
	path := filepath.Join(t.TempDir(), "server.log")
	os.WriteFile(path, fixture, 0600)
	handles := []string{}
	for _, options := range []string{`{}`, `{"max_lines":2}`} {
		job, e := svc.AnalyzeEvidence(EvidenceImportRequest{Path: path, Importer: "tomcat", OptionsJSON: options})
		if e != nil {
			t.Fatal(e)
		}
		job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
		if e != nil || job.State != "completed" {
			t.Fatal(job, e)
		}
		handles = append(handles, job.ResultHandle)
	}
	caps, e := svc.DescribeEvidenceResult(ctx, handles[0])
	if e != nil || caps.IdentityBasis != "declared" || !caps.Compatibility.Allowed {
		t.Fatal(caps, e)
	}
	available := map[string]bool{}
	for _, p := range caps.Providers {
		available[p.ID] = p.Availability == "available"
	}
	if !available["server:events"] || !available["compare:/summary/total_events"] {
		t.Fatal(available)
	}
	page, e := svc.QueryEvidenceProvider(ctx, handles[0], "server:events", 0, 1)
	if e != nil || len(page.Rows) != 1 || page.Next != 1 || page.ResultSHA256 != caps.ResultSHA256 {
		t.Fatal(page, e)
	}
	refused, e := svc.CompareEvidenceProvider(ctx, handles[0], handles[1], "compare:/summary/total_events")
	if e != nil || refused.Allowed || refused.Delta != nil || refused.Code != resultcontract.ComparisonSelectionMismatch {
		t.Fatal(refused, e)
	}
	equal, e := svc.CompareEvidenceProvider(ctx, handles[0], handles[0], "compare:/summary/total_events")
	if e != nil || !equal.Allowed || equal.Delta.Value == nil || *equal.Delta.Value != "0" || equal.Delta.Comparison.IdentityBasis != "declared" || equal.Delta.Comparison.Baseline.Handle != handles[0] {
		t.Fatal(equal, e)
	}
	// Unknown result snapshots remain viewable and exportable as opaque bytes.
	unknown := models.New("future", "synthetic")
	unknown.Summary["exact"] = json.Number("9007199254740993")
	unknown.Metadata.Extra["capabilities"] = []string{"compare", "timeline"}
	raw, _ := json.Marshal(unknown)
	saved := filepath.Join(t.TempDir(), "future.json")
	os.WriteFile(saved, raw, 0600)
	view, e := svc.OpenEvidenceResult(ctx, saved)
	if e != nil {
		t.Fatal(e)
	}
	fallback, e := svc.DescribeEvidenceResult(ctx, view.Handle)
	if e != nil || fallback.Compatibility.Allowed {
		t.Fatal(fallback, e)
	}
	exact, e := svc.QueryEvidenceProvider(ctx, view.Handle, "generic:summary", 0, 25)
	if e != nil || exact.Rows[0][1].Text != "9007199254740993" {
		t.Fatal(exact, e)
	}
	exported, e := svc.GetEvidenceResultJSON(view.Handle)
	if e != nil || exported != string(raw) {
		t.Fatal("display changed result", e)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, e = svc.OpenEvidenceResult(canceled, saved); !errors.Is(e, context.Canceled) {
		t.Fatal("cancellable load", e)
	}
	if _, e = svc.QueryEvidenceProvider(canceled, view.Handle, "generic:summary", 0, 25); !errors.Is(e, context.Canceled) {
		t.Fatal("cancellable query", e)
	}
	if _, e = svc.DescribeEvidenceResult(ctx, "result-00000000000000000000000000000000"); e == nil {
		t.Fatal("stale handle accepted")
	}
	// A separately saved snapshot is the recovery path; no shared deletion API.
	reopened, e := svc.OpenEvidenceResult(ctx, saved)
	if e != nil || reopened.Handle != view.Handle {
		t.Fatal(reopened, e)
	}
}
