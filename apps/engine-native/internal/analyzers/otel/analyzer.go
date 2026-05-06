// Package otel ports archscope_engine.analyzers.otel_analyzer.
//
// The analyzer consumes records produced by internal/parsers/otel and
// emits the AnalysisResult envelope (Type = "otel_logs"). It builds:
//
//   - per-trace service paths via parent-span DAG traversal (with a
//     timestamp/span-id fallback when no parent edges exist);
//   - failure propagation rows that surface downstream services hit
//     after the first error within a trace;
//   - span topology warnings (SELF_PARENT and PARENT_CYCLE) so
//     malformed parent chains surface as findings;
//   - the standard Counter-style severity / service / trace
//     distributions plus cross-service trace correlation.
//
// Findings (codes verbatim with Python):
//
//	OTEL_ERROR_RECORDS_PRESENT
//	OTEL_CROSS_SERVICE_TRACE
//	OTEL_FAILED_TRACE
//	OTEL_FAILURE_PROPAGATION
//	OTEL_SPAN_TOPOLOGY_INCONSISTENT
package otel

import (
	"regexp"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	otelparser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/otel"
)

const (
	// DefaultTopN mirrors the Python `top_n=20` default applied to
	// every Counter.most_common / table-truncation call.
	DefaultTopN = 20
	// SchemaVersion mirrors the Python analyzer's `schema_version`
	// metadata literal.
	SchemaVersion = "0.1.0"
	// ResultType is the envelope's `type` field. Matches Python's
	// `AnalysisResult(type="otel_logs", ...)` literal.
	ResultType = "otel_logs"
	// ParserName mirrors the Python `metadata.parser` literal.
	ParserName = "otel_jsonl"

	noTracePlaceholder   = "(no-trace)"
	unknownServiceLabel  = "(unknown)"
	defaultMaxSeverity   = "UNSPECIFIED"
	severityRankUnknown  = 0
)

// Options carries the analyzer-level knobs surfaced to callers. TopN
// is the same parameter Python exposes; zero (the Go default) routes
// to DefaultTopN so callers don't have to spell the number.
type Options struct {
	TopN int
}

func (o Options) topN() int {
	if o.TopN <= 0 {
		return DefaultTopN
	}
	return o.TopN
}

// Analyze parses the JSONL at `path` and returns the populated
// AnalysisResult. Mirrors Python `analyze_otel_jsonl`.
func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	records, diags, err := otelparser.ParseFile(path, otelparser.Options{})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return Build(records, path, diags, opts), nil
}

// Build assembles the AnalysisResult from already-parsed records.
// Splitting Analyze and Build matches the access-log analyzer's
// two-tier API and lets tests / CLI callers feed records they parsed
// themselves.
func Build(records []otelparser.Record, sourceFile string, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	topN := opts.topN()

	// ── Counter-equivalents (insertion order = first-seen order) ──
	traceCounts := newOrderedCounter[string]()
	serviceCounts := newOrderedCounter[string]()
	severityCounts := newOrderedCounter[string]()

	traceServices := newOrderedSetMap()
	traceRecords := newOrderedRecordMap()

	for _, record := range records {
		traceKey := derefOr(record.TraceID, noTracePlaceholder)
		serviceKey := derefOr(record.ServiceName, unknownServiceLabel)
		traceCounts.add(traceKey)
		serviceCounts.add(serviceKey)
		severityCounts.add(record.Severity)

		if record.TraceID != nil && record.ServiceName != nil {
			traceServices.add(*record.TraceID, *record.ServiceName)
		}
		if record.TraceID != nil {
			traceRecords.append(*record.TraceID, record)
		}
	}

	crossServiceTraceIDs := []string{}
	for _, trace := range traceServices.orderedKeys {
		if traceServices.cardinality(trace) > 1 {
			crossServiceTraceIDs = append(crossServiceTraceIDs, trace)
		}
	}

	// Sorted view of trace_records — Python `sorted(trace_records.items())`.
	sortedTraceIDs := append([]string(nil), traceRecords.orderedKeys...)
	sort.Strings(sortedTraceIDs)

	tracePaths := buildTracePaths(traceRecords, sortedTraceIDs)
	traceFailures := buildTraceFailures(traceRecords, sortedTraceIDs)
	failureProp := buildFailurePropagation(traceRecords, sortedTraceIDs)
	topologyWarnings := buildSpanTopologyWarnings(traceRecords, sortedTraceIDs)

	uniqueTraces := 0
	for _, key := range traceCounts.orderedKeys {
		if key != noTracePlaceholder {
			uniqueTraces++
		}
	}
	uniqueServices := 0
	for _, key := range serviceCounts.orderedKeys {
		if key != unknownServiceLabel {
			uniqueServices++
		}
	}

	errorRecords := 0
	for _, key := range severityCounts.orderedKeys {
		if isErrorSeverityName(key) {
			errorRecords += severityCounts.counts[key]
		}
	}

	summary := map[string]any{
		"total_records":              len(records),
		"unique_traces":              uniqueTraces,
		"unique_services":            uniqueServices,
		"cross_service_traces":       len(crossServiceTraceIDs),
		"failed_traces":              len(traceFailures),
		"failure_propagation_traces": len(failureProp),
		"span_topology_warnings":     len(topologyWarnings),
		"error_records":              errorRecords,
	}

	series := map[string]any{
		"severity_distribution": severityCounts.mostCommonRows("severity", topN),
		"service_distribution":  serviceCounts.mostCommonRows("service", topN),
		"top_traces":            traceCounts.mostCommonRows("trace_id", topN),
	}

	// Records table — first `topN` records in ingestion order.
	recordsTable := make([]map[string]any, 0, topN)
	for i, record := range records {
		if i >= topN {
			break
		}
		recordsTable = append(recordsTable, map[string]any{
			"timestamp":      ptrToAny(record.Timestamp),
			"trace_id":       ptrToAny(record.TraceID),
			"span_id":        ptrToAny(record.SpanID),
			"parent_span_id": ptrToAny(record.ParentSpanID),
			"service_name":   ptrToAny(record.ServiceName),
			"severity":       record.Severity,
			"body":           record.Body,
		})
	}

	crossRows := make([]map[string]any, 0, topN)
	sortedCross := append([]string(nil), crossServiceTraceIDs...)
	sort.Strings(sortedCross)
	for i, trace := range sortedCross {
		if i >= topN {
			break
		}
		services := traceServices.sortedValues(trace)
		crossRows = append(crossRows, map[string]any{
			"trace_id": trace,
			"services": services,
		})
	}

	tables := map[string]any{
		"records":                     recordsTable,
		"cross_service_traces":        crossRows,
		"trace_service_paths":         truncateRows(tracePaths, topN),
		"trace_failures":              truncateRows(traceFailures, topN),
		"service_failure_propagation": truncateRows(failureProp, topN),
		"trace_span_topology":         buildTraceSpanTopology(traceRecords, sortedTraceIDs, topN),
		"span_topology_warnings":      truncateRows(topologyWarnings, topN),
		"service_trace_matrix":        buildServiceTraceMatrix(traceRecords, sortedTraceIDs, topN),
	}

	if diags == nil {
		diags = diagnostics.New("otel_jsonl")
		diags.TotalLines = len(records)
		diags.ParsedRecords = len(records)
	}

	result := models.New(ResultType, ParserName)
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary
	result.Series = series
	result.Tables = tables
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = diags
	for _, finding := range buildFindings(summary) {
		result.Metadata.Findings = append(result.Metadata.Findings, finding)
	}
	return result
}

// ── Trace tables ────────────────────────────────────────────────────

func buildTracePaths(traceRecords *orderedRecordMap, sortedTraceIDs []string) []map[string]any {
	rows := make([]map[string]any, 0, len(sortedTraceIDs))
	for _, traceID := range sortedTraceIDs {
		recs := traceRecords.records[traceID]
		services := orderedServices(recs)
		severities := make([]string, 0, len(recs))
		hasError := false
		for _, r := range recs {
			severities = append(severities, r.Severity)
			if isErrorRecord(r) {
				hasError = true
			}
		}
		rows = append(rows, map[string]any{
			"trace_id":      traceID,
			"service_path":  strings.Join(services, " -> "),
			"service_count": len(services),
			"record_count":  len(recs),
			"has_error":     hasError,
			"max_severity":  maxSeverity(severities),
		})
	}
	return rows
}

func buildTraceFailures(traceRecords *orderedRecordMap, sortedTraceIDs []string) []map[string]any {
	rows := make([]map[string]any, 0)
	for _, traceID := range sortedTraceIDs {
		recs := traceRecords.records[traceID]
		ordered := orderedRecords(recs)
		errorRecs := make([]otelparser.Record, 0)
		for _, r := range ordered {
			if isErrorRecord(r) {
				errorRecs = append(errorRecs, r)
			}
		}
		if len(errorRecs) == 0 {
			continue
		}
		// Unique sorted error services.
		errorServiceSet := map[string]struct{}{}
		for _, r := range errorRecs {
			errorServiceSet[derefOr(r.ServiceName, unknownServiceLabel)] = struct{}{}
		}
		errorServices := make([]string, 0, len(errorServiceSet))
		for s := range errorServiceSet {
			errorServices = append(errorServices, s)
		}
		sort.Strings(errorServices)
		rows = append(rows, map[string]any{
			"trace_id":              traceID,
			"services":              strings.Join(orderedServices(recs), " -> "),
			"first_failing_service": derefOr(errorRecs[0].ServiceName, unknownServiceLabel),
			"error_services":        strings.Join(errorServices, ", "),
			"error_count":           len(errorRecs),
			"first_error":           errorRecs[0].Body,
		})
	}
	return rows
}

func buildFailurePropagation(traceRecords *orderedRecordMap, sortedTraceIDs []string) []map[string]any {
	rows := make([]map[string]any, 0)
	for _, traceID := range sortedTraceIDs {
		recs := traceRecords.records[traceID]
		ordered := orderedRecords(recs)
		firstErrorIndex := -1
		for i, r := range ordered {
			if isErrorRecord(r) {
				firstErrorIndex = i
				break
			}
		}
		if firstErrorIndex < 0 {
			continue
		}
		firstError := ordered[firstErrorIndex]
		downstream := make([]string, 0)
		seen := map[string]struct{}{}
		for _, r := range ordered[firstErrorIndex+1:] {
			service := derefOr(r.ServiceName, unknownServiceLabel)
			if _, ok := seen[service]; ok {
				continue
			}
			seen[service] = struct{}{}
			downstream = append(downstream, service)
		}
		if len(downstream) == 0 {
			continue
		}
		rows = append(rows, map[string]any{
			"trace_id":                  traceID,
			"service_path":              strings.Join(orderedServices(recs), " -> "),
			"first_failing_service":     derefOr(firstError.ServiceName, unknownServiceLabel),
			"first_failing_span_id":     ptrToAny(firstError.SpanID),
			"downstream_services":       downstream,
			"downstream_service_count":  len(downstream),
			"first_error":               firstError.Body,
		})
	}
	return rows
}

func buildServiceTraceMatrix(traceRecords *orderedRecordMap, sortedTraceIDs []string, topN int) []map[string]any {
	rows := make([]map[string]any, 0)
	limit := len(sortedTraceIDs)
	if topN < limit {
		limit = topN
	}
	for i := 0; i < limit; i++ {
		traceID := sortedTraceIDs[i]
		recs := traceRecords.records[traceID]
		counts := map[string]int{}
		for _, r := range recs {
			counts[derefOr(r.ServiceName, unknownServiceLabel)]++
		}
		services := make([]string, 0, len(counts))
		for s := range counts {
			services = append(services, s)
		}
		sort.Strings(services)
		for _, service := range services {
			rows = append(rows, map[string]any{
				"trace_id":     traceID,
				"service":      service,
				"record_count": counts[service],
			})
		}
	}
	return rows
}

func buildTraceSpanTopology(traceRecords *orderedRecordMap, sortedTraceIDs []string, topN int) []map[string]any {
	rows := make([]map[string]any, 0)
	limit := len(sortedTraceIDs)
	if topN < limit {
		limit = topN
	}
	for i := 0; i < limit; i++ {
		traceID := sortedTraceIDs[i]
		recs := traceRecords.records[traceID]
		spanReps := spanRepresentatives(recs)
		// Build children index — only counts parents that exist in
		// span_records (mirrors Python's `record.parent_span_id in
		// span_records else None`).
		childrenByParent := map[string][]string{}
		for spanID, rec := range spanReps {
			parentID := ""
			if rec.ParentSpanID != nil {
				if _, ok := spanReps[*rec.ParentSpanID]; ok {
					parentID = *rec.ParentSpanID
				}
			}
			childrenByParent[parentID] = append(childrenByParent[parentID], spanID)
		}
		spanIDs := make([]string, 0, len(spanReps))
		for sid := range spanReps {
			spanIDs = append(spanIDs, sid)
		}
		sort.Strings(spanIDs)
		for _, spanID := range spanIDs {
			rec := spanReps[spanID]
			hasError := false
			for _, item := range recs {
				if item.SpanID != nil && *item.SpanID == spanID && isErrorRecord(item) {
					hasError = true
					break
				}
			}
			rows = append(rows, map[string]any{
				"trace_id":       traceID,
				"span_id":        spanID,
				"parent_span_id": ptrToAny(rec.ParentSpanID),
				"service":        derefOr(rec.ServiceName, unknownServiceLabel),
				"child_count":    len(childrenByParent[spanID]),
				"has_error":      hasError,
			})
		}
	}
	return rows
}

func buildSpanTopologyWarnings(traceRecords *orderedRecordMap, sortedTraceIDs []string) []map[string]any {
	rows := make([]map[string]any, 0)
	for _, traceID := range sortedTraceIDs {
		recs := traceRecords.records[traceID]
		spanReps := spanRepresentatives(recs)

		spanIDs := make([]string, 0, len(spanReps))
		for sid := range spanReps {
			spanIDs = append(spanIDs, sid)
		}
		sort.Strings(spanIDs)

		// SELF_PARENT pass — parent points at self.
		for _, spanID := range spanIDs {
			rec := spanReps[spanID]
			if rec.ParentSpanID != nil && *rec.ParentSpanID == spanID {
				rows = append(rows, map[string]any{
					"trace_id":       traceID,
					"span_id":        spanID,
					"parent_span_id": ptrToAny(rec.ParentSpanID),
					"reason":         "SELF_PARENT",
				})
			}
		}

		// PARENT_CYCLE pass — DFS along parent edges (NOT children),
		// matching Python's `visit(parent_id, ...)` recursion. The
		// `path` list represents the chain we're currently exploring;
		// when we revisit a node on `visiting`, slice it out as the
		// cycle. `cycle != span_id` guards against self-parent
		// double-reporting (already covered above).
		visiting := map[string]struct{}{}
		visited := map[string]struct{}{}
		reportedCycles := map[string]struct{}{}

		var visit func(spanID string, path []string)
		visit = func(spanID string, path []string) {
			if _, ok := visited[spanID]; ok {
				return
			}
			if _, ok := visiting[spanID]; ok {
				cycleStart := -1
				for i, p := range path {
					if p == spanID {
						cycleStart = i
						break
					}
				}
				if cycleStart < 0 {
					return
				}
				cycle := path[cycleStart:]
				normalized := normalizeCycle(cycle)
				if _, seen := reportedCycles[normalized]; !seen {
					reportedCycles[normalized] = struct{}{}
					rows = append(rows, map[string]any{
						"trace_id":       traceID,
						"span_id":        spanID,
						"parent_span_id": ptrToAny(spanReps[spanID].ParentSpanID),
						"reason":         "PARENT_CYCLE",
						"cycle":          strings.Join(cycle, " -> "),
					})
				}
				return
			}
			visiting[spanID] = struct{}{}
			rec := spanReps[spanID]
			if rec.ParentSpanID != nil {
				parentID := *rec.ParentSpanID
				if _, ok := spanReps[parentID]; ok && parentID != spanID {
					visit(parentID, append(path, parentID))
				}
			}
			delete(visiting, spanID)
			visited[spanID] = struct{}{}
		}

		for _, spanID := range spanIDs {
			visit(spanID, []string{spanID})
		}
	}
	return rows
}

// normalizeCycle returns the lexicographically smallest rotation of
// `cycle` so the same cycle reported from any starting node maps to
// the same key. Mirrors Python's `_normalize_cycle`.
func normalizeCycle(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	working := cycle
	if len(cycle) > 1 && cycle[0] == cycle[len(cycle)-1] {
		working = cycle[:len(cycle)-1]
	}
	if len(working) == 0 {
		return ""
	}
	best := strings.Join(working, "\x00")
	for i := 1; i < len(working); i++ {
		rot := append([]string{}, working[i:]...)
		rot = append(rot, working[:i]...)
		candidate := strings.Join(rot, "\x00")
		if candidate < best {
			best = candidate
		}
	}
	return best
}

// ── Service / record ordering ──────────────────────────────────────

// orderedServices returns the list of service names along the parent
// chain when parent edges exist; otherwise falls back to the
// timestamp-sorted record sequence. Mirrors `_ordered_services`.
func orderedServices(recs []otelparser.Record) []string {
	if parentOrdered := orderedServicesByParent(recs); len(parentOrdered) > 0 {
		return parentOrdered
	}
	services := make([]string, 0)
	for _, r := range orderedRecords(recs) {
		service := derefOr(r.ServiceName, unknownServiceLabel)
		if len(services) == 0 || services[len(services)-1] != service {
			services = append(services, service)
		}
	}
	return services
}

// orderedServicesByParent walks the parent DAG in pre-order: roots
// (parents that don't appear in span_records) first, then any
// otherwise-unreachable spans by sort key. Children of each parent
// are sorted by `(timestamp, span_id)`.
func orderedServicesByParent(recs []otelparser.Record) []string {
	spanReps := spanRepresentatives(recs)
	if !hasAnyParent(spanReps) {
		return nil
	}
	childrenByParent := childrenIndex(spanReps)
	for parent := range childrenByParent {
		sortByRecordKey(childrenByParent[parent], spanReps)
	}

	ordered := make([]string, 0)
	visited := map[string]struct{}{}

	var visit func(spanID string)
	visit = func(spanID string) {
		if _, ok := visited[spanID]; ok {
			return
		}
		visited[spanID] = struct{}{}
		service := derefOr(spanReps[spanID].ServiceName, unknownServiceLabel)
		if len(ordered) == 0 || ordered[len(ordered)-1] != service {
			ordered = append(ordered, service)
		}
		for _, child := range childrenByParent[spanID] {
			visit(child)
		}
	}

	for _, root := range childrenByParent[""] {
		visit(root)
	}
	// Fallback: any remaining (cycle-only) spans.
	leftover := make([]string, 0, len(spanReps))
	for sid := range spanReps {
		leftover = append(leftover, sid)
	}
	sortByRecordKey(leftover, spanReps)
	for _, spanID := range leftover {
		visit(spanID)
	}
	return ordered
}

// orderedRecords mirrors `_ordered_records`: groups by span_id (in
// parent-DAG order) and sorts each group by `(timestamp, span_id)`.
// Records without a span_id sink to the end.
func orderedRecords(recs []otelparser.Record) []otelparser.Record {
	spanReps := spanRepresentatives(recs)
	orderedSpanIDs := orderedSpanIDsByParent(spanReps)
	if len(orderedSpanIDs) == 0 {
		out := append([]otelparser.Record(nil), recs...)
		sortRecordsByKey(out)
		return out
	}
	bySpan := map[string][]otelparser.Record{}
	noSpan := make([]otelparser.Record, 0)
	for _, r := range recs {
		if r.SpanID != nil {
			bySpan[*r.SpanID] = append(bySpan[*r.SpanID], r)
		} else {
			noSpan = append(noSpan, r)
		}
	}
	out := make([]otelparser.Record, 0, len(recs))
	for _, sid := range orderedSpanIDs {
		group := append([]otelparser.Record(nil), bySpan[sid]...)
		sortRecordsByKey(group)
		out = append(out, group...)
	}
	sortRecordsByKey(noSpan)
	out = append(out, noSpan...)
	return out
}

func orderedSpanIDsByParent(spanReps map[string]otelparser.Record) []string {
	if !hasAnyParent(spanReps) {
		return nil
	}
	childrenByParent := childrenIndex(spanReps)
	for parent := range childrenByParent {
		sortByRecordKey(childrenByParent[parent], spanReps)
	}
	ordered := make([]string, 0)
	visited := map[string]struct{}{}
	var visit func(spanID string)
	visit = func(spanID string) {
		if _, ok := visited[spanID]; ok {
			return
		}
		visited[spanID] = struct{}{}
		ordered = append(ordered, spanID)
		for _, child := range childrenByParent[spanID] {
			visit(child)
		}
	}
	for _, root := range childrenByParent[""] {
		visit(root)
	}
	leftover := make([]string, 0, len(spanReps))
	for sid := range spanReps {
		leftover = append(leftover, sid)
	}
	sortByRecordKey(leftover, spanReps)
	for _, spanID := range leftover {
		visit(spanID)
	}
	return ordered
}

func hasAnyParent(spanReps map[string]otelparser.Record) bool {
	for _, rec := range spanReps {
		if rec.ParentSpanID != nil {
			return true
		}
	}
	return false
}

// childrenIndex maps parent span_id → sorted list of children.
// Parents that don't appear in `spanReps` are bucketed under "" so
// they're treated as roots. Uses string "" rather than `*string` so
// the map zero-values nicely; spans always have non-empty IDs.
func childrenIndex(spanReps map[string]otelparser.Record) map[string][]string {
	childrenByParent := map[string][]string{}
	for spanID, rec := range spanReps {
		parentKey := ""
		if rec.ParentSpanID != nil {
			if _, ok := spanReps[*rec.ParentSpanID]; ok {
				parentKey = *rec.ParentSpanID
			}
		}
		childrenByParent[parentKey] = append(childrenByParent[parentKey], spanID)
	}
	return childrenByParent
}

// spanRepresentatives picks the earliest record (by `(timestamp,
// span_id)`) for each span_id and returns a map keyed by span_id.
// Mirrors Python `_span_representatives`.
func spanRepresentatives(recs []otelparser.Record) map[string]otelparser.Record {
	sorted := append([]otelparser.Record(nil), recs...)
	sortRecordsByKey(sorted)
	reps := map[string]otelparser.Record{}
	for _, r := range sorted {
		if r.SpanID == nil {
			continue
		}
		if _, ok := reps[*r.SpanID]; !ok {
			reps[*r.SpanID] = r
		}
	}
	return reps
}

func sortRecordsByKey(recs []otelparser.Record) {
	sort.SliceStable(recs, func(i, j int) bool {
		ki := recordSortKey(recs[i])
		kj := recordSortKey(recs[j])
		if ki[0] != kj[0] {
			return ki[0] < kj[0]
		}
		return ki[1] < kj[1]
	})
}

func sortByRecordKey(spanIDs []string, spanReps map[string]otelparser.Record) {
	sort.SliceStable(spanIDs, func(i, j int) bool {
		ki := recordSortKey(spanReps[spanIDs[i]])
		kj := recordSortKey(spanReps[spanIDs[j]])
		if ki[0] != kj[0] {
			return ki[0] < kj[0]
		}
		return ki[1] < kj[1]
	})
}

// recordSortKey mirrors Python `_record_sort_key`: `(timestamp or "",
// span_id or "")`.
func recordSortKey(r otelparser.Record) [2]string {
	return [2]string{derefOr(r.Timestamp, ""), derefOr(r.SpanID, "")}
}

// ── Severity classification ────────────────────────────────────────

var bodyErrorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\berror\b`),
	regexp.MustCompile(`\bexception\b`),
	regexp.MustCompile(`\bfailed\b`),
	regexp.MustCompile(`\bfailure\b`),
	regexp.MustCompile(`\btimeout\b`),
	regexp.MustCompile(`\binsufficient_stock\b`),
	regexp.MustCompile(`\bgateway_timeout\b`),
}

func isErrorRecord(record otelparser.Record) bool {
	severity := strings.ToUpper(strings.TrimSpace(record.Severity))
	switch severity {
	case "ERROR", "FATAL", "CRITICAL":
		return true
	case "TRACE", "DEBUG", "INFO", "NOTICE", "WARN", "WARNING":
		return false
	}
	body := strings.ToLower(record.Body)
	for _, pat := range bodyErrorPatterns {
		if pat.MatchString(body) {
			return true
		}
	}
	return false
}

func isErrorSeverityName(severity string) bool {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "ERROR", "FATAL", "CRITICAL":
		return true
	}
	return false
}

func severityRank(severity string) int {
	switch strings.ToUpper(severity) {
	case "UNSPECIFIED":
		return 0
	case "DEBUG":
		return 1
	case "INFO":
		return 2
	case "WARN", "WARNING":
		return 3
	case "ERROR":
		return 4
	case "FATAL", "CRITICAL":
		return 5
	}
	return severityRankUnknown
}

// maxSeverity mirrors Python `_max_severity`. Python's `max` is
// stable: ties keep the first occurrence. We emulate this by
// iterating in input order and keeping the first item whose rank
// strictly exceeds the running best.
func maxSeverity(severities []string) string {
	if len(severities) == 0 {
		return defaultMaxSeverity
	}
	best := severities[0]
	bestRank := severityRank(best)
	for _, sev := range severities[1:] {
		rank := severityRank(sev)
		if rank > bestRank {
			bestRank = rank
			best = sev
		}
	}
	return best
}

// ── Findings ───────────────────────────────────────────────────────

func buildFindings(summary map[string]any) []map[string]any {
	findings := []map[string]any{}
	if asInt(summary["error_records"]) > 0 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "OTEL_ERROR_RECORDS_PRESENT",
			"message":  "OpenTelemetry logs include error-level records.",
			"evidence": map[string]any{"error_records": summary["error_records"]},
		})
	}
	if asInt(summary["cross_service_traces"]) > 0 {
		findings = append(findings, map[string]any{
			"severity": "info",
			"code":     "OTEL_CROSS_SERVICE_TRACE",
			"message":  "At least one trace appears across multiple services.",
			"evidence": map[string]any{"cross_service_traces": summary["cross_service_traces"]},
		})
	}
	if asInt(summary["failed_traces"]) > 0 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "OTEL_FAILED_TRACE",
			"message":  "At least one distributed trace contains an error signal.",
			"evidence": map[string]any{"failed_traces": summary["failed_traces"]},
		})
	}
	if asInt(summary["failure_propagation_traces"]) > 0 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "OTEL_FAILURE_PROPAGATION",
			"message":  "Failure signals appear before downstream services in at least one trace.",
			"evidence": map[string]any{"failure_propagation_traces": summary["failure_propagation_traces"]},
		})
	}
	if asInt(summary["span_topology_warnings"]) > 0 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "OTEL_SPAN_TOPOLOGY_INCONSISTENT",
			"message":  "OpenTelemetry span parent relationships include cycles.",
			"evidence": map[string]any{"span_topology_warnings": summary["span_topology_warnings"]},
		})
	}
	return findings
}

// ── Helpers ────────────────────────────────────────────────────────

// orderedCounter mirrors `collections.Counter` but guarantees
// insertion order for `most_common(top_n)` ties — matching CPython's
// dict semantics. Generic over the key type so we can reuse it for
// trace_id (string) and severity (string) without copy-paste.
type orderedCounter[K comparable] struct {
	counts      map[K]int
	orderedKeys []K
}

func newOrderedCounter[K comparable]() *orderedCounter[K] {
	return &orderedCounter[K]{counts: map[K]int{}}
}

func (c *orderedCounter[K]) add(key K) {
	if _, ok := c.counts[key]; !ok {
		c.orderedKeys = append(c.orderedKeys, key)
	}
	c.counts[key]++
}

// mostCommonRows returns Counter.most_common(topN) projected as
// `[{labelKey: <key>, "count": <count>}]`. Stable on ties — keys
// preserve insertion order, mirroring CPython's `Counter`.
func (c *orderedCounter[K]) mostCommonRows(labelKey string, topN int) []map[string]any {
	type indexed struct {
		key   K
		count int
		order int
	}
	rows := make([]indexed, 0, len(c.orderedKeys))
	for i, k := range c.orderedKeys {
		rows = append(rows, indexed{k, c.counts[k], i})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].order < rows[j].order
	})
	if topN > 0 && len(rows) > topN {
		rows = rows[:topN]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{labelKey: r.key, "count": r.count})
	}
	return out
}

// orderedSetMap is a defaultdict[set] with stable ordering of keys.
// Used for trace_id → {service_name} membership.
type orderedSetMap struct {
	values      map[string]map[string]struct{}
	orderedKeys []string
}

func newOrderedSetMap() *orderedSetMap {
	return &orderedSetMap{values: map[string]map[string]struct{}{}}
}

func (m *orderedSetMap) add(key, value string) {
	bucket, ok := m.values[key]
	if !ok {
		bucket = map[string]struct{}{}
		m.values[key] = bucket
		m.orderedKeys = append(m.orderedKeys, key)
	}
	bucket[value] = struct{}{}
}

func (m *orderedSetMap) cardinality(key string) int {
	return len(m.values[key])
}

func (m *orderedSetMap) sortedValues(key string) []string {
	bucket := m.values[key]
	out := make([]string, 0, len(bucket))
	for v := range bucket {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// orderedRecordMap is a defaultdict[list[Record]] keyed by trace_id.
type orderedRecordMap struct {
	records     map[string][]otelparser.Record
	orderedKeys []string
}

func newOrderedRecordMap() *orderedRecordMap {
	return &orderedRecordMap{records: map[string][]otelparser.Record{}}
}

func (m *orderedRecordMap) append(traceID string, record otelparser.Record) {
	if _, ok := m.records[traceID]; !ok {
		m.orderedKeys = append(m.orderedKeys, traceID)
	}
	m.records[traceID] = append(m.records[traceID], record)
}

func derefOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}

func ptrToAny(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func truncateRows(rows []map[string]any, limit int) []map[string]any {
	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
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
