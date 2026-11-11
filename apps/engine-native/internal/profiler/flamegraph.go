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
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
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
type mutableNode = profilemodel.TrieNode[int]

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

	trie := profilemodel.NewTrie("root", "All", func() int { return 0 }, func(a, b int) int { return a + b }, childNodeID)
	path := make([]string, 0, 512)
	consumed := 0
	for stack, samples := range stacks {
		if samples <= 0 {
			continue
		}
		frames := splitStackInto(path[:0], stack)
		trie.Add(frames, samples)
		path = frames
		consumed++
		if pl != nil && consumed%25_000 == 0 {
			pl.Tick("build stacks=%d/%d nodes=%d", consumed, len(stacks), trie.Nodes-1)
			pl.Mem("build")
		}
	}
	if pl != nil {
		pl.Tick("build done: stacks=%d nodes=%d total_samples=%d", consumed, trie.Nodes-1, trie.Root.Value)
		pl.Mem("build-end")
	}
	return freezeNode(trie.Root, max(trie.Root.Value, 1))
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
		if children[i].Value == children[j].Value {
			return children[i].Name < children[j].Name
		}
		return children[i].Value > children[j].Value
	})
	out := FlameNode{
		ID:       node.ID,
		ParentID: node.ParentID,
		Name:     node.Name,
		Samples:  node.Value,
		Ratio:    ratio(node.Value, total, 4),
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

// pruneFlamegraph caps display nodes using a proportional child budget.
// Omitted descendant samples remain in the parent's inclusive Samples and
// are disclosed separately as HiddenSamples, even with a one-node budget.
// Fresh child slices keep canonical filter input untouched.
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

// boundedTree creates fresh child slices: pruning a display stage must never
// mutate the canonical tree used by the next filter. Any omitted descendant
// weight is explicit, even when there is no budget left for an "other" node.
func boundedTree(node FlameNode, budget int) (FlameNode, int) {
	out := node
	out.Children = nil
	used := 1
	total := 0
	for _, child := range node.Children {
		total += child.Samples
	}
	remaining := budget - 1
	for _, child := range node.Children {
		if remaining <= 0 {
			out.HiddenSamples += child.Samples
			continue
		}
		allocation := 1
		if total > 0 {
			allocation = max(1, int(float64(budget-1)*float64(child.Samples)/float64(total)))
		}
		allocation = min(allocation, remaining)
		next, n := boundedTree(child, allocation)
		out.Children = append(out.Children, next)
		used += n
		remaining -= n
	}
	return out, used
}

func pruneBudget(node *FlameNode, budget int) int {
	next, used := boundedTree(*node, budget)
	*node = next
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
		exclusive := node.Samples - childTotal - node.HiddenSamples
		if len(next) > 0 && exclusive > 0 {
			out = append(out, leafPath{Path: append([]string(nil), next...), Samples: exclusive})
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
