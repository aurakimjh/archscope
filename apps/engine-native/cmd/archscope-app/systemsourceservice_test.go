package main

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/systemtrace"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestT637BridgeInspectNormalizeOwnedProfileAndRefusals(t *testing.T) {
	ctx := context.Background()
	t.Setenv("ARCHSCOPE_STATE_ROOT", t.TempDir())
	svc := evidenceBridge(t, nil)
	original, _ := catalog.Contracts.ReadFile("contracts/system-evidence-perf-script.txt")
	capture, _ := catalog.Contracts.ReadFile("contracts/system-evidence-perf-capture.json")
	var c m.Capture
	json.Unmarshal(capture, &c)
	options, _ := json.Marshal(map[string]any{"format": "perf-script", "capture": c})
	dir := t.TempDir()
	path := filepath.Join(dir, "perf.txt")
	os.WriteFile(path, original, 0600)
	raw, e := svc.InspectSystemSource(ctx, path, string(options))
	if e != nil {
		t.Fatal(e)
	}
	var inventory a.SourceInventory
	json.Unmarshal([]byte(raw), &inventory)
	if inventory.Records != "3" || inventory.ToolVersion != "6.1.190" {
		t.Fatal(inventory)
	}
	output := filepath.Join(dir, "model.json")
	if _, e = svc.NormalizeSystemSource(ctx, path, string(options), strings.Repeat("0", 64), output); e == nil {
		t.Fatal("stale source accepted")
	}
	if _, e = os.Stat(output); !os.IsNotExist(e) {
		t.Fatal("failed normalization published output")
	}
	if _, e = svc.NormalizeSystemSource(ctx, path, string(options), inventory.SourceSHA256, output); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.NormalizeSystemSource(ctx, path, string(options), inventory.SourceSHA256, output); e == nil {
		t.Fatal("output overwrite accepted")
	}
	job, e := svc.AnalyzeEvidence(EvidenceImportRequest{Path: output, Importer: "system.evidence-json", OptionsJSON: "{}"})
	if e != nil {
		t.Fatal(e)
	}
	job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	raw, e = svc.DescribeSystemEvidence(ctx, job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	var saved struct {
		ResultSHA string `json:"result_sha256"`
	}
	json.Unmarshal([]byte(raw), &saved)
	if _, e = svc.MaterializeSystemProfile(ctx, job.ResultHandle, "stale", "period/cpu-clock:HG", "[]"); e == nil {
		t.Fatal("stale result projected")
	}
	raw, e = svc.MaterializeSystemProfile(ctx, job.ResultHandle, saved.ResultSHA, "period/cpu-clock:HG", "[]")
	if e != nil {
		t.Fatal(e)
	}
	var owned struct {
		View  analysis.ResultView  `json:"view"`
		Lease analysis.ResultLease `json:"lease"`
	}
	json.Unmarshal([]byte(raw), &owned)
	tree, e := svc.evidenceService.ProfileTree(ctx, owned.View.Handle)
	if e != nil || tree.Root.Inclusive != "6000000" || tree.Metric.Unit != "nanoseconds" {
		t.Fatal(tree, e)
	}
	if e = svc.evidenceService.ReleaseResultLease(owned.Lease.ID); e != nil {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = svc.NormalizeSystemSource(canceled, path, string(options), inventory.SourceSHA256, filepath.Join(dir, "canceled.json")); e == nil {
		t.Fatal("cancelled source published")
	}
	store, e := cases.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	svc.caseService = store
	manifest, e := store.Create("system fixture")
	if e != nil {
		t.Fatal(e)
	}
	manifest, e = store.Apply(ctx, cases.Edit{CaseID: manifest.ID, ExpectedRevision: manifest.Revision, Operation: "source", Path: path})
	if e != nil {
		t.Fatal(e)
	}
	request, _ := json.Marshal(cases.SaveResult{CaseID: manifest.ID, ExpectedRevision: manifest.Revision, SourceIDs: []string{manifest.Sources[0].ID}})
	value, e := svc.SaveCaseResult(job.ResultHandle, string(request))
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal([]byte(value), &manifest)
	modelBytes, _ := os.ReadFile(output)
	model, e := m.Decode(ctx, modelBytes)
	if e != nil {
		t.Fatal(e)
	}
	eventID := model.Records[0].ID
	opened, e := svc.OpenSystemEvidence(ctx, manifest.ID, manifest.Revision, manifest.Results[0].ID, saved.ResultSHA, manifest.Sources[0].ID, inventory.SourceSHA256, eventID)
	if e != nil || !strings.Contains(opened, `"number":"1"`) {
		t.Fatal(opened, e)
	}
	if _, e = svc.OpenSystemEvidence(ctx, manifest.ID, "stale", manifest.Results[0].ID, saved.ResultSHA, manifest.Sources[0].ID, inventory.SourceSHA256, eventID); e == nil {
		t.Fatal("stale case opened")
	}
	if _, e = svc.OpenSystemEvidence(ctx, manifest.ID, manifest.Revision, manifest.Results[0].ID, saved.ResultSHA, manifest.Sources[0].ID, inventory.SourceSHA256, "invented"); e == nil {
		t.Fatal("invented reference opened")
	}
	model.Records[0].Line = 999
	model.Records[0].ID = m.RecordID(model.SourceSHA256, model.Records[0])
	forged, e := a.Build(ctx, model, "forged coordinates")
	if e != nil {
		t.Fatal(e)
	}
	forgedBody, _ := json.Marshal(forged)
	manifest, e = store.SaveResult(cases.SaveResult{CaseID: manifest.ID, ExpectedRevision: manifest.Revision, SourceIDs: []string{manifest.Sources[0].ID}}, forgedBody)
	if e != nil {
		t.Fatal(e)
	}
	forgedRef := manifest.Results[len(manifest.Results)-1]
	if _, e = svc.OpenSystemEvidence(ctx, manifest.ID, manifest.Revision, forgedRef.ID, forgedRef.SHA256, manifest.Sources[0].ID, inventory.SourceSHA256, model.Records[0].ID); e == nil || !strings.Contains(e.Error(), "SYSTEM_ORIGINAL_REFERENCE_MISMATCH") {
		t.Fatal("forged coordinate trusted", e)
	}
	body, _ := os.ReadFile(output)
	if len(body) == 0 {
		t.Fatal("original model removed")
	}
}
