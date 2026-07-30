package httpcapture

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/aggregate"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/httpcapture"
)

const (
	ResultType             = "http_capture"
	DefaultTopN            = 50
	MaxTopN                = 500
	DefaultTimelineBuckets = 1000
)

type Options struct {
	Format                  string
	TopN                    int
	DiffTemplateLimit       int
	MaxEntries              int
	MaxBytes                int64
	MaxStringBytes          int
	MaxBodyBytes            int
	MaxDepth                int
	MaxFields               int
	MaxDecompressionRatio   int
	CustomRedactionPatterns []string
}

type captureProvenance struct {
	CaptureMode      string
	ObservationPoint string
	Fidelity         string
	Product          string
	ModeCounts       map[string]int
	FidelityCounts   map[string]int
	CoverageCounts   map[string]int
	RedactionMessage string
	AggregateID      string
	EmptyMessage     string
	TruncatedCode    string
}

func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	parsed, err := parser.ParseFile(path, parser.Options{
		Format:                  opts.Format,
		MaxEntries:              opts.MaxEntries,
		MaxBytes:                opts.MaxBytes,
		MaxStringBytes:          opts.MaxStringBytes,
		MaxBodyBytes:            opts.MaxBodyBytes,
		MaxDepth:                opts.MaxDepth,
		MaxFields:               opts.MaxFields,
		MaxDecompressionRatio:   opts.MaxDecompressionRatio,
		CustomRedactionPatterns: opts.CustomRedactionPatterns,
	})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return BuildParsed(parsed, path, opts), nil
}

// BuildLive constructs a finalized analysis from ArchScope-owned live capture
// transactions. Unlike the HAR path, provenance and aggregate fidelity are
// derived from the persisted rows and never claim semantic foreign-tool input.
func BuildLive(entries []models.CaptureTransaction, sourceFile, dialect string, redactionSummary redact.Summary, opts Options) models.AnalysisResult {
	parsed := parser.ParseResult{
		Format:            "archscope-live",
		Dialect:           dialect,
		Entries:           entries,
		Redaction:         redactionSummary,
		TimelineAvailable: true,
	}
	return buildParsed(parsed, sourceFile, opts, liveProvenance(entries, sourceFile))
}

func BuildParsed(parsed parser.ParseResult, sourceFile string, opts Options) models.AnalysisResult {
	return buildParsed(parsed, sourceFile, opts, harProvenance(len(parsed.Entries)))
}

func buildParsed(parsed parser.ParseResult, sourceFile string, opts Options, provenance captureProvenance) models.AnalysisResult {
	topN := normalizeTopN(opts.TopN)
	entries := parsed.Entries
	endpoints := map[string]*stat{}
	hosts := map[string]*stat{}
	minutes := map[string]*stat{}
	durations := make([]float64, 0, len(entries))
	errorCount := 0
	abortedCount := 0
	unknownRequestBytes := 0
	unknownResponseBytes := 0
	var requestBytes, responseBytes int64
	timingInconsistent := 0
	truncatedBodies := 0
	processes := map[string]struct{}{}

	for _, entry := range entries {
		key := first(entry.Method, "GET") + " " + first(entry.Path, "/")
		add(endpoints, key, entry)
		add(hosts, first(entry.Host, "(unknown)"), entry)
		if parsed.TimelineAvailable && entry.StartedAt != "" {
			if started, err := time.Parse(time.RFC3339Nano, entry.StartedAt); err == nil {
				add(minutes, started.UTC().Format("2006-01-02T15:04:00Z"), entry)
			}
		}
		if transactionError(entry) {
			errorCount++
		}
		if entry.State == models.TxAborted {
			abortedCount++
		} else if entry.State == models.TxComplete || entry.State == models.TxFailed {
			durations = append(durations, entry.TotalMS)
		}
		if entry.Request.BodySize >= 0 {
			requestBytes += entry.Request.BodySize
		} else {
			unknownRequestBytes++
		}
		responseSize := preferredResponseSize(entry.Response)
		if responseSize >= 0 {
			responseBytes += responseSize
		} else {
			unknownResponseBytes++
		}
		if entry.Timings.ImportedHAR != nil && entry.Timings.ImportedHAR.KnownSumMS() > entry.TotalMS+1 {
			timingInconsistent++
		}
		if entry.Request.BodyStorage == "truncated" || entry.Response.BodyStorage == "truncated" {
			truncatedBodies++
		}
		if entry.Process != nil {
			processes[processKey(entry.Process)] = struct{}{}
		}
	}
	sort.Float64s(durations)

	result := models.New(ResultType, "http_capture")
	result.SourceFiles = []string{sourceFile}
	result.Metadata.SchemaVersion = "1.0.0"
	result.Metadata.Diagnostics = parsed.Diagnostics
	result.Summary = map[string]any{
		"total_transactions":     len(entries),
		"error_transactions":     errorCount,
		"aborted_transactions":   abortedCount,
		"error_rate":             rate(errorCount, len(entries)),
		"unique_hosts":           len(hosts),
		"unique_endpoints":       len(endpoints),
		"unique_processes":       len(processes),
		"request_bytes":          requestBytes,
		"response_bytes":         responseBytes,
		"unknown_request_sizes":  unknownRequestBytes,
		"unknown_response_sizes": unknownResponseBytes,
		"duration_p50_ms":        percentile(durations, 0.50),
		"duration_p95_ms":        percentile(durations, 0.95),
		"duration_p99_ms":        percentile(durations, 0.99),
		"source_format":          parsed.Format,
		"dialect":                parsed.Dialect,
		"timeline_available":     parsed.TimelineAvailable,
		"redaction_applied":      parsed.Redaction.Applied,
		"redaction_known":        parsed.Redaction.Known,
	}
	if parsed.TimelineAvailable {
		result.Series["timeline"] = boundedTimelineRows(minutes, DefaultTimelineBuckets)
	} else {
		result.Series["timeline"] = []map[string]any{}
	}
	result.Tables = map[string]any{
		"transactions": transactionRows(entries, topN),
		"slowest":      slowestRows(entries, topN),
		"errors":       errorRows(entries, topN),
		"endpoints":    rows(endpoints, "endpoint", topN),
		"hosts":        rows(hosts, "host", topN),
	}
	truncated := len(entries) > topN
	result.Metadata.Extra["http_capture"] = map[string]any{
		"capture_schema_version": models.CaptureSchemaVersion,
		"dialect":                parsed.Dialect,
		"capture_mode":           provenance.CaptureMode,
		"capture_mode_counts":    provenance.ModeCounts,
		"observation_point":      provenance.ObservationPoint,
		"fidelity":               provenance.Fidelity,
		"fidelity_counts":        provenance.FidelityCounts,
		"coverage_counts":        provenance.CoverageCounts,
		"redaction":              parsed.Redaction,
		"detail_storage":         "bounded_inline",
		"table_limit":            topN,
		"timeline_bucket_limit":  DefaultTimelineBuckets,
		"transaction_rows":       min(len(entries), topN),
		"transactions":           len(entries),
		"truncated":              truncated,
		"timeline_available":     parsed.TimelineAvailable,
		"input_bytes":            parsed.InputBytes,
		"decompressed_bytes":     parsed.DecompressedBytes,
	}
	diffSource := buildDiffSourceProjection(entries, sourceFile, parsed.Format, provenance, parsed.TimelineAvailable, opts.DiffTemplateLimit)
	result.Metadata.Extra["http_capture_diff_source"] = diffSource
	result.Metadata.Extra["http_capture_diff_contract"] = DefaultDiffContract()
	result.Metadata.Extra["capture_session_ref"] = diffSource.Session
	if diffSource.TemplatesFolded > 0 || diffSource.HostsFolded > 0 || diffSource.ProcessesFolded > 0 {
		evidence := map[string]any{
			"dimension_limit":      diffSource.TemplateLimit,
			"templates_folded":     diffSource.TemplatesFolded,
			"hosts_folded":         diffSource.HostsFolded,
			"processes_folded":     diffSource.ProcessesFolded,
			"overflow_bucket":      "{other}",
			"url_template_version": diffSource.URLTemplateVersion,
		}
		if result.Metadata.Diagnostics != nil {
			result.Metadata.Diagnostics.AddWarning(0, "HTTP_DIFF_DIMENSIONS_FOLDED", "HTTP Diff source dimensions were folded into the bounded {other} bucket", "", false)
		} else {
			result.AddFinding("info", "HTTP_DIFF_DIMENSIONS_FOLDED", "HTTP Diff source dimensions were folded into the bounded {other} bucket", evidence)
		}
	}

	aggregator := aggregate.New(provenance.AggregateID, topN)
	aggregator.ApplyBatch(entries)
	result.Metadata.Extra["capture_aggregate_snapshot"] = aggregator.Snapshot()
	ingestion.AttachSourceMetadata(&result, ingestion.NewSourceMetadata(sourceFile, ingestion.SourceMetadataOptions{SourceKind: ingestion.SourceKindHTTPCapture, SourceFormat: parsed.Format, Product: provenance.Product}))

	addDiagnosticFindings(&result)
	if len(entries) == 0 {
		result.AddFinding("warning", "CAPTURE_EMPTY", provenance.EmptyMessage, nil)
	}
	if errorCount > 0 {
		result.AddFinding("warning", "HTTP_CAPTURE_ERRORS", "HTTP capture contains failed responses", map[string]any{"error_transactions": errorCount})
	}
	if timingInconsistent > 0 {
		result.AddFinding("warning", "CAPTURE_TIMING_INCONSISTENT", "Known HAR timing phases exceed transaction total", map[string]any{"transactions": timingInconsistent, "epsilon_ms": 1})
	}
	if truncatedBodies > 0 {
		result.AddFinding("info", "CAPTURE_TRUNCATED_BODIES", "One or more body previews were truncated", map[string]any{"transactions": truncatedBodies})
	}
	if parsed.Redaction.Applied {
		result.AddFinding("info", "CAPTURE_REDACTED", provenance.RedactionMessage, map[string]any{"policy_version": parsed.Redaction.Version, "rules": parsed.Redaction.Rules})
	}
	if truncated {
		result.AddFinding("info", provenance.TruncatedCode, "Inline transaction details are bounded", map[string]any{"rows": topN, "transactions": len(entries)})
		if parsed.Diagnostics != nil {
			parsed.Diagnostics.AddWarning(0, provenance.TruncatedCode, "inline transaction detail rows are bounded", "", false)
		}
	}
	return result
}

func harProvenance(entries int) captureProvenance {
	return captureProvenance{
		CaptureMode:      "har_import",
		ObservationPoint: "foreign_tool",
		Fidelity:         "semantic",
		Product:          "HAR import",
		ModeCounts:       map[string]int{"har_import": entries},
		FidelityCounts:   map[string]int{"semantic": entries},
		CoverageCounts:   map[string]int{},
		RedactionMessage: "Sensitive HTTP fields were redacted during import",
		AggregateID:      "offline-har",
		EmptyMessage:     "HAR contains no transactions",
		TruncatedCode:    "HAR_DETAILS_TRUNCATED",
	}
}

func liveProvenance(entries []models.CaptureTransaction, sourceFile string) captureProvenance {
	modeCounts := map[string]int{}
	fidelityCounts := map[string]int{}
	coverageCounts := map[string]int{}
	for _, entry := range entries {
		modeCounts[valueOrUnknown(entry.CaptureMode)]++
		fidelityCounts[valueOrUnknown(entry.Fidelity)]++
		coverageCounts[valueOrUnknown(entry.Coverage)]++
	}
	return captureProvenance{
		CaptureMode:      aggregateCaptureMode(modeCounts),
		ObservationPoint: "proxy",
		Fidelity:         weakestFidelity(fidelityCounts),
		Product:          "ArchScope live capture",
		ModeCounts:       modeCounts,
		FidelityCounts:   fidelityCounts,
		CoverageCounts:   coverageCounts,
		RedactionMessage: "Sensitive HTTP fields were redacted during live capture",
		AggregateID:      first(strings.TrimPrefix(sourceFile, "capture://"), "live-capture"),
		EmptyMessage:     "Live capture contains no transactions",
		TruncatedCode:    "CAPTURE_DETAILS_TRUNCATED",
	}
}

func aggregateCaptureMode(counts map[string]int) string {
	if len(counts) == 0 {
		return "unknown"
	}
	if len(counts) == 1 {
		for value := range counts {
			return value
		}
	}
	return "mixed"
}

func weakestFidelity(counts map[string]int) string {
	if len(counts) == 0 {
		return "unknown"
	}
	if counts["unsupported"] > 0 {
		return "unsupported"
	}
	known := map[string]bool{
		"unknown": true, "pending": true, "decoded_wire": true, "semantic": true,
	}
	for value := range counts {
		if !known[value] {
			return "unknown"
		}
	}
	for _, candidate := range []string{"unknown", "pending", "decoded_wire", "semantic"} {
		if counts[candidate] > 0 {
			return candidate
		}
	}
	return "unknown"
}

func valueOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

type stat struct {
	Count, Errors               int
	TotalMS                     float64
	RequestBytes, ResponseBytes int64
}

func add(target map[string]*stat, key string, entry models.CaptureTransaction) {
	item := target[key]
	if item == nil {
		item = &stat{}
		target[key] = item
	}
	item.Count++
	if transactionError(entry) {
		item.Errors++
	}
	item.TotalMS += entry.TotalMS
	if entry.Request.BodySize >= 0 {
		item.RequestBytes += entry.Request.BodySize
	}
	if responseSize := preferredResponseSize(entry.Response); responseSize >= 0 {
		item.ResponseBytes += responseSize
	}
}

func rows(source map[string]*stat, label string, limit int) []map[string]any {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if source[keys[i]].TotalMS != source[keys[j]].TotalMS {
			return source[keys[i]].TotalMS > source[keys[j]].TotalMS
		}
		return keys[i] < keys[j]
	})
	if len(keys) > limit {
		keys = keys[:limit]
	}
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		item := source[key]
		out = append(out, map[string]any{
			label:               key,
			"count":             item.Count,
			"errors":            item.Errors,
			"error_rate":        rate(item.Errors, item.Count),
			"total_duration_ms": item.TotalMS,
			"avg_duration_ms":   item.TotalMS / float64(item.Count),
			"request_bytes":     item.RequestBytes,
			"response_bytes":    item.ResponseBytes,
		})
	}
	return out
}

func transactionRows(entries []models.CaptureTransaction, limit int) []map[string]any {
	if len(entries) > limit {
		entries = entries[:limit]
	}
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		out = append(out, transactionRow(entry))
	}
	return out
}

func slowestRows(entries []models.CaptureTransaction, limit int) []map[string]any {
	copyEntries := append([]models.CaptureTransaction(nil), entries...)
	sort.SliceStable(copyEntries, func(i, j int) bool {
		if copyEntries[i].TotalMS != copyEntries[j].TotalMS {
			return copyEntries[i].TotalMS > copyEntries[j].TotalMS
		}
		return copyEntries[i].ID < copyEntries[j].ID
	})
	return transactionRows(copyEntries, limit)
}

func errorRows(entries []models.CaptureTransaction, limit int) []map[string]any {
	filtered := make([]models.CaptureTransaction, 0)
	for _, entry := range entries {
		if transactionError(entry) {
			filtered = append(filtered, entry)
		}
	}
	return transactionRows(filtered, limit)
}

func transactionRow(entry models.CaptureTransaction) map[string]any {
	return map[string]any{
		"id":                       entry.ID,
		"connection_id":            entry.ConnectionID,
		"sequence":                 entry.Sequence,
		"started_at":               entry.StartedAt,
		"ended_at":                 entry.EndedAt,
		"method":                   entry.Method,
		"url":                      entry.URL,
		"host":                     entry.Host,
		"path":                     entry.Path,
		"status":                   entry.StatusCode,
		"status_text":              entry.StatusText,
		"http_version":             entry.HTTPVersion,
		"state":                    entry.State,
		"duration_ms":              entry.TotalMS,
		"request_bytes":            entry.Request.BodySize,
		"response_bytes":           preferredResponseSize(entry.Response),
		"used_existing_connection": entry.UsedExistingConnection,
		"capture_mode":             entry.CaptureMode,
		"coverage":                 entry.Coverage,
		"fidelity":                 entry.Fidelity,
		"request":                  entry.Request,
		"response":                 entry.Response,
		"timings":                  entry.Timings,
		"process":                  exportedProcess(entry.Process),
	}
}

func exportedProcess(process *models.ProcessInstance) any {
	if process == nil {
		return nil
	}
	// commandLine and user are intentionally not exported. The parser still
	// redacts the common model so an internal consumer cannot receive raw
	// secrets, while the AnalysisResult honors the stricter default-export
	// contract for process metadata.
	return map[string]any{
		"pid":         process.Key.PID,
		"start_time":  process.Key.StartTime,
		"name":        process.Name,
		"exec_path":   process.ExecPath,
		"parent_pid":  process.ParentPID,
		"attribution": process.Attribution,
	}
}

func boundedTimelineRows(source map[string]*stat, limit int) []map[string]any {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return []map[string]any{}
	}
	groupSize := 1
	if len(keys) > limit {
		groupSize = int(math.Ceil(float64(len(keys)) / float64(limit)))
	}
	out := make([]map[string]any, 0, min(len(keys), limit))
	for start := 0; start < len(keys); start += groupSize {
		end := min(start+groupSize, len(keys))
		combined := &stat{}
		for _, key := range keys[start:end] {
			item := source[key]
			combined.Count += item.Count
			combined.Errors += item.Errors
			combined.TotalMS += item.TotalMS
			combined.RequestBytes += item.RequestBytes
			combined.ResponseBytes += item.ResponseBytes
		}
		out = append(out, map[string]any{
			"start":             keys[start],
			"end":               keys[end-1],
			"bucket_minutes":    end - start,
			"count":             combined.Count,
			"errors":            combined.Errors,
			"error_rate":        rate(combined.Errors, combined.Count),
			"request_bytes":     combined.RequestBytes,
			"response_bytes":    combined.ResponseBytes,
			"total_duration_ms": combined.TotalMS,
		})
	}
	return out
}

func addDiagnosticFindings(result *models.AnalysisResult) {
	if result.Metadata.Diagnostics == nil {
		return
	}
	seen := map[string]struct{}{}
	for _, issue := range result.Metadata.Diagnostics.Warnings {
		if _, ok := seen[issue.Reason]; ok {
			continue
		}
		seen[issue.Reason] = struct{}{}
		severity := "warning"
		switch issue.Reason {
		case "HAR_DIALECT_UNKNOWN", "HAR_SIZES_UNAVAILABLE", "HAR_BODIES_ABSENT", "HAR_REDIRECTS_UNLINKABLE", "HAR_PRESANITIZED", "HTTP_DIFF_DIMENSIONS_FOLDED":
			severity = "info"
		}
		result.AddFinding(severity, issue.Reason, issue.Message, nil)
	}
}

func transactionError(entry models.CaptureTransaction) bool {
	return entry.StatusCode >= 400 || entry.State == models.TxFailed
}

func preferredResponseSize(message models.HTTPMessage) int64 {
	if message.TransferSize >= 0 {
		return message.TransferSize
	}
	return message.BodySize
}

func processKey(process *models.ProcessInstance) string {
	if process == nil {
		return ""
	}
	return strconv.FormatInt(int64(process.Key.PID), 10) + ":" + process.Key.StartTime + ":" + process.Name
}

func percentile(sorted []float64, quantile float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	position := quantile * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return sorted[lower]
	}
	weight := position - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

func rate(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func normalizeTopN(value int) int {
	if value <= 0 {
		return DefaultTopN
	}
	if value > MaxTopN {
		return MaxTopN
	}
	return value
}

func first(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
