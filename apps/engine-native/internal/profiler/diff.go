// ─────────────────────────────────────────────────────────────────────
// [한글] diff — 두 profiler 결과(baseline vs target) 비교 분석.
//
// 책임/목적
//   같은 애플리케이션의 두 시점/버전 profiler 결과를 한 화면에서
//   비교한다. flame tree 의 각 노드에 baseline(a) / target(b) / delta
//   값을 동시에 부착해 어디서 좋아졌고 나빠졌는지 한눈에 보이게.
//
// 알고리즘 흐름
//   1. baseTotal/targetTotal 계산 → Normalize=true 면 각 사이드를
//      자기 합계로 나눠 ratio(0~1) 로 변환 (target 이 더 길게 돌았다고
//      해서 시각적으로 baseline 을 압도하지 않도록).
//   2. diffNode trie 에 baseline 은 A 채널, target 은 B 채널로 누적.
//   3. freezeDiff 로 immutable FlameNode 트리화 — 정렬 기준은
//      max(A,B) DESC. 노드별 metadata={a,b,delta} 부착.
//      샘플 수는 Python 호환을 위해 max(A,B) * 1_000_000 정수화.
//   4. 모든 노드 path 를 평탄화한 뒤 delta DESC/ASC 로 두 번 정렬해
//      biggest_increases / biggest_decreases 표 (각 30개 cap).
//   5. summary 에 baseline_total/target_total/normalized/common/added/
//      removed counts + max_increase/max_decrease 정보 채움.
//
// 주요 함수
//   - BuildDiffFlameTree    : 핵심 진입점, FlameNode + parts 반환
//   - freezeDiff            : mutable diffNode → immutable FlameNode
//   - rowToMap              : diff row 직렬화 (delta_ratio 계산 포함)
//   - AnalyzeProfilerDiff   : AnalysisResult 형태로 wrapping
//   - withDiffPayload       : 공통 AnalysisResult 에 diff 전용 필드 부착
//
// 트리키한 부분
//   • Normalize 모드에서 sample 수가 ratio(<1) 이지만 FlameNode.Samples
//     는 int 라 정밀도 보존을 위해 *1_000_000 후 round.
//   • freezeDiff 의 root 보정: 입력 시 Path=[] 인 root 는 자식 합으로
//     A/B 를 보정해야 정확한 delta 를 가진다.
//   • delta_ratio 계산은 baseline=0 이고 delta>0 이면 +∞ 로 마킹.
//   • Python 측 BiggestIncreases/Decreases 는 delta 0 인 노드는 break.
// ─────────────────────────────────────────────────────────────────────

package profiler

import (
	"math"
	"sort"
	"strings"
	"time"
)

// DiffOptions controls profiler diff behaviour.
//
// [한글] Normalize=true 면 양쪽 합계로 나눠 ratio(0~1) 비교, false 면
// raw sample 비교. 보통 비교 시 길이가 다르면 true 가 디폴트.
type DiffOptions struct {
	Normalize bool
}

type diffNode struct {
	Name     string
	A        float64
	B        float64
	Path     []string
	Children map[string]*diffNode
}

type diffRow struct {
	Path  string
	A     float64
	B     float64
	Delta float64
}

// BuildDiffFlameTree mirrors profiler_diff.build_diff_flame_tree.
//
// Both inputs are stack maps keyed `frame1;frame2;...;leaf`. When
// `Normalize` is true (default) each side is divided by its total before the
// subtraction so a 10x longer target run does not visually drown the
// baseline.
//
// The returned `FlameNode` carries per-node `metadata = { a, b, delta }`.
//
// [한글] BuildDiffFlameTree — 두 stack map 의 비교 트리 + summary/tables.
// 반환 (flameRoot, parts) 에서 parts["summary"], parts["tables"] 가
// 비교 표 데이터. flameRoot 의 각 노드 metadata 는 {a,b,delta} 를 들고 있어
// UI 에서 색칠할 때 사용.
func BuildDiffFlameTree(baseline, target map[string]int, options DiffOptions) (FlameNode, map[string]any) {
	var baseTotal, targetTotal int
	for _, v := range baseline {
		baseTotal += v
	}
	for _, v := range target {
		targetTotal += v
	}

	aScale, bScale := 1.0, 1.0
	if options.Normalize && baseTotal > 0 && targetTotal > 0 {
		aScale = 1.0 / float64(baseTotal)
		bScale = 1.0 / float64(targetTotal)
	}

	root := &diffNode{Name: "All", Children: map[string]*diffNode{}}

	add := func(stacks map[string]int, scale float64, side string) {
		for stack, samples := range stacks {
			if samples <= 0 {
				continue
			}
			value := float64(samples) * scale
			current := root
			path := []string{}
			for _, frame := range strings.Split(stack, ";") {
				if frame == "" {
					continue
				}
				path = append(path, frame)
				child := current.Children[frame]
				if child == nil {
					child = &diffNode{
						Name:     frame,
						Path:     append([]string(nil), path...),
						Children: map[string]*diffNode{},
					}
					current.Children[frame] = child
				}
				if side == "a" {
					child.A += value
				} else {
					child.B += value
				}
				current = child
			}
		}
	}

	add(baseline, aScale, "a")
	add(target, bScale, "b")

	flat := []diffRow{}
	flameRoot, rootTotal := freezeDiff(root, &flat)

	gains := append([]diffRow(nil), flat...)
	losses := append([]diffRow(nil), flat...)
	sort.Slice(gains, func(i, j int) bool { return gains[i].Delta > gains[j].Delta })
	sort.Slice(losses, func(i, j int) bool { return losses[i].Delta < losses[j].Delta })

	common, added, removed := 0, 0, 0
	for _, row := range flat {
		switch {
		case row.A > 0 && row.B > 0:
			common++
		case row.A == 0 && row.B > 0:
			added++
		case row.A > 0 && row.B == 0:
			removed++
		}
	}

	totalUnit := "samples"
	if options.Normalize {
		totalUnit = "ratio"
	}

	summary := map[string]any{
		"baseline_total": baseTotal,
		"target_total":   targetTotal,
		"normalized":     options.Normalize,
		"common_paths":   common,
		"added_paths":    added,
		"removed_paths":  removed,
		"max_increase":   nil,
		"max_decrease":   nil,
		"total_unit":     totalUnit,
		"tree_total":     rootTotal,
	}
	if len(gains) > 0 && gains[0].Delta > 0 {
		summary["max_increase"] = rowToMap(gains[0])
	}
	if len(losses) > 0 && losses[0].Delta < 0 {
		summary["max_decrease"] = rowToMap(losses[0])
	}

	biggestIncreases := []map[string]any{}
	for _, row := range gains {
		if row.Delta <= 0 {
			break
		}
		biggestIncreases = append(biggestIncreases, rowToMap(row))
		if len(biggestIncreases) >= 30 {
			break
		}
	}
	biggestDecreases := []map[string]any{}
	for _, row := range losses {
		if row.Delta >= 0 {
			break
		}
		biggestDecreases = append(biggestDecreases, rowToMap(row))
		if len(biggestDecreases) >= 30 {
			break
		}
	}

	parts := map[string]any{
		"summary": summary,
		"tables": map[string]any{
			"biggest_increases": biggestIncreases,
			"biggest_decreases": biggestDecreases,
		},
	}
	return flameRoot, parts
}

// [한글] freezeDiff — diffNode 재귀를 FlameNode 재귀로 변환하면서 flat
// (모든 path 평탄화 리스트) 도 동시에 누적. 정렬 키는 max(A,B) DESC.
// root(Path=[]) 인 경우 자식 합으로 A/B 보정 (부모는 자식 누적치를 가져야
// 정확한 delta 가 나오기 때문). FlameNode.Samples 는 max(A,B)*1e6 으로
// 정수화해 ratio 가 매우 작은 경우에도 정밀도 보존.
func freezeDiff(node *diffNode, flat *[]diffRow) (FlameNode, float64) {
	children := make([]*diffNode, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		return math.Max(children[i].A, children[i].B) > math.Max(children[j].A, children[j].B)
	})

	childNodes := make([]FlameNode, 0, len(children))
	sumA, sumB := 0.0, 0.0
	for _, child := range children {
		sumA += child.A
		sumB += child.B
		if len(child.Path) > 0 {
			*flat = append(*flat, diffRow{
				Path:  strings.Join(child.Path, ";"),
				A:     child.A,
				B:     child.B,
				Delta: child.B - child.A,
			})
		}
		childNode, _ := freezeDiff(child, flat)
		childNodes = append(childNodes, childNode)
	}

	if len(node.Path) == 0 {
		node.A = math.Max(node.A, sumA)
		node.B = math.Max(node.B, sumB)
	}

	delta := node.B - node.A
	samples := math.Max(node.A, node.B)

	id := "root"
	var parentID *string
	if len(node.Path) > 0 {
		id = strings.Join(node.Path, ";")
		var pid string
		if len(node.Path) > 1 {
			pid = strings.Join(node.Path[:len(node.Path)-1], ";")
		} else {
			pid = "root"
		}
		parentID = &pid
	}

	samplesInt := 0
	if samples > 0 {
		samplesInt = int(math.Round(samples * 1_000_000))
	}

	flameNode := FlameNode{
		ID:       id,
		ParentID: parentID,
		Name:     node.Name,
		Samples:  samplesInt,
		Ratio:    0.0,
		Path:     append([]string(nil), node.Path...),
		Children: childNodes,
		Metadata: map[string]any{
			"a":     round(node.A, 6),
			"b":     round(node.B, 6),
			"delta": round(delta, 6),
		},
	}
	return flameNode, math.Max(node.A, node.B)
}

func rowToMap(row diffRow) map[string]any {
	deltaRatio := 0.0
	switch {
	case row.A > 0:
		deltaRatio = round(row.Delta/row.A, 4)
	case row.Delta > 0:
		deltaRatio = math.Inf(1)
	}
	return map[string]any{
		"stack":       row.Path,
		"baseline":    round(row.A, 6),
		"target":      round(row.B, 6),
		"delta":       round(row.Delta, 6),
		"delta_ratio": deltaRatio,
	}
}

// AnalyzeProfilerDiff produces a `profiler_diff` AnalysisResult from the two
// stack maps.
//
// [한글] AnalyzeProfilerDiff — diff 트리/표를 공통 AnalysisResult 형식
// 으로 wrapping. Type="profiler_diff", parser="profiler_diff",
// metadata.diff_summary/diff_tables 에 비교 전용 데이터 부착.
// 다른 분석기와 동일한 AnalysisResult 컨트랙트를 유지해 export/UI 가
// 동일 코드 경로로 처리할 수 있게 한다.
func AnalyzeProfilerDiff(baseline, target map[string]int, baselinePath, targetPath string, options DiffOptions) AnalysisResult {
	flameRoot, parts := BuildDiffFlameTree(baseline, target, options)
	summary := parts["summary"].(map[string]any)
	tables := parts["tables"].(map[string]any)

	sourceFiles := []string{}
	if baselinePath != "" {
		sourceFiles = append(sourceFiles, baselinePath)
	}
	if targetPath != "" {
		sourceFiles = append(sourceFiles, targetPath)
	}

	totalSamples := 0
	if v, ok := summary["target_total"].(int); ok {
		totalSamples = v
	}

	return AnalysisResult{
		Type:        "profiler_diff",
		SourceFiles: sourceFiles,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		Summary: Summary{
			ProfileKind:      "diff",
			TotalSamples:     totalSamples,
			IntervalMS:       0,
			EstimatedSeconds: 0,
		},
		Tables: Tables{
			TopStacks:        []TopStackTableRow{},
			TopChildFrames:   []TopChildFrameRow{},
			TimelineAnalysis: []TimelineRow{},
		},
		Charts: Charts{
			Flamegraph:      flameRoot,
			DrilldownStages: []DrilldownStage{},
		},
		Metadata: Metadata{
			Parser:        "profiler_diff",
			SchemaVersion: "0.1.0",
			Diagnostics: ParserDiagnostics{
				Format:          "profiler_diff",
				SkippedByReason: map[string]int{},
			},
			TimelineScope: TimelineScope{
				Mode:      "diff",
				MatchMode: "n/a",
				ViewMode:  "n/a",
				Warnings:  []TimelineWarning{},
			},
		},
		Series: Series{
			TopStacks:          []TopStackSeriesRow{},
			ComponentBreakdown: []ComponentBreakdownRow{},
			ExecutionBreakdown: []ExecutionBreakdownRow{},
			TimelineAnalysis:   []TimelineRow{},
			Threads:            []ThreadRow{},
		},
		// Pass through the diff-specific tables/summary in the metadata
		// payload so the renderer keeps a clean type-safe path while still
		// having access to the diff-shaped fields.
	}.withDiffPayload(summary, tables)
}

// withDiffPayload exposes the profiler_diff–specific summary/tables on the
// shared AnalysisResult shape. The renderer reaches them through
// `metadata.diff_summary` / `metadata.diff_tables` to keep the existing
// AnalysisResult contract stable.
func (r AnalysisResult) withDiffPayload(summary, tables map[string]any) AnalysisResult {
	r.Metadata.DiffSummary = summary
	r.Metadata.DiffTables = tables
	return r
}
