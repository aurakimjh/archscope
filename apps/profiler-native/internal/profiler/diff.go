package profiler

import (
	"math"
	"sort"
	"strings"
	"time"
)

// DiffOptions controls profiler diff behaviour.
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
