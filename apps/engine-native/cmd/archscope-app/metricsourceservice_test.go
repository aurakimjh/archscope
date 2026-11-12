package main

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestT638MetricWailsNormalizeSavedReadAndQuery(t *testing.T) {
	svc := evidenceBridge(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	body, e := os.ReadFile("../../internal/parsers/metrichistory/testdata/sdk-python-1.38.0.otlp.json")
	if e != nil {
		t.Fatal(e)
	}
	original := filepath.Join(t.TempDir(), "original.json")
	os.WriteFile(original, body, 0600)
	text, e := svc.InspectMetricSource(ctx, original, `{}`)
	if e != nil {
		t.Fatal(e)
	}
	var inv p.Inventory
	if e = json.Unmarshal([]byte(text), &inv); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "model.json")
	if _, e = svc.NormalizeMetricSource(ctx, original, `{}`, "stale", out); e == nil {
		t.Fatal("stale original accepted")
	}
	if _, e = svc.NormalizeMetricSource(ctx, original, `{}`, inv.SourceSHA256, out); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.NormalizeMetricSource(ctx, original, `{}`, inv.SourceSHA256, out); e == nil {
		t.Fatal("existing output overwritten")
	}
	selection := m.Selection{SeriesIDs: []string{inv.Series[0].ID}}
	options, _ := json.Marshal(selection)
	job, e := svc.AnalyzeEvidence(EvidenceImportRequest{Path: out, Importer: "metrics.history-json", OptionsJSON: string(options)})
	if e != nil {
		t.Fatal(e)
	}
	job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	text, e = svc.DescribeMetricQuery(ctx, job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	var query analysis.MetricInventory
	if e = json.Unmarshal([]byte(text), &query); e != nil {
		t.Fatal(e)
	}
	q, _ := json.Marshal(m.Query{Selection: selection, Limit: 1})
	text, e = svc.QueryMetricHistory(ctx, job.ResultHandle, query.ResultSHA256, string(q))
	if e != nil {
		t.Fatal(e)
	}
	var page m.Page
	if e = json.Unmarshal([]byte(text), &page); e != nil || page.Total != 2 || page.Next == "" {
		t.Fatal(text, e)
	}
	csvPath := filepath.Join(t.TempDir(), "intervals.csv")
	if e = svc.ExportMetricIntervalCSV(ctx, job.ResultHandle, query.ResultSHA256, string(options), csvPath); e != nil {
		t.Fatal(e)
	}
	csvBody, _ := os.ReadFile(csvPath)
	if !strings.Contains(string(csvBody), "observed_histogram_json") {
		t.Fatal("missing interval CSV")
	}
	if e = svc.ExportMetricIntervalCSV(ctx, job.ResultHandle, query.ResultSHA256, string(options), csvPath); e == nil {
		t.Fatal("CSV overwrite")
	}
	if e = svc.ExportMetricIntervalCSV(ctx, job.ResultHandle, "stale", string(options), filepath.Join(t.TempDir(), "stale.csv")); e == nil {
		t.Fatal("stale CSV")
	}
	saved, e := svc.GetEvidenceResultJSON(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "saved.json")
	os.WriteFile(file, []byte(saved), 0600)
	view, e := svc.LoadEvidenceResult(file)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	again, e := svc.GetEvidenceResultJSON(view.Handle)
	if e != nil || again != saved {
		t.Fatal("saved bytes changed", e)
	}
}

func TestT638OpaqueMetricIdentityRetainsLargeNumericAttributes(t *testing.T) {
	model, e := p.ParseBytes(context.Background(), []byte(`{"schema_version":"1.0.0","snapshots":[{"time_unix_nano":"1","metrics":[{"name":"x","kind":"gauge","value":"1","attributes":{"worker":9007199254740993}}]}]}`), "synthetic.json", p.Options{})
	if e != nil {
		t.Fatal(e)
	}
	raw, e := metricInventoryJSON(p.Inspect(model), nil)
	if e != nil {
		t.Fatal(e)
	}
	var output struct {
		Series []struct {
			IdentityJSON string `json:"identity_json"`
		}
	}
	if e = json.Unmarshal([]byte(raw), &output); e != nil {
		t.Fatal(e)
	}
	if len(output.Series) != 1 || !strings.Contains(output.Series[0].IdentityJSON, "9007199254740993") {
		t.Fatal("identity crossed JS as a number", raw)
	}
}

func TestT643AppleWailsNormalizeSavedReadAndQuery(t *testing.T) {
	svc := evidenceBridge(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	body, e := os.ReadFile("../../internal/catalog/contracts/apple-ollama-capture.json")
	if e != nil {
		t.Fatal(e)
	}
	original := filepath.Join(t.TempDir(), "original.json")
	os.WriteFile(original, body, 0600)
	text, e := svc.InspectMetricSource(ctx, original, `{}`)
	if e != nil {
		t.Fatal(e)
	}
	var inv p.Inventory
	if e = json.Unmarshal([]byte(text), &inv); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "model.json")
	if _, e = svc.NormalizeMetricSource(ctx, original, `{}`, "stale", out); e == nil {
		t.Fatal("stale original accepted")
	}
	if _, e = svc.NormalizeMetricSource(ctx, original, `{}`, inv.SourceSHA256, out); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.NormalizeMetricSource(ctx, original, `{}`, inv.SourceSHA256, out); e == nil {
		t.Fatal("existing output overwritten")
	}
	selection := m.Selection{SeriesIDs: []string{inv.Series[0].ID}}
	options, _ := json.Marshal(selection)
	job, e := svc.AnalyzeEvidence(EvidenceImportRequest{Path: out, Importer: "metrics.history-json", OptionsJSON: string(options)})
	if e != nil {
		t.Fatal(e)
	}
	job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	text, e = svc.DescribeMetricQuery(ctx, job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	var query analysis.MetricInventory
	if e = json.Unmarshal([]byte(text), &query); e != nil {
		t.Fatal(e)
	}
	q, _ := json.Marshal(m.Query{Selection: selection, Limit: 1})
	text, e = svc.QueryMetricHistory(ctx, job.ResultHandle, query.ResultSHA256, string(q))
	if e != nil {
		t.Fatal(e)
	}
	var page m.Page
	if e = json.Unmarshal([]byte(text), &page); e != nil || page.Total != 3 || page.Next == "" {
		t.Fatal(text, e)
	}
	saved, e := svc.GetEvidenceResultJSON(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "saved.json")
	os.WriteFile(file, []byte(saved), 0600)
	view, e := svc.LoadEvidenceResult(file)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	again, e := svc.GetEvidenceResultJSON(view.Handle)
	if e != nil || again != saved {
		t.Fatal("saved bytes changed", e)
	}
}
