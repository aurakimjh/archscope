package main

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/traceimport"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestT640BridgeSourceDigestOpaqueQueryAndCancellation(t *testing.T) {
	ctx := context.Background()
	t.Setenv("ARCHSCOPE_STATE_ROOT", t.TempDir())
	svc := evidenceBridge(t, nil)
	dir := t.TempDir()
	raw, _ := catalog.Contracts.ReadFile("contracts/llm-trace-sdk-python-1.38.0.otlp.json")
	opts, _ := catalog.Contracts.ReadFile("contracts/llm-trace-neutral-options.json")
	path := filepath.Join(dir, "source.json")
	os.WriteFile(path, raw, 0600)
	value, e := svc.InspectLLMTraceSource(ctx, path, string(opts))
	if e != nil {
		t.Fatal(e)
	}
	var inv analysis.LLMInventory
	json.Unmarshal([]byte(value), &inv)
	if inv.Spans != 6 || inv.SourceSHA == "" {
		t.Fatal(inv)
	}
	output := filepath.Join(dir, "model.json")
	if _, e = svc.NormalizeLLMTraceSource(ctx, path, string(opts), strings.Repeat("0", 64), output); e == nil {
		t.Fatal("stale original accepted")
	}
	if _, e = os.Stat(output); !os.IsNotExist(e) {
		t.Fatal("failed output published")
	}
	if _, e = svc.NormalizeLLMTraceSource(ctx, path, string(opts), inv.SourceSHA, output); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.NormalizeLLMTraceSource(ctx, path, string(opts), inv.SourceSHA, output); e == nil {
		t.Fatal("overwrite accepted")
	}
	job, e := svc.AnalyzeEvidence(EvidenceImportRequest{Path: output, Importer: "trace.llm-json", OptionsJSON: "{}"})
	if e != nil {
		t.Fatal(e)
	}
	job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	value, e = svc.DescribeLLMTrace(ctx, job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal([]byte(value), &inv)
	if inv.ResultSHA == "" {
		t.Fatal(inv)
	}
	if _, e = svc.QueryLLMTrace(ctx, job.ResultHandle, "stale", `{"kind":"records","limit":1}`); e == nil {
		t.Fatal("stale query accepted")
	}
	value, e = svc.QueryLLMTrace(ctx, job.ResultHandle, inv.ResultSHA, `{"kind":"records","limit":1}`)
	if e != nil {
		t.Fatal(e)
	}
	var page a.LLMPage
	json.Unmarshal([]byte(value), &page)
	if page.Total != 6 || page.Next == "" || len(page.Rows) != 1 {
		t.Fatal(page)
	}
	query, _ := json.Marshal(a.LLMQuery{Kind: "events", Limit: 1, Cursor: page.Next})
	if _, e = svc.QueryLLMTrace(ctx, job.ResultHandle, inv.ResultSHA, string(query)); e == nil {
		t.Fatal("cursor selector crossed")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = svc.QueryLLMTrace(cancelled, job.ResultHandle, inv.ResultSHA, `{"kind":"records","limit":1}`); e == nil {
		t.Fatal("cancel ignored")
	}
	os.WriteFile(path, append(raw, '\n'), 0600)
	if _, e = svc.NormalizeLLMTraceSource(ctx, path, string(opts), inv.SourceSHA, filepath.Join(dir, "changed.json")); e == nil {
		t.Fatal("changed source accepted")
	}
}
