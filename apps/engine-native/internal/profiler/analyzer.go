// ─────────────────────────────────────────────────────────────────────
// [한글] analyzer — profiler 패키지의 외부 진입점, 입력 형식별 분석 진입점.
//
// 책임/목적
//   - profiler 입력 4가지 형식(collapsed text / flamegraph HTML / SVG /
//     Jennifer CSV) 을 받아 단일 AnalysisResult (Type="profiler_collapsed"
//     또는 "profiler_jennifer") 형태로 반환.
//   - 분석 단계 (flame tree / timeline / breakdown / topN / drilldown
//     root stage / debug log) 를 한 곳에서 조립.
//
// 진입 함수
//   - AnalyzeCollapsedFile        : "frame1;frame2 12345" 형식 텍스트
//   - AnalyzeFlamegraphHTMLFile   : async-profiler / perf HTML
//   - AnalyzeFlamegraphSVGFile    : flamegraph.pl SVG
//   - AnalyzeJenniferFile         : Jennifer APM CSV
//
// 공통 흐름
//   1) normalizeOptions: IntervalMS=100, TopN=20, ProfileKind="wall" 기본값.
//   2) 형식별 parser → (stacks 또는 root, diagnostics) 산출.
//   3) AnalyzeCollapsedStacks 또는 AnalyzeJenniferTree 가 공통 조립:
//      flame tree 빌드 → timeline → topStacks → execution breakdown →
//      component breakdown → threads → drilldown root stage → metadata.
//   4) writeDebugLogIfNeeded 로 디버그 로그를 disk 에 기록.
//
// 트리키한 부분
//   • Jennifer 는 입력 자체가 트리라 buildFlameTree 를 호출하지 않고
//     parser 가 만든 root 를 그대로 사용. 다른 형식은 collapsed map 부터
//     buildFlameTree 로 트리화.
//   • detectThreads 는 stack key 의 첫 frame 이 "[threadName]" 패턴이면
//     thread 별 samples 합산. 80% 이상의 stack 이 이 패턴일 때만 thread
//     분석을 활성화 (false-positive 회피).
//   • Type/Parser 는 형식별로 덮어쓴다. HTML 은 Type="profiler_collapsed"
//     + Parser="flamegraph_html" 로 일관 처리.
// ─────────────────────────────────────────────────────────────────────

package profiler

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"
)

// rssMB returns current Alloc in MiB after a GC pass. The GC is
// cheap relative to the analyses we run it around, and it makes the
// MemStats reading reflect retained (not garbage) memory.
//
// [한글] rssMB — runtime.GC() 직후의 Alloc 크기를 MiB 로 보고.
// GC 한 번 돌리고 측정하므로 garbage 가 빠진 retained 메모리만 잡힘.
func rssMB() uint64 {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.Alloc / (1024 * 1024)
}

// checkRSSLimit returns a friendly error when current Alloc has
// crossed the configured ceiling. We deliberately fail closed — the
// alternative is an OS-level SIGKILL that leaves no usable artifact.
//
// [한글] checkRSSLimit — 메모리 ceiling 초과 시 친절한 에러 반환.
// fail-closed 정책 — 그냥 두면 OS SIGKILL 로 아무 산출물 없이 죽음.
// 사용자에게 MaxUniqueStacks/MaxStackDepth 조정 또는 MaxRSSMB 상향
// 가이드를 메시지에 포함.
func checkRSSLimit(options Options, phase string) error {
	limit := options.MaxRSSMB
	if limit <= 0 {
		limit = defaultMaxRSSMB
	}
	used := rssMB()
	if options.ProgressLog != nil {
		options.ProgressLog.Tick("rss-check phase=%s used=%dMB limit=%dMB", phase, used, limit)
	}
	if used >= uint64(limit) {
		return fmt.Errorf("memory ceiling reached during %s: %d MB allocated, limit %d MB. "+
			"Tighten MaxUniqueStacks / MaxStackDepth in the analyzer options or raise MaxRSSMB",
			phase, used, limit)
	}
	return nil
}

// [한글] normalizeOptions — 옵션의 비어있는/잘못된 필드에 default 적용.
// IntervalMS<=0 → 100ms (10Hz), TopN<=0 → 20, ProfileKind 빈 문자열 → "wall".
func normalizeOptions(options *Options) {
	if options.IntervalMS <= 0 {
		options.IntervalMS = 100
	}
	if options.TopN <= 0 {
		options.TopN = 20
	}
	if strings.TrimSpace(options.ProfileKind) == "" {
		options.ProfileKind = "wall"
	}
}

// writeDebugLogIfNeeded writes the debug log to disk when content was collected.
func writeDebugLogIfNeeded(options Options) {
	dl := options.DebugLog
	if dl == nil || !dl.HasContent() {
		return
	}
	dl.WriteJSON(options.DebugLogDir) //nolint:errcheck
}

// [한글] AnalyzeCollapsedFile — collapsed text 입력 진입점.
// "frame1;frame2;...;leaf 12345\n" 형식. ParseCollapsedFile 로 stack map
// 추출 후 AnalyzeCollapsedStacks 에 위임.
func AnalyzeCollapsedFile(path string, options Options) (AnalysisResult, error) {
	normalizeOptions(&options)

	// Auto-attach a progress log when the caller didn't supply one.
	// Large wall profiles (70 MB+) routinely take 30s+ to parse and
	// users need a tailable artifact to debug crashes; auto-opening
	// here means the debug-log path is always present in the result
	// metadata so the renderer can show it without any opt-in flag.
	autoLog := options.ProgressLog == nil
	if autoLog {
		if pl, plErr := OpenProgressLog(options.ProgressLogDir, path); plErr == nil {
			options.ProgressLog = pl
			defer pl.Close()
		}
	}
	pl := options.ProgressLog
	defer pl.Recover("AnalyzeCollapsedFile")

	stacks, diagnostics, err := ParseCollapsedFileWithOptions(path, options)
	if err != nil {
		return AnalysisResult{}, err
	}
	if rssErr := checkRSSLimit(options, "post-parse"); rssErr != nil {
		pl.Phase("rss-abort", rssErr.Error())
		return AnalysisResult{}, rssErr
	}
	pl.Phase("analyze-start", fmt.Sprintf("stacks=%d", len(stacks)))
	result := AnalyzeCollapsedStacks(stacks, path, diagnostics, options)
	// Drop the stacks map — the analyzer no longer needs it and the
	// upcoming JSON encode allocates aggressively. Releasing this
	// reference lets the next runtime.GC() pass reclaim ~ hundreds of
	// MB on big inputs.
	stacks = nil
	runtime.GC()
	pl.Mem("post-analyze-gc")
	if rssErr := checkRSSLimit(options, "post-analyze"); rssErr != nil {
		pl.Phase("rss-warning", rssErr.Error())
		// Don't abort here — analysis already produced a result we can
		// hand back. The phase line above is the breadcrumb for the
		// user; renderer should treat large results carefully.
	}
	pl.Phase("analyze-end")
	writeDebugLogIfNeeded(options)
	if pl != nil {
		result.Metadata.ProgressLogPath = pl.Path()
	}
	return result, nil
}

// [한글] AnalyzeFlamegraphHTMLFile — HTML 입력 진입점.
// ParseHtmlProfilerFile (inline SVG / async-profiler JS) 로 stack 추출 후
// AnalyzeCollapsedStacks 에 위임. Type="profiler_collapsed" + Parser="flamegraph_html".
func AnalyzeFlamegraphHTMLFile(path string, options Options) (AnalysisResult, error) {
	normalizeOptions(&options)
	if options.DebugLog != nil {
		options.DebugLog.SetEncodingDetected("utf-8")
	}
	parseResult, err := ParseHtmlProfilerFile(path, options.DebugLog)
	if err != nil {
		return AnalysisResult{}, err
	}
	result := AnalyzeCollapsedStacks(parseResult.Stacks, path, parseResult.Diagnostics, options)
	result.Type = "profiler_collapsed"
	result.Metadata.Parser = "flamegraph_html"
	result.Metadata.SchemaVersion = "0.1.0"
	writeDebugLogIfNeeded(options)
	return result, nil
}

// [한글] AnalyzeFlamegraphSVGFile — SVG 입력 진입점.
// ParseSvgFlamegraphFile (XML 트리에서 rect+title 추출) 로 stack 추출 후
// AnalyzeCollapsedStacks 에 위임. Type="profiler_collapsed" + Parser="flamegraph_svg".
func AnalyzeFlamegraphSVGFile(path string, options Options) (AnalysisResult, error) {
	normalizeOptions(&options)
	if options.DebugLog != nil {
		options.DebugLog.SetEncodingDetected("utf-8")
	}
	parseResult, err := ParseSvgFlamegraphFile(path, options.DebugLog)
	if err != nil {
		return AnalysisResult{}, err
	}
	result := AnalyzeCollapsedStacks(parseResult.Stacks, path, parseResult.Diagnostics, options)
	result.Type = "profiler_collapsed"
	result.Metadata.Parser = "flamegraph_svg"
	writeDebugLogIfNeeded(options)
	return result, nil
}

// [한글] AnalyzeJenniferFile — Jennifer CSV 입력 진입점.
// ParseJenniferFlamegraphCSV 가 이미 트리(FlameNode root) 를 만들어 주므로
// AnalyzeJenniferTree 에 그대로 위임. Type="profiler_jennifer".
func AnalyzeJenniferFile(path string, options Options) (AnalysisResult, error) {
	normalizeOptions(&options)
	parseResult, err := ParseJenniferFlamegraphCSV(path, options.DebugLog)
	if err != nil {
		return AnalysisResult{}, err
	}
	result := AnalyzeJenniferTree(parseResult.Root, path, parseResult.Diagnostics, options)
	writeDebugLogIfNeeded(options)
	return result, nil
}

// [한글] AnalyzeJenniferTree — Jennifer 입력에 특화된 분석 조립.
// AnalyzeCollapsedStacks 와 거의 동일하지만 buildFlameTree 호출 없이
// 입력 root 를 그대로 사용. component breakdown 을 위해 stacksFromTree 로
// leaf path 를 collapsed map 으로 역추출해 사용.
func AnalyzeJenniferTree(root FlameNode, sourceFile string, diagnostics ParserDiagnostics, options Options) AnalysisResult {
	totalSamples := root.Samples
	intervalSeconds := options.IntervalMS / 1000
	estimatedSeconds := round(float64(totalSamples)*intervalSeconds, 3)
	stacks := stacksFromTree(root)
	timelineRows, timelineScope := buildTimeline(root, options, totalSamples)
	topSeries, topTables := topStacksFromCollapsed(stacks, totalSamples, intervalSeconds, options)
	rootTopStacks := topStacksFromTree(root, options.TopN)
	topChildFramesRoot := topChildFrames(root, options.TopN)
	// Same pruning + DrilldownStage de-duplication as the collapsed
	// path — Jennifer CSV inputs can also produce huge trees.
	maxNodes := options.MaxFlamegraphNodes
	if maxNodes == 0 {
		maxNodes = defaultMaxFlamegraphNodes
	}
	if maxNodes > 0 {
		_, _ = pruneFlamegraph(&root, maxNodes)
	}
	rootStage := DrilldownStage{
		Index:      0,
		Label:      "All",
		Breadcrumb: []string{"All"},
		Filter:     nil,
		Metrics: map[string]any{
			"total_samples":      totalSamples,
			"matched_samples":    root.Samples,
			"estimated_seconds":  estimatedSeconds,
			"total_ratio":        100.0,
			"parent_stage_ratio": 100.0,
			"elapsed_ratio":      elapsedRatio(estimatedSeconds, options.ElapsedSec, 4),
		},
		Flamegraph:       FlameNode{},
		TimelineAnalysis: timelineRows,
		TopStacks:        rootTopStacks,
		TopChildFrames:   topChildFramesRoot,
		Diagnostics:      nil,
	}
	return AnalysisResult{
		Type:        "profiler_jennifer",
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
			ExecutionBreakdown: buildExecutionBreakdown(root, options, totalSamples, max(root.Samples, 1)),
			TimelineAnalysis:   timelineRows,
			Threads:            detectThreads(stacks, totalSamples),
		},
		Tables: Tables{
			TopStacks:        topTables,
			TopChildFrames:   topChildFramesRoot,
			TimelineAnalysis: timelineRows,
		},
		Charts: Charts{
			Flamegraph:      root,
			DrilldownStages: []DrilldownStage{rootStage},
		},
		Metadata: Metadata{
			Parser:        "jennifer_flamegraph_csv",
			SchemaVersion: "0.1.0",
			Diagnostics:   diagnostics,
			TimelineScope: timelineScope,
		},
	}
}

func stacksFromTree(root FlameNode) map[string]int {
	stacks := map[string]int{}
	leaves := iterLeafPaths(root)
	for _, leaf := range leaves {
		if len(leaf.Path) == 0 || leaf.Samples <= 0 {
			continue
		}
		stacks[joinPath(leaf.Path)] += leaf.Samples
	}
	return stacks
}

// [한글] AnalyzeCollapsedStacks — 4개 입력 모두 결국 거치는 공통 분석 조립.
// stacks 합계로 totalSamples, buildFlameTree 로 트리, buildTimeline / topStacks /
// breakdown / threads / drilldown root stage 를 모두 채워 AnalysisResult 반환.
// 호출 전후로 Type/Parser 를 호출자가 덮어쓰는 것이 정상 (HTML/SVG 케이스).
func AnalyzeCollapsedStacks(stacks map[string]int, sourceFile string, diagnostics ParserDiagnostics, options Options) AnalysisResult {
	pl := options.ProgressLog
	totalSamples := 0
	for _, samples := range stacks {
		if samples > 0 {
			totalSamples += samples
		}
	}
	intervalSeconds := options.IntervalMS / 1000
	estimatedSeconds := round(float64(totalSamples)*intervalSeconds, 3)
	pl.Phase("build-flamegraph-start", fmt.Sprintf("stacks=%d total_samples=%d", len(stacks), totalSamples))
	flamegraph := buildFlameTreeWithLog(stacks, pl)
	rawNodes := countNodes(flamegraph)
	pl.Phase("build-flamegraph-end", fmt.Sprintf("nodes_total=%d nodes_root=%d", rawNodes, len(flamegraph.Children)))
	pl.Mem("post-flamegraph")
	pl.Phase("timeline-start")
	timelineRows, timelineScope := buildTimeline(flamegraph, options, totalSamples)
	pl.Phase("timeline-end", fmt.Sprintf("rows=%d", len(timelineRows)))
	pl.Phase("topstacks-start")
	topSeries, topTables := topStacksFromCollapsed(stacks, totalSamples, intervalSeconds, options)
	pl.Phase("topstacks-end", fmt.Sprintf("rows=%d", len(topSeries)))
	rootTopStacks := topStacksFromTree(flamegraph, options.TopN)
	topChildFramesRoot := topChildFrames(flamegraph, options.TopN)
	// Build everything that needs the FULL tree BEFORE pruning so
	// the breakdown stats stay accurate. The pruner only affects the
	// tree we ship to the renderer for visualization.
	executionBreakdown := buildExecutionBreakdown(flamegraph, options, totalSamples, max(flamegraph.Samples, 1))
	rootStageMetrics := map[string]any{
		"total_samples":      totalSamples,
		"matched_samples":    flamegraph.Samples,
		"estimated_seconds":  estimatedSeconds,
		"total_ratio":        100.0,
		"parent_stage_ratio": 100.0,
		"elapsed_ratio":      elapsedRatio(estimatedSeconds, options.ElapsedSec, 4),
	}
	// Prune the FlameNode tree to a sane node count BEFORE the result
	// gets handed to Wails for IPC encoding. On 1M+ node trees the
	// JSON payload alone can hit several gigabytes — we'd survive
	// analyze() only to be killed during marshal. The pruner replaces
	// the tail of each level with a "…other" synthetic sibling so the
	// hidden weight is still visible on the flame graph.
	maxNodes := options.MaxFlamegraphNodes
	if maxNodes == 0 {
		maxNodes = defaultMaxFlamegraphNodes
	}
	if maxNodes > 0 {
		pl.Phase("prune-start", fmt.Sprintf("max_nodes=%d", maxNodes))
		kept, dropped := pruneFlamegraph(&flamegraph, maxNodes)
		if dropped > 0 {
			pl.Phase("prune-end", fmt.Sprintf("kept=%d dropped=%d", kept, dropped))
		} else {
			pl.Phase("prune-end", "no pruning needed")
		}
	}
	// Force a GC pass: build/freeze allocates aggressively (mutableNode
	// tree, intermediate slices), and Go's collector won't run on its
	// own until the next allocation pressure point — which on the
	// post-analyze IPC path is the JSON encoder allocating hundreds of
	// MB of buffers. Reclaiming now keeps the encode-time peak low.
	runtime.GC()
	pl.Mem("post-prune")
	// rootStage.Flamegraph deliberately holds an EMPTY FlameNode so
	// the result doesn't double-encode the tree (Charts.Flamegraph is
	// the single source of truth). The renderer's drilldown stages
	// come from a separate IPC call anyway.
	rootStage := DrilldownStage{
		Index:            0,
		Label:            "All",
		Breadcrumb:       []string{"All"},
		Filter:           nil,
		Metrics:          rootStageMetrics,
		Flamegraph:       FlameNode{},
		TimelineAnalysis: timelineRows,
		TopStacks:        rootTopStacks,
		TopChildFrames:   topChildFramesRoot,
		Diagnostics:      nil,
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
			ExecutionBreakdown: executionBreakdown,
			TimelineAnalysis:   timelineRows,
			Threads:            detectThreads(stacks, totalSamples),
		},
		Tables: Tables{
			TopStacks:        topTables,
			TopChildFrames:   topChildFramesRoot,
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

// [한글] detectThreads — stack 의 첫 frame 이 "[threadName]" 형식이면 thread 별 합산.
// Java/Spring Boot async-profiler 의 thread-prefix 옵션 결과를 인식.
// 매치된 stack 의 sample 합계가 전체의 80% 미만이면 false-positive 로 간주해 빈 리스트.
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
