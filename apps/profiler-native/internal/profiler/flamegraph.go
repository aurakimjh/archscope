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

// pruneFlamegraph collapses the tail of each level's children when
// the total node count exceeds `maxNodes`. The tail is replaced with
// a single synthetic "...other (N nodes)" sibling whose Samples is
// the sum of the dropped children — that way the visual flame graph
// stays balanced and the user still sees how much aggregate weight
// was hidden. Pruning only fires for very large trees (typically
// 1M+ nodes); ordinary inputs pass through unchanged.
//
// We keep the topK children per level by descending Samples; the
// list is already sorted by freezeNode so we just slice. Recurses
// into kept children so deeper subtrees also get pruned.
func pruneFlamegraph(root *FlameNode, maxNodes int) (kept int, dropped int) {
	if root == nil {
		return 0, 0
	}
	total := countNodes(*root)
	if maxNodes <= 0 || total <= maxNodes {
		return total, 0
	}
	// Per-level cap derived from maxNodes; each kept node's children
	// are similarly capped. log scale because a uniform cap would
	// either flood depth-1 or starve depth-N. The exponent 1.4 was
	// chosen empirically on a 1.5M-node test tree to land near
	// maxNodes after one pass.
	perLevelCap := perLevelCapFor(maxNodes, total)
	dropped = pruneSubtree(root, perLevelCap)
	return countNodes(*root), dropped
}

func perLevelCapFor(maxNodes, total int) int {
	if total <= 0 || maxNodes <= 0 {
		return 50
	}
	cap := maxNodes / 100
	if cap < 32 {
		return 32
	}
	if cap > 256 {
		return 256
	}
	return cap
}

func pruneSubtree(node *FlameNode, perLevelCap int) int {
	if node == nil || len(node.Children) == 0 {
		return 0
	}
	dropped := 0
	if len(node.Children) > perLevelCap {
		tailSamples := 0
		tailCount := 0
		for i := perLevelCap; i < len(node.Children); i++ {
			tailSamples += node.Children[i].Samples
			tailCount += 1 + countChildrenDeep(node.Children[i])
		}
		dropped += tailCount
		kept := node.Children[:perLevelCap]
		if tailSamples > 0 {
			kept = append(kept, FlameNode{
				ID:      node.ID + "/...other",
				Name:    fmt.Sprintf("…other (%d frames)", len(node.Children)-perLevelCap),
				Samples: tailSamples,
				Ratio:   float64(tailSamples) / float64(maxInt(node.Samples, 1)),
			})
		}
		node.Children = kept
	}
	for i := range node.Children {
		dropped += pruneSubtree(&node.Children[i], perLevelCap)
	}
	return dropped
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
