package profile

import (
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	coreprofiler "github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
)

const ResultType = "profile_evidence"

type Options struct {
	Format      string
	TopN        int
	IntervalMS  float64
	ProfileKind string
}

func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	parsed, diags, err := parser.ParseFile(path, opts.Format, parser.Options{})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return Build(parsed, path, diags, opts), nil
}

func Build(parsed parser.Parsed, sourceFile string, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = 50
	}
	intervalMS := opts.IntervalMS
	if intervalMS <= 0 {
		intervalMS = 100
	}
	stacks := collapsedStacks(parsed.Samples)
	profileKind := firstNonEmpty(opts.ProfileKind, dominantProfileKind(parsed.Samples), "wall")
	core := coreprofiler.AnalyzeCollapsedStacks(stacks, sourceFile, profilerDiagnostics(diags, parsed.Format), coreprofiler.Options{
		IntervalMS:  intervalMS,
		TopN:        topN,
		ProfileKind: profileKind,
	})

	result := models.New(ResultType, "profile_evidence")
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary(parsed, stacks, intervalMS)
	result.Summary["profile_kind"] = profileKind
	result.Series = map[string]any{
		"top_stacks":            core.Series.TopStacks,
		"component_breakdown":   core.Series.ComponentBreakdown,
		"execution_breakdown":   core.Series.ExecutionBreakdown,
		"runtime_distribution":  rowsFromCounts(sampleRuntimeCounts(parsed.Samples), "runtime", topN),
		"language_distribution": rowsFromCounts(sampleLanguageCounts(parsed.Samples), "language", topN),
		"source_formats":        rowsFromCounts(sampleFormatCounts(parsed.Samples), "source_format", topN),
	}
	result.Tables = map[string]any{
		"top_stacks":       core.Tables.TopStacks,
		"top_child_frames": core.Tables.TopChildFrames,
		"frames":           frameRows(parsed.Samples, topN),
		"profile_samples":  sampleRows(parsed.Samples, topN),
	}
	result.Charts = map[string]any{
		"flamegraph":       core.Charts.Flamegraph,
		"drilldown_stages": core.Charts.DrilldownStages,
	}
	result.Metadata.SchemaVersion = "0.1.0"
	result.Metadata.Diagnostics = diags
	result.Metadata.Extra["format"] = firstNonEmpty(parsed.Format, opts.Format, "auto")
	result.Metadata.Extra["unified_profile_schema"] = map[string]any{
		"frames": []string{"name", "function", "file", "line", "language", "runtime", "kind", "native", "async"},
		"samples": []string{
			"stack", "value", "thread", "process", "runtime", "language", "profile_kind", "source_format", "labels",
		},
		"sample_unit": "samples",
	}
	result.Metadata.Extra["flamegraph_rollup"] = map[string]any{
		"unique_stacks": len(stacks),
		"interval_ms":   intervalMS,
		"source":        "internal/profiler.AnalyzeCollapsedStacks",
	}
	if parsed.ValueUnit == "microseconds" {
		runs, activity := sampledCPURuns(parsed.Samples, topN)
		result.Tables["cpu_sample_runs"] = runs
		result.Series["cpu_activity"] = activity
		result.Metadata.Extra["temporal_semantics"] = "sampled_cpu_runs; not browser Long Tasks"
		for _, run := range runs {
			if duration, _ := run["duration_ms"].(float64); duration >= 100 {
				result.AddFinding("warning", "SAMPLED_CPU_HOTSPOT", "Sampled CPU run exceeded 100ms; this is not a browser Long Task.", map[string]any{"stack": run["stack"], "duration_ms": duration})
			}
		}
	}
	if parsed.Metadata != nil {
		result.Metadata.Extra["parser_metadata"] = parsed.Metadata
	}
	addFindings(&result)
	ingestion.AttachSourceMetadata(&result, ingestion.NewSourceMetadata(sourceFile, ingestion.SourceMetadataOptions{
		SourceKind:   ingestion.SourceKindRuntimeProfile,
		SourceFormat: firstNonEmpty(parsed.Format, opts.Format, "auto"),
		Product:      "runtime profiler",
	}))
	return result
}

// sampledCPURuns consumes V8's pre-collapse ordered samples. A run ends when
// the stack changes, an idle sample appears, or a gap exceeds 10× the median
// delta. This preserves temporal semantics independently of flamegraph rollup.
func sampledCPURuns(samples []parser.Sample, limit int) ([]map[string]any, []map[string]any) {
	deltas := make([]int64, 0, len(samples))
	for _, sample := range samples {
		if sample.Value > 0 {
			deltas = append(deltas, sample.Value)
		}
	}
	sort.Slice(deltas, func(i, j int) bool { return deltas[i] < deltas[j] })
	median := int64(0)
	if len(deltas) > 0 {
		median = deltas[len(deltas)/2]
	}
	gap := median * 10
	type run struct {
		stack                string
		start, end, duration int64
	}
	runs := []run{}
	var current *run
	flush := func() {
		if current != nil && current.duration > 0 {
			runs = append(runs, *current)
		}
		current = nil
	}
	for _, sample := range samples {
		if sample.Value <= 0 {
			flush()
			continue
		}
		stack := stackKey(sample.Stack)
		if stack == "" {
			flush()
			continue
		}
		if current == nil || current.stack != stack || (gap > 0 && sample.TimestampUS-current.end > gap) {
			flush()
			current = &run{stack: stack, start: sample.TimestampUS, end: sample.TimestampUS}
		}
		current.end = sample.TimestampUS + sample.Value
		current.duration += sample.Value
	}
	flush()
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].duration > runs[j].duration })
	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}
	rows := make([]map[string]any, 0, len(runs))
	activity := make([]map[string]any, 0, len(runs))
	for _, run := range runs {
		rows = append(rows, map[string]any{"stack": run.stack, "start_us": run.start, "end_us": run.end, "duration_us": run.duration, "duration_ms": round(float64(run.duration)/1000, 3)})
		activity = append(activity, map[string]any{"ts_us": run.start, "duration_us": run.duration, "active_us": run.duration})
	}
	return rows, activity
}

func collapsedStacks(samples []parser.Sample) map[string]int {
	out := map[string]int{}
	for _, sample := range samples {
		if sample.Value == 0 && sample.TimestampUS != 0 {
			continue // V8's first sample has no preceding duration to attribute.
		}
		key := stackKey(sample.Stack)
		if key == "" {
			continue
		}
		value := int(sample.Value)
		if value <= 0 {
			value = 1
		}
		out[key] += value
	}
	return out
}

func summary(parsed parser.Parsed, stacks map[string]int, intervalMS float64) map[string]any {
	total := 0
	native := 0
	managed := 0
	async := 0
	maxDepth := 0
	threads := map[string]int{}
	processes := map[string]int{}
	for _, sample := range parsed.Samples {
		value := int(sample.Value)
		if value == 0 && sample.TimestampUS == 0 {
			value = 1
		}
		total += value
		if len(sample.Stack) > maxDepth {
			maxDepth = len(sample.Stack)
		}
		if sample.Thread != "" {
			threads[sample.Thread] += value
		}
		if sample.Process != "" {
			processes[sample.Process] += value
		}
		hasNative := false
		hasManaged := false
		hasAsync := false
		for _, frame := range sample.Stack {
			if frame.Native {
				hasNative = true
			} else {
				hasManaged = true
			}
			if frame.Async {
				hasAsync = true
			}
		}
		if hasNative {
			native += value
		}
		if hasManaged {
			managed += value
		}
		if hasAsync {
			async += value
		}
	}
	result := map[string]any{
		"total_samples":        total,
		"unique_stacks":        len(stacks),
		"runtime_count":        len(sampleRuntimeCounts(parsed.Samples)),
		"language_count":       len(sampleLanguageCounts(parsed.Samples)),
		"native_samples":       native,
		"managed_samples":      managed,
		"async_frame_samples":  async,
		"thread_count":         len(threads),
		"process_count":        len(processes),
		"max_stack_depth":      maxDepth,
		"interval_ms":          intervalMS,
		"estimated_seconds":    round(float64(total)*intervalMS/1000, 3),
		"source_format_count":  len(sampleFormatCounts(parsed.Samples)),
		"profile_sample_count": len(parsed.Samples),
		"value_unit":           parsed.ValueUnit,
	}
	if parsed.ValueUnit == "microseconds" {
		result["total_duration_us"] = total
		result["total_duration_ms"] = round(float64(total)/1000, 3)
		result["estimated_seconds"] = round(float64(total)/1_000_000, 3)
	}
	return result
}

func frameRows(samples []parser.Sample, limit int) []map[string]any {
	type aggregate struct {
		frame   parser.Frame
		samples int
	}
	counts := map[string]aggregate{}
	for _, sample := range samples {
		value := int(sample.Value)
		if value == 0 && sample.TimestampUS == 0 {
			value = 1
		}
		for _, frame := range sample.Stack {
			key := strings.Join([]string{frame.Name, frame.File, frame.Runtime, frame.Language}, "\x00")
			item := counts[key]
			item.frame = frame
			item.samples += value
			counts[key] = item
		}
	}
	items := make([]aggregate, 0, len(counts))
	for _, item := range counts {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].samples != items[j].samples {
			return items[i].samples > items[j].samples
		}
		return items[i].frame.Name < items[j].frame.Name
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"name":     item.frame.Name,
			"function": item.frame.Function,
			"file":     item.frame.File,
			"line":     item.frame.Line,
			"language": item.frame.Language,
			"runtime":  item.frame.Runtime,
			"kind":     item.frame.Kind,
			"native":   item.frame.Native,
			"async":    item.frame.Async,
			"samples":  item.samples,
		})
	}
	return out
}

func sampleRows(samples []parser.Sample, limit int) []map[string]any {
	type row struct {
		stack  string
		sample parser.Sample
	}
	rows := make([]row, 0, len(samples))
	for _, sample := range samples {
		rows = append(rows, row{stack: stackKey(sample.Stack), sample: sample})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].sample.Value != rows[j].sample.Value {
			return rows[i].sample.Value > rows[j].sample.Value
		}
		return rows[i].stack < rows[j].stack
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, item := range rows {
		out = append(out, map[string]any{
			"stack":         item.stack,
			"frames":        frameNames(item.sample.Stack),
			"samples":       maxInt(1, int(item.sample.Value)),
			"timestamp_us":  item.sample.TimestampUS,
			"thread":        item.sample.Thread,
			"process":       item.sample.Process,
			"runtime":       item.sample.Runtime,
			"language":      item.sample.Language,
			"profile_kind":  item.sample.ProfileKind,
			"source_format": item.sample.SourceFormat,
			"labels":        item.sample.Labels,
		})
	}
	return out
}

func addFindings(result *models.AnalysisResult) {
	total := asInt(result.Summary["total_samples"])
	if total == 0 {
		result.AddFinding("warning", "PROFILE_EMPTY", "Profile evidence did not contain stack samples.", nil)
		return
	}
	if asInt(result.Summary["runtime_count"]) > 1 {
		result.AddFinding("info", "PROFILE_MIXED_RUNTIMES", "Profile evidence contains multiple runtimes or languages.", map[string]any{"runtime_count": result.Summary["runtime_count"]})
	}
	if ratio(asInt(result.Summary["native_samples"]), total) >= 0.30 {
		result.AddFinding("warning", "PROFILE_NATIVE_HOTSPOT", "Native frames account for a material share of profile samples.", map[string]any{"native_samples": result.Summary["native_samples"], "total_samples": total})
	}
	if asInt(result.Summary["async_frame_samples"]) > 0 {
		result.AddFinding("info", "PROFILE_ASYNC_FRAMES_PRESENT", "Async or continuation frames are present in the profile evidence.", map[string]any{"async_frame_samples": result.Summary["async_frame_samples"]})
	}
}

func profilerDiagnostics(diags *diagnostics.ParserDiagnostics, format string) coreprofiler.ParserDiagnostics {
	out := coreprofiler.ParserDiagnostics{
		Format:          firstNonEmpty(format, "profile_evidence"),
		SkippedByReason: map[string]int{},
		Samples:         []coreprofiler.DiagnosticSample{},
		Warnings:        []coreprofiler.DiagnosticSample{},
		Errors:          []coreprofiler.DiagnosticSample{},
	}
	if diags == nil {
		return out
	}
	out.SourceFile = diags.SourceFile
	out.Format = firstNonEmpty(diags.Format, format, "profile_evidence")
	out.TotalLines = diags.TotalLines
	out.ParsedRecords = diags.ParsedRecords
	out.SkippedLines = diags.SkippedLines
	out.WarningCount = diags.WarningCount
	out.ErrorCount = diags.ErrorCount
	for reason, count := range diags.SkippedByReason {
		out.SkippedByReason[reason] = count
	}
	for _, sample := range diags.Samples {
		out.Samples = append(out.Samples, coreprofiler.DiagnosticSample(sample))
	}
	for _, sample := range diags.Warnings {
		out.Warnings = append(out.Warnings, coreprofiler.DiagnosticSample(sample))
	}
	for _, sample := range diags.Errors {
		out.Errors = append(out.Errors, coreprofiler.DiagnosticSample(sample))
	}
	return out
}

func stackKey(frames []parser.Frame) string {
	names := frameNames(frames)
	return strings.Join(names, ";")
}

func frameNames(frames []parser.Frame) []string {
	names := make([]string, 0, len(frames))
	for _, frame := range frames {
		name := strings.TrimSpace(frame.Name)
		if name == "" {
			continue
		}
		name = strings.ReplaceAll(name, ";", "_")
		names = append(names, name)
	}
	return names
}

func sampleRuntimeCounts(samples []parser.Sample) map[string]int {
	counts := map[string]int{}
	for _, sample := range samples {
		counts[firstNonEmpty(sample.Runtime, "unknown")] += maxInt(1, int(sample.Value))
	}
	return counts
}

func sampleLanguageCounts(samples []parser.Sample) map[string]int {
	counts := map[string]int{}
	for _, sample := range samples {
		counts[firstNonEmpty(sample.Language, "unknown")] += maxInt(1, int(sample.Value))
	}
	return counts
}

func sampleFormatCounts(samples []parser.Sample) map[string]int {
	counts := map[string]int{}
	for _, sample := range samples {
		counts[firstNonEmpty(sample.SourceFormat, "unknown")] += maxInt(1, int(sample.Value))
	}
	return counts
}

func dominantProfileKind(samples []parser.Sample) string {
	counts := map[string]int{}
	for _, sample := range samples {
		counts[sample.ProfileKind] += maxInt(1, int(sample.Value))
	}
	best := ""
	bestCount := 0
	for key, count := range counts {
		if key != "" && count > bestCount {
			best = key
			bestCount = count
		}
	}
	return best
}

func rowsFromCounts(counts map[string]int, key string, limit int) []map[string]any {
	type kv struct {
		key   string
		count int
	}
	items := make([]kv, 0, len(counts))
	for k, v := range counts {
		items = append(items, kv{k, v})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].key < items[j].key
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{key: item.key, "samples": item.count})
	}
	return out
}

func asInt(v any) int {
	if n, ok := v.(int); ok {
		return n
	}
	return 0
}

func ratio(part, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) / float64(total)
}

func round(value float64, places int) float64 {
	pow := 1.0
	for i := 0; i < places; i++ {
		pow *= 10
	}
	if value >= 0 {
		return float64(int(value*pow+0.5)) / pow
	}
	return float64(int(value*pow-0.5)) / pow
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
