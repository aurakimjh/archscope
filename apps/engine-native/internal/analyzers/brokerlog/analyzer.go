package brokerlog

import (
	"sort"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/brokerlog"
)

const (
	ResultType  = "broker_log"
	ParserName  = "broker_log"
	DefaultTopN = 50
)

type Options struct {
	Format string
	TopN   int
	Strict bool
}

func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	records, diags, err := parser.ParseFile(path, opts.Format, parser.Options{Strict: opts.Strict})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return Build(records, path, diags, opts), nil
}

func Build(records []parser.Record, sourceFile string, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}
	eventCounts := map[string]int{}
	brokerCounts := map[string]int{}
	errorCount := 0
	var earliest, latest time.Time
	rows := []map[string]any{}
	edges := map[string]int{}
	for _, record := range records {
		eventCounts[record.EventType]++
		brokerCounts[record.Broker]++
		if record.Severity == "ERROR" {
			errorCount++
		}
		if !record.Timestamp.IsZero() {
			if earliest.IsZero() || record.Timestamp.Before(earliest) {
				earliest = record.Timestamp
			}
			if latest.IsZero() || record.Timestamp.After(latest) {
				latest = record.Timestamp
			}
		}
		target := firstNonEmpty(record.Topic, record.Queue, record.Broker)
		if target != "" {
			edges["broker:"+target]++
		}
		if len(rows) < topN {
			rows = append(rows, row(record))
		}
	}
	result := models.New(ResultType, ParserName)
	result.SourceFiles = []string{sourceFile}
	result.Summary = map[string]any{
		"total_events":                len(records),
		"error_count":                 errorCount,
		"rebalance_count":             eventCounts["rebalance"],
		"replication_issue_count":     eventCounts["replication"],
		"queue_pressure_count":        eventCounts["queue_pressure"],
		"dead_letter_count":           eventCounts["dead_letter"],
		"partition_event_count":       eventCounts["partition"],
		"broker_health_count":         eventCounts["broker_health"],
		"slow_consumer_count":         eventCounts["slow_consumer"],
		"authorization_failure_count": eventCounts["authorization_failure"],
		"earliest_timestamp":          timeString(earliest),
		"latest_timestamp":            timeString(latest),
	}
	result.Series = map[string]any{
		"event_type_distribution": rowsFromCounts(eventCounts, "event_type", topN),
		"broker_distribution":     rowsFromCounts(brokerCounts, "broker", topN),
	}
	result.Tables = map[string]any{
		"events":               rows,
		"service_dependencies": dependencyRows(edges),
	}
	if diags == nil {
		diags = diagnostics.New(firstNonEmpty(opts.Format, "auto"))
		diags.TotalLines = len(records)
		diags.ParsedRecords = len(records)
	}
	result.Metadata.SchemaVersion = "0.1.0"
	result.Metadata.Diagnostics = diags
	result.Metadata.Extra["format"] = firstNonEmpty(opts.Format, "auto")
	addFindings(&result)
	ingestion.AttachSourceMetadata(&result, ingestion.NewSourceMetadata(sourceFile, ingestion.SourceMetadataOptions{
		SourceKind:   ingestion.SourceKindBrokerLog,
		SourceFormat: firstNonEmpty(opts.Format, "auto"),
		Product:      dominant(brokerCounts),
	}))
	return result
}

func row(record parser.Record) map[string]any {
	return map[string]any{"timestamp": timeString(record.Timestamp), "broker": record.Broker, "severity": record.Severity, "event_type": record.EventType, "topic": record.Topic, "queue": record.Queue, "partition": record.Partition, "node": record.Node, "message": record.Message}
}

func addFindings(result *models.AnalysisResult) {
	checks := []struct{ key, code, msg, sev string }{
		{"rebalance_count", "BROKER_REBALANCE", "Broker rebalance evidence is present.", "warning"},
		{"replication_issue_count", "BROKER_REPLICATION_ISSUE", "Broker replication or ISR issue evidence is present.", "critical"},
		{"queue_pressure_count", "BROKER_QUEUE_PRESSURE", "Queue/backlog pressure evidence is present.", "warning"},
		{"dead_letter_count", "BROKER_DEAD_LETTER", "Dead-letter evidence is present.", "warning"},
		{"broker_health_count", "BROKER_HEALTH_EVENT", "Broker health event evidence is present.", "warning"},
		{"slow_consumer_count", "BROKER_SLOW_CONSUMER", "Slow consumer evidence is present.", "warning"},
		{"authorization_failure_count", "BROKER_AUTHORIZATION_FAILURE", "Authorization failure evidence is present.", "critical"},
	}
	for _, check := range checks {
		if asInt(result.Summary[check.key]) > 0 {
			result.AddFinding(check.sev, check.code, check.msg, map[string]any{check.key: result.Summary[check.key]})
		}
	}
}

func dependencyRows(edges map[string]int) []map[string]any {
	rows := []map[string]any{}
	for callee, count := range edges {
		rows = append(rows, map[string]any{"caller": "application", "callee": callee, "call_count": count, "error_count": 0, "error_rate": 0.0})
	}
	return rows
}

func rowsFromCounts(counts map[string]int, key string, limit int) []map[string]any {
	type kv struct {
		key   string
		count int
	}
	items := []kv{}
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
	out := []map[string]any{}
	for _, item := range items {
		out = append(out, map[string]any{key: item.key, "count": item.count})
	}
	return out
}

func dominant(counts map[string]int) string {
	rows := rowsFromCounts(counts, "value", 1)
	if len(rows) == 0 {
		return ""
	}
	if s, ok := rows[0]["value"].(string); ok {
		return s
	}
	return ""
}

func timeString(ts time.Time) string {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
