package profiler

func buildExecutionBreakdown(root FlameNode, options Options, originalTotal int, parentTotal int) []ExecutionBreakdownRow {
	byCategory := map[string]int{}
	methods := map[string]map[string]int{}
	stacks := map[string]map[string]int{}
	classifications := map[string]stackClassification{}

	for _, leaf := range iterLeafPaths(root) {
		classification := classifyFrames(leaf.Path)
		key := classification.PrimaryCategory
		byCategory[key] += leaf.Samples
		if methods[key] == nil {
			methods[key] = map[string]int{}
			stacks[key] = map[string]int{}
			classifications[key] = classification
		}
		increment(methods[key], methodName(leaf.Path), leaf.Samples)
		increment(stacks[key], joinPath(leaf.Path), leaf.Samples)
	}

	rows := []ExecutionBreakdownRow{}
	intervalSeconds := options.IntervalMS / 1000
	for _, key := range sortedSegmentsBySamples(byCategory) {
		samples := byCategory[key]
		estimated := round(float64(samples)*intervalSeconds, 3)
		classification := classifications[key]
		rows = append(rows, ExecutionBreakdownRow{
			Category:         key,
			ExecutiveLabel:   classification.Label,
			PrimaryCategory:  key,
			WaitReason:       classification.WaitReason,
			Samples:          samples,
			EstimatedSeconds: estimated,
			TotalRatio:       ratio(samples, originalTotal, 4),
			ParentStageRatio: ratio(samples, parentTotal, 4),
			ElapsedRatio:     elapsedRatio(estimated, options.ElapsedSec, 4),
			TopMethods:       topCounter(methods[key], options.TopN),
			TopStacks:        topCounter(stacks[key], options.TopN),
		})
	}
	return rows
}

func componentBreakdown(stacks map[string]int) []ComponentBreakdownRow {
	counter := map[string]int{}
	for stack, samples := range stacks {
		counter[classifyStack(stack)] += samples
	}
	top := topCounter(counter, 0)
	out := make([]ComponentBreakdownRow, 0, len(top))
	for _, item := range top {
		out = append(out, ComponentBreakdownRow{Component: item.Name, Samples: item.Samples})
	}
	return out
}
