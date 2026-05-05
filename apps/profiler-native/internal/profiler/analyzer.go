package profiler

import (
	"sort"
	"strings"
	"time"
)

func AnalyzeCollapsedFile(path string, options Options) (AnalysisResult, error) {
	if options.IntervalMS <= 0 {
		options.IntervalMS = 100
	}
	if options.TopN <= 0 {
		options.TopN = 20
	}
	if strings.TrimSpace(options.ProfileKind) == "" {
		options.ProfileKind = "wall"
	}
	stacks, diagnostics, err := ParseCollapsedFile(path)
	if err != nil {
		return AnalysisResult{}, err
	}
	return AnalyzeCollapsedStacks(stacks, path, diagnostics, options), nil
}

func AnalyzeCollapsedStacks(stacks map[string]int, sourceFile string, diagnostics ParserDiagnostics, options Options) AnalysisResult {
	totalSamples := 0
	for _, samples := range stacks {
		if samples > 0 {
			totalSamples += samples
		}
	}
	intervalSeconds := options.IntervalMS / 1000
	estimatedSeconds := round(float64(totalSamples)*intervalSeconds, 3)
	flamegraph := buildFlameTree(stacks)
	timelineRows, timelineScope := buildTimeline(flamegraph, options, totalSamples)
	topSeries, topTables := topStacksFromCollapsed(stacks, totalSamples, intervalSeconds, options)
	rootTopStacks := topStacksFromTree(flamegraph, options.TopN)
	rootStage := DrilldownStage{
		Index:      0,
		Label:      "All",
		Breadcrumb: []string{"All"},
		Filter:     nil,
		Metrics: map[string]any{
			"total_samples":      totalSamples,
			"matched_samples":    flamegraph.Samples,
			"estimated_seconds":  estimatedSeconds,
			"total_ratio":        100.0,
			"parent_stage_ratio": 100.0,
			"elapsed_ratio":      elapsedRatio(estimatedSeconds, options.ElapsedSec, 4),
		},
		Flamegraph:     flamegraph,
		TopStacks:      rootTopStacks,
		TopChildFrames: topChildFrames(flamegraph, options.TopN),
		Diagnostics:    nil,
	}
	return AnalysisResult{
		Type:        "profiler_collapsed",
		SourceFiles: []string{sourceFile},
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		Summary: Summary{
			ProfileKind:      options.ProfileKind,
			TotalSamples:     totalSamples,
			IntervalMS:       options.IntervalMS,
			EstimatedSeconds: estimatedSeconds,
			ElapsedSeconds:   options.ElapsedSec,
		},
		Series: Series{
			TopStacks:          topSeries,
			ComponentBreakdown: componentBreakdown(stacks),
			ExecutionBreakdown: buildExecutionBreakdown(flamegraph, options, totalSamples, max(flamegraph.Samples, 1)),
			TimelineAnalysis:   timelineRows,
			Threads:            detectThreads(stacks, totalSamples),
		},
		Tables: Tables{
			TopStacks:        topTables,
			TopChildFrames:   rootStage.TopChildFrames,
			TimelineAnalysis: timelineRows,
		},
		Charts: Charts{
			Flamegraph:      flamegraph,
			DrilldownStages: []DrilldownStage{rootStage},
		},
		Metadata: Metadata{
			Parser:        "async_profiler_collapsed",
			SchemaVersion: "0.1.0",
			Diagnostics:   diagnostics,
			TimelineScope: timelineScope,
		},
	}
}

func topStacksFromCollapsed(stacks map[string]int, totalSamples int, intervalSeconds float64, options Options) ([]TopStackSeriesRow, []TopStackTableRow) {
	type item struct {
		Stack   string
		Samples int
	}
	items := make([]item, 0, len(stacks))
	for stack, samples := range stacks {
		items = append(items, item{Stack: stack, Samples: samples})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Samples == items[j].Samples {
			return items[i].Stack < items[j].Stack
		}
		return items[i].Samples > items[j].Samples
	})
	if options.TopN > 0 && len(items) > options.TopN {
		items = items[:options.TopN]
	}
	series := make([]TopStackSeriesRow, 0, len(items))
	tables := make([]TopStackTableRow, 0, len(items))
	for _, item := range items {
		estimated := round(float64(item.Samples)*intervalSeconds, 3)
		elapsed := elapsedRatio(estimated, options.ElapsedSec, 2)
		sampleRatio := ratio(item.Samples, totalSamples, 2)
		series = append(series, TopStackSeriesRow{
			Stack:            item.Stack,
			Samples:          item.Samples,
			EstimatedSeconds: estimated,
			SampleRatio:      sampleRatio,
			ElapsedRatio:     elapsed,
		})
		tables = append(tables, TopStackTableRow{
			Stack:            item.Stack,
			Samples:          item.Samples,
			EstimatedSeconds: estimated,
			SampleRatio:      sampleRatio,
			ElapsedRatio:     elapsed,
			Frames:           splitStack(item.Stack),
		})
	}
	return series, tables
}

func detectThreads(stacks map[string]int, totalSamples int) []ThreadRow {
	if totalSamples <= 0 {
		return []ThreadRow{}
	}
	counts := map[string]int{}
	matched := 0
	for stack, samples := range stacks {
		first := stack
		if index := strings.Index(first, ";"); index >= 0 {
			first = first[:index]
		}
		if len(first) >= 3 && strings.HasPrefix(first, "[") && strings.Contains(first, "]") {
			name := first[1:strings.Index(first, "]")]
			counts[name] += samples
			matched += samples
		}
	}
	if float64(matched)/float64(totalSamples) < 0.8 {
		return []ThreadRow{}
	}
	top := topCounter(counts, 0)
	rows := make([]ThreadRow, 0, len(top))
	for _, item := range top {
		rows = append(rows, ThreadRow{
			Name:    item.Name,
			Samples: item.Samples,
			Ratio:   round(float64(item.Samples)/float64(totalSamples), 6),
		})
	}
	return rows
}
