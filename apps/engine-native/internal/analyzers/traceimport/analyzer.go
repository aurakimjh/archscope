// Package traceimport analyzes canonical spans parsed from external APM
// trace export files.
package traceimport

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	traceparser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/traceimport"
)

const (
	ResultType    = "trace_import"
	ParserName    = "trace_import"
	SchemaVersion = "0.2.0"
	DefaultTopN   = 50

	slowTraceP95ThresholdMS           = 1_000.0
	unbalancedServiceLatencyRatio     = 0.70
	highErrorServiceEdgeRateThreshold = 0.20
	clockSkewToleranceNanos           = int64(10 * 1_000_000)
)

type Options struct {
	Format string
	TopN   int
}

func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	parsed, err := traceparser.ParseFile(path, traceparser.Options{Format: opts.Format})
	if err != nil && len(parsed.Spans) == 0 {
		return models.AnalysisResult{}, err
	}
	result := Build(parsed.Spans, path, parsed.Format, opts)
	result.Metadata.Diagnostics = parsed.Diagnostics
	return result, err
}

func Build(spans []traceparser.Span, sourceFile, sourceFormat string, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}

	spanByID := make(map[string]traceparser.Span, len(spans))
	traceStats := map[string]*traceStat{}
	serviceCounts := newCounter()
	kindCounts := newCounter()
	serviceStats := map[string]*serviceStat{}

	for _, span := range spans {
		key := spanKey(span.TraceID, span.SpanID)
		spanByID[key] = span
		trace := getTraceStat(traceStats, span.TraceID)
		trace.add(span)
		service := labelOr(span.ServiceName, "(unknown)")
		serviceCounts.add(service)
		kindCounts.add(labelOr(span.Kind, "UNSPECIFIED"))
		ss := getServiceStat(serviceStats, service)
		ss.Count++
		ss.TotalDurationNanos += span.DurationNanos
		if span.Error {
			ss.ErrorCount++
		}
	}

	dependencies := buildDependencies(spans, spanByID)
	missingParents := countMissingParents(spans, spanByID)
	clockSkewCount := countClockSkewSuspects(spans, spanByID)
	criticalPaths := buildCriticalPaths(spans, 0)
	unavailablePaths := 0
	for _, path := range criticalPaths {
		if path.Unavailable != "" {
			unavailablePaths++
		}
	}
	traceDurations := make([]float64, 0, len(traceStats))
	for _, trace := range traceStats {
		traceDurations = append(traceDurations, nanosToMillis(trace.DurationNanos()))
	}
	p95TraceDurationMS := percentile(traceDurations, 0.95)
	maxTraceDurationMS := 0.0
	for _, duration := range traceDurations {
		if duration > maxTraceDurationMS {
			maxTraceDurationMS = duration
		}
	}
	unbalancedServices := countUnbalancedServices(serviceStats)
	highErrorEdges := countHighErrorEdges(dependencies)
	errorSpans := 0
	rootSpans := 0
	for _, span := range spans {
		if span.Error {
			errorSpans++
		}
		if span.ParentSpanID == "" {
			rootSpans++
		}
	}

	summary := map[string]any{
		"source_format":                   sourceFormat,
		"total_spans":                     len(spans),
		"unique_traces":                   len(traceStats),
		"unique_services":                 len(nonUnknownKeys(serviceStats)),
		"unique_dependencies":             len(dependencies),
		"root_spans":                      rootSpans,
		"error_spans":                     errorSpans,
		"missing_parent_spans":            missingParents,
		"clock_skew_suspected_spans":      clockSkewCount,
		"p95_trace_duration_ms":           p95TraceDurationMS,
		"max_trace_duration_ms":           maxTraceDurationMS,
		"unbalanced_service_count":        unbalancedServices,
		"high_error_service_edges":        highErrorEdges,
		"path_elapsed_available_traces":   len(criticalPaths) - unavailablePaths,
		"path_elapsed_unavailable_traces": unavailablePaths,
	}

	result := models.New(ResultType, ParserName)
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary
	result.Series = map[string]any{
		"service_distribution": serviceCounts.rows("service", topN),
		"kind_distribution":    kindCounts.rows("kind", topN),
		"top_traces":           topTraceRows(traceStats, topN),
		"top_slow_spans":       topSlowSpanRows(spans, topN),
	}
	result.Tables = map[string]any{
		"spans":                spanRows(spans, topN),
		"traces":               traceRows(traceStats, topN),
		"service_dependencies": dependencyRows(dependencies, topN),
		"service_summary":      serviceRows(serviceStats, topN),
		"critical_paths":       criticalPathRows(criticalPaths, topN),
	}
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Extra["dependency_call_policy"] = map[string]any{"version": "1.0.0", "identity": "trace_id_and_direct_client_server_parent", "paired_timing": "client", "paired_error": "either_side", "multiple_server_attempts": "individual_server_timing_and_error"}
	result.Metadata.Extra["path_latency"] = map[string]any{"version": "1.0.0", "semantics": "observed_span_chain_elapsed_estimate_v1", "causal_critical_path_proven": false, "available_traces": len(criticalPaths) - unavailablePaths, "unavailable_traces": unavailablePaths}
	if unavailablePaths > 0 {
		result.AddFinding("warning", "TRACE_PATH_LATENCY_UNAVAILABLE", "Some trace path elapsed estimates are unavailable because timestamps or graph structure could not be validated.", map[string]any{"trace_count": unavailablePaths})
	}
	result.Metadata.Extra["analysis_options"] = map[string]any{
		"format": opts.Format,
		"top_n":  topN,
	}
	result.Metadata.Extra["trace_import"] = map[string]any{
		"source_format": sourceFormat,
		"canonical_model": []string{
			"trace_id",
			"span_id",
			"parent_span_id",
			"service_name",
			"remote_service",
			"start_unix_nanos",
			"duration_nanos",
		},
	}
	ingestion.AttachSourceMetadata(&result, ingestion.NewSourceMetadata(sourceFile, ingestion.SourceMetadataOptions{
		SourceKind:   ingestion.SourceKindTrace,
		SourceFormat: sourceFormat,
		Product:      traceProduct(sourceFormat),
	}))
	ingestion.AttachCorrelationKeys(&result, traceCorrelationKeys(spans, topN))
	for _, finding := range findings(summary, traceStats, dependencies) {
		result.Metadata.Findings = append(result.Metadata.Findings, finding)
	}
	return result
}

type traceStat struct {
	TraceID       string
	SpanCount     int
	ServiceSet    map[string]struct{}
	ErrorCount    int
	MinStartNanos int64
	MaxEndNanos   int64
	TotalNanos    int64
	SlowestSpan   traceparser.Span
}

func (t *traceStat) add(span traceparser.Span) {
	t.SpanCount++
	if t.ServiceSet == nil {
		t.ServiceSet = map[string]struct{}{}
	}
	t.ServiceSet[labelOr(span.ServiceName, "(unknown)")] = struct{}{}
	if span.Error {
		t.ErrorCount++
	}
	if span.StartUnixNanos > 0 {
		if t.MinStartNanos == 0 || span.StartUnixNanos < t.MinStartNanos {
			t.MinStartNanos = span.StartUnixNanos
		}
		end := span.StartUnixNanos + span.DurationNanos
		if end > t.MaxEndNanos {
			t.MaxEndNanos = end
		}
	}
	t.TotalNanos += span.DurationNanos
	if span.DurationNanos > t.SlowestSpan.DurationNanos {
		t.SlowestSpan = span
	}
}

func (t *traceStat) DurationNanos() int64 {
	if t.MinStartNanos > 0 && t.MaxEndNanos > t.MinStartNanos {
		return t.MaxEndNanos - t.MinStartNanos
	}
	return t.TotalNanos
}

type serviceStat struct {
	ServiceName        string
	Count              int
	TotalDurationNanos int64
	ErrorCount         int
}

type dependencyStat struct {
	Caller             string
	Callee             string
	Count              int
	TotalDurationNanos int64
	ErrorCount         int
}

func buildDependencies(spans []traceparser.Span, spanByID map[string]traceparser.Span) map[string]*dependencyStat {
	// Pair only explicit direct CLIENT -> SERVER representations in one trace.
	// Service-name equality alone would collapse distinct requests or retries.
	servers := map[string][]traceparser.Span{}
	for _, span := range spans {
		if strings.ToUpper(span.Kind) != "SERVER" || span.ParentSpanID == "" {
			continue
		}
		parent, ok := spanByID[spanKey(span.TraceID, span.ParentSpanID)]
		if ok && strings.ToUpper(parent.Kind) == "CLIENT" && parent.ServiceName != "" && span.ServiceName != "" && parent.ServiceName != span.ServiceName && (parent.RemoteService == "" || parent.RemoteService == span.ServiceName) {
			key := spanKey(parent.TraceID, parent.SpanID)
			servers[key] = append(servers[key], span)
		}
	}
	pairedServers := map[string]bool{}
	for _, children := range servers {
		if len(children) == 1 {
			pairedServers[spanKey(children[0].TraceID, children[0].SpanID)] = true
		}
	}
	out := map[string]*dependencyStat{}
	for _, span := range spans {
		key := spanKey(span.TraceID, span.SpanID)
		caller, callee := "", ""
		duration, isError := span.DurationNanos, span.Error
		kind := strings.ToUpper(span.Kind)
		children := servers[key]
		if kind == "CLIENT" && len(children) > 1 {
			// Several distinct SERVER children are separately observed attempts. The
			// aggregate CLIENT interval cannot be assigned once to every retry.
			continue
		}
		if kind == "SERVER" {
			parent, ok := spanByID[spanKey(span.TraceID, span.ParentSpanID)]
			if pairedServers[key] {
				continue
			}
			if ok && parent.ServiceName != "" && parent.ServiceName != span.ServiceName {
				caller, callee = parent.ServiceName, span.ServiceName
			} else if span.RemoteService != "" {
				caller, callee = span.RemoteService, span.ServiceName
			}
		} else if len(children) == 1 {
			caller, callee = span.ServiceName, children[0].ServiceName
			isError = isError || children[0].Error
		} else if span.RemoteService != "" {
			caller, callee = labelOr(span.ServiceName, "(unknown)"), span.RemoteService
		} else if parent, ok := spanByID[spanKey(span.TraceID, span.ParentSpanID)]; ok && parent.ServiceName != "" && parent.ServiceName != span.ServiceName {
			caller, callee = parent.ServiceName, labelOr(span.ServiceName, "(unknown)")
		}
		if caller == "" || callee == "" || caller == callee {
			continue
		}
		edgeKey := caller + "\x00" + callee
		stat := out[edgeKey]
		if stat == nil {
			stat = &dependencyStat{Caller: caller, Callee: callee}
			out[edgeKey] = stat
		}
		stat.Count++
		stat.TotalDurationNanos += duration
		if isError {
			stat.ErrorCount++
		}
	}
	return out
}

func traceProduct(sourceFormat string) string {
	switch sourceFormat {
	case traceparser.FormatOTLPJSON:
		return "OpenTelemetry"
	case traceparser.FormatZipkinV2:
		return "Zipkin"
	case traceparser.FormatElasticSearchJSON, traceparser.FormatElasticSourceNDJSON:
		return "Elastic APM"
	case traceparser.FormatJaegerQueryJSON:
		return "Jaeger"
	case traceparser.FormatSkyWalkingGraphQL:
		return "Apache SkyWalking"
	default:
		return sourceFormat
	}
}

func traceCorrelationKeys(spans []traceparser.Span, limit int) []ingestion.CorrelationKeys {
	if limit <= 0 {
		limit = DefaultTopN
	}
	ordered := append([]traceparser.Span(nil), spans...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].TraceID != ordered[j].TraceID {
			return ordered[i].TraceID < ordered[j].TraceID
		}
		if ordered[i].StartUnixNanos != ordered[j].StartUnixNanos {
			return ordered[i].StartUnixNanos < ordered[j].StartUnixNanos
		}
		return ordered[i].SpanID < ordered[j].SpanID
	})
	keys := make([]ingestion.CorrelationKeys, 0, minInt(limit, len(ordered)))
	seen := map[string]struct{}{}
	for _, span := range ordered {
		if span.TraceID == "" && span.SpanID == "" {
			continue
		}
		key := ingestion.CorrelationKeys{
			TraceID:      span.TraceID,
			SpanID:       span.SpanID,
			ParentSpanID: span.ParentSpanID,
			RequestID:    firstNonEmpty(span.Attributes["http.request_id"], span.Attributes["request.id"], span.Attributes["x-request-id"]),
			ContainerID:  firstNonEmpty(span.Attributes["container.id"], span.Attributes["container_id"]),
			PodUID:       firstNonEmpty(span.Attributes["k8s.pod.uid"], span.Attributes["pod.uid"]),
			HostID:       firstNonEmpty(span.Attributes["host.id"], span.Attributes["host.name"], span.Attributes["hostname"]),
		}
		if span.StartUnixNanos > 0 {
			key.Window = ingestion.TimestampWindowFor(time.Unix(0, span.StartUnixNanos), ingestion.DefaultCorrelationWindow)
		}
		key = ingestion.NormalizeCorrelationKeys(key)
		if key.StableID == "" {
			continue
		}
		if _, ok := seen[key.StableID]; ok {
			continue
		}
		seen[key.StableID] = struct{}{}
		keys = append(keys, key)
		if len(keys) >= limit {
			break
		}
	}
	return keys
}

func countMissingParents(spans []traceparser.Span, spanByID map[string]traceparser.Span) int {
	count := 0
	for _, span := range spans {
		if span.ParentSpanID == "" {
			continue
		}
		if _, ok := spanByID[spanKey(span.TraceID, span.ParentSpanID)]; !ok {
			count++
		}
	}
	return count
}

func countClockSkewSuspects(spans []traceparser.Span, spanByID map[string]traceparser.Span) int {
	count := 0
	for _, span := range spans {
		if span.ParentSpanID == "" || span.StartUnixNanos == 0 {
			continue
		}
		parent, ok := spanByID[spanKey(span.TraceID, span.ParentSpanID)]
		if !ok || parent.StartUnixNanos == 0 {
			continue
		}
		parentEnd := parent.StartUnixNanos + parent.DurationNanos
		childEnd := span.StartUnixNanos + span.DurationNanos
		if span.StartUnixNanos+clockSkewToleranceNanos < parent.StartUnixNanos ||
			childEnd > parentEnd+clockSkewToleranceNanos {
			count++
		}
	}
	return count
}

// A trace export establishes ancestry and observed intervals, not scheduling
// dependencies. This is an observed-chain elapsed estimate, never a causal
// critical-path proof. Inclusive work is exposed separately for comparison.
type criticalPath struct {
	TraceID       string
	DurationNanos int64
	SpanCount     int
	Services      map[string]struct{}
	SpanNames     []string
	CumulativeMS  float64
	Unavailable   string
}

func buildCriticalPaths(spans []traceparser.Span, limit int) []criticalPath {
	byTrace := map[string][]traceparser.Span{}
	for _, span := range spans {
		byTrace[span.TraceID] = append(byTrace[span.TraceID], span)
	}
	out := make([]criticalPath, 0, len(byTrace))
	for id, rows := range byTrace {
		out = append(out, longestSpanChain(id, rows))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Unavailable == "") != (out[j].Unavailable == "") {
			return out[i].Unavailable == ""
		}
		if out[i].DurationNanos != out[j].DurationNanos {
			return out[i].DurationNanos > out[j].DurationNanos
		}
		return out[i].TraceID < out[j].TraceID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func longestSpanChain(traceID string, spans []traceparser.Span) criticalPath {
	fail := func(reason string) criticalPath {
		return criticalPath{TraceID: traceID, SpanCount: len(spans), Services: map[string]struct{}{}, SpanNames: []string{}, Unavailable: reason}
	}
	byID := map[string]traceparser.Span{}
	children := map[string][]traceparser.Span{}
	for _, span := range spans {
		if span.SpanID == "" {
			return fail("missing_span_id")
		}
		if _, exists := byID[span.SpanID]; exists {
			return fail("duplicate_span_id")
		}
		if span.StartUnixNanos <= 0 {
			return fail("missing_timestamp")
		}
		if span.DurationNanos < 0 || span.StartUnixNanos > math.MaxInt64-span.DurationNanos {
			return fail("invalid_interval")
		}
		byID[span.SpanID] = span
		children[span.ParentSpanID] = append(children[span.ParentSpanID], span)
	}
	// [한글] 재귀 대신 부모 포인터를 색칠하며 순환을 검사한다. 깊은 트리도
	// 호출 스택을 늘리지 않는다. 시간 구간이 부모 밖이면 추정치를 만들지 않는다.
	colors := map[string]int{}
	for _, span := range spans {
		if colors[span.SpanID] == 2 {
			continue
		}
		chain := []string{}
		id := span.SpanID
		for id != "" {
			node, exists := byID[id]
			if !exists {
				break
			}
			if colors[id] == 1 {
				return fail("cyclic_parent_graph")
			}
			if colors[id] == 2 {
				break
			}
			colors[id] = 1
			chain = append(chain, id)
			if parent, ok := byID[node.ParentSpanID]; ok && (node.StartUnixNanos < parent.StartUnixNanos || node.StartUnixNanos+node.DurationNanos > parent.StartUnixNanos+parent.DurationNanos) {
				return fail("parent_child_clock_skew")
			}
			id = node.ParentSpanID
		}
		for _, id := range chain {
			colors[id] = 2
		}
	}
	roots := []traceparser.Span{}
	for _, span := range spans {
		if _, ok := byID[span.ParentSpanID]; !ok {
			roots = append(roots, span)
		}
	}
	if len(roots) == 0 {
		return fail("missing_root")
	}
	less := func(a, b traceparser.Span) bool {
		if a.DurationNanos != b.DurationNanos {
			return a.DurationNanos > b.DurationNanos
		}
		if a.StartUnixNanos != b.StartUnixNanos {
			return a.StartUnixNanos < b.StartUnixNanos
		}
		return a.SpanID < b.SpanID
	}
	sort.SliceStable(roots, func(i, j int) bool { return less(roots[i], roots[j]) })
	for id := range children {
		rows := children[id]
		sort.SliceStable(rows, func(i, j int) bool { return less(rows[i], rows[j]) })
	}
	root := roots[0]
	path := criticalPath{TraceID: traceID, DurationNanos: root.DurationNanos, Services: map[string]struct{}{}, SpanNames: []string{}}
	// All selected descendants are contained in the root interval, so their
	// interval union has the root's duration. They must not be added again.
	for node := root; ; {
		path.SpanCount++
		path.SpanNames = append(path.SpanNames, node.Name)
		path.CumulativeMS += nanosToMillis(node.DurationNanos)
		if node.ServiceName != "" {
			path.Services[node.ServiceName] = struct{}{}
		}
		next := children[node.SpanID]
		if len(next) == 0 {
			break
		}
		node = next[0]
	}
	return path
}

func criticalPathRows(paths []criticalPath, limit int) []map[string]any {
	if limit > 0 && len(paths) > limit {
		paths = paths[:limit]
	}
	rows := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		var elapsed, work any
		if path.Unavailable == "" {
			elapsed = nanosToMillis(path.DurationNanos)
			work = path.CumulativeMS
		}
		rows = append(rows, map[string]any{"trace_id": path.TraceID, "critical_path_ms": elapsed, "observed_path_elapsed_ms": elapsed, "selected_chain_cumulative_ms": work, "duration_unavailable_reason": path.Unavailable, "path_semantics": "observed_span_chain_elapsed_estimate_v1", "causal_critical_path_proven": false, "span_count": path.SpanCount, "services": sortedSet(path.Services), "span_names": path.SpanNames})
	}
	return rows
}

func countUnbalancedServices(stats map[string]*serviceStat) int {
	total := int64(0)
	for _, stat := range stats {
		total += stat.TotalDurationNanos
	}
	if total <= 0 || len(nonUnknownKeys(stats)) < 2 {
		return 0
	}
	count := 0
	for _, stat := range stats {
		if stat.ServiceName == "(unknown)" {
			continue
		}
		if float64(stat.TotalDurationNanos)/float64(total) >= unbalancedServiceLatencyRatio {
			count++
		}
	}
	return count
}

func countHighErrorEdges(stats map[string]*dependencyStat) int {
	return len(highErrorEdgeRows(stats))
}

func highErrorEdgeRows(stats map[string]*dependencyStat) []map[string]any {
	rows := []map[string]any{}
	for _, stat := range stats {
		if stat.Count <= 0 || stat.ErrorCount <= 0 {
			continue
		}
		rate := float64(stat.ErrorCount) / float64(stat.Count)
		if rate < highErrorServiceEdgeRateThreshold {
			continue
		}
		rows = append(rows, map[string]any{
			"caller":     stat.Caller,
			"callee":     stat.Callee,
			"error_rate": rate,
			"errors":     stat.ErrorCount,
			"calls":      stat.Count,
		})
	}
	return rows
}

func findings(summary map[string]any, traces map[string]*traceStat, dependencies map[string]*dependencyStat) []map[string]any {
	out := []map[string]any{}
	if asFloat(summary["p95_trace_duration_ms"]) >= slowTraceP95ThresholdMS {
		out = append(out, map[string]any{
			"severity": "warning",
			"code":     "SLOW_TRACE_P95",
			"message":  "Trace p95 duration is above the slow-trace threshold.",
			"evidence": map[string]any{
				"p95_trace_duration_ms": summary["p95_trace_duration_ms"],
				"threshold_ms":          slowTraceP95ThresholdMS,
			},
		})
	}
	if asInt(summary["error_spans"]) > 0 {
		out = append(out, map[string]any{
			"severity": "warning",
			"code":     "TRACE_IMPORT_ERROR_SPANS",
			"message":  "Imported traces include spans marked as errors.",
			"evidence": map[string]any{"error_spans": summary["error_spans"]},
		})
	}
	if asInt(summary["missing_parent_spans"]) > 0 {
		out = append(out, map[string]any{
			"severity": "warning",
			"code":     "TRACE_IMPORT_MISSING_PARENT",
			"message":  "Some spans reference parent spans that were not present in the export.",
			"evidence": map[string]any{"missing_parent_spans": summary["missing_parent_spans"]},
		})
	}
	if asInt(summary["clock_skew_suspected_spans"]) > 0 {
		out = append(out, map[string]any{
			"severity": "warning",
			"code":     "CLOCK_SKEW_SUSPECTED",
			"message":  "Some child spans appear outside their parent time window.",
			"evidence": map[string]any{"clock_skew_suspected_spans": summary["clock_skew_suspected_spans"]},
		})
	}
	if asInt(summary["unbalanced_service_count"]) > 0 {
		out = append(out, map[string]any{
			"severity": "warning",
			"code":     "UNBALANCED_SERVICE_LATENCY",
			"message":  "One service accounts for most imported span latency.",
			"evidence": map[string]any{
				"unbalanced_service_count": summary["unbalanced_service_count"],
				"ratio_threshold":          unbalancedServiceLatencyRatio,
			},
		})
	}
	if highErrorEdges := highErrorEdgeRows(dependencies); len(highErrorEdges) > 0 {
		out = append(out, map[string]any{
			"severity": "critical",
			"code":     "HIGH_ERROR_SERVICE_EDGE",
			"message":  "One or more service edges have a high error rate.",
			"evidence": map[string]any{
				"high_error_service_edges": len(highErrorEdges),
				"error_rate_threshold":     highErrorServiceEdgeRateThreshold,
			},
		})
	}
	crossService := 0
	dominant := 0
	for _, trace := range traces {
		if len(trace.ServiceSet) > 1 {
			crossService++
		}
		duration := trace.DurationNanos()
		if duration > 0 && trace.SlowestSpan.DurationNanos*100 >= duration*80 {
			dominant++
		}
	}
	if crossService > 0 {
		out = append(out, map[string]any{
			"severity": "info",
			"code":     "TRACE_IMPORT_CROSS_SERVICE_TRACE",
			"message":  "At least one imported trace spans multiple services.",
			"evidence": map[string]any{"cross_service_traces": crossService},
		})
	}
	if dominant > 0 {
		out = append(out, map[string]any{
			"severity": "info",
			"code":     "TRACE_IMPORT_SLOW_SPAN_DOMINATES_TRACE",
			"message":  "At least one trace is dominated by a single span.",
			"evidence": map[string]any{"dominant_span_traces": dominant},
		})
	}
	return out
}

func spanRows(spans []traceparser.Span, limit int) []map[string]any {
	ordered := append([]traceparser.Span(nil), spans...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].TraceID != ordered[j].TraceID {
			return ordered[i].TraceID < ordered[j].TraceID
		}
		if ordered[i].StartUnixNanos != ordered[j].StartUnixNanos {
			return ordered[i].StartUnixNanos < ordered[j].StartUnixNanos
		}
		return ordered[i].SpanID < ordered[j].SpanID
	})
	if limit > 0 && len(ordered) > limit {
		ordered = ordered[:limit]
	}
	rows := make([]map[string]any, 0, len(ordered))
	for _, span := range ordered {
		rows = append(rows, map[string]any{
			"trace_id":         span.TraceID,
			"span_id":          span.SpanID,
			"parent_span_id":   emptyToNil(span.ParentSpanID),
			"name":             span.Name,
			"service_name":     emptyToNil(span.ServiceName),
			"remote_service":   emptyToNil(span.RemoteService),
			"kind":             span.Kind,
			"start_unix_nanos": span.StartUnixNanos,
			"duration_ms":      nanosToMillis(span.DurationNanos),
			"status_code":      emptyToNil(span.StatusCode),
			"error":            span.Error,
			"source_format":    span.SourceFormat,
		})
	}
	return rows
}

func traceRows(stats map[string]*traceStat, limit int) []map[string]any {
	items := traceStatsSorted(stats)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, map[string]any{
			"trace_id":        item.TraceID,
			"span_count":      item.SpanCount,
			"service_count":   len(item.ServiceSet),
			"services":        sortedSet(item.ServiceSet),
			"duration_ms":     nanosToMillis(item.DurationNanos()),
			"total_span_ms":   nanosToMillis(item.TotalNanos),
			"error_count":     item.ErrorCount,
			"slowest_span":    item.SlowestSpan.Name,
			"slowest_span_ms": nanosToMillis(item.SlowestSpan.DurationNanos),
		})
	}
	return rows
}

func serviceRows(stats map[string]*serviceStat, limit int) []map[string]any {
	items := make([]*serviceStat, 0, len(stats))
	for _, stat := range stats {
		items = append(items, stat)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].TotalDurationNanos != items[j].TotalDurationNanos {
			return items[i].TotalDurationNanos > items[j].TotalDurationNanos
		}
		return items[i].ServiceName < items[j].ServiceName
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, map[string]any{
			"service":           item.ServiceName,
			"span_count":        item.Count,
			"total_duration_ms": nanosToMillis(item.TotalDurationNanos),
			"avg_duration_ms":   avgMillis(item.TotalDurationNanos, item.Count),
			"error_count":       item.ErrorCount,
		})
	}
	return rows
}

func dependencyRows(stats map[string]*dependencyStat, limit int) []map[string]any {
	items := make([]*dependencyStat, 0, len(stats))
	for _, stat := range stats {
		items = append(items, stat)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		if items[i].Caller != items[j].Caller {
			return items[i].Caller < items[j].Caller
		}
		return items[i].Callee < items[j].Callee
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, map[string]any{
			"caller":            item.Caller,
			"callee":            item.Callee,
			"call_count":        item.Count,
			"total_duration_ms": nanosToMillis(item.TotalDurationNanos),
			"avg_duration_ms":   avgMillis(item.TotalDurationNanos, item.Count),
			"error_count":       item.ErrorCount,
			"error_rate":        errorRate(item.ErrorCount, item.Count),
		})
	}
	return rows
}

func topTraceRows(stats map[string]*traceStat, limit int) []map[string]any {
	items := traceStatsSorted(stats)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, map[string]any{
			"trace_id":    item.TraceID,
			"duration_ms": nanosToMillis(item.DurationNanos()),
			"span_count":  item.SpanCount,
		})
	}
	return rows
}

func topSlowSpanRows(spans []traceparser.Span, limit int) []map[string]any {
	ordered := append([]traceparser.Span(nil), spans...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].DurationNanos != ordered[j].DurationNanos {
			return ordered[i].DurationNanos > ordered[j].DurationNanos
		}
		if ordered[i].TraceID != ordered[j].TraceID {
			return ordered[i].TraceID < ordered[j].TraceID
		}
		return ordered[i].SpanID < ordered[j].SpanID
	})
	if limit > 0 && len(ordered) > limit {
		ordered = ordered[:limit]
	}
	rows := make([]map[string]any, 0, len(ordered))
	for _, span := range ordered {
		rows = append(rows, map[string]any{
			"trace_id":    span.TraceID,
			"span_id":     span.SpanID,
			"name":        span.Name,
			"service":     labelOr(span.ServiceName, "(unknown)"),
			"duration_ms": nanosToMillis(span.DurationNanos),
			"error":       span.Error,
		})
	}
	return rows
}

func traceStatsSorted(stats map[string]*traceStat) []*traceStat {
	items := make([]*traceStat, 0, len(stats))
	for _, stat := range stats {
		items = append(items, stat)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].DurationNanos() != items[j].DurationNanos() {
			return items[i].DurationNanos() > items[j].DurationNanos()
		}
		return items[i].TraceID < items[j].TraceID
	})
	return items
}

type counter struct {
	counts map[string]int
	order  []string
}

func newCounter() *counter {
	return &counter{counts: map[string]int{}}
}

func (c *counter) add(key string) {
	if _, ok := c.counts[key]; !ok {
		c.order = append(c.order, key)
	}
	c.counts[key]++
}

func (c *counter) rows(label string, limit int) []map[string]any {
	keys := append([]string(nil), c.order...)
	sort.SliceStable(keys, func(i, j int) bool {
		if c.counts[keys[i]] != c.counts[keys[j]] {
			return c.counts[keys[i]] > c.counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, map[string]any{label: key, "count": c.counts[key]})
	}
	return rows
}

func getTraceStat(stats map[string]*traceStat, traceID string) *traceStat {
	if traceID == "" {
		traceID = "(no-trace)"
	}
	stat := stats[traceID]
	if stat == nil {
		stat = &traceStat{TraceID: traceID}
		stats[traceID] = stat
	}
	return stat
}

func getServiceStat(stats map[string]*serviceStat, service string) *serviceStat {
	stat := stats[service]
	if stat == nil {
		stat = &serviceStat{ServiceName: service}
		stats[service] = stat
	}
	return stat
}

func nonUnknownKeys(stats map[string]*serviceStat) []string {
	keys := make([]string, 0, len(stats))
	for key := range stats {
		if key != "(unknown)" {
			keys = append(keys, key)
		}
	}
	return keys
}

func sortedSet(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func spanKey(traceID, spanID string) string {
	return traceID + "\x00" + spanID
}

func labelOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func emptyToNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nanosToMillis(nanos int64) float64 {
	return float64(nanos) / 1_000_000
}

func avgMillis(nanos int64, count int) float64 {
	if count <= 0 {
		return 0
	}
	return nanosToMillis(nanos) / float64(count)
}

func errorRate(errors, count int) float64 {
	if count <= 0 {
		return 0
	}
	return float64(errors) / float64(count)
}

func percentile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	if q <= 0 {
		return sorted[0]
	}
	if q >= 1 {
		return sorted[len(sorted)-1]
	}
	pos := q * float64(len(sorted)-1)
	lower := int(pos)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[lower]
	}
	weight := pos - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}
