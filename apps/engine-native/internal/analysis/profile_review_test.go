package analysis_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
)

func TestR4LargeProfileChunkImportOverFiftyThousandNodes(t *testing.T) {
	raw, e := catalog.Contracts.ReadFile("contracts/profile-scale-cases.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases struct {
		Cases []struct {
			ID            string
			Stacks, Depth int
			Total         string `json:"total_weight"`
			Nodes         int    `json:"tree_nodes"`
		}
	}
	if e = json.Unmarshal(raw, &cases); e != nil {
		t.Fatal(e)
	}
	tc := cases.Cases[1]
	stacks := map[string]int{}
	for i := 0; i < tc.Stacks; i++ {
		frames := []string{fmt.Sprintf("task-%06d", i)}
		for depth := 1; depth < tc.Depth; depth++ {
			frames = append(frames, fmt.Sprintf("phase-%02d", depth))
		}
		stacks[strings.Join(frames, ";")] = 1
	}
	set, selection, e := m.FromLegacyCollapsed(stacks, strings.Repeat("a", 64), m.Metric{ID: "samples", Kind: "sample_count", Unit: "count", Scale: "1"})
	if e != nil {
		t.Fatal(e)
	}
	model, e := json.Marshal(set)
	if e != nil || len(model) <= 1<<20 {
		t.Fatal(e, len(model))
	}
	s, _, root := setup(t, nil)
	path := filepath.Join(t.TempDir(), "large-model.json")
	os.WriteFile(path, model, 0600)
	options, _ := json.Marshal(selection)
	started := start(t, s, analysis.Request{Path: path, Importer: "profile-set", Options: options})
	// Preserve the full 54k-node fixture under slower race-instrumented imports.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	job, e := s.Wait(ctx, started.ID, started.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	cancel()
	result, parsed, e := s.Read(job.ResultHandle)
	if e != nil || parsed.Summary["total_weight"] != tc.Total {
		t.Fatal(e, parsed.Summary)
	}
	nodes := 0
	var walk func(any)
	walk = func(value any) {
		node := value.(map[string]any)
		nodes++
		for _, child := range node["children"].([]any) {
			walk(child)
		}
	}
	walk(parsed.Charts["flamegraph"])
	if nodes != tc.Nodes || nodes < 50_000 {
		t.Fatal(nodes, tc.Nodes)
	}
	saved := filepath.Join(t.TempDir(), "saved.json")
	os.WriteFile(saved, result, 0600)
	// Reopening is another operation, not part of the import wait's deadline.
	loadCtx, loadCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer loadCancel()
	view, e := s.Load(loadCtx, saved)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	scalar, e := s.Scalar(view.Handle, "/summary/total_weight")
	if e != nil || scalar.Value == nil || *scalar.Value != tc.Total {
		t.Fatal(scalar, e)
	}
	again, e := s.Export(view.Handle, "json")
	if e != nil || string(again) != string(result) {
		t.Fatal("saved bytes changed", e)
	}
	clean(t, root)
	t.Logf("scale %s: model=%d result=%d ratio=%.3f nodes=%d", tc.ID, len(model), len(result), float64(len(result))/float64(len(model)), nodes)
	// Lowering the caller's record ceiling still refuses a chunk atomically.
	// This is a separate job. Large result reads/exports under -race can outlive
	// the first job's wait deadline; they must not cancel this rejection check.
	refusalCtx, refusalCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer refusalCancel()
	refusedStart := start(t, s, analysis.Request{Path: path, Importer: "profile-set", Options: options, Budgets: ingestion.Budgets{RecordBytes: 128}})
	refused, e := s.Wait(refusalCtx, refusedStart.ID, refusedStart.Generation)
	if e != nil {
		t.Fatal(e)
	}
	if refused.State != "failed" || refused.ResultHandle != "" || refused.Problem == nil || refused.Problem.Detail == nil || refused.Problem.Detail.ReasonCode != "record_bytes" || *refused.Problem.Detail.Limit != "128" {
		t.Fatal(refused)
	}
	clean(t, root)
}

func TestR4QualityCopyTamperRefusedAfterValidationCache(t *testing.T) {
	s, _, _ := setup(t, nil)
	job := importModel(t, s, "duration")
	raw, _, e := s.Read(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	var document map[string]any
	if e = json.Unmarshal(raw, &document); e != nil {
		t.Fatal(e)
	}
	document["metadata"].(map[string]any)["quality"].(map[string]any)["lost"] = "999"
	changed, _ := json.Marshal(document)
	path := filepath.Join(t.TempDir(), "tampered.json")
	os.WriteFile(path, changed, 0600)
	view, e := s.Load(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if view.Compatibility.Allowed {
		t.Fatal("quality copy forgery validated")
	}
	for _, op := range []string{"json", "csv", "html", "report-pack"} {
		if _, e = s.Export(view.Handle, op); e == nil {
			t.Fatal("tampered export", op)
		}
	}
	if _, e = s.ProfileTree(context.Background(), view.Handle); e == nil {
		t.Fatal("tampered compute")
	}
	if _, e = s.ProfileCompare(context.Background(), job.ResultHandle, view.Handle); e == nil {
		t.Fatal("tampered compare")
	}
	original, e := s.Export(job.ResultHandle, "json")
	if e != nil || string(original) != string(raw) {
		t.Fatal("cache poisoned", e)
	}
}

func TestR4TransportCeilingHasNumericJobDetail(t *testing.T) {
	s, _, root := setup(t, nil)
	path := filepath.Join(t.TempDir(), "over-limit.json")
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(catalog.MaxProfileTransportBytes + 1); e != nil {
		t.Fatal(e)
	}
	f.Close()
	job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "profile-set", Options: json.RawMessage(`{"profile_ids":["aggregate"],"metric_id":"samples"}`)}))
	if job.State != "failed" || job.ResultHandle != "" || job.Problem == nil || job.Problem.Code != "IMPORT_BUDGET_EXCEEDED" || job.Problem.Detail == nil {
		t.Fatal(job)
	}
	detail := job.Problem.Detail
	if detail.ReasonCode != "profile_transport_bytes" || detail.Actual == nil || *detail.Actual != "33554433" || detail.Limit == nil || *detail.Limit != "33554432" {
		t.Fatal(detail)
	}
	clean(t, root)
}

func TestR4ChunkedModelHonorsLoweredStackDepth(t *testing.T) {
	s, _, root := setup(t, nil)
	body, _ := catalog.Contracts.ReadFile("contracts/profile-set.json")
	path := filepath.Join(t.TempDir(), "model.json")
	os.WriteFile(path, body, 0600)
	job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "profile-set", Options: json.RawMessage(`{"profile_ids":["worker-a","worker-b"],"metric_id":"duration"}`), Budgets: ingestion.Budgets{StackDepth: 1}}))
	if job.State != "failed" || job.ResultHandle != "" || job.Problem == nil || job.Problem.Detail == nil || job.Problem.Detail.ReasonCode != "stack_depth" || *job.Problem.Detail.Limit != "1" {
		t.Fatal(job)
	}
	clean(t, root)
}
