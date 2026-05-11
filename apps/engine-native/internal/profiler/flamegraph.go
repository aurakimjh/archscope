// ─────────────────────────────────────────────────────────────────────
// [한글] flamegraph — collapsed stack map 을 트리(FlameNode)로 변환.
//
// 책임/목적
//   profiler 의 핵심 데이터 구조인 "flame tree" 를 만든다.
//   입력: map[stackKey]samples 형태의 collapsed stack 카운터
//         (stackKey 예: "frame1;frame2;leaf")
//   출력: FlameNode (재귀 트리). 각 노드는 ID/ParentID/Name/Samples/
//         Ratio/Path/Children 을 갖는다.
//
// 알고리즘 흐름
//   1. mutableNode 트리(map 기반)로 누적
//      - 각 collapsed key 를 ";" 로 split → 프레임 시퀀스
//      - 루트부터 한 프레임씩 내려가며 children map 으로 인덱싱
//      - 동일 child 가 이미 있으면 Samples 누적 (inclusive count)
//   2. freezeNode 로 immutable FlameNode 트리화
//      - children 을 Samples DESC, 동률이면 Name ASC 정렬
//      - 각 노드의 Ratio = Samples / total * 100 (소수 4자리 round)
//   3. iterLeafPaths 는 분석 단계에서 사용 — 각 leaf 의 exclusive
//      sample count 를 계산해 breakdown/topN 등을 지원.
//
// 주요 타입/함수
//   - mutableNode  : 빌드 단계의 in-place mutable 트리
//   - buildFlameTree : 외부 진입점 (stacks → FlameNode)
//   - freezeNode   : mutable → immutable 변환 + 정렬 + ratio 계산
//   - iterLeafPaths: 트리에서 leaf path 추출 (exclusive samples)
//   - topStacksFromTree / topChildFrames : 표 데이터 산출
//   - nodeID / slug : DOM-safe ID 생성 (160자 cap, 특수문자 치환)
//
// 트리키한 부분
//   • Samples 는 inclusive (해당 노드 + 하위 트리 합). exclusive 는
//     iterLeafPaths 에서 (parent.Samples - sum(child.Samples)) 로 계산.
//   • iterLeafPaths 는 "중간 노드도 exclusive>0 이면 leaf 로 취급"
//     하는 점이 트리키. 한 stack 이 다른 stack 의 prefix 인 경우를 처리.
//   • slug 는 단순 치환만 한다. URL-safe 가 아닌 DOM-id-safe 이므로
//     Python 측과 byte 단위로 일치해야 한다.
// ─────────────────────────────────────────────────────────────────────

package profiler

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
)

// mutableNode is the in-progress tree node. Memory-tight on
// purpose — historically this carried a per-node Path []string clone
// that ballooned to multi-GB on 60 MB+ wall inputs (1M+ nodes ×
// avg-50-frame paths). The path is now reconstructed during
// freezeNode from the recursion stack, which costs nothing extra
// because freezeNode already walks parent→child top-down.
//
// [한글] mutableNode — 트리 빌드 단계에서만 쓰이는 mutable 표현.
// children 을 map 으로 들고 있어 O(1) 인덱싱이 가능. freezeNode 단계
// 에서 sorted slice 기반 FlameNode 로 변환.
//
// 메모리 회귀 주의: 과거에는 노드별 Path []string 복제본을 들고 있어
// 60MB+ wall 입력 (1M+ 노드 × 50프레임 경로) 에서 multi-GB 폭주 발생.
// 현재는 freezeNode 의 재귀 스택에서 path 재구성하므로 추가 비용 0.
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
//   - splitStack returns a slice that lives only until the next
//     call; we don't retain it.
//   - Node IDs are short hashes derived from parent ID + frame name.
//     Keeping full path strings in every node was a large multiplier
//     once the depth cap was raised for wall profiles.
//
// [한글] buildFlameTree — collapsed stack 카운터를 FlameNode 트리로.
// 매개변수 stacks: map[";"-joined frame sequence]samples.
// 동작: 음수/0 sample 은 무시. 각 stack 을 ";" split 하여 trie 형태로
// 누적, freezeNode 로 정렬·ratio 계산까지 끝낸 immutable 트리 반환.
// 400 MB+ wall profile 의 hot path 이므로 split 결과 재사용, 짧은 ID 등
// allocation 최소화 패턴 적용.
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
	path := make([]string, 0, 512)
	consumed := 0
	const tickEvery = 25_000
	totalNodes := 0
	for stack, samples := range stacks {
		if samples <= 0 {
			continue
		}
		current := root
		frames := splitStackInto(path[:0], stack)
		for _, frame := range frames {
			child := current.Children[frame]
			if child == nil {
				parentID := current.ID
				child = &mutableNode{
					ID:       childNodeID(current.ID, frame),
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
		path = frames
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
	return freezeNode(root, max(total, 1))
}

// freezeNode reconstructs the immutable FlameNode from the in-progress
// mutableNode tree. Path is no longer stored per node. Consumers that
// need full paths call iterLeafPaths, which reconstructs and clones
// only leaf paths.
//
// [한글] freezeNode — mutableNode 재귀 → FlameNode 재귀 변환.
// children 을 Samples DESC (동률이면 Name ASC) 로 정렬해 deterministic
// 한 결과를 보장하고, 각 노드의 Ratio 를 total 대비 백분율 계산.
// total=0 가 되면 ratio 가 NaN 이 되므로 호출부에서 max(total,1) 로 보호.
// Path slice 는 leaf 계산 시점에만 재구성하여 deep stack 에서 메모리를 절감.
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
	out := FlameNode{
		ID:       node.ID,
		ParentID: node.ParentID,
		Name:     node.Name,
		Samples:  node.Samples,
		Ratio:    ratio(node.Samples, total, 4),
		Children: make([]FlameNode, 0, len(children)),
	}
	for _, child := range children {
		out.Children = append(out.Children, freezeNode(child, total))
	}
	return out
}

// countNodes walks the frozen tree and returns the total number of
// nodes. Cheap (O(N)) and useful for the IPC-payload sanity check.
//
// [한글] countNodes — 트리 전체 노드 수 O(N) 카운트. IPC payload 크기
// sanity check 와 pruneFlamegraph 의 before/after 비교에 사용.
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
//
// [한글] pruneFlamegraph — 트리 노드 수를 maxNodes 이하로 제한.
// 알고리즘: greedy budget propagation. 각 subtree 가 자기 sample 비율
// 만큼의 budget 을 받음. 옛 per-level cap (256 children) 은 deep tree
// 에서 256^depth 폭주를 못 막았음 — 60MB wall profile 이 best-effort
// prune 후에도 1M 노드 도달. budget 기반 알고리즘은 총 노드 수가 진짜로
// maxNodes 이하임을 보장. 잘려나간 노드들은 "…other (N frames)" 하나로
// merge 해 사용자가 무엇이 잘렸는지 인식 가능.
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
//
// [한글] pruneBudget — budget 노드 (자기 포함) 를 소비하고 keep 수 반환.
// children 에게 sample 비율로 budget 분배, 매 child 는 최소 1 보장.
// 못 들어간 children 은 droppedSamples 에 누적되고 마지막에 한 "…other"
// sibling 으로 merge.
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

// [한글] iterLeafPaths — 트리에서 leaf path 들을 exclusive sample 단위로.
// 한 stack 이 다른 stack 의 prefix 인 경우(예: "A;B" 가 "A;B;C" 의 부분
// 집합) 도 처리하기 위해, 중간 노드라도 exclusive>0 이면 leaf 로 보고.
// 진짜 자식이 없는 leaf 는 (예외적으로 exclusive<=0 인 케이스 포함) 자기
// Samples 를 그대로 사용. 결과는 breakdown/top stacks 계산의 기반.
// 메모리 절감을 위해 FlameNode 마다 Path 를 보관하지 않고, DFS 중 현재
// 경로를 재구성한 뒤 leaf 로 채택되는 순간에만 clone 한다.
func iterLeafPaths(root FlameNode) []leafPath {
	out := []leafPath{}
	path := make([]string, 0, 64)
	var walk func(node FlameNode, current []string, isRoot bool)
	walk = func(node FlameNode, current []string, isRoot bool) {
		next := current
		if isRoot && len(node.Path) > 0 {
			next = append([]string(nil), node.Path...)
		} else if !isRoot {
			next = append(current, node.Name)
		}
		childTotal := 0
		for _, child := range node.Children {
			childTotal += child.Samples
		}
		exclusive := node.Samples - childTotal
		if len(next) > 0 && exclusive > 0 {
			out = append(out, leafPath{Path: append([]string(nil), next...), Samples: exclusive})
		}
		if len(node.Children) == 0 && len(next) > 0 && node.Samples > 0 && exclusive <= 0 {
			out = append(out, leafPath{Path: append([]string(nil), next...), Samples: node.Samples})
		}
		for _, child := range node.Children {
			walk(child, next, false)
		}
	}
	walk(root, path, true)
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

// [한글] childNodeID / slug — DOM 에서 사용할 노드 식별자 생성.
// 예전에는 전체 path 를 ID 문자열로 들고 있어 depth 가 큰 wall profile 에서
// 메모리 사용량이 커졌다. parentID + frame 을 해시해 짧고 안정적인 ID 를
// 만들고, 사람이 읽을 수 있도록 마지막 frame slug 를 앞에 붙인다.
func childNodeID(parentID string, frame string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(parentID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(frame))
	name := slug(frame)
	if len(name) > 80 {
		name = name[:80]
	}
	return "frame:" + name + "-" + strconv.FormatUint(h.Sum64(), 16)
}

func slug(value string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ";", "_", " ", "_")
	out := replacer.Replace(value)
	if len(out) > 160 {
		return out[:160]
	}
	return out
}
