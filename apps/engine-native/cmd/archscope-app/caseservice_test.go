package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
)

func TestT630WailsSavedCaseExactReopenAfterCacheLoss(t *testing.T) {
	svc := evidenceBridge(t, nil)
	store, e := cases.Open(filepath.Join(t.TempDir(), "case-store"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	svc.caseService = store
	body, _ := catalog.Contracts.ReadFile("contracts/profile-set.json")
	input := filepath.Join(t.TempDir(), "profile.json")
	os.WriteFile(input, body, 0600)
	job, e := svc.ImportEvidence(EvidenceImportRequest{Path: input, Importer: "profile-set", OptionsJSON: `{"profile_ids":["worker-a","worker-b"],"metric_id":"memory"}`})
	if e != nil {
		t.Fatal(e)
	}
	job, e = svc.evidenceService.Wait(context.Background(), job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	raw, e := svc.GetEvidenceResultJSON(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	text, e := svc.CreateCase("synthetic bridge")
	if e != nil {
		t.Fatal(e)
	}
	var m cases.Manifest
	json.Unmarshal([]byte(text), &m)
	request, _ := json.Marshal(cases.SaveResult{CaseID: m.ID, ExpectedRevision: m.Revision, SourceIDs: []string{}, Selection: json.RawMessage(`{"none":null,"zero":0,"large":9007199254740993}`)})
	saved, e := svc.SaveCaseResult(job.ResultHandle, string(request))
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal([]byte(saved), &m)
	if len(m.Results) != 1 {
		t.Fatal(m)
	}
	// No handles are saved in case references. Simulate eviction and a fresh
	// evidence service without changing the durable case store.
	svc.evidenceService.Close()
	c, r, _ := catalog.Builtin()
	fresh, e := analysis.New(filepath.Join(t.TempDir(), "empty-cache"), c, r)
	if e != nil {
		t.Fatal(e)
	}
	defer fresh.Close()
	svc.evidenceService = fresh
	reopened, e := svc.ReopenCaseResult(m.ID, m.Results[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	var owned struct {
		View  analysis.ResultView  `json:"view"`
		Lease analysis.ResultLease `json:"lease"`
	}
	json.Unmarshal([]byte(reopened), &owned)
	after, e := svc.GetEvidenceResultJSON(owned.View.Handle)
	if e != nil || after != raw {
		t.Fatal("bridge snapshot changed", e)
	}
	tree, e := svc.GetProfileSetTree(context.Background(), owned.View.Handle)
	if e != nil || !strings.Contains(tree, `"inclusive":"9007199254740993"`) {
		t.Fatal(tree, e)
	}
	second, e := svc.AcquireEvidenceResult(owned.View.Handle)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.ReleaseEvidenceLease(owned.Lease.ID); e != nil {
		t.Fatal(e)
	}
	if e = fresh.Release(owned.View.Handle); e == nil {
		t.Fatal("another caller's snapshot released")
	}
	if e = svc.ReleaseEvidenceLease(second.ID); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "saved.json")
	if e = svc.ExportCaseResult(m.ID, m.Results[0].ID, out); e != nil {
		t.Fatal(e)
	}
	outBody, _ := os.ReadFile(out)
	if !bytes.Equal(outBody, []byte(raw)) {
		t.Fatal("export changed exact source")
	}
	if _, e = svc.RebuildCaseIndex(); e != nil {
		t.Fatal(e)
	}
	if e = svc.CleanupCaseCache(); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.GetCase(m.ID); e != nil {
		t.Fatal(e)
	}
}
