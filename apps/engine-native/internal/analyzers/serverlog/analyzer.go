package serverlog

import (
	"sort"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/serverlog"
)

const (
	ResultType    = "server_log"
	ParserName    = "server_log"
	SchemaVersion = "0.1.0"
	DefaultTopN   = 25
)

type Options struct {
	MaxLines int
	TopN     int
	Strict   bool
}

func Analyze(path, format string, opts Options) (models.AnalysisResult, error) {
	var diags *diagnostics.ParserDiagnostics
	result, err := buildFromIterator(path, format, nil, opts, func(yield func(parser.Record) error) error {
		var err error
		diags, err = parser.ForEachRecord(path, format, parser.Options{
			MaxLines: opts.MaxLines,
			Strict:   opts.Strict,
		}, yield)
		return err
	})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	result.Metadata.Diagnostics = diags
	return result, nil
}

func Build(records []parser.Record, sourceFile, format string, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	result, _ := buildFromIterator(sourceFile, format, diags, opts, func(yield func(parser.Record) error) error {
		for _, record := range records {
			if err := yield(record); err != nil {
				return err
			}
		}
		return nil
	})
	return result
}

func buildFromIterator(
	sourceFile string,
	format string,
	diags *diagnostics.ParserDiagnostics,
	opts Options,
	iter func(func(parser.Record) error) error,
) (models.AnalysisResult, error) {
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}
	total := 0
	errorCount := 0
	warningCount := 0
	correlated := 0
	var earliest, latest *time.Time
	severityCounts := map[string]int{}
	productCounts := map[string]int{}
	componentCounts := map[string]int{}
	eventCounts := map[string]int{}
	rows := make([]map[string]any, 0, topN)
	correlationKeys := []ingestion.CorrelationKeys{}

	if iter == nil {
		iter = func(func(parser.Record) error) error { return nil }
	}
	if err := iter(func(record parser.Record) error {
		total++
		severity := record.Severity
		if severity == "" {
			severity = "INFO"
		}
		severityCounts[severity]++
		if severity == "ERROR" || severity == "FATAL" || severity == "CRITICAL" {
			errorCount++
		}
		if severity == "WARN" || severity == "WARNING" {
			warningCount++
		}
		productCounts[firstNonEmpty(record.Product, "unknown")]++
		componentCounts[firstNonEmpty(record.Component, "unknown")]++
		eventCounts[firstNonEmpty(record.EventType, "server_event")]++
		if record.TraceID != "" || record.RequestID != "" {
			correlated++
			correlationKeys = append(correlationKeys, ingestion.CorrelationKeys{
				TraceID:   record.TraceID,
				RequestID: record.RequestID,
				HostID:    record.Host,
				Window:    ingestion.TimestampWindowFor(record.Timestamp, ingestion.DefaultCorrelationWindow),
			})
		}
		if !record.Timestamp.IsZero() {
			ts := record.Timestamp
			if earliest == nil || ts.Before(*earliest) {
				earliest = &ts
			}
			if latest == nil || ts.After(*latest) {
				latest = &ts
			}
		}
		if len(rows) < topN {
			rows = append(rows, row(record))
		}
		return nil
	}); err != nil {
		return models.AnalysisResult{}, err
	}

	summary := map[string]any{
		"total_events":                total,
		"error_count":                 errorCount,
		"warning_count":               warningCount,
		"startup_count":               eventCounts["startup"],
		"deployment_event_count":      eventCounts["deployment"],
		"datasource_event_count":      eventCounts["datasource_pool"],
		"stuck_thread_count":          eventCounts["stuck_thread"],
		"hung_thread_count":           eventCounts["hung_thread"],
		"thread_pool_pressure_count":  eventCounts["thread_pool_pressure"],
		"worker_error_count":          eventCounts["worker_error"],
		"managed_server_health_count": eventCounts["managed_server_health"],
		"severe_error_count":          eventCounts["severe_error"],
		"correlated_event_count":      correlated,
		"earliest_timestamp":          timeString(earliest),
		"latest_timestamp":            timeString(latest),
	}

	tables := map[string]any{
		"events":                 rows,
		"deployment_events":      filterRows(rows, "event_type", "deployment"),
		"datasource_events":      filterRows(rows, "event_type", "datasource_pool"),
		"thread_events":          filterRowsAny(rows, "event_type", "stuck_thread", "hung_thread", "thread_pool_pressure"),
		"worker_errors":          filterRows(rows, "event_type", "worker_error"),
		"correlation_candidates": filterCorrelated(rows),
	}
	result := models.New(ResultType, ParserName)
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary
	result.Series = map[string]any{
		"severity_distribution":   commonRows(severityCounts, "severity", topN),
		"product_distribution":    commonRows(productCounts, "product", topN),
		"component_distribution":  commonRows(componentCounts, "component", topN),
		"event_type_distribution": commonRows(eventCounts, "event_type", topN),
	}
	result.Tables = tables
	result.Metadata.SchemaVersion = SchemaVersion
	if diags == nil {
		diags = diagnostics.New(firstNonEmpty(format, "auto"))
		diags.TotalLines = total
		diags.ParsedRecords = total
	}
	result.Metadata.Diagnostics = diags
	result.Metadata.Findings = buildFindings(summary, rows)
	result.Metadata.Extra["format"] = firstNonEmpty(format, "auto")
	result.Metadata.Extra["analysis_options"] = map[string]any{
		"max_lines": opts.MaxLines,
		"top_n":     topN,
	}
	ingestion.AttachSourceMetadata(&result, ingestion.NewSourceMetadata(sourceFile, ingestion.SourceMetadataOptions{
		SourceKind:   ingestion.SourceKindServerLog,
		SourceFormat: firstNonEmpty(format, "auto"),
		Product:      dominant(productCounts),
	}))
	ingestion.AttachCorrelationKeys(&result, correlationKeys)
	return result, nil
}

func row(record parser.Record) map[string]any {
	return map[string]any{
		"timestamp":    timeStringValue(record.Timestamp),
		"severity":     record.Severity,
		"product":      record.Product,
		"component":    record.Component,
		"thread":       record.Thread,
		"host":         record.Host,
		"service_name": record.ServiceName,
		"event_type":   record.EventType,
		"message":      record.Message,
		"trace_id":     record.TraceID,
		"request_id":   record.RequestID,
	}
}

func buildFindings(summary map[string]any, rows []map[string]any) []map[string]any {
	findings := []map[string]any{}
	add := func(severity, code, message string, evidence map[string]any) {
		findings = append(findings, map[string]any{
			"severity": severity,
			"code":     code,
			"message":  message,
			"evidence": evidence,
		})
	}
	if asInt(summary["error_count"]) > 0 {
		add("warning", "SERVER_SEVERE_ERRORS", "Server logs contain error or severe records.", map[string]any{"error_count": summary["error_count"]})
	}
	if count := asInt(summary["deployment_event_count"]); count > 0 && hasMessage(rows, "deployment", "fail") {
		add("critical", "DEPLOYMENT_FAILURE", "Deployment failure evidence is present.", map[string]any{"deployment_event_count": count})
	}
	if count := asInt(summary["datasource_event_count"]); count > 0 {
		add("warning", "DATASOURCE_POOL_WARNING", "Datasource or JDBC pool warning evidence is present.", map[string]any{"datasource_event_count": count})
	}
	if count := asInt(summary["stuck_thread_count"]); count > 0 {
		add("critical", "STUCK_THREAD_DETECTED", "Stuck thread evidence is present.", map[string]any{"stuck_thread_count": count})
	}
	if count := asInt(summary["hung_thread_count"]); count > 0 {
		add("critical", "HUNG_THREAD_DETECTED", "Hung thread evidence is present.", map[string]any{"hung_thread_count": count})
	}
	if count := asInt(summary["thread_pool_pressure_count"]); count > 0 {
		add("warning", "THREAD_POOL_PRESSURE", "Thread pool pressure evidence is present.", map[string]any{"thread_pool_pressure_count": count})
	}
	if count := asInt(summary["worker_error_count"]); count > 0 {
		add("warning", "WORKER_ERROR_PRESENT", "Web-server worker or upstream error evidence is present.", map[string]any{"worker_error_count": count})
	}
	if count := asInt(summary["managed_server_health_count"]); count > 0 {
		add("warning", "MANAGED_SERVER_HEALTH", "Managed-server health evidence is present.", map[string]any{"managed_server_health_count": count})
	}
	return findings
}

func commonRows(counts map[string]int, key string, limit int) []map[string]any {
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
		out = append(out, map[string]any{key: item.key, "count": item.count})
	}
	return out
}

func filterRows(rows []map[string]any, key, value string) []map[string]any {
	return filterRowsAny(rows, key, value)
}

func filterRowsAny(rows []map[string]any, key string, values ...string) []map[string]any {
	allowed := map[string]struct{}{}
	for _, value := range values {
		allowed[value] = struct{}{}
	}
	out := []map[string]any{}
	for _, row := range rows {
		if _, ok := allowed[asString(row[key])]; ok {
			out = append(out, row)
		}
	}
	return out
}

func filterCorrelated(rows []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, row := range rows {
		if asString(row["trace_id"]) != "" || asString(row["request_id"]) != "" {
			out = append(out, row)
		}
	}
	return out
}

func hasMessage(rows []map[string]any, eventType, needle string) bool {
	for _, row := range rows {
		if asString(row["event_type"]) == eventType && strings.Contains(strings.ToLower(asString(row["message"])), needle) {
			return true
		}
	}
	return false
}

func dominant(counts map[string]int) string {
	rows := commonRows(counts, "value", 1)
	if len(rows) == 0 {
		return ""
	}
	return asString(rows[0]["value"])
}

func timeString(ts *time.Time) any {
	if ts == nil || ts.IsZero() {
		return nil
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func timeStringValue(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func asInt(v any) int {
	if n, ok := v.(int); ok {
		return n
	}
	return 0
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
