package profiler

import (
	"fmt"
	"sort"
	"strings"
)

// mutableNode is the in-progress tree node. Memory-tight on
// purpose — historically this carried a per-node Path []string clone
// that ballooned to multi-GB on 60 MB+ wall inputs (1M+ nodes ×
// avg-50-frame paths). The path is now reconstructed during
// freezeNode from the recursion stack, which costs nothing extra
// because freezeNode already walks parent→child top-down.
type mutableNode struct {
	ID       string
	ParentID *string
	Name     string
	Samples  int
	Children map[string]*mutableNode
}

// buildFlameTree builds the canonical flame tree from a stack→count
// map. Hot path on 400 MB+ wall profiles, so the loop body is
// careful about allocations:
//   - the per-stack `path` slice is hoisted outside the outer loop
//     and truncated (path[:0]) instead of re-allocated; the only
//     cost paid per frame is one append-may-grow which tops out
//     after the first deep stack reaches the depth ceiling.
//   - splitStack returns a slice that lives only until the next
//     call; we don't retain it.
//   - mutableNode.Path is cloned via slices.Clone-equivalent because
//     freezeNode shares it directly with the FlameNode and the path
//     buffer above keeps mutating.
func buildFlameTree(stacks map[string]int) FlameNode {
	return buildFlameTreeWithLog(stacks, nil)
}

// buildFlameTreeWithLog is the progress-aware variant. Emits a tick
// every 25k stacks consumed so a 60 MB collapsed input that survives
// parse but explodes during tree construction still leaves a trail
// in the log file. Cheap — one runtime.ReadMemStats per ~25k stacks.
func buildFlameTreeWithLog(stacks map[string]int, pl *ProgressLog) FlameNode {
	total := 0
	for _, samples := range stacks {
		if samples > 0 {
			total += samples
		}
	}
	root := &mutableNode{
		ID:       "root",
		Name:     "All",
		Samples:  total,
		Children: map[string]*mutableNode{},
	}
	path := make([]string, 0, 256)
	consumed := 0
	const tickEvery = 25_000
	totalNodes := 0
	for stack, samples := range stacks {
		if samples <= 0 {
			continue
		}
		current := root
		path = path[:0]
		for _, frame := range splitStack(stack) {
			path = append(path, frame)
			child := current.Children[frame]
			if child == nil {
				parentID := current.ID
				child = &mutableNode{
					ID:       nodeID(path),
					ParentID: &parentID,
					Name:     frame,
					Children: map[string]*mutableNode{},
				}
				current.Children[frame] = child
				totalNodes++
			}
			child.Samples += samples
			current = child
		}
		consumed++
		if pl != nil && consumed%tickEvery == 0 {
			pl.Tick("build stacks=%d/%d nodes=%d", consumed, len(stacks), totalNodes)
			pl.Mem("build")
		}
	}
	if pl != nil {
		pl.Tick("build done: stacks=%d nodes=%d total_samples=%d", consumed, totalNodes, total)
		pl.Mem("build-end")
	}
	pathBuf := make([]string, 0, 256)
	return freezeNode(root, max(total, 1), pathBuf)
}

// freezeNode reconstructs the immutable FlameNode from the in-progress
// mutableNode tree. The path slice is rebuilt from the recursion
// stack rather than stored on every mutableNode — saves ~50%
// memory during build on deep stacks. Each FlameNode receives its
// own freshly cloned Path because consumers (iterLeafPaths,
// timeline classifier) retain it across the result lifetime.
func freezeNode(node *mutableNode, total int, pathBuf []string) FlameNode {
	children := make([]*mutableNode, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		if children[i].Samples == children[j].Samples {
			return children[i].Name < children[j].Name
		}
		return children[i].Samples > children[j].Samples
	})
	var clonedPath []string
	if len(pathBuf) > 0 {
		clonedPath = make([]string, len(pathBuf))
		copy(clonedPath, pathBuf)
	}
	out := FlameNode{
		ID:       node.ID,
		ParentID: node.ParentID,
		Name:     node.Name,
		Samples:  node.Samples,
		Ratio:    ratio(node.Samples, total, 4),
		Children: make([]FlameNode, 0, len(children)),
		Path:     clonedPath,
	}
	for _, child := range children {
		pathBuf = append(pathBuf, child.Name)
		out.Children = append(out.Children, freezeNode(child, total, pathBuf))
		pathBuf = pathBuf[:len(pathBuf)-1]
	}
	return out
}

// countNodes walks the frozen tree and returns the total number of
// nodes. Cheap (O(N)) and useful for the IPC-payload sanity check.
func countNodes(root FlameNode) int {
	n := 1
	for i := range root.Children {
		n += countNodes(root.Children[i])
	}
	return n
}

// pruneFlamegraph caps the total node count of the tree by allocating
// a per-subtree node budget proportional to that subtree's samples
// share. The previous per-level cap (256 children) was insufficient
// because deep trees still had 256^depth potential — a 60 MB wall
// profile easily reached 1 M nodes after a "best effort" prune.
//
// Algorithm (greedy budget propagation):
//   - Each call receives a remaining `budget`. The node itself
//     consumes 1; the rest is split among children proportionally
//     to child.samples / sum(child.samples).
//   - Children whose allocation rounds to 0 are dropped; their
//     samples are merged into a single "…other (N frames)" sibling.
//   - Recursion uses the actual nodes-kept count so the next sibling
//     still has accurate remaining budget.
//
// The function returns the actual kept-node count so callers can log
// before/after for the progress trace.
func pruneFlamegraph(root *FlameNode, maxNodes int) (kept int, dropped int) {
	if root == nil {
		return 0, 0
	}
	if maxNodes <= 0 {
		return countNodes(*root), 0
	}
	before := countNodes(*root)
	if before <= maxNodes {
		return before, 0
	}
	pruneBudget(root, maxNodes)
	after := countNodes(*root)
	return after, before - after
}

// pruneBudget consumes `budget` nodes (including this one). Recurses
// into children with allocations derived from samples share; drops
// children that don't fit. Returns the number of kept nodes in the
// subtree (1 for self plus all kept descendants).
func pruneBudget(node *FlameNode, budget int) int {
	if budget <= 1 {
		node.Children = nil
		return 1
	}
	if len(node.Children) == 0 {
		return 1
	}
	totalChildSamples := 0
	for i := range node.Children {
		totalChildSamples += node.Children[i].Samples
	}
	if totalChildSamples <= 0 {
		node.Children = nil
		return 1
	}
	remaining := budget - 1
	used := 1

	kept := node.Children[:0]
	droppedSamples := 0
	droppedCount := 0

	for i := range node.Children {
		c := node.Children[i]
		if remaining <= 0 {
			droppedSamples += c.Samples
			droppedCount += 1 + countChildrenDeep(c)
			continue
		}
		// Proportional share — but every kept child needs at least 1.
		ratio := float64(c.Samples) / float64(totalChildSamples)
		alloc := int(float64(budget-1) * ratio)
		if alloc < 1 {
			alloc = 1
		}
		if alloc > remaining {
			alloc = remaining
		}
		// One last guard: very small children (alloc would round to 0
		// in the budget but we forced 1) still drop if there's no
		// remaining budget for at least themselves.
		if alloc < 1 {
			droppedSamples += c.Samples
			droppedCount += 1 + countChildrenDeep(c)
			continue
		}
		actual := pruneBudget(&c, alloc)
		kept = append(kept, c)
		remaining -= actual
		used += actual
	}
	if droppedSamples > 0 && remaining >= 1 {
		kept = append(kept, FlameNode{
			ID:      node.ID + "/...other",
			Name:    fmt.Sprintf("…other (%d frames)", droppedCount),
			Samples: droppedSamples,
			Ratio:   float64(droppedSamples) / float64(maxInt(node.Samples, 1)),
		})
		used += 1
	}
	node.Children = kept
	return used
}

func countChildrenDeep(node FlameNode) int {
	n := 0
	for i := range node.Children {
		n += 1 + countChildrenDeep(node.Children[i])
	}
	return n
}

func maxInt(a, b int) int {
	if a >= b {
		return a
	}
	return b
}

func iterLeafPaths(root FlameNode) []leafPath {
	out := []leafPath{}
	var walk func(node FlameNode)
	walk = func(node FlameNode) {
		childTotal := 0
		for _, child := range node.Children {
			childTotal += child.Samples
		}
		exclusive := node.Samples - childTotal
		// Memory: share the Path slice from the FlameNode rather
		// than cloning. The flame tree is read-only after freeze, so
		// downstream consumers (timeline, top-stacks, classifier)
		// only read these slices. This is the single largest
		// allocation drop on 70MB wall inputs.
		if len(node.Path) > 0 && exclusive > 0 {
			out = append(out, leafPath{Path: node.Path, Samples: exclusive})
		}
		if len(node.Children) == 0 && len(node.Path) > 0 && node.Samples > 0 && exclusive <= 0 {
			out = append(out, leafPath{Path: node.Path, Samples: node.Samples})
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(root)
	return out
}

func topStacksFromTree(root FlameNode, limit int) []TopItem {
	leaves := iterLeafPaths(root)
	sort.Slice(leaves, func(i, j int) bool {
		if leaves[i].Samples == leaves[j].Samples {
			return joinPath(leaves[i].Path) < joinPath(leaves[j].Path)
		}
		return leaves[i].Samples > leaves[j].Samples
	})
	if limit > 0 && len(leaves) > limit {
		leaves = leaves[:limit]
	}
	out := make([]TopItem, 0, len(leaves))
	for _, leaf := range leaves {
		out = append(out, TopItem{Name: joinPath(leaf.Path), Samples: leaf.Samples})
	}
	return out
}

func topChildFrames(root FlameNode, limit int) []TopChildFrameRow {
	children := append([]FlameNode(nil), root.Children...)
	sort.Slice(children, func(i, j int) bool {
		if children[i].Samples == children[j].Samples {
			return children[i].Name < children[j].Name
		}
		return children[i].Samples > children[j].Samples
	})
	if limit > 0 && len(children) > limit {
		children = children[:limit]
	}
	out := make([]TopChildFrameRow, 0, len(children))
	for _, child := range children {
		out = append(out, TopChildFrameRow{Frame: child.Name, Samples: child.Samples, Ratio: child.Ratio})
	}
	return out
}

func nodeID(path []string) string {
	parts := make([]string, 0, len(path))
	for _, item := range path {
		parts = append(parts, slug(item))
	}
	return "frame:" + strings.Join(parts, "/")
}

func slug(value string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ";", "_", " ", "_")
	out := replacer.Replace(value)
	if len(out) > 160 {
		return out[:160]
	}
	return out
}
