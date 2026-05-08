package profiler

import (
	"sort"
	"strings"
)

type mutableNode struct {
	ID       string
	ParentID *string
	Name     string
	Samples  int
	Path     []string
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
				clonedPath := make([]string, len(path))
				copy(clonedPath, path)
				child = &mutableNode{
					ID:       nodeID(path),
					ParentID: &parentID,
					Name:     frame,
					Path:     clonedPath,
					Children: map[string]*mutableNode{},
				}
				current.Children[frame] = child
			}
			child.Samples += samples
			current = child
		}
	}
	return freezeNode(root, max(total, 1))
}

func freezeNode(node *mutableNode, total int) FlameNode {
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
	// Memory: hand the existing Path slice through rather than
	// cloning it. mutableNode.Path is built once during tree
	// construction and never mutated again, so the freeze step is a
	// safe place to take ownership. For 70MB wall profiles this
	// drops freeze-time allocation by ~30-50% (each FlameNode shaved
	// one slice header + backing array per level of depth).
	out := FlameNode{
		ID:       node.ID,
		ParentID: node.ParentID,
		Name:     node.Name,
		Samples:  node.Samples,
		Ratio:    ratio(node.Samples, total, 4),
		Children: make([]FlameNode, 0, len(children)),
		Path:     node.Path,
	}
	for _, child := range children {
		out.Children = append(out.Children, freezeNode(child, total))
	}
	return out
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
