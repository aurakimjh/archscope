// Package traceimport analyzes canonical spans parsed from external APM
// trace export files.
package traceimport

import (
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	traceparser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/traceimport"
)

const (
	ResultType    = "trace_import"
	ParserName    = "trace_import"
	SchemaVersion = "0.1.0"
	DefaultTopN   = 50
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
		"source_format":        sourceFormat,
		"total_spans":          len(spans),
		"unique_traces":        len(traceStats),
		"unique_services":      len(nonUnknownKeys(serviceStats)),
		"unique_dependencies":  len(dependencies),
		"root_spans":           rootSpans,
		"error_spans":          errorSpans,
		"missing_parent_spans": missingParents,
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
	}
	result.Metadata.SchemaVersion = SchemaVersion
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
	for _, finding := range findings(summary, traceStats) {
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
	out := map[string]*dependencyStat{}
	for _, span := range spans {
		caller := ""
		callee := ""
		if span.RemoteService != "" {
			caller = labelOr(span.ServiceName, "(unknown)")
			callee = span.RemoteService
		} else if span.ParentSpanID != "" {
			parent, ok := spanByID[spanKey(span.TraceID, span.ParentSpanID)]
			if ok && parent.ServiceName != "" && parent.ServiceName != span.ServiceName {
				caller = parent.ServiceName
				callee = labelOr(span.ServiceName, "(unknown)")
			}
		}
		if caller == "" || callee == "" || caller == callee {
			continue
		}
		key := caller + "\x00" + callee
		stat := out[key]
		if stat == nil {
			stat = &dependencyStat{Caller: caller, Callee: callee}
			out[key] = stat
		}
		stat.Count++
		stat.TotalDurationNanos += span.DurationNanos
		if span.Error {
			stat.ErrorCount++
		}
	}
	return out
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

func findings(summary map[string]any, traces map[string]*traceStat) []map[string]any {
	out := []map[string]any{}
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
