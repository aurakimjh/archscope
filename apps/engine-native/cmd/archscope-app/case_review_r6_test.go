package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestR602WailsFullCacheSavedCaseReopenCleanupAndFreshImport(t *testing.T) {
	svc := evidenceBridge(t, nil)
	store, e := cases.Open(filepath.Join(t.TempDir(), "cases"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	svc.caseService = store
	m, e := store.Create("synthetic saved case")
	if e != nil {
		t.Fatal(e)
	}
	r := models.New("future", "fixture")
	r.Summary["exact"] = json.Number("9007199254740993")
	raw, _ := json.Marshal(r)
	m, e = store.SaveResult(cases.SaveResult{CaseID: m.ID, ExpectedRevision: m.Revision}, raw)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 1024; i++ {
		snapshot := models.New("synthetic-cache", "fixture")
		snapshot.Summary["index"] = i
		b, _ := json.Marshal(snapshot)
		if _, e = svc.evidenceService.LoadSnapshot(context.Background(), b); e != nil {
			t.Fatal(i, e)
		}
	}
	text, e := svc.ReopenCaseResult(m.ID, m.Results[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	var owned struct {
		View  analysis.ResultView  `json:"view"`
		Lease analysis.ResultLease `json:"lease"`
	}
	if e = json.Unmarshal([]byte(text), &owned); e != nil {
		t.Fatal(e)
	}
	if body, e := svc.GetEvidenceResultJSON(owned.View.Handle); e != nil || body != string(raw) {
		t.Fatal("reopen lost exact bytes", e)
	}
	report, e := svc.CleanupEvidenceCache()
	if e != nil || report.Removed != 1024 || report.RetainedOwned != 1 {
		t.Fatal(report, e)
	}
	if body, e := svc.GetEvidenceResultJSON(owned.View.Handle); e != nil || body != string(raw) {
		t.Fatal("cleanup removed owned snapshot", e)
	}
	fixture, _ := catalog.Contracts.ReadFile("contracts/server.log")
	p := filepath.Join(t.TempDir(), "server.log")
	os.WriteFile(p, fixture, 0600)
	job, e := svc.ImportEvidence(EvidenceImportRequest{Path: p, Importer: "tomcat", OptionsJSON: `{}`})
	if e != nil {
		t.Fatal(e)
	}
	job, e = svc.evidenceService.Wait(context.Background(), job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	if e = svc.ReleaseEvidenceLease(owned.Lease.ID); e != nil {
		t.Fatal(e)
	}
	if body, e := store.ExportSnapshot(m.ID, m.Results[0].ID); e != nil || string(body) != string(raw) {
		t.Fatal("saved authority changed", e)
	}
}
