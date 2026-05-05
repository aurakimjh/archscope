package profiler

import (
	"strings"
	"testing"
)

func makeDrilldownRoot() FlameNode {
	stacks := map[string]int{
		"a;b;c":         5,
		"a;b;d":         3,
		"a;X;c":         2,
		"main;runtime":  4,
		"sql;query;run": 7,
	}
	return buildFlameTree(stacks)
}

func TestDrilldownIncludeText(t *testing.T) {
	root := makeDrilldownRoot()
	stages := BuildDrilldownStages(root, []DrilldownFilter{
		{Pattern: "sql", FilterType: "include_text", MatchMode: "anywhere", ViewMode: "preserve_full_path"},
	}, 100, nil, 10)
	if len(stages) != 2 {
		t.Fatalf("expected 2 stages; got %d", len(stages))
	}
	if got, want := stages[1].Flamegraph.Samples, 7; got != want {
		t.Fatalf("matched samples = %d, want %d", got, want)
	}
	if got := stages[1].Label; !strings.Contains(got, "sql") {
		t.Fatalf("label = %q, expected to contain 'sql'", got)
	}
}

func TestDrilldownExcludeText(t *testing.T) {
	root := makeDrilldownRoot()
	stages := BuildDrilldownStages(root, []DrilldownFilter{
		{Pattern: "sql", FilterType: "exclude_text", MatchMode: "anywhere", ViewMode: "preserve_full_path"},
	}, 100, nil, 10)
	if got, want := stages[1].Flamegraph.Samples, 14; got != want { // 5+3+2+4
		t.Fatalf("matched samples = %d, want %d", got, want)
	}
}

func TestDrilldownRegexInclude(t *testing.T) {
	root := makeDrilldownRoot()
	stages := BuildDrilldownStages(root, []DrilldownFilter{
		{Pattern: "^a$", FilterType: "regex_include", MatchMode: "anywhere", ViewMode: "preserve_full_path"},
	}, 100, nil, 10)
	if got, want := stages[1].Flamegraph.Samples, 10; got != want { // a;b;c=5, a;b;d=3, a;X;c=2
		t.Fatalf("matched samples = %d, want %d", got, want)
	}
}

func TestDrilldownOrderedMatch(t *testing.T) {
	root := makeDrilldownRoot()
	stages := BuildDrilldownStages(root, []DrilldownFilter{
		{Pattern: "a > b > c", FilterType: "include_text", MatchMode: "ordered", ViewMode: "preserve_full_path"},
	}, 100, nil, 10)
	if got, want := stages[1].Flamegraph.Samples, 5; got != want {
		t.Fatalf("matched samples = %d, want %d (a;b;c only)", got, want)
	}
}

func TestDrilldownRerootAtMatch(t *testing.T) {
	root := makeDrilldownRoot()
	stages := BuildDrilldownStages(root, []DrilldownFilter{
		{Pattern: "b", FilterType: "include_text", MatchMode: "anywhere", ViewMode: "reroot_at_match"},
	}, 100, nil, 10)
	stage := stages[1]
	if got, want := stage.Flamegraph.Samples, 8; got != want {
		t.Fatalf("rerooted samples = %d, want %d", got, want)
	}
	// After rerooting at "b", the rerooted paths start with "b" so the
	// stage root's only child is "b" — and "b" has c, d as grandchildren.
	if len(stage.Flamegraph.Children) != 1 || stage.Flamegraph.Children[0].Name != "b" {
		names := []string{}
		for _, child := range stage.Flamegraph.Children {
			names = append(names, child.Name)
		}
		t.Fatalf("expected single 'b' child after reroot; got %v", names)
	}
	bNode := stage.Flamegraph.Children[0]
	leafNames := map[string]bool{}
	for _, child := range bNode.Children {
		leafNames[child.Name] = true
	}
	for _, name := range []string{"c", "d"} {
		if !leafNames[name] {
			t.Fatalf("expected grandchild %q under reroot 'b'; got %v", name, leafNames)
		}
	}
}

func TestDrilldownStageBreadcrumbAndChain(t *testing.T) {
	root := makeDrilldownRoot()
	stages := BuildDrilldownStages(root, []DrilldownFilter{
		{Pattern: "a", FilterType: "include_text", MatchMode: "anywhere", ViewMode: "preserve_full_path"},
		{Pattern: "c", FilterType: "include_text", MatchMode: "anywhere", ViewMode: "preserve_full_path"},
	}, 100, nil, 10)
	if got := len(stages); got != 3 {
		t.Fatalf("expected 3 stages; got %d", got)
	}
	if got, want := stages[2].Flamegraph.Samples, 7; got != want { // a;b;c=5 + a;X;c=2
		t.Fatalf("stage 2 samples = %d, want %d", got, want)
	}
	if got := len(stages[2].Breadcrumb); got != 3 {
		t.Fatalf("breadcrumb length = %d, want 3", got)
	}
}

func TestDrilldownInvalidRegexEmitsDiagnostic(t *testing.T) {
	root := makeDrilldownRoot()
	stages := BuildDrilldownStages(root, []DrilldownFilter{
		{Pattern: "([", FilterType: "regex_include", MatchMode: "anywhere"},
	}, 100, nil, 10)
	stage := stages[1]
	if stage.Flamegraph.Samples != 0 {
		t.Fatalf("invalid regex stage should be empty; got %d", stage.Flamegraph.Samples)
	}
	diag, ok := stage.Diagnostics.(*filterDiagnostic)
	if !ok || diag.Reason != "INVALID_REGEX" {
		t.Fatalf("expected INVALID_REGEX diagnostic; got %+v", stage.Diagnostics)
	}
}

func TestDrilldownUnsafeRegexLength(t *testing.T) {
	long := strings.Repeat("a", 600)
	root := makeDrilldownRoot()
	stages := BuildDrilldownStages(root, []DrilldownFilter{
		{Pattern: long, FilterType: "regex_include", MatchMode: "anywhere"},
	}, 100, nil, 10)
	diag, ok := stages[1].Diagnostics.(*filterDiagnostic)
	if !ok || diag.Reason != "UNSAFE_REGEX" {
		t.Fatalf("expected UNSAFE_REGEX diagnostic; got %+v", stages[1].Diagnostics)
	}
}

func TestDrilldownCaseInsensitive(t *testing.T) {
	root := makeDrilldownRoot()
	stages := BuildDrilldownStages(root, []DrilldownFilter{
		{Pattern: "SQL", FilterType: "include_text", MatchMode: "anywhere", CaseSensitive: false},
	}, 100, nil, 10)
	if got, want := stages[1].Flamegraph.Samples, 7; got != want {
		t.Fatalf("matched samples = %d, want %d (case-insensitive)", got, want)
	}
}

func TestDrilldownRootStageMetrics(t *testing.T) {
	root := makeDrilldownRoot()
	elapsed := 2.1
	stage := CreateRootStage(root, 100, &elapsed, 5)
	if total, _ := stage.Metrics["total_samples"].(int); total != 21 {
		t.Fatalf("total_samples = %v, want 21", stage.Metrics["total_samples"])
	}
	if got := stage.Metrics["total_ratio"].(float64); got != 100 {
		t.Fatalf("total_ratio = %v, want 100", got)
	}
}
