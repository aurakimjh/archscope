package analysis_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
)

func TestT635VerifiedDurableBaselineReimportPolicyAndTamper(t *testing.T) {
	service, _, _ := setup(t, nil)
	store, e := cases.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	manifest, e := store.Create("baseline")
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	save := func(name, raw, options string, forge bool) (string, string, string) {
		p := filepath.Join(t.TempDir(), name+".raw")
		os.WriteFile(p, []byte(raw), 0600)
		manifest, e = store.Apply(ctx, cases.Edit{CaseID: manifest.ID, ExpectedRevision: manifest.Revision, Operation: "source", Path: p})
		if e != nil {
			t.Fatal(e)
		}
		source := manifest.Sources[len(manifest.Sources)-1]
		model, e := parser.ParseSourceBytes(ctx, []byte(raw), parser.SourceOptions{Format: "pyspy-raw", Producer: "py-spy", ProducerVersion: "0.4.1", CaptureOptions: json.RawMessage(options)})
		if e != nil {
			t.Fatal(e)
		}
		if forge {
			model.Profiles[0].Samples[0].Weight = "99"
		}
		result, e := analyzer.BuildSet(ctx, model, m.Selection{ProfileIDs: []string{"profile-0"}, MetricID: "samples"}, name)
		if e != nil {
			t.Fatal(e)
		}
		body, _ := json.Marshal(result)
		manifest, e = store.SaveResult(cases.SaveResult{CaseID: manifest.ID, ExpectedRevision: manifest.Revision, SourceIDs: []string{source.ID}}, body)
		if e != nil {
			t.Fatal(e)
		}
		return manifest.Results[len(manifest.Results)-1].ID, source.ID, p
	}
	left, sourceLeft, _ := save("left", "root;leaf 0.25\nroot 0\n", `{"output_path":"a","rate":100}`, false)
	right, sourceRight, rightPath := save("right", "root;leaf 0.5\nroot 0\n", `{"output_path":"b","rate":100}`, false)
	req := analysis.BaselineRequest{CaseID: manifest.ID, Revision: manifest.Revision, ResultID: left, SourceID: sourceLeft}
	manifest, e = service.PinCaseBaseline(ctx, store, req)
	if e != nil {
		t.Fatal(e)
	}
	pin := manifest.Baselines[0]
	pinned := string(pin.Conditions)
	compare := analysis.BaselineComparisonRequest{CaseID: manifest.ID, Revision: manifest.Revision, BaselineID: pin.ID, TargetResultID: right, TargetSourceID: sourceRight, Limit: 1}
	diff, e := service.CompareCaseBaseline(ctx, store, compare)
	if e != nil || diff.SourceVerification != "verified_by_reimport" || diff.Difference.IdentityBasis != "declared" || len(diff.Difference.IncidentalDifferences) != 1 || diff.Next == "" {
		t.Fatal(diff, e)
	}
	compare.Cursor = diff.Next
	next, e := service.CompareCaseBaseline(ctx, store, compare)
	if e != nil || next.Difference.Rows[0].ID == diff.Difference.Rows[0].ID {
		t.Fatal(next, e)
	}
	compare.Cursor = ""
	forged, forgedSource, _ := save("forged", "root 1\n", `{"rate":100}`, true)
	if _, e = service.PinCaseBaseline(ctx, store, analysis.BaselineRequest{CaseID: manifest.ID, Revision: manifest.Revision, ResultID: forged, SourceID: forgedSource}); e == nil || !strings.Contains(e.Error(), "BASELINE_REIMPORT_MISMATCH") {
		t.Fatal("edited model gained verification", e)
	}
	if string(manifest.Baselines[0].Conditions) != pinned || manifest.Baselines[0].ResultSHA256 != pin.ResultSHA256 {
		t.Fatal("old baseline followed new result")
	}
	// A forged arbitrary baseline condition still has to replay originals on reuse.
	var claim map[string]any
	json.Unmarshal(pin.Conditions, &claim)
	claim["model_sha256"] = strings.Repeat("a", 64)
	condition, _ := json.Marshal(claim)
	manifest, e = store.Apply(ctx, cases.Edit{CaseID: manifest.ID, ExpectedRevision: manifest.Revision, Operation: "baseline", ResultID: left, Conditions: condition})
	if e != nil {
		t.Fatal(e)
	}
	compare.Revision = manifest.Revision
	compare.BaselineID = manifest.Baselines[1].ID
	if _, e = service.CompareCaseBaseline(ctx, store, compare); e == nil || !strings.Contains(e.Error(), "BASELINE_BINDING_CHANGED") {
		t.Fatal("trusted forged claim", e)
	}
	originalSnapshot, err := store.ExportSnapshot(manifest.ID, left)
	if err != nil {
		t.Fatal(err)
	}
	manifest, e = store.SaveResult(cases.SaveResult{CaseID: manifest.ID, ExpectedRevision: manifest.Revision, SourceIDs: []string{sourceLeft}, OptionsHash: strings.Repeat("0", 64)}, originalSnapshot)
	if e != nil {
		t.Fatal(e)
	}
	editedOptions := manifest.Results[len(manifest.Results)-1].ID
	if _, e = service.PinCaseBaseline(ctx, store, analysis.BaselineRequest{CaseID: manifest.ID, Revision: manifest.Revision, ResultID: editedOptions, SourceID: sourceLeft}); e == nil || !strings.Contains(e.Error(), "BASELINE_OPTIONS_BINDING_MISMATCH") {
		t.Fatal("forged saved options hash", e)
	}
	compare.Revision = manifest.Revision
	compare.BaselineID = pin.ID
	os.WriteFile(rightPath, []byte("changed"), 0600)
	if _, e = service.CompareCaseBaseline(ctx, store, compare); e == nil {
		t.Fatal("reused missing original")
	}
	ctxCancel, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = service.PinCaseBaseline(ctxCancel, store, analysis.BaselineRequest{CaseID: manifest.ID, Revision: manifest.Revision, ResultID: left, SourceID: sourceLeft}); e == nil {
		t.Fatal("ignored canceled pin")
	}
}
func TestT635ProfileQueryMaterializesNewImmutableSelection(t *testing.T) {
	service, _, _ := setup(t, nil)
	job := importModel(t, service, "duration")
	ctx := context.Background()
	inv, e := service.ProfileInventory(ctx, job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	selection := inv.Selection
	selection.ProfileIDs = []string{"worker-a"}
	page, e := service.QueryProfile(ctx, job.ResultHandle, inv.ResultSHA256, m.Query{Selection: selection, View: "top", Limit: 10})
	if e != nil || page.Denominator != "0.25" {
		t.Fatal(page, e)
	}
	view, lease, e := service.MaterializeProfileSelection(ctx, job.ResultHandle, inv.ResultSHA256, selection)
	if e != nil {
		t.Fatal(e)
	}
	defer service.ReleaseResultLease(lease.ID)
	if view.Handle == job.ResultHandle {
		t.Fatal("overwrote selected snapshot")
	}
	after, e := service.ProfileInventory(ctx, job.ResultHandle)
	if e != nil || len(after.Selection.ProfileIDs) != 2 {
		t.Fatal(after, e)
	}
	if _, e = service.QueryProfile(ctx, job.ResultHandle, strings.Repeat("a", 64), m.Query{Selection: selection, View: "tree", Limit: 10}); e == nil {
		t.Fatal("stale result accepted")
	}
}
