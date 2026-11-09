package profiler

import (
	"strings"
	"testing"
)

func TestOrderedRegexValidatesUnreachableTermsBeforeTraversal(t *testing.T) {
	for _, kind := range []string{"regex_include", "regex_exclude"} {
		for _, root := range []FlameNode{makeDrilldownRoot(), buildFlameTree(nil)} {
			for _, tc := range []struct{ pattern, reason string }{
				{"a>[", "INVALID_REGEX"}, {"never-matches>[", "INVALID_REGEX"},
				{"never-matches>(a+)+", "UNSAFE_REGEX"},
				{strings.Repeat("a;", 251), "UNSAFE_REGEX"},
				{"> ; ", "INVALID_REGEX"},
			} {
				stages := BuildDrilldownStages(root, []DrilldownFilter{{Pattern: tc.pattern, FilterType: kind, MatchMode: "ordered"}}, 100, nil, 10)
				stage := stages[1]
				diag, ok := stage.Diagnostics.(*filterDiagnostic)
				if !ok || diag.Reason != tc.reason || stage.Flamegraph.Samples != 0 {
					t.Fatalf("%s %q: invalid regex masquerades as match/no-match: %+v", kind, tc.pattern, stage)
				}
			}
		}
	}
}

func TestPreparedOrderedRegexPreservesMatchingAndReroot(t *testing.T) {
	root := makeDrilldownRoot()
	filter := DrilldownFilter{Pattern: "^A$>^B$", FilterType: "regex_include", MatchMode: "ordered", ViewMode: "reroot_at_match"}
	prepared, diag := compileFilter(filter)
	if diag != nil || len(prepared.ordered) != 2 {
		t.Fatal("ordered terms not prepared")
	}
	stage := ApplyDrilldownFilter(CreateRootStage(root, 100, nil, 10), filter, 100, nil, 10)
	if stage.Diagnostics != nil || stage.Flamegraph.Samples != 8 || stage.Flamegraph.Children[0].Name != "a" {
		t.Fatal("ordered include/reroot semantics changed")
	}
	filter.CaseSensitive = true
	stage = ApplyDrilldownFilter(CreateRootStage(root, 100, nil, 10), filter, 100, nil, 10)
	if stage.Flamegraph.Samples != 0 || stage.Diagnostics != nil {
		t.Fatal("case-sensitive no-match is not a valid empty result")
	}
	filter.CaseSensitive = false
	filter.FilterType = "regex_exclude"
	stage = ApplyDrilldownFilter(CreateRootStage(root, 100, nil, 10), filter, 100, nil, 10)
	if stage.Flamegraph.Samples != root.Samples-8 {
		t.Fatal("ordered exclude changed")
	}
}
