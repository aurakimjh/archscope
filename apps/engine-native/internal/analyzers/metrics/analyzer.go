package metrics

import (
	"math"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrics"
)

const (
	ResultType    = "metrics_snapshot"
	ParserName    = "openmetrics"
	SchemaVersion = "0.1.0"
	DefaultTopN   = 50
)

type Options struct {
	MaxLines int
	TopN     int
	Strict   bool
}

func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	var diags *diagnostics.ParserDiagnostics
	result, err := build(path, nil, opts, func(yield func(parser.Sample) error) error {
		var err error
		diags, err = parser.ForEachSample(path, parser.Options{MaxLines: opts.MaxLines, Strict: opts.Strict}, yield)
		return err
	})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	result.Metadata.Diagnostics = diags
	return result, nil
}

func Build(samples []parser.Sample, sourceFile string, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	result, _ := build(sourceFile, diags, opts, func(yield func(parser.Sample) error) error {
		for _, sample := range samples {
			if err := yield(sample); err != nil {
				return err
			}
		}
		return nil
	})
	return result
}

func build(sourceFile string, diags *diagnostics.ParserDiagnostics, opts Options, iter func(func(parser.Sample) error) error) (models.AnalysisResult, error) {
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}
	total := 0
	metricCounts := map[string]int{}
	nonFinite := 0
	errorSamples := 0
	rows := []map[string]any{}
	if err := iter(func(sample parser.Sample) error {
		total++
		metricCounts[sample.Name]++
		if math.IsNaN(sample.Value) || math.IsInf(sample.Value, 0) {
			nonFinite++
		}
		if strings.Contains(strings.ToLower(sample.Name), "error") || strings.Contains(strings.ToLower(sample.Name), "5xx") {
			if sample.Value > 0 {
				errorSamples++
			}
		}
		if len(rows) < topN {
			rows = append(rows, map[string]any{
				"name":      sample.Name,
				"labels":    sample.Labels,
				"value":     sample.Value,
				"timestamp": sample.Timestamp,
			})
		}
		return nil
	}); err != nil {
		return models.AnalysisResult{}, err
	}
	result := models.New(ResultType, ParserName)
	result.SourceFiles = []string{sourceFile}
	result.Summary = map[string]any{
		"total_samples":        total,
		"metric_count":         len(metricCounts),
		"non_finite_samples":   nonFinite,
		"error_metric_samples": errorSamples,
	}
	result.Series = map[string]any{
		"metric_distribution": commonRows(metricCounts, "metric", topN),
	}
	result.Tables = map[string]any{
		"samples":                  rows,
		"golden_signal_candidates": goldenSignalRows(rows),
	}
	if diags == nil {
		diags = diagnostics.New(ParserName)
		diags.TotalLines = total
		diags.ParsedRecords = total
	}
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = diags
	result.Metadata.Extra["analysis_options"] = map[string]any{"max_lines": opts.MaxLines, "top_n": topN}
	if nonFinite > 0 {
		result.AddFinding("warning", "METRICS_NON_FINITE_SAMPLES", "Metrics snapshot contains non-finite samples.", map[string]any{"non_finite_samples": nonFinite})
	}
	if errorSamples > 0 {
		result.AddFinding("warning", "METRICS_ERROR_SIGNALS_PRESENT", "Metrics snapshot contains positive error-oriented samples.", map[string]any{"error_metric_samples": errorSamples})
	}
	ingestion.AttachSourceMetadata(&result, ingestion.NewSourceMetadata(sourceFile, ingestion.SourceMetadataOptions{
		SourceKind:   ingestion.SourceKindMetricsSnapshot,
		SourceFormat: "openmetrics",
		Product:      "Prometheus/OpenMetrics",
	}))
	return result, nil
}

func goldenSignalRows(rows []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, row := range rows {
		name := strings.ToLower(asString(row["name"]))
		kind := ""
		switch {
		case strings.Contains(name, "duration") || strings.Contains(name, "latency"):
			kind = "latency"
		case strings.Contains(name, "requests") || strings.Contains(name, "total"):
			kind = "traffic"
		case strings.Contains(name, "error") || strings.Contains(name, "5xx"):
			kind = "errors"
		case strings.Contains(name, "cpu") || strings.Contains(name, "memory") || strings.Contains(name, "queue"):
			kind = "saturation"
		}
		if kind != "" {
			next := map[string]any{}
			for k, v := range row {
				next[k] = v
			}
			next["kind"] = kind
			out = append(out, next)
		}
	}
	return out
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

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
