package main

import (
	"context"
	"encoding/json"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestT634OriginalSourceWailsSavedCompareAndExactExport(t *testing.T) {
	svc := evidenceBridge(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dir := t.TempDir()
	original := filepath.Join(dir, "original.raw")
	if e := os.WriteFile(original, []byte("root;leaf 0.25\nroot 0\n"), 0600); e != nil {
		t.Fatal(e)
	}
	importOriginal := func(name, options string) string {
		text, e := svc.InspectProfileSource(ctx, original, options)
		if e != nil {
			t.Fatal(e)
		}
		var inv parser.SourceInventory
		if e = json.Unmarshal([]byte(text), &inv); e != nil {
			t.Fatal(e)
		}
		out := filepath.Join(dir, name+".json")
		if _, e = svc.NormalizeProfileSource(ctx, original, options, "stale", out); e == nil {
			t.Fatal("accepted stale source")
		}
		if _, e = os.Stat(out); !os.IsNotExist(e) {
			t.Fatal("published stale result")
		}
		if _, e = svc.NormalizeProfileSource(ctx, original, options, inv.SourceSHA256, out); e != nil {
			t.Fatal(e)
		}
		if _, e = svc.NormalizeProfileSource(ctx, original, options, inv.SourceSHA256, out); e == nil {
			t.Fatal("overwrote existing model")
		}
		job, e := svc.AnalyzeEvidence(EvidenceImportRequest{Path: out, Importer: "profile.source-set-json", OptionsJSON: `{"profile_ids":["profile-0"],"metric_id":"samples"}`})
		if e != nil {
			t.Fatal(e)
		}
		job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
		if e != nil || job.State != "completed" {
			t.Fatal(job, e)
		}
		return job.ResultHandle
	}
	a := importOriginal("before", `{"format":"pyspy-raw","producer":"py-spy","producer_version":"0.4.1","capture_options":{"rate":100,"output_path":"a"}}`)
	b := importOriginal("after", `{"format":"pyspy-raw","producer":"py-spy","producer_version":"0.4.1","capture_options":{"rate":100,"output_path":"b"}}`)
	text, e := svc.GetProfileSetTree(ctx, a)
	if e != nil || !strings.Contains(text, `"inclusive":"0.25"`) {
		t.Fatal(text, e)
	}
	diff, e := svc.CompareProfileSets(ctx, a, b)
	if e != nil || !strings.Contains(diff, `"incidental_differences":["output_path"]`) || !strings.Contains(diff, `"identity_basis":"declared"`) {
		t.Fatal(diff, e)
	}
	saved, e := svc.GetEvidenceResultJSON(a)
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(dir, "saved.json")
	os.WriteFile(file, []byte(saved), 0600)
	view, e := svc.LoadEvidenceResult(file)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	after, e := svc.GetEvidenceResultJSON(view.Handle)
	if e != nil || after != saved {
		t.Fatal("saved result changed", e)
	}
	if e = svc.ExportProfileSetPprof(ctx, view.Handle, "0.01", filepath.Join(dir, "ticks.pprof")); e != nil {
		t.Fatal(e)
	}
	if e = svc.ExportProfileSetPprof(ctx, view.Handle, "1", filepath.Join(dir, "rounded.pprof")); e == nil {
		t.Fatal("rounded export")
	}
}
