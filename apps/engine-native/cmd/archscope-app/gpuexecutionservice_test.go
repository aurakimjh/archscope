package main

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gputrace"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestT644GPUBridgeSourcePinOwnershipSavedReadAndSVG(t *testing.T) {
	ctx := context.Background()
	t.Setenv("ARCHSCOPE_STATE_ROOT", t.TempDir())
	svc := evidenceBridge(t, nil)
	raw, _ := catalog.Contracts.ReadFile("contracts/gpu-execution-nsight-sanitized.sqlite")
	options, _ := catalog.Contracts.ReadFile("contracts/gpu-execution-nsight-options.json")
	path := filepath.Join(t.TempDir(), "source.sqlite")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	value, err := svc.InspectGPUExecutionSource(ctx, path, string(options))
	if err != nil {
		t.Fatal(err)
	}
	var inv analysis.GPUExecutionInventory
	if err = json.Unmarshal([]byte(value), &inv); err != nil {
		t.Fatal(err)
	}
	if inv.Events != 19 || inv.SourceSHA == "" {
		t.Fatal(inv)
	}
	output := filepath.Join(t.TempDir(), "model.json")
	if _, err = svc.NormalizeGPUExecutionSource(ctx, path, string(options), "stale", output); err == nil {
		t.Fatal("stale source")
	}
	if _, err = os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("failed normalize published")
	}
	if _, err = svc.NormalizeGPUExecutionSource(ctx, path, string(options), inv.SourceSHA, output); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.NormalizeGPUExecutionSource(ctx, path, string(options), inv.SourceSHA, output); err == nil {
		t.Fatal("overwrite")
	}
	job, err := svc.AnalyzeEvidence(EvidenceImportRequest{Path: output, Importer: "trace.gpu-execution-json", OptionsJSON: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	job, err = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
	if err != nil || job.State != "completed" {
		t.Fatal(job, err)
	}
	value, err = svc.DescribeGPUExecution(ctx, job.ResultHandle)
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal([]byte(value), &inv)
	if inv.ResultSHA == "" {
		t.Fatal(inv)
	}
	q, _ := json.Marshal(a.Query{Kind: "events", Source: inv.Sources[0].ID, Limit: 1})
	if _, err = svc.QueryGPUExecution(ctx, job.ResultHandle, "stale", string(q)); err == nil {
		t.Fatal("stale result")
	}
	value, err = svc.QueryGPUExecution(ctx, job.ResultHandle, inv.ResultSHA, string(q))
	if err != nil {
		t.Fatal(err)
	}
	var page a.Page
	json.Unmarshal([]byte(value), &page)
	if page.Total != 19 || len(page.Rows) != 1 || page.Next == "" {
		t.Fatal(page)
	}
	svg := filepath.Join(t.TempDir(), "selected.svg")
	if _, err = svc.ExportGPUExecutionSVG(ctx, job.ResultHandle, inv.ResultSHA, string(q), svg); err != nil {
		t.Fatal(err)
	}
	bytes, _ := os.ReadFile(svg)
	if !strings.Contains(string(bytes), inv.ResultSHA) {
		t.Fatal("SVG revision absent")
	}
	saved, err := svc.GetEvidenceResultJSON(job.ResultHandle)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "saved.json")
	os.WriteFile(file, []byte(saved), 0600)
	view, err := svc.LoadEvidenceResult(file)
	if err != nil || !view.Compatibility.Allowed {
		t.Fatal(view, err)
	}
	again, err := svc.GetEvidenceResultJSON(view.Handle)
	if err != nil || again != saved {
		t.Fatal("saved bytes changed", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = svc.QueryGPUExecution(cancelled, job.ResultHandle, inv.ResultSHA, string(q)); err == nil {
		t.Fatal("cancel ignored")
	}
	os.WriteFile(path, append(raw, 0), 0600)
	if _, err = svc.NormalizeGPUExecutionSource(ctx, path, string(options), inv.SourceSHA, filepath.Join(t.TempDir(), "changed.json")); err == nil {
		t.Fatal("changed original")
	}
	if _, err = svc.QueryGPUExecution(ctx, "missing", inv.ResultSHA, string(q)); err == nil {
		t.Fatal("missing owner")
	}
}
