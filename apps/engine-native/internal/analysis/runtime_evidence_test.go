package analysis_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/runtimeevidence"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"os"
	"path/filepath"
	"testing"
)

func TestT641RuntimeCommonConsumersOwnershipAndCache(t *testing.T) {
	s, _, _ := setup(t, nil)
	ctx := context.Background()
	raw, e := catalog.Contracts.ReadFile("contracts/runtime-evidence-set.json")
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "runtime.json")
	os.WriteFile(path, raw, 0600)
	job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "runtime.evidence-json", Options: []byte(`{}`)}))
	if job.State != "completed" {
		t.Fatal(job)
	}
	inventory, e := s.RuntimeInventory(ctx, job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	first, e := s.QueryRuntime(ctx, job.ResultHandle, inventory.ResultSHA, a.Query{Kind: "allocation", Limit: 25})
	if e != nil || first.Total == 0 {
		t.Fatal(first, e)
	}
	inventory.Entities[0].Name = "poison"
	fresh, e := s.RuntimeInventory(ctx, job.ResultHandle)
	if e != nil || fresh.Entities[0].Name == "poison" {
		t.Fatal("mutable cache leaked", e)
	}
	for i := 0; i < 3; i++ {
		if _, e = s.QueryRuntime(ctx, job.ResultHandle, fresh.ResultSHA, a.Query{Kind: "frames", Limit: 1}); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = s.QueryRuntime(ctx, job.ResultHandle, "bad", a.Query{Kind: "frames", Limit: 1}); e == nil {
		t.Fatal("stale digest")
	}
	var row struct {
		Entity string `json:"entity_id"`
		Ref    struct {
			Source string `json:"source_id"`
		} `json:"ref"`
	}
	json.Unmarshal(first.Rows[0], &row)
	view, lease, e := s.MaterializeRuntimeAllocation(ctx, job.ResultHandle, fresh.ResultSHA, row.Entity, row.Ref.Source, "live_bytes")
	if e != nil || lease.ID == "" || view.Type != "profile_evidence" {
		t.Fatal(view, lease, e)
	}
	if e = s.ReleaseResultLease(lease.ID); e != nil {
		t.Fatal(e)
	}
	snapshot, _, e := s.Read(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	for _, format := range []string{"json", "csv", "html", "report-pack"} {
		b, e := s.Export(job.ResultHandle, format)
		if e != nil || len(b) == 0 {
			t.Fatal(format, e)
		}
		if format == "json" && !bytes.Equal(snapshot, b) {
			t.Fatal("bytes changed")
		}
	}
	store, e := cases.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	c, e := store.Create("runtime synthetic")
	if e != nil {
		t.Fatal(e)
	}
	c, e = store.SaveResult(cases.SaveResult{CaseID: c.ID, ExpectedRevision: c.Revision}, snapshot)
	if e != nil {
		t.Fatal(e)
	}
	restored, e := store.ExportSnapshot(c.ID, c.Results[0].ID)
	if e != nil || !bytes.Equal(restored, snapshot) {
		t.Fatal(e)
	}
	if _, e = s.LoadSnapshot(ctx, restored); e != nil {
		t.Fatal(e)
	}
}
