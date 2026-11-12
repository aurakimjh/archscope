package main

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/execution"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestT642WailsSourcePinAndComparisonPageExport(t *testing.T) {
	ctx := context.Background()
	s := evidenceBridge(t, nil)
	raw, err := catalog.Contracts.ReadFile("contracts/execution-evidence-set.json")
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "input.json")
	output := filepath.Join(t.TempDir(), "new-model.json")
	os.WriteFile(input, raw, 0600)
	inventory, err := s.InspectExecutionSource(ctx, input, `{"format":"execution-evidence-json"}`)
	if err != nil {
		t.Fatal(err)
	}
	var inv analysis.ExecutionInventory
	json.Unmarshal([]byte(inventory), &inv)
	changed := strings.Replace(string(raw), "synthetic-run", "changed-run", 1)
	os.WriteFile(input, []byte(changed), 0600)
	if _, err = s.NormalizeExecutionSource(ctx, input, `{"format":"execution-evidence-json"}`, inv.SourceSHA, output); err == nil || !strings.Contains(err.Error(), "SOURCE_CHANGED") {
		t.Fatal(err)
	}
	if _, err = os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("stale model published")
	}
	inventory, err = s.InspectExecutionSource(ctx, input, `{"format":"execution-evidence-json"}`)
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal([]byte(inventory), &inv)
	if _, err = s.NormalizeExecutionSource(ctx, input, `{"format":"execution-evidence-json"}`, inv.SourceSHA, output); err != nil {
		t.Fatal(err)
	}
	if _, err = s.NormalizeExecutionSource(ctx, input, `{"format":"execution-evidence-json"}`, inv.SourceSHA, output); err == nil {
		t.Fatal("overwrite allowed")
	}
	model, err := e.Decode(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.Build(ctx, model, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(result)
	view, lease, err := s.evidenceService.LoadOwnedSnapshot(ctx, body)
	if err != nil {
		t.Fatal(err)
	}
	defer s.evidenceService.ReleaseResultLease(lease.ID)
	inv, err = s.evidenceService.ExecutionInventory(ctx, view.Handle)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "comparison-page.json")
	page, err := s.ExportExecutionComparison(ctx, view.Handle, view.Handle, inv.ResultSHA, inv.ResultSHA, `{"limit":1}`, path)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(path)
	if err != nil || string(stored) != page {
		t.Fatal(err)
	}
	var comparison e.Comparison
	json.Unmarshal(stored, &comparison)
	if len(comparison.Rows) != 1 || comparison.Next == "" || comparison.VerificationScope != "validated_normalized_snapshots_not_original_reimport" {
		t.Fatal(comparison)
	}
	if _, err = s.ExportExecutionComparison(ctx, view.Handle, view.Handle, inv.ResultSHA, inv.ResultSHA, `{"limit":1}`, path); err == nil {
		t.Fatal("comparison overwrite allowed")
	}
	if _, err = s.AnalyzeMultiThread(MultiThreadRequest{Paths: []string{"never-opened"}, CorrelationContextJSON: `{"unknown":true}`}); err == nil {
		t.Fatal("bad context scheduled async task")
	}
}
