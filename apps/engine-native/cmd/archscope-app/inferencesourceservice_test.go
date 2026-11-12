package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/inference"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
)

func TestT639BridgeOriginalOwnershipRefusalsAndSourceBinding(t *testing.T) {
	ctx := context.Background()
	t.Setenv("ARCHSCOPE_STATE_ROOT", t.TempDir())
	svc := evidenceBridge(t, nil)
	dir := t.TempDir()
	raw, _ := catalog.Contracts.ReadFile("contracts/inference-evidence-nvidia-query.csv")
	capture, _ := catalog.Contracts.ReadFile("contracts/inference-evidence-nvidia-capture.json")
	var c any
	json.Unmarshal(capture, &c)
	opts, _ := json.Marshal(map[string]any{"format": "nvidia-smi-csv", "capture": c})
	path := filepath.Join(dir, "nvidia.csv")
	os.WriteFile(path, raw, 0600)
	value, e := svc.InspectInferenceSource(ctx, path, string(opts))
	if e != nil {
		t.Fatal(e)
	}
	var inv a.Inventory
	json.Unmarshal([]byte(value), &inv)
	if len(inv.Signals) != 14 || inv.MetricPoints != 21 || inv.SourceSHA256 == "" {
		t.Fatal(inv)
	}
	output := filepath.Join(dir, "model.json")
	if _, e = svc.NormalizeInferenceSource(ctx, path, string(opts), strings.Repeat("0", 64), output); e == nil {
		t.Fatal("stale original accepted")
	}
	if _, e = os.Stat(output); !os.IsNotExist(e) {
		t.Fatal("failed operation published output")
	}
	if _, e = svc.NormalizeInferenceSource(ctx, path, string(opts), inv.SourceSHA256, output); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.NormalizeInferenceSource(ctx, path, string(opts), inv.SourceSHA256, output); e == nil {
		t.Fatal("overwrite accepted")
	}
	job, e := svc.AnalyzeEvidence(EvidenceImportRequest{Path: output, Importer: "inference.evidence-json", OptionsJSON: "{}"})
	if e != nil {
		t.Fatal(e)
	}
	job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	value, e = svc.DescribeInferenceEvidence(ctx, job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal([]byte(value), &inv)
	if inv.ResultSHA256 == "" {
		t.Fatal("missing saved digest")
	}
	signal := inv.Signals[0].ID
	if _, e = svc.QueryInferenceMetric(ctx, job.ResultHandle, "stale", signal, "", 25); e == nil {
		t.Fatal("stale query accepted")
	}
	if _, e = svc.QueryInferenceMetric(ctx, job.ResultHandle, inv.ResultSHA256, "unmapped", "", 25); e == nil {
		t.Fatal("unbound signal queried")
	}
	value, e = svc.QueryInferenceMetric(ctx, job.ResultHandle, inv.ResultSHA256, signal, "", 1)
	if e != nil {
		t.Fatal(e)
	}
	var page m.Page
	json.Unmarshal([]byte(value), &page)
	if page.ResultSHA256 != inv.ResultSHA256 || page.Next == "" || len(page.Rows) != 1 {
		t.Fatal(page)
	}
	if _, e = svc.QueryInferenceMetric(ctx, job.ResultHandle, inv.ResultSHA256, inv.Signals[1].ID, page.Next, 1); e == nil {
		t.Fatal("cursor crossed signal")
	}
	value, e = svc.MaterializeInferenceMetrics(ctx, job.ResultHandle, inv.ResultSHA256, `["`+signal+`"]`)
	if e != nil {
		t.Fatal(e)
	}
	var owned struct {
		View  analysis.ResultView  `json:"view"`
		Lease analysis.ResultLease `json:"lease"`
	}
	json.Unmarshal([]byte(value), &owned)
	if owned.Lease.ID == "" || owned.View.Type != "metrics_timeseries" {
		t.Fatal(owned)
	}
	if _, _, e = svc.evidenceService.Read(owned.View.Handle); e != nil {
		t.Fatal(e)
	}
	if e = svc.ReleaseEvidenceLease(owned.Lease.ID); e != nil {
		t.Fatal(e)
	}
	if e = svc.ReleaseEvidenceLease(owned.Lease.ID); e == nil {
		t.Fatal("released lease remained active")
	}
	if _, e = svc.CleanupEvidenceCache(); e != nil {
		t.Fatal(e)
	}
	if _, _, e = svc.evidenceService.Read(owned.View.Handle); e == nil {
		t.Fatal("unowned derived cache survived cleanup")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = svc.QueryInferenceMetric(cancelled, job.ResultHandle, inv.ResultSHA256, signal, "", 25); e == nil {
		t.Fatal("cancel ignored")
	}
	os.WriteFile(path, append(raw, '\n'), 0600)
	if _, e = svc.NormalizeInferenceSource(ctx, path, string(opts), inv.SourceSHA256, filepath.Join(dir, "changed.json")); e == nil {
		t.Fatal("changed source accepted")
	}
}

func TestR16DeclaredWorkspaceViewKeepsOriginalAndDeclaration(t *testing.T) {
	ctx := context.Background()
	t.Setenv("ARCHSCOPE_STATE_ROOT", t.TempDir())
	svc := evidenceBridge(t, nil)
	raw, _ := catalog.Contracts.ReadFile("contracts/inference-evidence-r16-declared-set.json")
	path := filepath.Join(t.TempDir(), "declared.json")
	os.WriteFile(path, raw, 0600)
	job, e := svc.AnalyzeEvidence(EvidenceImportRequest{Path: path, Importer: "inference.evidence-json", OptionsJSON: "{}"})
	if e != nil {
		t.Fatal(e)
	}
	job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	value, e := svc.DescribeInferenceEvidence(ctx, job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	var inv a.Inventory
	json.Unmarshal([]byte(value), &inv)
	signal := "signal-synthetic_output_tokens"
	value, e = svc.MaterializeInferenceMetrics(ctx, job.ResultHandle, inv.ResultSHA256, `["`+signal+`"]`)
	if e != nil {
		t.Fatal(e)
	}
	var owned struct {
		View  analysis.ResultView  `json:"view"`
		Lease analysis.ResultLease `json:"lease"`
	}
	json.Unmarshal([]byte(value), &owned)
	defer svc.ReleaseEvidenceLease(owned.Lease.ID)
	_, result, e := svc.evidenceService.Read(owned.View.Handle)
	if e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(result.Metadata.Extra["inference_projection"])
	var metadata struct {
		Original     m.Selection `json:"original_selection"`
		Declarations map[string]struct {
			Unit string `json:"unit"`
		} `json:"declared_metadata"`
	}
	json.Unmarshal(body, &metadata)
	if metadata.Declarations[signal].Unit != "tokens" || len(metadata.Original.SeriesIDs) != 1 {
		t.Fatal(string(body))
	}
	for _, v := range inv.Signals {
		if v.ID == signal && metadata.Original.SeriesIDs[0] != v.SeriesID {
			t.Fatal("original identity changed")
		}
	}
}
