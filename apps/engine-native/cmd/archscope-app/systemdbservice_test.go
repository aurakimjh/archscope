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

func TestT645SnapshotCaptureUsesPinnedMetricWailsAndSavedReaders(t *testing.T) {
	for _, name := range []string{"postgres", "linux-ss"} {
		t.Run(name, func(t *testing.T) {
			svc := evidenceBridge(t, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			raw, e := os.ReadFile("../../internal/catalog/contracts/system-db-" + name + "-capture.json")
			if e != nil {
				t.Fatal(e)
			}
			original := filepath.Join(t.TempDir(), name+".json")
			if e = os.WriteFile(original, raw, 0600); e != nil {
				t.Fatal(e)
			}
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
				t.Fatal("stale accepted")
			}
			if _, e = svc.NormalizeMetricSource(ctx, original, `{}`, inv.SourceSHA256, out); e != nil {
				t.Fatal(e)
			}
			if _, e = svc.NormalizeMetricSource(ctx, original, `{}`, inv.SourceSHA256, out); e == nil {
				t.Fatal("overwrote model")
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
			var inventory analysis.MetricInventory
			if e = json.Unmarshal([]byte(text), &inventory); e != nil {
				t.Fatal(e)
			}
			q, _ := json.Marshal(m.Query{Selection: selection, Limit: 1})
			page, e := svc.QueryMetricHistory(ctx, job.ResultHandle, inventory.ResultSHA256, string(q))
			if e != nil || !strings.Contains(page, `"next"`) {
				t.Fatal(page, e)
			}
			if _, e = svc.QueryMetricHistory(ctx, job.ResultHandle, strings.Repeat("0", 64), string(q)); e == nil {
				t.Fatal("stale result accepted")
			}
			saved, e := svc.GetEvidenceResultJSON(job.ResultHandle)
			if e != nil {
				t.Fatal(e)
			}
			file := filepath.Join(t.TempDir(), "saved.json")
			if e = os.WriteFile(file, []byte(saved), 0600); e != nil {
				t.Fatal(e)
			}
			view, e := svc.LoadEvidenceResult(file)
			if e != nil || !view.Compatibility.Allowed {
				t.Fatal(view, e)
			}
			again, e := svc.GetEvidenceResultJSON(view.Handle)
			if e != nil || again != saved {
				t.Fatal("saved bytes changed", e)
			}
			if e = os.WriteFile(original, append(raw, ' '), 0600); e != nil {
				t.Fatal(e)
			}
			if _, e = svc.NormalizeMetricSource(ctx, original, `{}`, inv.SourceSHA256, filepath.Join(t.TempDir(), "changed.json")); e == nil {
				t.Fatal("changed source accepted")
			}
		})
	}
}
