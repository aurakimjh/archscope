package profiler

import (
	"testing"
)

// Build a synthetic deep+wide tree with ~N nodes so we can verify
// the budget-pruner actually bounds the result.
func makeWideDeepTree(width, depth int, samples int) FlameNode {
	if depth == 0 {
		return FlameNode{Name: "leaf", Samples: samples}
	}
	root := FlameNode{Name: "root", Samples: samples * width, Children: make([]FlameNode, width)}
	for i := 0; i < width; i++ {
		root.Children[i] = makeWideDeepTree(width, depth-1, samples)
		root.Children[i].Name = "node"
	}
	return root
}

func TestPruneFlamegraphBoundsNodeCount(t *testing.T) {
	cases := []struct {
		width int
		depth int
		max   int
	}{
		{10, 5, 1000},   // 10^5 = 100k → prune to 1k
		{20, 4, 5000},   // 20^4 = 160k → prune to 5k
		{5, 8, 500},     // 5^8 = ~390k → prune to 500
	}
	for _, c := range cases {
		root := makeWideDeepTree(c.width, c.depth, 1)
		before := countNodes(root)
		kept, dropped := pruneFlamegraph(&root, c.max)
		if kept > c.max+10 { // +10 for "...other" siblings overhead
			t.Errorf("width=%d depth=%d max=%d: kept %d > max", c.width, c.depth, c.max, kept)
		}
		if before-kept != dropped {
			t.Errorf("dropped accounting off: before=%d kept=%d dropped=%d", before, kept, dropped)
		}
	}
}
