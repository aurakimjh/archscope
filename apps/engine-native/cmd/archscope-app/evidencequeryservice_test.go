package main

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestT635WailsSourceSavedQueryBaselineSearchAndLinks(t *testing.T) {
	svc := evidenceBridge(t, nil)
	store, e := cases.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	svc.caseService = store
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	original := filepath.Join(t.TempDir(), "original.raw")
	os.WriteFile(original, []byte("root;leaf 0.25\nroot 0\n"), 0600)
	opts := `{"format":"pyspy-raw","producer":"py-spy","producer_version":"0.4.1","capture_options":{"rate":100}}`
	inventory, e := svc.InspectProfileSource(ctx, original, opts)
	if e != nil {
		t.Fatal(e)
	}
	var source parser.SourceInventory
	json.Unmarshal([]byte(inventory), &source)
	normalized := filepath.Join(t.TempDir(), "model.json")
	if _, e = svc.NormalizeProfileSource(ctx, original, opts, source.SourceSHA256, normalized); e != nil {
		t.Fatal(e)
	}
	job, e := svc.AnalyzeEvidence(EvidenceImportRequest{Path: normalized, Importer: "profile.source-set-json", OptionsJSON: `{"profile_ids":["profile-0"],"metric_id":"samples"}`})
	if e != nil {
		t.Fatal(e)
	}
	job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	inv, e := svc.DescribeProfileQuery(ctx, job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	var description struct {
		SHA string `json:"result_sha256"`
	}
	json.Unmarshal([]byte(inv), &description)
	q := `{"selection":{"profile_ids":["profile-0"],"metric_id":"samples"},"view":"evidence","limit":1}`
	page, e := svc.QueryProfileEvidence(ctx, job.ResultHandle, description.SHA, q)
	if e != nil || !strings.Contains(page, `"denominator":"0.25"`) || !strings.Contains(page, `"source_ref":"line:1"`) {
		t.Fatal(page, e)
	}
	if _, e = svc.QueryProfileEvidence(ctx, job.ResultHandle, description.SHA, `{"unknown":1}`); e == nil {
		t.Fatal("unknown query field")
	}
	manifest, e := store.Create("query")
	if e != nil {
		t.Fatal(e)
	}
	manifest, e = store.Apply(ctx, cases.Edit{CaseID: manifest.ID, ExpectedRevision: manifest.Revision, Operation: "source", Path: original})
	if e != nil {
		t.Fatal(e)
	}
	req, _ := json.Marshal(cases.SaveResult{CaseID: manifest.ID, ExpectedRevision: manifest.Revision, SourceIDs: []string{manifest.Sources[0].ID}})
	saved, e := svc.SaveCaseResult(job.ResultHandle, string(req))
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal([]byte(saved), &manifest)
	pinRequest := map[string]any{"case_id": manifest.ID, "revision": manifest.Revision, "result_id": manifest.Results[0].ID, "source_id": manifest.Sources[0].ID}
	raw, _ := json.Marshal(pinRequest)
	pinned, e := svc.PinVerifiedCaseBaseline(ctx, string(raw))
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal([]byte(pinned), &manifest)
	raw, _ = json.Marshal(map[string]any{"case_id": manifest.ID, "revision": manifest.Revision, "baseline_id": manifest.Baselines[0].ID, "target_result_id": manifest.Results[0].ID, "target_source_id": manifest.Sources[0].ID})
	compared, e := svc.CompareVerifiedCaseBaseline(ctx, string(raw))
	if e != nil || !strings.Contains(compared, `"source_verification":"verified_by_reimport"`) || !strings.Contains(compared, `"identity_basis":"declared"`) {
		t.Fatal(compared, e)
	}
	raw, _ = json.Marshal(map[string]any{"case_id": manifest.ID, "revision": manifest.Revision, "text": "leaf", "limit": 25})
	searched, e := svc.SearchCaseEvidence(ctx, string(raw))
	if e != nil || !strings.Contains(searched, `"number":"1"`) {
		t.Fatal(searched, e)
	}
	linked, err := svc.OpenProfileEvidence(ctx, manifest.ID, manifest.Revision, manifest.Results[0].ID, description.SHA, manifest.Sources[0].ID, source.SourceSHA256, "line:1")
	if err != nil || !strings.Contains(linked, `"number":"1"`) {
		t.Fatal(linked, err)
	}
	if _, err = svc.OpenProfileEvidence(ctx, manifest.ID, manifest.Revision, manifest.Results[0].ID, description.SHA, manifest.Sources[0].ID, source.SourceSHA256, "line:999"); err == nil {
		t.Fatal("invented profile reference")
	}
	opened, e := svc.OpenCaseEvidence(ctx, manifest.ID, manifest.Revision, manifest.Sources[0].ID, source.SourceSHA256, "1")
	if e != nil || !strings.Contains(opened, `"indexed_lines":"2"`) {
		t.Fatal(opened, e)
	}
}
