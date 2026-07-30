package httpcapture

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const (
	DiffResultType              = "http_capture_diff"
	DiffSchemaVersion           = "1.0.0"
	DiffContractVersion         = 1
	URLTemplateVersion          = 1
	DefaultDiffTemplateLimit    = 1000
	MaxDiffTemplateLimit        = 1000
	DefaultDiffTableLimit       = 50
	MaxDiffTableLimit           = 500
	diffFindingEvidenceRowLimit = 10
)

var (
	errDiffSourceMissing = errors.New("http_capture result does not contain a diff source projection; re-analyze the session")
	numericSegmentRE     = regexp.MustCompile(`^[0-9]+$`)
	uuidSegmentRE        = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	hexSegmentRE         = regexp.MustCompile(`(?i)^[0-9a-f]{16,}$`)
	base64URLSegmentRE   = regexp.MustCompile(`^[A-Za-z0-9_-]{22,}$`)
	emailSegmentRE       = regexp.MustCompile(`(?i)^[^/@\s]+@[^/@\s]+\.[^/@\s]+$`)
)

// DiffOptions controls only the bounded output tables. Source projections are
// already bounded when the http_capture result is created, so diff/export never
// needs to rescan a HAR file or live session store.
type DiffOptions struct {
	TopN int
}

type HttpCaptureDiffContract struct {
	SchemaVersion              int      `json:"schema_version"`
	SourceResultType           string   `json:"source_result_type"`
	ResultType                 string   `json:"result_type"`
	URLTemplateVersion         int      `json:"url_template_version"`
	SupportedSourceVersions    []int    `json:"supported_source_versions"`
	DefaultTemplateLimit       int      `json:"default_template_limit"`
	MaxTemplateLimit           int      `json:"max_template_limit"`
	DefaultTableLimit          int      `json:"default_table_limit"`
	MaxTableLimit              int      `json:"max_table_limit"`
	TimeAlignmentGrades        []string `json:"time_alignment_grades"`
	WorkspaceRoute             string   `json:"workspace_route"`
	WorkspaceSelectionCount    int      `json:"workspace_selection_count"`
	CompareMethod              string   `json:"compare_method"`
	LegacyDiffSupported        bool     `json:"legacy_diff_supported"`
	RequiresNewNavKey          bool     `json:"requires_new_nav_key"`
	StoreRescanOnDiffOrExport  bool     `json:"store_rescan_on_diff_or_export"`
	ProcessRequiresRealSources bool     `json:"process_requires_real_sources"`
}

func DefaultDiffContract() HttpCaptureDiffContract {
	return HttpCaptureDiffContract{
		SchemaVersion:              DiffContractVersion,
		SourceResultType:           ResultType,
		ResultType:                 DiffResultType,
		URLTemplateVersion:         URLTemplateVersion,
		SupportedSourceVersions:    []int{DiffContractVersion},
		DefaultTemplateLimit:       DefaultDiffTemplateLimit,
		MaxTemplateLimit:           MaxDiffTemplateLimit,
		DefaultTableLimit:          DefaultDiffTableLimit,
		MaxTableLimit:              MaxDiffTableLimit,
		TimeAlignmentGrades:        []string{"aligned", "duration_only", "none"},
		WorkspaceRoute:             DiffResultType,
		WorkspaceSelectionCount:    2,
		CompareMethod:              "AnalyzeHttpCaptureDiff",
		LegacyDiffSupported:        false,
		RequiresNewNavKey:          false,
		StoreRescanOnDiffOrExport:  false,
		ProcessRequiresRealSources: true,
	}
}

type WorkspaceComparisonRoute struct {
	Supported  bool   `json:"supported"`
	Route      string `json:"route,omitempty"`
	Method     string `json:"method,omitempty"`
	ResultType string `json:"result_type,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// ResolveWorkspaceComparison makes the backend routing decision explicit. The
// legacy generic/profiler Diff paths remain unrelated to HTTP capture results.
func ResolveWorkspaceComparison(beforeType, afterType string) WorkspaceComparisonRoute {
	if beforeType == ResultType && afterType == ResultType {
		return WorkspaceComparisonRoute{
			Supported: true, Route: DiffResultType,
			Method: "AnalyzeHttpCaptureDiff", ResultType: DiffResultType,
		}
	}
	return WorkspaceComparisonRoute{
		Supported: false,
		Reason:    "HTTP session comparison requires exactly two http_capture results",
	}
}

type DiffSessionRef struct {
	SessionID       string `json:"session_id"`
	SourceKind      string `json:"source_kind"`
	SourceFormat    string `json:"source_format,omitempty"`
	SnapshotVersion uint64 `json:"snapshot_version,omitempty"`
	Transactions    int    `json:"transactions"`
}

type DiffTimeProjection struct {
	AbsoluteTimeTrusted bool    `json:"absolute_time_trusted"`
	Degenerate          bool    `json:"degenerate"`
	Start               string  `json:"start,omitempty"`
	End                 string  `json:"end,omitempty"`
	CaptureDurationMS   float64 `json:"capture_duration_ms,omitempty"`
	RelativeDurationMS  float64 `json:"relative_duration_ms,omitempty"`
	RateAvailable       bool    `json:"rate_available"`
	RateUnavailableCode string  `json:"rate_unavailable_code,omitempty"`
}

type DiffDimensionStat struct {
	Key             string  `json:"key"`
	Count           int     `json:"count"`
	Errors          int     `json:"errors"`
	RequestBytes    int64   `json:"request_bytes"`
	ResponseBytes   int64   `json:"response_bytes"`
	DurationP50MS   float64 `json:"duration_p50_ms"`
	DurationP95MS   float64 `json:"duration_p95_ms"`
	DurationP99MS   float64 `json:"duration_p99_ms"`
	DurationSamples int     `json:"duration_samples"`
}

type DiffSourceProjection struct {
	SchemaVersion      int                 `json:"schema_version"`
	URLTemplateVersion int                 `json:"url_template_version"`
	TemplateLimit      int                 `json:"template_limit"`
	Session            DiffSessionRef      `json:"session"`
	CaptureMode        string              `json:"capture_mode"`
	Time               DiffTimeProjection  `json:"time"`
	Total              DiffDimensionStat   `json:"total"`
	Endpoints          []DiffDimensionStat `json:"endpoints"`
	Hosts              []DiffDimensionStat `json:"hosts"`
	Processes          []DiffDimensionStat `json:"processes"`
	ProcessAvailable   bool                `json:"process_available"`
	ProcessReason      string              `json:"process_unavailable_reason,omitempty"`
	EndpointTotal      int                 `json:"endpoint_total"`
	HostTotal          int                 `json:"host_total"`
	ProcessTotal       int                 `json:"process_total,omitempty"`
	TemplatesFolded    int                 `json:"templates_folded"`
	HostsFolded        int                 `json:"hosts_folded"`
	ProcessesFolded    int                 `json:"processes_folded"`
}

type mutableDiffStat struct {
	count, errors               int
	requestBytes, responseBytes int64
	durations                   []float64
}

func buildDiffSourceProjection(entries []models.CaptureTransaction, sourceFile, sourceFormat string, provenance captureProvenance, timelineAvailable bool, requestedLimit int) DiffSourceProjection {
	limit := normalizeDiffTemplateLimit(requestedLimit)
	endpoints := map[string]*mutableDiffStat{}
	hosts := map[string]*mutableDiffStat{}
	processes := map[string]*mutableDiffStat{}
	total := &mutableDiffStat{}
	realProcessObserved := false

	for _, entry := range entries {
		addDiffStat(total, entry)
		addDiffStatForKey(endpoints, endpointTemplateKey(entry), entry)
		addDiffStatForKey(hosts, canonicalHost(entry.Host), entry)
		if entry.Process != nil && entry.Process.Key.PID > 0 && strings.TrimSpace(entry.Process.Name) != "" {
			realProcessObserved = true
			addDiffStatForKey(processes, canonicalProcess(entry.Process), entry)
		} else {
			addDiffStatForKey(processes, "(unattributed)", entry)
		}
	}

	processAvailable := provenance.CaptureMode != "har_import" && realProcessObserved
	processReason := ""
	if provenance.CaptureMode == "har_import" {
		processReason = "HAR pseudo-process sessions do not provide comparable process attribution"
	} else if !realProcessObserved {
		processReason = "session contains no real process attribution"
	}
	endpointRows, templatesFolded := boundedDiffStats(endpoints, limit)
	hostRows, hostsFolded := boundedDiffStats(hosts, limit)
	processRows, processesFolded := boundedDiffStats(processes, limit)
	if !processAvailable {
		processRows = []DiffDimensionStat{}
		processesFolded = 0
	}

	sourceKind := "har_import"
	sessionID := ""
	if strings.HasPrefix(sourceFile, "capture://") {
		sourceKind = "live_capture"
		sessionID = strings.TrimPrefix(sourceFile, "capture://")
	} else {
		identity := ingestion.NewFileIdentity(sourceFile, ingestion.SourceKindHTTPCapture, sourceFormat)
		sessionID = "har:" + identity.SanitizedID
	}
	if sessionID == "" {
		sessionID = sourceKind + ":unknown"
	}

	projection := DiffSourceProjection{
		SchemaVersion:      DiffContractVersion,
		URLTemplateVersion: URLTemplateVersion,
		TemplateLimit:      limit,
		Session: DiffSessionRef{
			SessionID: sessionID, SourceKind: sourceKind, SourceFormat: sourceFormat,
			Transactions: len(entries),
		},
		CaptureMode:      provenance.CaptureMode,
		Time:             buildDiffTimeProjection(entries, timelineAvailable),
		Total:            freezeDiffStat("total", total),
		Endpoints:        endpointRows,
		Hosts:            hostRows,
		Processes:        processRows,
		ProcessAvailable: processAvailable,
		ProcessReason:    processReason,
		EndpointTotal:    sumDiffCounts(endpointRows),
		HostTotal:        sumDiffCounts(hostRows),
		TemplatesFolded:  templatesFolded,
		HostsFolded:      hostsFolded,
		ProcessesFolded:  processesFolded,
	}
	if processAvailable {
		projection.ProcessTotal = sumDiffCounts(processRows)
	}
	return projection
}

// SetDiffCaptureSessionRef binds the finalized live-store snapshot to the
// already-computed source projection without exposing or rescanning the store.
func SetDiffCaptureSessionRef(result *models.AnalysisResult, sessionID string, snapshotVersion uint64, transactions int) {
	if result == nil {
		return
	}
	value, ok := result.Metadata.Extra["http_capture_diff_source"]
	if !ok {
		return
	}
	projection, ok := value.(DiffSourceProjection)
	if !ok {
		return
	}
	projection.Session.SessionID = sessionID
	projection.Session.SourceKind = "live_capture"
	projection.Session.SnapshotVersion = snapshotVersion
	projection.Session.Transactions = transactions
	result.Metadata.Extra["http_capture_diff_source"] = projection
	result.Metadata.Extra["capture_session_ref"] = projection.Session
}

func addDiffStatForKey(target map[string]*mutableDiffStat, key string, entry models.CaptureTransaction) {
	if strings.TrimSpace(key) == "" {
		key = "(unknown)"
	}
	item := target[key]
	if item == nil {
		item = &mutableDiffStat{}
		target[key] = item
	}
	addDiffStat(item, entry)
}

func addDiffStat(item *mutableDiffStat, entry models.CaptureTransaction) {
	item.count++
	if transactionError(entry) {
		item.errors++
	}
	if entry.Request.BodySize >= 0 {
		item.requestBytes += entry.Request.BodySize
	}
	if size := preferredResponseSize(entry.Response); size >= 0 {
		item.responseBytes += size
	}
	if entry.State == models.TxComplete || entry.State == models.TxFailed {
		item.durations = append(item.durations, entry.TotalMS)
	}
}

func freezeDiffStat(key string, item *mutableDiffStat) DiffDimensionStat {
	durations := append([]float64(nil), item.durations...)
	sort.Float64s(durations)
	return DiffDimensionStat{
		Key: key, Count: item.count, Errors: item.errors,
		RequestBytes: item.requestBytes, ResponseBytes: item.responseBytes,
		DurationP50MS:   percentile(durations, 0.50),
		DurationP95MS:   percentile(durations, 0.95),
		DurationP99MS:   percentile(durations, 0.99),
		DurationSamples: len(durations),
	}
}

func boundedDiffStats(source map[string]*mutableDiffStat, limit int) ([]DiffDimensionStat, int) {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if source[keys[i]].count != source[keys[j]].count {
			return source[keys[i]].count > source[keys[j]].count
		}
		return keys[i] < keys[j]
	})
	folded := 0
	if len(keys) > limit {
		folded = len(keys) - limit
	}

	// Keep an immutable copy before folding so selection is deterministic and
	// independent of map traversal order.
	selected := make([]DiffDimensionStat, 0, min(len(keys), limit)+1)
	upper := min(len(keys), limit)
	for _, key := range keys[:upper] {
		selected = append(selected, freezeDiffStat(key, source[key]))
	}
	if len(keys) > limit {
		other := &mutableDiffStat{}
		for _, key := range keys[limit:] {
			mergeMutableDiffStat(other, source[key])
		}
		selected = append(selected, freezeDiffStat("{other}", other))
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Key < selected[j].Key })
	return selected, folded
}

func mergeMutableDiffStat(target, source *mutableDiffStat) {
	target.count += source.count
	target.errors += source.errors
	target.requestBytes += source.requestBytes
	target.responseBytes += source.responseBytes
	target.durations = append(target.durations, source.durations...)
}

func sumDiffCounts(rows []DiffDimensionStat) int {
	total := 0
	for _, row := range rows {
		total += row.Count
	}
	return total
}

func endpointTemplateKey(entry models.CaptureTransaction) string {
	method := strings.ToUpper(strings.TrimSpace(entry.Method))
	if method == "" {
		method = "GET"
	}
	return method + " " + canonicalHost(entry.Host) + " " + templatePathAndQuery(entry.Path, entry.Query)
}

func canonicalHost(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "(unknown)"
	}
	return value
}

func canonicalProcess(process *models.ProcessInstance) string {
	name := strings.ToLower(strings.TrimSpace(process.Name))
	if name == "" && process.ExecPath != "" {
		name = strings.ToLower(filepath.Base(process.ExecPath))
	}
	if name == "" {
		return "(unattributed)"
	}
	return name
}

func templatePathAndQuery(path, rawQuery string) string {
	if parsed, err := url.Parse(path); err == nil {
		if rawQuery == "" {
			rawQuery = parsed.RawQuery
		}
		path = parsed.EscapedPath()
	}
	if path == "" {
		path = "/"
	}
	parts := strings.Split(path, "/")
	for index, part := range parts {
		if part == "" {
			continue
		}
		decoded, err := url.PathUnescape(part)
		if err != nil {
			decoded = part
		}
		templated := templateURLSegment(decoded)
		if templated == decoded {
			parts[index] = part
		} else {
			parts[index] = templated
		}
	}
	template := strings.Join(parts, "/")
	if !strings.HasPrefix(template, "/") {
		template = "/" + template
	}
	keys := queryKeys(rawQuery)
	if len(keys) > 0 {
		template += "?" + strings.Join(keys, "&")
	}
	return template
}

func templateURLSegment(segment string) string {
	switch {
	case numericSegmentRE.MatchString(segment):
		return "{id}"
	case uuidSegmentRE.MatchString(segment):
		return "{uuid}"
	case hexSegmentRE.MatchString(segment):
		return "{hash}"
	case base64URLSegmentRE.MatchString(segment):
		return "{token}"
	case emailSegmentRE.MatchString(segment):
		return "{email}"
	default:
		return segment
	}
}

func queryKeys(raw string) []string {
	seen := map[string]struct{}{}
	for _, field := range strings.Split(raw, "&") {
		if field == "" {
			continue
		}
		key := strings.SplitN(field, "=", 2)[0]
		if decoded, err := url.QueryUnescape(key); err == nil {
			key = decoded
		}
		key = strings.TrimSpace(key)
		if key != "" {
			seen[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func buildDiffTimeProjection(entries []models.CaptureTransaction, timelineAvailable bool) DiffTimeProjection {
	var minStart, maxEnd time.Time
	relativeDurationMS := 0.0
	for _, entry := range entries {
		if entry.TotalMS > relativeDurationMS {
			relativeDurationMS = entry.TotalMS
		}
		start, startErr := time.Parse(time.RFC3339Nano, entry.StartedAt)
		if startErr != nil {
			continue
		}
		start = start.UTC()
		end, endErr := time.Parse(time.RFC3339Nano, entry.EndedAt)
		if endErr != nil {
			end = start.Add(time.Duration(entry.TotalMS * float64(time.Millisecond)))
		} else {
			end = end.UTC()
		}
		if minStart.IsZero() || start.Before(minStart) {
			minStart = start
		}
		if maxEnd.IsZero() || end.After(maxEnd) {
			maxEnd = end
		}
	}
	projection := DiffTimeProjection{
		Degenerate:         !timelineAvailable,
		RelativeDurationMS: relativeDurationMS,
	}
	if !minStart.IsZero() && maxEnd.After(minStart) {
		projection.Start = minStart.Format(time.RFC3339Nano)
		projection.End = maxEnd.Format(time.RFC3339Nano)
		projection.CaptureDurationMS = float64(maxEnd.Sub(minStart)) / float64(time.Millisecond)
	}
	projection.AbsoluteTimeTrusted = timelineAvailable && projection.Start != "" && projection.End != ""
	projection.RateAvailable = projection.AbsoluteTimeTrusted && projection.CaptureDurationMS > 0
	switch {
	case projection.Degenerate:
		projection.RateUnavailableCode = "timestamps_degenerate"
	case !projection.RateAvailable:
		projection.RateUnavailableCode = "capture_duration_unavailable"
	}
	return projection
}

func normalizeDiffTemplateLimit(value int) int {
	if value <= 0 {
		return DefaultDiffTemplateLimit
	}
	if value > MaxDiffTemplateLimit {
		return MaxDiffTemplateLimit
	}
	return value
}

func normalizeDiffTableLimit(value int) int {
	if value <= 0 {
		return DefaultDiffTableLimit
	}
	if value > MaxDiffTableLimit {
		return MaxDiffTableLimit
	}
	return value
}

type explicitRate struct {
	Numerator   int     `json:"numerator"`
	Denominator int     `json:"denominator"`
	Value       float64 `json:"value"`
}

type normalizedRate struct {
	Numerator          int     `json:"numerator"`
	DenominatorMinutes float64 `json:"denominator_minutes"`
	ValuePerMinute     float64 `json:"value_per_minute"`
}

type sideMetrics struct {
	Count             int             `json:"count"`
	Errors            int             `json:"errors"`
	ErrorRate         explicitRate    `json:"error_rate"`
	TrafficShare      explicitRate    `json:"traffic_share"`
	CountPerMinute    *normalizedRate `json:"count_per_minute,omitempty"`
	DurationP50MS     float64         `json:"duration_p50_ms"`
	DurationP95MS     float64         `json:"duration_p95_ms"`
	DurationP99MS     float64         `json:"duration_p99_ms"`
	DurationSamples   int             `json:"duration_samples"`
	RequestBytes      int64           `json:"request_bytes"`
	ResponseBytes     int64           `json:"response_bytes"`
	RateUnavailable   string          `json:"rate_unavailable_code,omitempty"`
	CaptureDurationMS float64         `json:"capture_duration_ms,omitempty"`
}

func metricsFor(stat DiffDimensionStat, total int, timeline DiffTimeProjection) sideMetrics {
	metrics := sideMetrics{
		Count: stat.Count, Errors: stat.Errors,
		ErrorRate:       makeExplicitRate(stat.Errors, stat.Count),
		TrafficShare:    makeExplicitRate(stat.Count, total),
		DurationP50MS:   stat.DurationP50MS,
		DurationP95MS:   stat.DurationP95MS,
		DurationP99MS:   stat.DurationP99MS,
		DurationSamples: stat.DurationSamples,
		RequestBytes:    stat.RequestBytes,
		ResponseBytes:   stat.ResponseBytes,
	}
	if timeline.RateAvailable {
		minutes := timeline.CaptureDurationMS / 60_000
		metrics.CountPerMinute = &normalizedRate{
			Numerator: stat.Count, DenominatorMinutes: minutes,
			ValuePerMinute: safeDivide(float64(stat.Count), minutes),
		}
		metrics.CaptureDurationMS = timeline.CaptureDurationMS
	} else {
		metrics.RateUnavailable = timeline.RateUnavailableCode
	}
	return metrics
}

func makeExplicitRate(numerator, denominator int) explicitRate {
	return explicitRate{
		Numerator: numerator, Denominator: denominator,
		Value: safeDivide(float64(numerator), float64(denominator)),
	}
}

func safeDivide(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

type timeAlignment struct {
	Grade          string `json:"grade"`
	OverlayAllowed bool   `json:"overlay_allowed"`
	Reason         string `json:"reason"`
}

func alignmentFor(before, after DiffTimeProjection) timeAlignment {
	if before.Degenerate || after.Degenerate {
		return timeAlignment{Grade: "none", OverlayAllowed: false, Reason: "one or both sessions have degenerate timestamps"}
	}
	if before.AbsoluteTimeTrusted && after.AbsoluteTimeTrusted {
		return timeAlignment{Grade: "aligned", OverlayAllowed: true, Reason: "both sessions have trusted absolute timestamps"}
	}
	if before.RelativeDurationMS > 0 && after.RelativeDurationMS > 0 {
		return timeAlignment{Grade: "duration_only", OverlayAllowed: true, Reason: "only relative request durations are comparable"}
	}
	return timeAlignment{Grade: "none", OverlayAllowed: false, Reason: "comparable time evidence is unavailable"}
}

// AnalyzeDiff consumes only the bounded source projections already carried by
// two http_capture result envelopes.
func AnalyzeDiff(beforePayload, afterPayload map[string]any, opts DiffOptions) (models.AnalysisResult, error) {
	before, err := decodeDiffSource(beforePayload)
	if err != nil {
		return models.AnalysisResult{}, fmt.Errorf("before: %w", err)
	}
	after, err := decodeDiffSource(afterPayload)
	if err != nil {
		return models.AnalysisResult{}, fmt.Errorf("after: %w", err)
	}
	if before.URLTemplateVersion != after.URLTemplateVersion {
		return models.AnalysisResult{}, fmt.Errorf("url template version mismatch (%d vs %d); re-analyze both sessions with version %d", before.URLTemplateVersion, after.URLTemplateVersion, URLTemplateVersion)
	}
	if before.URLTemplateVersion != URLTemplateVersion {
		return models.AnalysisResult{}, fmt.Errorf("unsupported url template version %d", before.URLTemplateVersion)
	}
	if before.TemplateLimit != after.TemplateLimit {
		return models.AnalysisResult{}, fmt.Errorf("source template limit mismatch (%d vs %d); re-analyze both sessions with the same limit", before.TemplateLimit, after.TemplateLimit)
	}
	if err := validateDiffProjection(before); err != nil {
		return models.AnalysisResult{}, fmt.Errorf("before: %w", err)
	}
	if err := validateDiffProjection(after); err != nil {
		return models.AnalysisResult{}, fmt.Errorf("after: %w", err)
	}

	tableLimit := normalizeDiffTableLimit(opts.TopN)
	alignment := alignmentFor(before.Time, after.Time)
	result := models.New(DiffResultType, DiffResultType)
	result.Metadata.SchemaVersion = DiffSchemaVersion
	result.Summary = buildDiffSummary(before, after, alignment, tableLimit)
	endpointsChanged, endpointsAdded, endpointsRemoved := compareEndpointDimensions(before, after, tableLimit)
	result.Tables = map[string]any{
		"endpoints_changed": endpointsChanged,
		"endpoints_added":   endpointsAdded,
		"endpoints_removed": endpointsRemoved,
		"hosts_changed":     compareDimension(before.Hosts, after.Hosts, before, after, tableLimit, true),
		"processes_changed": []map[string]any{},
	}
	processComparable := before.ProcessAvailable && after.ProcessAvailable
	processReason := ""
	if processComparable {
		result.Tables["processes_changed"] = compareDimension(before.Processes, after.Processes, before, after, tableLimit, true)
	} else {
		processReason = strings.TrimSpace(strings.Join(nonEmptyStrings(before.ProcessReason, after.ProcessReason), "; "))
	}
	result.Metadata.Extra["http_capture_diff"] = map[string]any{
		"contract":                  DefaultDiffContract(),
		"before_session":            before.Session,
		"after_session":             after.Session,
		"url_template_version":      before.URLTemplateVersion,
		"source_projection_version": before.SchemaVersion,
		"table_limit":               tableLimit,
		"template_limit":            max(before.TemplateLimit, after.TemplateLimit),
		"time_alignment":            alignment,
		"process_dimension": map[string]any{
			"available": processComparable,
			"reason":    processReason,
		},
		"dimension_totals": map[string]any{
			"before": dimensionTotals(before),
			"after":  dimensionTotals(after),
		},
		"store_rescanned":   false,
		"export_projection": "analysis_result_envelope",
		"workspace_route":   ResolveWorkspaceComparison(ResultType, ResultType),
		"finding_thresholds": map[string]any{
			"traffic_share_delta": 0.10,
			"error_rate_delta":    0.05,
			"p95_min_delta_ms":    50.0,
			"p95_min_ratio":       1.20,
		},
	}
	addDiffFindings(&result, before, after, alignment)
	return result, nil
}

func decodeDiffSource(payload map[string]any) (DiffSourceProjection, error) {
	if payload == nil {
		return DiffSourceProjection{}, errDiffSourceMissing
	}
	if resultType, _ := payload["type"].(string); resultType != ResultType {
		return DiffSourceProjection{}, fmt.Errorf("expected result type %q, got %q", ResultType, resultType)
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok {
		return DiffSourceProjection{}, errDiffSourceMissing
	}
	raw, ok := metadata["http_capture_diff_source"]
	if !ok {
		return DiffSourceProjection{}, errDiffSourceMissing
	}
	if err := validateRawProjectionBounds(raw); err != nil {
		return DiffSourceProjection{}, err
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return DiffSourceProjection{}, fmt.Errorf("encode source projection: %w", err)
	}
	var projection DiffSourceProjection
	if err := json.Unmarshal(body, &projection); err != nil {
		return DiffSourceProjection{}, fmt.Errorf("decode source projection: %w", err)
	}
	return projection, nil
}

func validateDiffProjection(projection DiffSourceProjection) error {
	if projection.SchemaVersion != DiffContractVersion {
		return fmt.Errorf("unsupported source projection version %d", projection.SchemaVersion)
	}
	if projection.TemplateLimit <= 0 || projection.TemplateLimit > MaxDiffTemplateLimit {
		return fmt.Errorf("invalid template limit %d", projection.TemplateLimit)
	}
	if projection.Session.Transactions < 0 || projection.Total.Count < 0 {
		return fmt.Errorf("negative transaction total")
	}
	if projection.Session.Transactions != projection.Total.Count {
		return fmt.Errorf("session total %d does not match projection total %d", projection.Session.Transactions, projection.Total.Count)
	}
	if projection.EndpointTotal != projection.Total.Count || projection.HostTotal != projection.Total.Count {
		return fmt.Errorf("cross-dimension totals do not match total transactions")
	}
	if projection.ProcessAvailable && projection.ProcessTotal != projection.Total.Count {
		return fmt.Errorf("process dimension total does not match total transactions")
	}
	if len(projection.Endpoints) > projection.TemplateLimit+1 || len(projection.Hosts) > projection.TemplateLimit+1 || len(projection.Processes) > projection.TemplateLimit+1 {
		return fmt.Errorf("source projection exceeds declared template limit")
	}
	return nil
}

func validateRawProjectionBounds(raw any) error {
	projection, ok := raw.(map[string]any)
	if !ok {
		// Internal callers may retain the typed value until the ordinary
		// AnalysisResult JSON boundary.
		if _, typed := raw.(DiffSourceProjection); typed {
			return nil
		}
		return fmt.Errorf("source projection must be an object")
	}
	for _, key := range []string{"endpoints", "hosts", "processes"} {
		rows, exists := projection[key]
		if !exists {
			continue
		}
		values, ok := rows.([]any)
		if !ok {
			return fmt.Errorf("source projection %s must be an array", key)
		}
		if len(values) > MaxDiffTemplateLimit+1 {
			return fmt.Errorf("source projection %s exceeds hard row limit %d", key, MaxDiffTemplateLimit+1)
		}
	}
	return nil
}

func buildDiffSummary(before, after DiffSourceProjection, alignment timeAlignment, tableLimit int) map[string]any {
	beforeMetrics := metricsFor(before.Total, before.Total.Count, before.Time)
	afterMetrics := metricsFor(after.Total, after.Total.Count, after.Time)
	return map[string]any{
		"before_session":       before.Session,
		"after_session":        after.Session,
		"before":               beforeMetrics,
		"after":                afterMetrics,
		"delta":                deltaMetrics(beforeMetrics, afterMetrics),
		"time_alignment":       alignment,
		"url_template_version": before.URLTemplateVersion,
		"table_limit":          tableLimit,
	}
}

func deltaMetrics(before, after sideMetrics) map[string]any {
	out := map[string]any{
		"count":           after.Count - before.Count,
		"errors":          after.Errors - before.Errors,
		"error_rate":      after.ErrorRate.Value - before.ErrorRate.Value,
		"duration_p50_ms": after.DurationP50MS - before.DurationP50MS,
		"duration_p95_ms": after.DurationP95MS - before.DurationP95MS,
		"duration_p99_ms": after.DurationP99MS - before.DurationP99MS,
		"request_bytes":   after.RequestBytes - before.RequestBytes,
		"response_bytes":  after.ResponseBytes - before.ResponseBytes,
	}
	if before.CountPerMinute != nil && after.CountPerMinute != nil {
		out["count_per_minute"] = after.CountPerMinute.ValuePerMinute - before.CountPerMinute.ValuePerMinute
	} else {
		out["count_per_minute"] = nil
	}
	return out
}

func compareEndpointDimensions(before, after DiffSourceProjection, limit int) ([]map[string]any, []map[string]any, []map[string]any) {
	beforeMap := dimensionMap(before.Endpoints)
	afterMap := dimensionMap(after.Endpoints)
	changed := make([]map[string]any, 0)
	added := make([]map[string]any, 0)
	removed := make([]map[string]any, 0)
	for key, beforeStat := range beforeMap {
		afterStat, ok := afterMap[key]
		if !ok {
			removed = append(removed, comparisonRow(key, "removed", beforeStat, DiffDimensionStat{}, before, after))
			continue
		}
		if !equalDiffStats(beforeStat, afterStat) {
			changed = append(changed, comparisonRow(key, "changed", beforeStat, afterStat, before, after))
		}
	}
	for key, afterStat := range afterMap {
		if _, ok := beforeMap[key]; !ok {
			added = append(added, comparisonRow(key, "added", DiffDimensionStat{}, afterStat, before, after))
		}
	}
	sortComparisonRows(changed)
	sortComparisonRows(added)
	sortComparisonRows(removed)
	return limitRows(changed, limit), limitRows(added, limit), limitRows(removed, limit)
}

func compareDimension(beforeRows, afterRows []DiffDimensionStat, before, after DiffSourceProjection, limit int, includeOneSided bool) []map[string]any {
	beforeMap := dimensionMap(beforeRows)
	afterMap := dimensionMap(afterRows)
	rows := make([]map[string]any, 0)
	keys := map[string]struct{}{}
	for key := range beforeMap {
		keys[key] = struct{}{}
	}
	for key := range afterMap {
		keys[key] = struct{}{}
	}
	for key := range keys {
		beforeStat, beforeOK := beforeMap[key]
		afterStat, afterOK := afterMap[key]
		if !includeOneSided && (!beforeOK || !afterOK) {
			continue
		}
		if beforeOK && afterOK && equalDiffStats(beforeStat, afterStat) {
			continue
		}
		change := "changed"
		if !beforeOK {
			change = "added"
		} else if !afterOK {
			change = "removed"
		}
		rows = append(rows, comparisonRow(key, change, beforeStat, afterStat, before, after))
	}
	sortComparisonRows(rows)
	return limitRows(rows, limit)
}

func comparisonRow(key, change string, beforeStat, afterStat DiffDimensionStat, before, after DiffSourceProjection) map[string]any {
	beforeMetrics := metricsFor(beforeStat, before.Total.Count, before.Time)
	afterMetrics := metricsFor(afterStat, after.Total.Count, after.Time)
	return map[string]any{
		"key": key, "change": change,
		"before": beforeMetrics,
		"after":  afterMetrics,
		"delta":  deltaMetrics(beforeMetrics, afterMetrics),
	}
}

func dimensionMap(rows []DiffDimensionStat) map[string]DiffDimensionStat {
	out := make(map[string]DiffDimensionStat, len(rows))
	for _, row := range rows {
		out[row.Key] = row
	}
	return out
}

func equalDiffStats(a, b DiffDimensionStat) bool {
	return a.Count == b.Count && a.Errors == b.Errors &&
		a.RequestBytes == b.RequestBytes && a.ResponseBytes == b.ResponseBytes &&
		a.DurationP50MS == b.DurationP50MS && a.DurationP95MS == b.DurationP95MS &&
		a.DurationP99MS == b.DurationP99MS && a.DurationSamples == b.DurationSamples
}

func sortComparisonRows(rows []map[string]any) {
	sort.Slice(rows, func(i, j int) bool {
		left := comparisonMagnitude(rows[i])
		right := comparisonMagnitude(rows[j])
		if left != right {
			return left > right
		}
		return fmt.Sprint(rows[i]["key"]) < fmt.Sprint(rows[j]["key"])
	})
}

func comparisonMagnitude(row map[string]any) float64 {
	delta, _ := row["delta"].(map[string]any)
	if value, ok := delta["count_per_minute"].(float64); ok {
		return math.Abs(value)
	}
	switch value := delta["count"].(type) {
	case int:
		return math.Abs(float64(value))
	case float64:
		return math.Abs(value)
	default:
		return 0
	}
}

func limitRows(rows []map[string]any, limit int) []map[string]any {
	if len(rows) > limit {
		rows = rows[:limit]
	}
	if rows == nil {
		return []map[string]any{}
	}
	return rows
}

func addDiffFindings(result *models.AnalysisResult, before, after DiffSourceProjection, alignment timeAlignment) {
	beforeMetrics := metricsFor(before.Total, before.Total.Count, before.Time)
	afterMetrics := metricsFor(after.Total, after.Total.Count, after.Time)
	if shifted := trafficShiftEvidence(before, after); len(shifted) > 0 {
		result.AddFinding("warning", "HTTP_DIFF_TRAFFIC_SHIFT", "HTTP traffic distribution shifted between sessions", map[string]any{
			"threshold_share_delta": 0.10,
			"endpoints":             shifted,
		})
	}
	if afterMetrics.ErrorRate.Value-beforeMetrics.ErrorRate.Value >= 0.05 {
		result.AddFinding("warning", "HTTP_DIFF_ERROR_RATE_UP", "HTTP error rate increased", map[string]any{
			"before":          beforeMetrics.ErrorRate,
			"after":           afterMetrics.ErrorRate,
			"delta":           afterMetrics.ErrorRate.Value - beforeMetrics.ErrorRate.Value,
			"threshold_delta": 0.05,
		})
	}
	if beforeMetrics.DurationSamples > 0 && afterMetrics.DurationSamples > 0 &&
		afterMetrics.DurationP95MS-beforeMetrics.DurationP95MS >= 50 &&
		afterMetrics.DurationP95MS >= beforeMetrics.DurationP95MS*1.20 {
		result.AddFinding("warning", "HTTP_DIFF_LATENCY_REGRESSION", "HTTP p95 latency regressed", map[string]any{
			"before_p95_ms":    beforeMetrics.DurationP95MS,
			"after_p95_ms":     afterMetrics.DurationP95MS,
			"delta_ms":         afterMetrics.DurationP95MS - beforeMetrics.DurationP95MS,
			"minimum_delta_ms": 50,
			"minimum_ratio":    1.20,
		})
	}
	if endpoints := newErrorEndpointEvidence(before, after); len(endpoints) > 0 {
		result.AddFinding("warning", "HTTP_DIFF_NEW_ERROR_ENDPOINT", "New endpoint errors appeared in the target session", map[string]any{
			"endpoints": endpoints,
		})
	}
	if alignment.Grade != "aligned" {
		result.AddFinding("info", "HTTP_DIFF_ALIGNMENT_LOW", "HTTP session time alignment is limited", map[string]any{
			"grade": alignment.Grade, "overlay_allowed": alignment.OverlayAllowed, "reason": alignment.Reason,
		})
	}
}

func trafficShiftEvidence(before, after DiffSourceProjection) []map[string]any {
	beforeMap := dimensionMap(before.Endpoints)
	afterMap := dimensionMap(after.Endpoints)
	keys := map[string]struct{}{}
	for key := range beforeMap {
		keys[key] = struct{}{}
	}
	for key := range afterMap {
		keys[key] = struct{}{}
	}
	rows := make([]map[string]any, 0)
	for key := range keys {
		beforeShare := makeExplicitRate(beforeMap[key].Count, before.Total.Count)
		afterShare := makeExplicitRate(afterMap[key].Count, after.Total.Count)
		delta := afterShare.Value - beforeShare.Value
		if math.Abs(delta) >= 0.10 {
			rows = append(rows, map[string]any{"key": key, "before": beforeShare, "after": afterShare, "delta": delta})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		left, _ := rows[i]["delta"].(float64)
		right, _ := rows[j]["delta"].(float64)
		if math.Abs(left) != math.Abs(right) {
			return math.Abs(left) > math.Abs(right)
		}
		return fmt.Sprint(rows[i]["key"]) < fmt.Sprint(rows[j]["key"])
	})
	return limitRows(rows, diffFindingEvidenceRowLimit)
}

func newErrorEndpointEvidence(before, after DiffSourceProjection) []map[string]any {
	beforeMap := dimensionMap(before.Endpoints)
	rows := make([]map[string]any, 0)
	for _, stat := range after.Endpoints {
		if stat.Errors > 0 && beforeMap[stat.Key].Errors == 0 {
			rows = append(rows, map[string]any{
				"key":    stat.Key,
				"before": makeExplicitRate(beforeMap[stat.Key].Errors, beforeMap[stat.Key].Count),
				"after":  makeExplicitRate(stat.Errors, stat.Count),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i]["after"].(explicitRate)
		right := rows[j]["after"].(explicitRate)
		if left.Numerator != right.Numerator {
			return left.Numerator > right.Numerator
		}
		return fmt.Sprint(rows[i]["key"]) < fmt.Sprint(rows[j]["key"])
	})
	return limitRows(rows, diffFindingEvidenceRowLimit)
}

func dimensionTotals(projection DiffSourceProjection) map[string]any {
	return map[string]any{
		"transactions":      projection.Total.Count,
		"endpoints":         projection.EndpointTotal,
		"hosts":             projection.HostTotal,
		"processes":         projection.ProcessTotal,
		"process_available": projection.ProcessAvailable,
		"cross_check_passed": projection.EndpointTotal == projection.Total.Count &&
			projection.HostTotal == projection.Total.Count &&
			(!projection.ProcessAvailable || projection.ProcessTotal == projection.Total.Count),
	}
}

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
