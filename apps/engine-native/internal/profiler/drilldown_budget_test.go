package profiler

import "testing"

func TestDisplayPruningDoesNotMutateCanonicalTree(t *testing.T) {
	root := buildFlameTree(map[string]int{"a;hot": 100, "a;rare": 10, "z": 5})
	before := countNodes(root)
	for _, limit := range []int{1, 2, 3, 4, 100} {
		stages := BuildDrilldownStagesWithOptions(root, []DrilldownFilter{
			{Pattern: "a", FilterType: "include_text", MatchMode: "anywhere"},
			{Pattern: "rare", FilterType: "include_text", MatchMode: "anywhere"},
		}, Options{MaxFlamegraphNodes: limit}, root.Samples)
		if countNodes(root) != before || stages[2].Flamegraph.Samples != 10 {
			t.Fatalf("canonical data mutated at limit %d", limit)
		}
		for _, stage := range stages {
			if countNodes(stage.Flamegraph) > limit {
				t.Fatal("unbounded stage")
			}
			var check func(FlameNode)
			check = func(n FlameNode) {
				children := 0
				for _, c := range n.Children {
					children += c.Samples
					check(c)
				}
				if n.HiddenSamples < 0 || children+n.HiddenSamples > n.Samples {
					t.Fatalf("invalid hidden/self accounting: %+v", n)
				}
			}
			check(stage.Flamegraph)
		}
	}
}

func TestHiddenDescendantsAreNotReclassifiedAsSelf(t *testing.T) {
	root := buildFlameTree(map[string]int{"a;hot": 100, "a;rare": 10})
	pruneFlamegraph(&root, 3)
	if root.Children[0].HiddenSamples != 10 {
		t.Fatalf("lost omitted descendant disclosure: %+v", root)
	}
	paths := iterLeafPaths(root)
	for _, path := range paths {
		if len(path.Path) == 1 && path.Path[0] == "a" {
			t.Fatal("hidden rare weight reclassified as A self")
		}
	}
}
