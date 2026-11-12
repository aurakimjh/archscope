package analysis_test

import (
	"bytes"
	"context"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/traceimport"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"os"
	"path/filepath"
	"testing"
)

func TestT640GenericProvidersExportsAndCaseRoundTrip(t *testing.T) {
	s, _, _ := setup(t, nil)
	ctx := context.Background()
	raw, e := catalog.Contracts.ReadFile("contracts/llm-trace-set.json")
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "sanitized.json")
	os.WriteFile(path, raw, 0600)
	job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "trace.llm-json", Options: []byte(`{}`)}))
	if job.State != "completed" {
		t.Fatal(job)
	}
	inventory, e := s.LLMInventory(ctx, job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	page, e := s.QueryLLMTrace(ctx, job.ResultHandle, inventory.ResultSHA, a.LLMQuery{Kind: "attempts", Limit: 25})
	if e != nil || page.Total != 1 {
		t.Fatal(page, e)
	}
	snapshot, _, e := s.Read(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	for _, format := range []string{"json", "csv", "html", "report-pack"} {
		body, e := s.Export(job.ResultHandle, format)
		if e != nil || len(body) == 0 {
			t.Fatal(format, e)
		}
		if format == "json" && !bytes.Equal(snapshot, body) {
			t.Fatal("snapshot changed")
		}
		if bytes.Contains(body, []byte("SYNTHETIC_BODY_MARKER")) {
			t.Fatal("body export", format)
		}
	}
	caps, e := s.DescribeResult(ctx, job.ResultHandle)
	if e != nil || caps.Type != "trace_import" {
		t.Fatal(caps, e)
	}
	store, e := cases.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	c, e := store.Create("LLM synthetic")
	if e != nil {
		t.Fatal(e)
	}
	c, e = store.SaveResult(cases.SaveResult{CaseID: c.ID, ExpectedRevision: c.Revision}, snapshot)
	if e != nil {
		t.Fatal(e)
	}
	restored, e := store.ExportSnapshot(c.ID, c.Results[0].ID)
	if e != nil || !bytes.Equal(restored, snapshot) {
		t.Fatal("case bytes changed", e)
	}
	loaded, e := s.LoadSnapshot(ctx, restored)
	if e != nil || !loaded.Compatibility.Allowed {
		t.Fatal(loaded, e)
	}
}
