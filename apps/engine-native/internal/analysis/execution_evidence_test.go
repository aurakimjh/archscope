package analysis_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/execution"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestT642CommonConsumersCacheOwnershipCaseAndBaseline(t *testing.T) {
	s, _, _ := setup(t, nil)
	ctx := context.Background()
	raw, err := catalog.Contracts.ReadFile("contracts/execution-evidence-set.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "execution.json")
	os.WriteFile(path, raw, 0600)
	job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "execution.evidence-json", Options: []byte(`{}`)}))
	if job.State != "completed" {
		t.Fatal(job)
	}
	inventory, err := s.ExecutionInventory(ctx, job.ResultHandle)
	if err != nil {
		t.Fatal(err)
	}
	inventory.Context.Scenario = "poison"
	fresh, err := s.ExecutionInventory(ctx, job.ResultHandle)
	if err != nil || fresh.Context.Scenario == "poison" {
		t.Fatal("mutable DTO cache escape", err)
	}
	page, err := s.QueryExecution(ctx, job.ResultHandle, fresh.ResultSHA, a.Query{Kind: "metrics", Limit: 1})
	if err != nil || len(page.Rows) != 1 {
		t.Fatal(page, err)
	}
	snapshot, _, err := s.Read(job.ResultHandle)
	if err != nil {
		t.Fatal(err)
	}
	view, lease, err := s.LoadOwnedSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ReleaseResultLease(lease.ID)
	cmp, err := s.CompareExecution(ctx, job.ResultHandle, view.Handle, fresh.ResultSHA, fresh.ResultSHA, a.CompareQuery{Limit: 25})
	if err != nil || cmp.Status != "compatible_declarations" {
		t.Fatal(cmp, err)
	}
	for i := range cmp.Rows {
		if cmp.Rows[i].Baseline != nil {
			*cmp.Rows[i].Baseline = "poison"
			break
		}
	}
	cmp, err = s.CompareExecution(ctx, job.ResultHandle, view.Handle, fresh.ResultSHA, fresh.ResultSHA, a.CompareQuery{Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range cmp.Rows {
		if row.Baseline != nil && *row.Baseline == "poison" {
			t.Fatal("comparison pointer poisoned cache")
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.CompareExecution(ctx, job.ResultHandle, view.Handle, fresh.ResultSHA, fresh.ResultSHA, a.CompareQuery{Limit: 1}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for _, format := range []string{"json", "csv", "html", "report-pack"} {
		body, err := s.Export(job.ResultHandle, format)
		if err != nil || len(body) == 0 {
			t.Fatal(format, err)
		}
		if format == "json" && !bytes.Equal(body, snapshot) {
			t.Fatal("saved bytes changed")
		}
		if bytes.Contains(body, []byte("SYNTHETIC_BODY_NOT_RETAINED")) {
			t.Fatal("raw body exported")
		}
	}
	store, err := cases.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	c, err := store.Create("synthetic execution")
	if err != nil {
		t.Fatal(err)
	}
	c, err = store.SaveResult(cases.SaveResult{CaseID: c.ID, ExpectedRevision: c.Revision}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := store.ExportSnapshot(c.ID, c.Results[0].ID)
	if err != nil || !bytes.Equal(restored, snapshot) {
		t.Fatal(err)
	}
	if _, err = s.LoadSnapshot(ctx, restored); err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	json.Unmarshal(page.Rows[0], &object)
	if object["unit"] == nil {
		t.Fatal("unit missing")
	}
	if _, err = s.QueryExecution(ctx, job.ResultHandle, strings.Repeat("f", 64), a.Query{Kind: "metrics", Limit: 1}); err == nil {
		t.Fatal("stale revision accepted")
	}
}
