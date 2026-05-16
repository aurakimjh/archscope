package observability

import (
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/observability"
)

const ResultType = "observability_evidence"

type Options struct {
	Format string
	TopN   int
}

func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	records, diags, err := parser.ParseFile(path, opts.Format, parser.Options{})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return Build(records, path, diags, opts), nil
}

func Build(records []parser.Record, sourceFile string, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = 50
	}
	kindCounts := map[string]int{}
	severityCounts := map[string]int{}
	errorCount := 0
	traceRefs := 0
	rows := []map[string]any{}
	for _, record := range records {
		kindCounts[record.Kind]++
		severityCounts[record.Severity]++
		if record.Severity == "ERROR" {
			errorCount++
		}
		if record.TraceID != "" {
			traceRefs++
		}
		if len(rows) < topN {
			rows = append(rows, map[string]any{
				"kind":        record.Kind,
				"timestamp":   record.Timestamp,
				"severity":    record.Severity,
				"service":     record.Service,
				"message":     record.Message,
				"trace_id":    record.TraceID,
				"span_id":     record.SpanID,
				"dashboard":   record.Dashboard,
				"panel_id":    record.PanelID,
				"panel_title": record.PanelTitle,
				"attributes":  record.Attributes,
			})
		}
	}
	result := models.New(ResultType, "observability_json")
	result.SourceFiles = []string{sourceFile}
	result.Summary = map[string]any{
		"total_records":         len(records),
		"error_count":           errorCount,
		"trace_reference_count": traceRefs,
		"dashboard_panel_count": kindCounts["grafana_panel"],
	}
	result.Series = map[string]any{
		"kind_distribution":     rowsFromCounts(kindCounts, "kind", topN),
		"severity_distribution": rowsFromCounts(severityCounts, "severity", topN),
	}
	result.Tables = map[string]any{"records": rows}
	result.Metadata.SchemaVersion = "0.1.0"
	result.Metadata.Diagnostics = diags
	result.Metadata.Extra["format"] = opts.Format
	if errorCount > 0 {
		result.AddFinding("warning", "OBSERVABILITY_ERRORS_PRESENT", "Loki or Tempo evidence contains error records.", map[string]any{"error_count": errorCount})
	}
	if kindCounts["grafana_panel"] > 0 {
		result.AddFinding("info", "GRAFANA_DASHBOARD_REFERENCES", "Grafana dashboard panels are available as evidence references.", map[string]any{"dashboard_panel_count": kindCounts["grafana_panel"]})
	}
	ingestion.AttachSourceMetadata(&result, ingestion.NewSourceMetadata(sourceFile, ingestion.SourceMetadataOptions{
		SourceKind:   ingestion.SourceKindObservabilityLog,
		SourceFormat: opts.Format,
		Product:      "LGTM/Grafana",
	}))
	return result
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
