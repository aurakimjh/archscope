package profiler

import (
	"math"
	"testing"
)

func TestDiffNormalizedDelta(t *testing.T) {
	baseline := map[string]int{
		"a;b;c": 100,
		"a;d":   100,
	}
	target := map[string]int{
		"a;b;c": 50,
		"a;d":   150,
	}
	flame, parts := BuildDiffFlameTree(baseline, target, DiffOptions{Normalize: true})
	if flame.Name != "All" {
		t.Fatalf("root name = %q", flame.Name)
	}
	summary := parts["summary"].(map[string]any)
	// flat collects every non-root path (intermediates included): a, a;b, a;b;c, a;d.
	if summary["common_paths"].(int) != 4 {
		t.Fatalf("common_paths = %v, want 4", summary["common_paths"])
	}
	if summary["normalized"].(bool) != true {
		t.Fatalf("normalized flag should be true")
	}
	tables := parts["tables"].(map[string]any)
	gains := tables["biggest_increases"].([]map[string]any)
	if len(gains) == 0 || gains[0]["stack"] != "a;d" {
		t.Fatalf("biggest gain should be a;d; got %+v", gains)
	}
	losses := tables["biggest_decreases"].([]map[string]any)
	// `a;b;c` and its inclusive parent `a;b` both lose the same amount when
	// normalized (the parent's count == its only child's count). Either
	// stack is correct as the worst regression.
	if len(losses) == 0 {
		t.Fatalf("expected a regression entry; got %+v", losses)
	}
	stack := losses[0]["stack"]
	if stack != "a;b;c" && stack != "a;b" {
		t.Fatalf("biggest loss should be a;b;c or a;b; got %+v", losses)
	}
}

func TestDiffAddedAndRemovedPaths(t *testing.T) {
	baseline := map[string]int{
		"only_baseline": 5,
		"shared":        4,
	}
	target := map[string]int{
		"only_target": 6,
		"shared":      3,
	}
	_, parts := BuildDiffFlameTree(baseline, target, DiffOptions{Normalize: false})
	summary := parts["summary"].(map[string]any)
	if summary["added_paths"].(int) != 1 {
		t.Fatalf("added_paths = %v, want 1", summary["added_paths"])
	}
	if summary["removed_paths"].(int) != 1 {
		t.Fatalf("removed_paths = %v, want 1", summary["removed_paths"])
	}
}

func TestDiffMetadataCarriesABDelta(t *testing.T) {
	baseline := map[string]int{"a;b": 4}
	target := map[string]int{"a;b": 10}
	flame, _ := BuildDiffFlameTree(baseline, target, DiffOptions{Normalize: false})
	// Walk to the a;b leaf and confirm metadata carries the values.
	var leaf *FlameNode
	var walk func(node *FlameNode)
	walk = func(node *FlameNode) {
		if node.Name == "b" && len(node.Children) == 0 {
			leaf = node
			return
		}
		for index := range node.Children {
			walk(&node.Children[index])
			if leaf != nil {
				return
			}
		}
	}
	walk(&flame)
	if leaf == nil {
		t.Fatalf("could not find leaf 'b'; tree=%+v", flame)
	}
	meta, ok := leaf.Metadata.(map[string]any)
	if !ok {
		t.Fatalf("expected metadata map, got %T", leaf.Metadata)
	}
	if !approxEqual(meta["a"].(float64), 4) {
		t.Fatalf("a = %v, want 4", meta["a"])
	}
	if !approxEqual(meta["b"].(float64), 10) {
		t.Fatalf("b = %v, want 10", meta["b"])
	}
	if !approxEqual(meta["delta"].(float64), 6) {
		t.Fatalf("delta = %v, want 6", meta["delta"])
	}
}

func TestAnalyzeProfilerDiffShape(t *testing.T) {
	baseline := map[string]int{"a;b;c": 5, "a;d": 3}
	target := map[string]int{"a;b;c": 8, "a;d": 1}
	result := AnalyzeProfilerDiff(baseline, target, "/tmp/a.collapsed", "/tmp/b.collapsed", DiffOptions{Normalize: false})
	if result.Type != "profiler_diff" {
		t.Fatalf("type = %q, want profiler_diff", result.Type)
	}
	if result.Metadata.Parser != "profiler_diff" {
		t.Fatalf("parser = %q, want profiler_diff", result.Metadata.Parser)
	}
	if result.Metadata.DiffSummary == nil {
		t.Fatalf("expected DiffSummary on metadata")
	}
	if result.Metadata.DiffTables == nil {
		t.Fatalf("expected DiffTables on metadata")
	}
	if got := len(result.SourceFiles); got != 2 {
		t.Fatalf("source_files = %d, want 2", got)
	}
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
