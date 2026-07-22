// Package lighthouse converts parsed Lighthouse reports into the common
// AnalysisResult contract used by CLI, Wails, Workspace, and exporters.
package lighthouse

import (
	"sort"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/lighthouse"
)

const (
	ResultType        = "browser_audit_evidence"
	ParserName        = "lighthouse"
	SchemaVersion     = "0.1.0"
	UIContractVersion = "1.0.0"
	ScoreSource       = "imported_lighthouse_report"
	ScoreDisclosure   = "Scores are preserved from the imported Lighthouse report; ArchScope does not recompute them."
	DefaultTopN       = 50
)

type Options struct {
	TopN     int
	MaxBytes int64
}

func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	parsed, err := parser.ParseFile(path, parser.Options{MaxBytes: opts.MaxBytes})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	result := Build(parsed.Report, path, opts)
	result.Metadata.Diagnostics = parsed.Diagnostics
	return result, nil
}

func Build(report parser.Report, sourceFile string, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = parser.DefaultMaxBytes
	}

	result := models.New(ResultType, ParserName)
	result.SourceFiles = []string{sourceFile}
	result.Metadata.SchemaVersion = SchemaVersion
	result.Summary = summary(report)
	result.Series = map[string]any{
		"category_scores":            categoryRows(report.Categories),
		"core_metrics":               coreMetricRows(report.Audits),
		"resource_type_distribution": resourceRows(report),
	}
	result.Tables = map[string]any{
		"audits":           auditRows(report.Audits, topN),
		"network_requests": networkRequestRows(report.Resources, topN),
		"resource_summary": resourceSummaryRows(report),
	}
	result.Metadata.Extra["format"] = parser.FormatLighthouseJSON
	result.Metadata.Extra["analysis_options"] = map[string]any{"top_n": topN, "max_bytes": maxBytes}
	result.Metadata.Extra["browser_audit_contract"] = map[string]any{
		"version":           UIContractVersion,
		"result_type":       ResultType,
		"score_source":      ScoreSource,
		"score_disclosure":  ScoreDisclosure,
		"scores_recomputed": false,
		"bounded_tables":    map[string]any{"audits": topN, "network_requests": topN},
		"redaction":         report.Redaction,
		"view_keys": map[string]any{
			"series": []string{"category_scores", "core_metrics", "resource_type_distribution"},
			"tables": []string{"audits", "network_requests", "resource_summary"},
		},
		"evidence_sources": []string{
			"metadata.findings", "series.category_scores", "series.core_metrics",
			"tables.audits", "tables.network_requests", "tables.resource_summary",
		},
		"export_formats": []string{"json", "html", "pptx", "csv", "csv_dir"},
	}

	addFindings(&result, report)
	ingestion.AttachSourceMetadata(&result, ingestion.NewSourceMetadata(sourceFile, ingestion.SourceMetadataOptions{
		SourceKind:     ingestion.SourceKindBrowserAudit,
		SourceFormat:   parser.FormatLighthouseJSON,
		Product:        "Lighthouse",
		ProductVersion: report.LighthouseVersion,
	}))
	return result
}

func summary(report parser.Report) map[string]any {
	scored := 0
	poor := 0
	transfer := int64(0)
	resourceBytes := int64(0)
	for _, audit := range report.Audits {
		if audit.Score != nil {
			scored++
			if *audit.Score < 0.5 {
				poor++
			}
		}
	}
	for _, resource := range report.Resources {
		transfer += resource.TransferBytes
		resourceBytes += resource.ResourceBytes
	}
	out := map[string]any{
		"source_format":         parser.FormatLighthouseJSON,
		"score_source":          ScoreSource,
		"score_disclosure":      ScoreDisclosure,
		"scores_recomputed":     false,
		"lighthouse_version":    report.LighthouseVersion,
		"requested_url":         report.RequestedURL,
		"final_url":             report.FinalURL,
		"fetch_time":            report.FetchTime,
		"form_factor":           report.FormFactor,
		"throttling_method":     report.ThrottlingMethod,
		"audit_count":           len(report.Audits),
		"scored_audit_count":    scored,
		"poor_audit_count":      poor,
		"network_request_count": len(report.Resources),
		"transfer_size_bytes":   transfer,
		"resource_size_bytes":   resourceBytes,
		"run_warning_count":     len(report.RunWarnings),
		"runtime_error":         report.RuntimeError != "",
	}
	for _, category := range report.Categories {
		if category.ID == "performance" && category.Score != nil {
			out["performance_score_pct"] = round(*category.Score*100, 1)
		}
	}
	for _, audit := range report.Audits {
		key := metricSummaryKey(audit.ID)
		if key != "" && audit.NumericValue != nil {
			out[key] = round(*audit.NumericValue, metricPrecision(audit.ID))
		}
	}
	return out
}

func categoryRows(categories []parser.Category) []map[string]any {
	rows := make([]map[string]any, 0, len(categories))
	for _, category := range categories {
		row := map[string]any{"id": category.ID, "title": category.Title, "source_ref": "category:" + category.ID}
		if category.Score != nil {
			row["score"] = *category.Score
			row["score_pct"] = round(*category.Score*100, 1)
		}
		rows = append(rows, row)
	}
	return rows
}

func coreMetricRows(audits []parser.Audit) []map[string]any {
	byID := map[string]parser.Audit{}
	for _, audit := range audits {
		byID[audit.ID] = audit
	}
	ids := []string{"first-contentful-paint", "largest-contentful-paint", "interaction-to-next-paint", "cumulative-layout-shift", "total-blocking-time", "speed-index", "interactive", "server-response-time"}
	rows := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		audit, ok := byID[id]
		if !ok || audit.NumericValue == nil {
			continue
		}
		row := map[string]any{
			"id": id, "title": audit.Title, "value": *audit.NumericValue,
			"unit": audit.NumericUnit, "display_value": audit.DisplayValue,
			"source_ref": "audit:" + id,
		}
		if audit.Score != nil {
			row["score"] = *audit.Score
		}
		rows = append(rows, row)
	}
	return rows
}

func auditRows(audits []parser.Audit, limit int) []map[string]any {
	ordered := append([]parser.Audit(nil), audits...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := scoreSortValue(ordered[i].Score), scoreSortValue(ordered[j].Score)
		if left != right {
			return left < right
		}
		return ordered[i].ID < ordered[j].ID
	})
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	rows := make([]map[string]any, 0, len(ordered))
	for _, audit := range ordered {
		row := map[string]any{
			"id": audit.ID, "title": audit.Title, "description": audit.Description,
			"score_display_mode": audit.ScoreDisplayMode, "numeric_unit": audit.NumericUnit,
			"display_value": audit.DisplayValue, "source_ref": "audit:" + audit.ID,
		}
		if audit.Score != nil {
			row["score"] = *audit.Score
			row["score_pct"] = round(*audit.Score*100, 1)
		}
		if audit.NumericValue != nil {
			row["numeric_value"] = *audit.NumericValue
		}
		rows = append(rows, row)
	}
	return rows
}

func networkRequestRows(resources []parser.Resource, limit int) []map[string]any {
	ordered := append([]parser.Resource(nil), resources...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].TransferBytes != ordered[j].TransferBytes {
			return ordered[i].TransferBytes > ordered[j].TransferBytes
		}
		return ordered[i].URL < ordered[j].URL
	})
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	rows := make([]map[string]any, 0, len(ordered))
	for index, resource := range ordered {
		rows = append(rows, map[string]any{
			"url": resource.URL, "protocol": resource.Protocol, "status_code": resource.StatusCode,
			"mime_type": resource.MIMEType, "resource_type": resource.ResourceType,
			"transfer_size_bytes": resource.TransferBytes, "resource_size_bytes": resource.ResourceBytes,
			"start_ms": resource.StartMS, "end_ms": resource.EndMS, "duration_ms": resource.DurationMS,
			"initiator_type": resource.InitiatorType,
			"source_ref":     "network_request:" + strconv.Itoa(index+1),
		})
	}
	return rows
}

func resourceSummaryRows(report parser.Report) []map[string]any {
	rows := make([]map[string]any, 0, len(report.ResourceSummaries))
	for _, item := range report.ResourceSummaries {
		rows = append(rows, map[string]any{
			"resource_type": item.ResourceType, "request_count": item.RequestCount,
			"transfer_size_bytes": item.TransferBytes, "source_ref": "resource_type:" + strings.ToLower(item.ResourceType),
		})
	}
	return rows
}

func resourceRows(report parser.Report) []map[string]any {
	if len(report.ResourceSummaries) > 0 {
		return resourceSummaryRows(report)
	}
	type stat struct {
		count    int
		transfer int64
	}
	stats := map[string]stat{}
	for _, resource := range report.Resources {
		key := strings.TrimSpace(resource.ResourceType)
		if key == "" {
			key = "other"
		}
		value := stats[key]
		value.count++
		value.transfer += resource.TransferBytes
		stats[key] = value
	}
	keys := make([]string, 0, len(stats))
	for key := range stats {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, map[string]any{
			"resource_type": key, "request_count": stats[key].count,
			"transfer_size_bytes": stats[key].transfer, "source_ref": "resource_type:" + strings.ToLower(key),
		})
	}
	return rows
}

func addFindings(result *models.AnalysisResult, report parser.Report) {
	for _, category := range report.Categories {
		if category.ID == "performance" && category.Score != nil && *category.Score < 0.5 {
			result.AddFinding("warning", "LIGHTHOUSE_PERFORMANCE_POOR", "The imported Lighthouse report rated performance below 50.", map[string]any{
				"score_pct": round(*category.Score*100, 1), "source_ref": "category:performance", "score_source": ScoreSource,
			})
		}
	}
	poor := make([]string, 0)
	for _, audit := range report.Audits {
		if audit.Score != nil && *audit.Score < 0.5 && audit.ScoreDisplayMode != "notApplicable" && audit.ScoreDisplayMode != "manual" {
			poor = append(poor, audit.ID)
		}
	}
	sort.Strings(poor)
	if len(poor) > 0 {
		shown := poor
		if len(shown) > 10 {
			shown = shown[:10]
		}
		result.AddFinding("warning", "LIGHTHOUSE_AUDITS_POOR", "One or more scored Lighthouse audits are below 50.", map[string]any{
			"count": len(poor), "audit_ids": shown, "source_ref": "audits:poor", "score_source": ScoreSource,
		})
	}
	if report.RuntimeError != "" {
		result.AddFinding("error", "LIGHTHOUSE_RUNTIME_ERROR", "Lighthouse reported a runtime error; results may be incomplete.", map[string]any{
			"runtime_error": report.RuntimeError, "source_ref": "runtime_error",
		})
	}
	if len(report.RunWarnings) > 0 {
		warnings := report.RunWarnings
		if len(warnings) > 10 {
			warnings = warnings[:10]
		}
		result.AddFinding("info", "LIGHTHOUSE_RUN_WARNINGS", "Lighthouse emitted run warnings; review collection conditions before comparison.", map[string]any{
			"count": len(report.RunWarnings), "warnings": warnings, "source_ref": "run_warnings",
		})
	}
}

func metricSummaryKey(id string) string {
	switch id {
	case "first-contentful-paint":
		return "first_contentful_paint_ms"
	case "largest-contentful-paint":
		return "largest_contentful_paint_ms"
	case "interaction-to-next-paint":
		return "interaction_to_next_paint_ms"
	case "cumulative-layout-shift":
		return "cumulative_layout_shift"
	case "total-blocking-time":
		return "total_blocking_time_ms"
	case "speed-index":
		return "speed_index_ms"
	case "interactive":
		return "interactive_ms"
	case "server-response-time":
		return "server_response_time_ms"
	default:
		return ""
	}
}

func metricPrecision(id string) int {
	if id == "cumulative-layout-shift" {
		return 4
	}
	return 1
}

func scoreSortValue(value *float64) float64 {
	if value == nil {
		return 2
	}
	return *value
}

func round(value float64, places int) float64 {
	factor := 1.0
	for i := 0; i < places; i++ {
		factor *= 10
	}
	if value >= 0 {
		return float64(int64(value*factor+0.5)) / factor
	}
	return float64(int64(value*factor-0.5)) / factor
}
