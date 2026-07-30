// Package httpcorrelation correlates bounded HTTP capture evidence with
// browser CPU runs, Jennifer network-gap estimates, and access-log records.
//
// The analyzer consumes AnalysisResult envelopes only. It never reopens source
// files or live capture stores, and it never upgrades temporal correlation into
// a causal claim.
package httpcorrelation

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/httpcapture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const (
	ResultType           = "http_evidence_correlation"
	SchemaVersion        = "1.0.0"
	ContractVersion      = 1
	DefaultTopN          = 50
	MaxTopN              = 500
	DefaultToleranceMS   = 1000
	MaxToleranceMS       = 60_000
	maxSourceRows        = 2000
	findingEvidenceLimit = 10
)

var errHTTPSourceMissing = errors.New("http_capture result is required")

// Options controls bounded output and explicit clock alignment. ProfileWallClockStart
// is the RFC3339 wall-clock instant corresponding to the V8 profile startTime.
// Without it, V8 monotonic timestamps must not be overlaid on HTTP wall time.
type Options struct {
	TopN                  int
	TimeToleranceMS       float64
	ProfileWallClockStart string
}

type Contract struct {
	SchemaVersion             int      `json:"schema_version"`
	ResultType                string   `json:"result_type"`
	HTTPResultType            string   `json:"http_result_type"`
	OptionalResultTypes       []string `json:"optional_result_types"`
	AlignmentGrades           []string `json:"alignment_grades"`
	ConfidenceGrades          []string `json:"confidence_grades"`
	DefaultTopN               int      `json:"default_top_n"`
	MaxTopN                   int      `json:"max_top_n"`
	DefaultTimeToleranceMS    float64  `json:"default_time_tolerance_ms"`
	MaxTimeToleranceMS        float64  `json:"max_time_tolerance_ms"`
	AnalyzeMethod             string   `json:"analyze_method"`
	RequiresProfileWallAnchor bool     `json:"requires_profile_wall_clock_anchor"`
	StoreOrFileRescan         bool     `json:"store_or_file_rescan"`
	CausalClaimsAllowed       bool     `json:"causal_claims_allowed"`
}

func DefaultContract() Contract {
	return Contract{
		SchemaVersion:             ContractVersion,
		ResultType:                ResultType,
		HTTPResultType:            httpcapture.ResultType,
		OptionalResultTypes:       []string{"profile_evidence", "jennifer_profile", "access_log"},
		AlignmentGrades:           []string{"aligned", "duration_only", "none"},
		ConfidenceGrades:          []string{"high", "medium", "low", "none"},
		DefaultTopN:               DefaultTopN,
		MaxTopN:                   MaxTopN,
		DefaultTimeToleranceMS:    DefaultToleranceMS,
		MaxTimeToleranceMS:        MaxToleranceMS,
		AnalyzeMethod:             "AnalyzeHttpEvidenceCorrelation",
		RequiresProfileWallAnchor: true,
		StoreOrFileRescan:         false,
		CausalClaimsAllowed:       false,
	}
}

type sourceDiagnostic struct {
	Source          string `json:"source"`
	ResultType      string `json:"result_type"`
	AlignmentGrade  string `json:"alignment_grade"`
	Confidence      string `json:"confidence"`
	OverlayAllowed  bool   `json:"overlay_allowed"`
	Reason          string `json:"reason"`
	InputRows       int    `json:"input_rows"`
	RowsUsed        int    `json:"rows_used"`
	CandidateRows   int    `json:"candidate_rows"`
	OutputRows      int    `json:"output_rows"`
	SourceTruncated bool   `json:"source_truncated"`
	OutputTruncated bool   `json:"output_truncated"`
}

type httpTransaction struct {
	ID        string
	Method    string
	Host      string
	Path      string
	Template  string
	Status    int
	Duration  float64
	Start     time.Time
	End       time.Time
	HasTime   bool
	RequestID string
	Timings   map[string]any
}

type timedCPURun struct {
	Stack      string
	Start      time.Time
	End        time.Time
	DurationMS float64
}

// Analyze consumes already-produced AnalysisResult envelopes. At least one
// secondary source must be provided.
func Analyze(httpResult, profileResult, jenniferResult, accessResult map[string]any, opts Options) (models.AnalysisResult, error) {
	if httpResult == nil {
		return models.AnalysisResult{}, errHTTPSourceMissing
	}
	if err := requireResultType(httpResult, httpcapture.ResultType, "http"); err != nil {
		return models.AnalysisResult{}, err
	}
	if profileResult == nil && jenniferResult == nil && accessResult == nil {
		return models.AnalysisResult{}, errors.New("at least one profile_evidence, jennifer_profile, or access_log result is required")
	}
	if profileResult != nil {
		if err := requireResultType(profileResult, "profile_evidence", "profile"); err != nil {
			return models.AnalysisResult{}, err
		}
	}
	if jenniferResult != nil {
		if err := requireResultType(jenniferResult, "jennifer_profile", "jennifer"); err != nil {
			return models.AnalysisResult{}, err
		}
	}
	if accessResult != nil {
		if err := requireResultType(accessResult, "access_log", "access log"); err != nil {
			return models.AnalysisResult{}, err
		}
	}

	topN := normalizeTopN(opts.TopN)
	toleranceMS := normalizeTolerance(opts.TimeToleranceMS)
	httpRows := tableRows(httpResult, "transactions")
	httpRowsUsed, httpTruncated := boundedRows(httpRows, maxSourceRows)
	transactions := parseHTTPTransactions(httpRowsUsed)

	result := models.New(ResultType, "http_evidence_correlation")
	result.Metadata.SchemaVersion = SchemaVersion
	result.SourceFiles = mergedSourceFiles(httpResult, profileResult, jenniferResult, accessResult)
	result.Metadata.Extra["http_evidence_correlation"] = map[string]any{
		"contract":                  DefaultContract(),
		"top_n":                     topN,
		"time_tolerance_ms":         toleranceMS,
		"input_projection":          "analysis_result_envelope",
		"store_or_file_rescanned":   false,
		"causal_claims_allowed":     false,
		"profile_wall_clock_anchor": strings.TrimSpace(opts.ProfileWallClockStart) != "",
	}
	result.Metadata.Extra["source_provenance"] = provenanceRows(httpResult, profileResult, jenniferResult, accessResult)

	diagnostics := []sourceDiagnostic{{
		Source: "http_capture", ResultType: httpcapture.ResultType,
		AlignmentGrade: httpAlignmentGrade(transactions), Confidence: confidenceForHTTP(transactions),
		OverlayAllowed: false, Reason: "primary observation source; overlays are decided per secondary source",
		InputRows: len(httpRows), RowsUsed: len(httpRowsUsed), SourceTruncated: httpTruncated,
	}}

	profileMatches := []map[string]any{}
	if profileResult != nil {
		var diagnostic sourceDiagnostic
		profileMatches, diagnostic = correlateProfile(transactions, profileResult, opts.ProfileWallClockStart, topN)
		diagnostics = append(diagnostics, diagnostic)
		if diagnostic.AlignmentGrade == "none" {
			result.AddFinding("info", "HTTP_CORRELATION_PROFILE_CLOCK_INCOMPATIBLE",
				"CPU profile timestamps were not overlaid because no compatible wall-clock anchor and V8 timestamp base were available.",
				map[string]any{"reason": diagnostic.Reason})
		}
	}

	jenniferMatches := []map[string]any{}
	if jenniferResult != nil {
		var diagnostic sourceDiagnostic
		jenniferMatches, diagnostic = correlateJennifer(transactions, jenniferResult, topN, toleranceMS)
		diagnostics = append(diagnostics, diagnostic)
		addJenniferFindings(&result, jenniferMatches, toleranceMS)
	}

	accessMatches := []map[string]any{}
	if accessResult != nil {
		var diagnostic sourceDiagnostic
		accessMatches, diagnostic = correlateAccess(transactions, accessResult, topN, toleranceMS)
		diagnostics = append(diagnostics, diagnostic)
		addAccessFindings(&result, accessMatches, toleranceMS)
	}

	for index := range diagnostics {
		switch diagnostics[index].Source {
		case "profile_evidence":
			diagnostics[index].OutputRows = len(profileMatches)
		case "jennifer_profile":
			diagnostics[index].OutputRows = len(jenniferMatches)
		case "access_log":
			diagnostics[index].OutputRows = len(accessMatches)
		}
	}

	result.Tables["http_profile_overlaps"] = profileMatches
	result.Tables["jennifer_network_gap_checks"] = jenniferMatches
	result.Tables["access_log_matches"] = accessMatches
	result.Tables["alignment_diagnostics"] = diagnostics
	result.Summary = map[string]any{
		"http_transaction_rows":      len(transactions),
		"profile_overlap_count":      len(profileMatches),
		"jennifer_check_count":       len(jenniferMatches),
		"access_log_match_count":     len(accessMatches),
		"aligned_source_count":       countAlignment(diagnostics, "aligned"),
		"duration_only_source_count": countAlignment(diagnostics, "duration_only"),
		"incompatible_source_count":  countAlignment(diagnostics, "none"),
		"causal_claims_allowed":      false,
		"store_or_file_rescanned":    false,
	}
	return result, nil
}

func correlateProfile(httpRows []httpTransaction, result map[string]any, anchorRaw string, topN int) ([]map[string]any, sourceDiagnostic) {
	rows := tableRows(result, "cpu_sample_runs")
	used, truncated := boundedRows(rows, maxSourceRows)
	diagnostic := sourceDiagnostic{
		Source: "profile_evidence", ResultType: "profile_evidence",
		InputRows: len(rows), RowsUsed: len(used), SourceTruncated: truncated,
		AlignmentGrade: "none", Confidence: "none", OverlayAllowed: false,
	}
	if len(used) == 0 {
		diagnostic.Reason = "profile result contains no exact cpu_sample_runs projection"
		return []map[string]any{}, diagnostic
	}
	anchor, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(anchorRaw))
	if err != nil {
		diagnostic.Reason = "V8 timestamps are monotonic; provide profileWallClockStart as an RFC3339 wall-clock anchor"
		return []map[string]any{}, diagnostic
	}
	v8StartUS, ok := microseconds(nested(result, "metadata", "parser_metadata", "v8_start_time_us"))
	if !ok {
		diagnostic.Reason = "profile metadata is missing a valid numeric parser_metadata.v8_start_time_us timestamp base"
		return []map[string]any{}, diagnostic
	}
	runs := make([]timedCPURun, 0, len(used))
	for _, row := range used {
		startUS, startOK := microseconds(row["start_us"])
		endUS, endOK := microseconds(row["end_us"])
		if !startOK || !endOK || endUS <= startUS {
			continue
		}
		start := anchor.Add(time.Duration(startUS-v8StartUS) * time.Microsecond)
		end := anchor.Add(time.Duration(endUS-v8StartUS) * time.Microsecond)
		runs = append(runs, timedCPURun{Stack: text(row["stack"]), Start: start, End: end, DurationMS: float64(endUS-startUS) / 1000})
	}
	if !hasTimedHTTP(httpRows) {
		diagnostic.Reason = "HTTP result contains no parseable absolute transaction timestamps"
		return []map[string]any{}, diagnostic
	}
	diagnostic.AlignmentGrade = "aligned"
	diagnostic.Confidence = "high"
	diagnostic.OverlayAllowed = true
	diagnostic.Reason = "explicit profile wall-clock anchor maps V8 monotonic timestamps to HTTP wall time"

	matches := make([]map[string]any, 0)
	for _, transaction := range httpRows {
		if !transaction.HasTime {
			continue
		}
		for _, run := range runs {
			overlap := overlapDuration(transaction.Start, transaction.End, run.Start, run.End)
			if overlap <= 0 {
				continue
			}
			denominator := math.Min(transaction.Duration, run.DurationMS)
			ratio := 0.0
			if denominator > 0 {
				ratio = overlap / denominator
			}
			confidence := "medium"
			if ratio >= 0.5 {
				confidence = "high"
			}
			matches = append(matches, map[string]any{
				"http_id": transaction.ID, "endpoint": transaction.Template,
				"http_started_at": transaction.Start.Format(time.RFC3339Nano),
				"http_ended_at":   transaction.End.Format(time.RFC3339Nano),
				"cpu_stack":       run.Stack, "cpu_started_at": run.Start.Format(time.RFC3339Nano),
				"cpu_ended_at": run.End.Format(time.RFC3339Nano),
				"overlap_ms":   round(overlap, 3), "overlap_ratio": round(ratio, 4),
				"alignment_grade": "aligned", "confidence": confidence,
				"match_basis":          "explicit_wall_clock_anchor+time_overlap",
				"causal_claim_allowed": false,
			})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return number(matches[i]["overlap_ms"]) > number(matches[j]["overlap_ms"])
	})
	diagnostic.CandidateRows = len(matches)
	diagnostic.OutputTruncated = len(matches) > topN
	return limitRows(matches, topN), diagnostic
}

func correlateJennifer(httpRows []httpTransaction, result map[string]any, topN int, toleranceMS float64) ([]map[string]any, sourceDiagnostic) {
	rows := tableRows(result, "msa_edges")
	used, truncated := boundedRows(rows, maxSourceRows)
	diagnostic := sourceDiagnostic{
		Source: "jennifer_profile", ResultType: "jennifer_profile",
		AlignmentGrade: "duration_only", Confidence: "low", OverlayAllowed: false,
		Reason:    "Jennifer edge timestamps are ms-since-midnight without a date/offset; host and duration checks cannot prove temporal overlap",
		InputRows: len(rows), RowsUsed: len(used), SourceTruncated: truncated,
	}
	matches := make([]map[string]any, 0)
	for _, edge := range used {
		gap, ok := optionalNumber(edge["adjusted_network_gap_ms"])
		if !ok {
			continue
		}
		target := firstText(text(edge["external_call_target"]), text(edge["external_call_url"]))
		host := canonicalHost(target)
		if host == "" {
			continue
		}
		elapsed := number(edge["external_call_elapsed_ms"])
		bestIndex := -1
		bestDelta := math.MaxFloat64
		for index, transaction := range httpRows {
			if canonicalHost(transaction.Host) != host {
				continue
			}
			delta := math.Abs(transaction.Duration - elapsed)
			if delta < bestDelta {
				bestDelta, bestIndex = delta, index
			}
		}
		if bestIndex < 0 {
			continue
		}
		transaction := httpRows[bestIndex]
		observed, observedCount := observedNetworkMS(transaction.Timings)
		confidence := "low"
		if elapsed > 0 && bestDelta <= math.Max(toleranceMS, elapsed*0.25) {
			confidence = "medium"
		}
		row := map[string]any{
			"http_id": transaction.ID, "endpoint": transaction.Template,
			"jennifer_guid": text(edge["guid"]), "target_host": host,
			"external_call_elapsed_ms": elapsed, "http_duration_ms": transaction.Duration,
			"duration_delta_ms":            round(bestDelta, 3),
			"jennifer_network_gap_ms":      gap,
			"observed_network_phase_count": observedCount,
			"alignment_grade":              "duration_only", "confidence": confidence,
			"match_basis":          "target_host+nearest_duration",
			"causal_claim_allowed": false,
		}
		if observedCount > 0 {
			row["http_observed_network_ms"] = round(observed, 3)
			row["network_gap_delta_ms"] = round(observed-gap, 3)
		} else {
			row["network_gap_unavailable_reason"] = "HTTP transaction has no known DNS/connect/TLS/send/receive timing phases"
		}
		matches = append(matches, row)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return number(matches[i]["duration_delta_ms"]) < number(matches[j]["duration_delta_ms"])
	})
	diagnostic.CandidateRows = len(matches)
	diagnostic.OutputTruncated = len(matches) > topN
	return limitRows(matches, topN), diagnostic
}

func correlateAccess(httpRows []httpTransaction, result map[string]any, topN int, toleranceMS float64) ([]map[string]any, sourceDiagnostic) {
	rows := tableRows(result, "sample_records")
	used, truncated := boundedRows(rows, maxSourceRows)
	diagnostic := sourceDiagnostic{
		Source: "access_log", ResultType: "access_log",
		InputRows: len(rows), RowsUsed: len(used), SourceTruncated: truncated,
		AlignmentGrade: "none", Confidence: "none", OverlayAllowed: false,
		Reason: "no compatible access-log records were available",
	}
	matches := make([]map[string]any, 0)
	claimed := map[int]bool{}
	for _, transaction := range httpRows {
		bestIndex := -1
		bestDelta := math.MaxFloat64
		bestBasis := ""
		bestClockCompared := false
		for index, record := range used {
			if claimed[index] {
				continue
			}
			requestID := text(record["request_id"])
			if transaction.RequestID != "" && requestID != "" && strings.EqualFold(transaction.RequestID, requestID) {
				bestIndex, bestBasis = index, "request_id"
				if recordTime, err := time.Parse(time.RFC3339Nano, text(record["timestamp"])); err == nil && transaction.HasTime {
					bestDelta = math.Abs(float64(recordTime.Sub(transaction.Start)) / float64(time.Millisecond))
					bestClockCompared = true
				}
				break
			}
			if !sameRequestShape(transaction, record) || !transaction.HasTime {
				continue
			}
			recordTime, err := time.Parse(time.RFC3339Nano, text(record["timestamp"]))
			if err != nil {
				continue
			}
			delta := math.Abs(float64(recordTime.Sub(transaction.Start)) / float64(time.Millisecond))
			if delta <= toleranceMS && delta < bestDelta {
				bestIndex, bestDelta, bestBasis = index, delta, "method+path_template+status+time"
				bestClockCompared = true
			}
		}
		if bestIndex < 0 {
			continue
		}
		claimed[bestIndex] = true
		record := used[bestIndex]
		serverMS := number(record["response_time_ms"])
		outsideMS := transaction.Duration - serverMS
		confidence := "medium"
		if bestBasis == "request_id" {
			confidence = "high"
		}
		alignmentGrade := "duration_only"
		if bestClockCompared && bestDelta <= toleranceMS {
			alignmentGrade = "aligned"
		}
		match := map[string]any{
			"http_id": transaction.ID, "endpoint": transaction.Template,
			"access_uri": text(record["uri"]), "request_id": firstText(transaction.RequestID, text(record["request_id"])),
			"http_duration_ms": transaction.Duration, "server_response_ms": serverMS,
			"outside_server_ms": round(outsideMS, 3),
			"alignment_grade":   alignmentGrade, "confidence": confidence,
			"match_basis": bestBasis, "causal_claim_allowed": false,
			"clock_compared": bestClockCompared,
		}
		switch {
		case bestClockCompared:
			match["timestamp_delta_ms"] = round(bestDelta, 3)
			if alignmentGrade != "aligned" {
				match["timestamp_alignment_reason"] = "request identity matched, but the absolute timestamp delta exceeds the configured tolerance"
			}
		default:
			match["timestamp_delta_unavailable_reason"] = "request identity matched, but both observations did not provide parseable absolute timestamps"
		}
		matches = append(matches, match)
	}
	if len(matches) > 0 {
		diagnostic.Confidence = strongestConfidence(matches)
		if allRowsAligned(matches) {
			diagnostic.AlignmentGrade = "aligned"
			diagnostic.OverlayAllowed = true
			diagnostic.Reason = "every emitted access-log match compared compatible absolute timestamps within the configured tolerance"
		} else {
			diagnostic.AlignmentGrade = "duration_only"
			diagnostic.OverlayAllowed = false
			diagnostic.Reason = "request identity paired client and server observations, but at least one pair lacked compatible absolute timestamps"
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i]["confidence"] != matches[j]["confidence"] {
			return matches[i]["confidence"] == "high"
		}
		leftDelta, leftOK := optionalNumber(matches[i]["timestamp_delta_ms"])
		rightDelta, rightOK := optionalNumber(matches[j]["timestamp_delta_ms"])
		if leftOK != rightOK {
			return leftOK
		}
		return leftOK && leftDelta < rightDelta
	})
	diagnostic.CandidateRows = len(matches)
	diagnostic.OutputTruncated = len(matches) > topN
	return limitRows(matches, topN), diagnostic
}

func addJenniferFindings(result *models.AnalysisResult, rows []map[string]any, toleranceMS float64) {
	evidence := make([]map[string]any, 0, findingEvidenceLimit)
	for _, row := range rows {
		delta, ok := optionalNumber(row["network_gap_delta_ms"])
		if !ok {
			continue
		}
		gap := math.Abs(number(row["jennifer_network_gap_ms"]))
		if math.Abs(delta) > math.Max(toleranceMS, gap*0.5) {
			if len(evidence) < findingEvidenceLimit {
				evidence = append(evidence, row)
			}
		}
	}
	if len(evidence) > 0 {
		result.AddFinding("warning", "HTTP_JENNIFER_NETWORK_GAP_MISMATCH",
			"Jennifer network-gap estimates differ materially from known HTTP network phases; review observation points and coverage before drawing conclusions.",
			map[string]any{"matches": evidence, "causal_claim_allowed": false})
	}
}

func addAccessFindings(result *models.AnalysisResult, rows []map[string]any, toleranceMS float64) {
	evidence := make([]map[string]any, 0, findingEvidenceLimit)
	for _, row := range rows {
		if number(row["outside_server_ms"]) < -toleranceMS && len(evidence) < findingEvidenceLimit {
			evidence = append(evidence, row)
		}
	}
	if len(evidence) > 0 {
		result.AddFinding("warning", "HTTP_ACCESS_OBSERVATION_MISMATCH",
			"Server response time materially exceeds the matched client observation; verify clocks, request identity, and measurement semantics.",
			map[string]any{"matches": evidence, "causal_claim_allowed": false})
	}
}

func parseHTTPTransactions(rows []map[string]any) []httpTransaction {
	out := make([]httpTransaction, 0, len(rows))
	for _, row := range rows {
		start, startErr := time.Parse(time.RFC3339Nano, text(row["started_at"]))
		end, endErr := time.Parse(time.RFC3339Nano, text(row["ended_at"]))
		duration := number(row["duration_ms"])
		hasTime := startErr == nil
		if hasTime && (endErr != nil || !end.After(start)) && duration > 0 {
			end = start.Add(time.Duration(duration * float64(time.Millisecond)))
			endErr = nil
		}
		if hasTime && (endErr != nil || end.Before(start)) {
			hasTime = false
		}
		method := strings.ToUpper(firstText(text(row["method"]), "GET"))
		host := canonicalHost(firstText(text(row["host"]), text(row["url"])))
		path := firstText(text(row["path"]), text(row["url"]), "/")
		out = append(out, httpTransaction{
			ID: text(row["id"]), Method: method, Host: host, Path: path,
			Template: httpcapture.NormalizeCorrelationEndpoint(method, host, path),
			Status:   int(number(row["status"])), Duration: duration,
			Start: start, End: end, HasTime: hasTime,
			RequestID: requestID(asMap(row["request"])), Timings: asMap(row["timings"]),
		})
	}
	return out
}

func requestID(request map[string]any) string {
	for _, header := range valueRows(request["headers"]) {
		name := strings.ToLower(strings.TrimSpace(text(header["name"])))
		if name == "x-request-id" || name == "request-id" {
			if header["redacted"] == true {
				return ""
			}
			return strings.TrimSpace(text(header["value"]))
		}
	}
	return ""
}

func sameRequestShape(transaction httpTransaction, record map[string]any) bool {
	method := strings.ToUpper(firstText(text(record["method"]), "GET"))
	status := int(number(record["status"]))
	template := httpcapture.NormalizeCorrelationEndpoint(method, transaction.Host, text(record["uri"]))
	return method == transaction.Method && status == transaction.Status && template == transaction.Template
}

func observedNetworkMS(timings map[string]any) (float64, int) {
	var phases map[string]any
	for _, key := range []string{"proxyUpstream", "proxy_upstream", "importedHar", "imported_har", "clientProxy", "client_proxy"} {
		if candidate := asMap(timings[key]); candidate != nil {
			phases = candidate
			break
		}
	}
	total, count := 0.0, 0
	for _, key := range []string{"dns", "connect", "tls", "send", "receive"} {
		phase := asMap(phases[key])
		if strings.EqualFold(text(phase["state"]), "known") {
			total += number(phase["ms"])
			count++
		}
	}
	return total, count
}

func requireResultType(result map[string]any, expected, label string) error {
	got := text(result["type"])
	if got != expected {
		return fmt.Errorf("%s result type is %q; want %q", label, got, expected)
	}
	return nil
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

func normalizeTolerance(value float64) float64 {
	if value <= 0 {
		return DefaultToleranceMS
	}
	if value > MaxToleranceMS {
		return MaxToleranceMS
	}
	return value
}

func tableRows(result map[string]any, key string) []map[string]any {
	return valueRows(asMap(result["tables"])[key])
}

func valueRows(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if mapped := asMap(row); mapped != nil {
				out = append(out, mapped)
			}
		}
		return out
	default:
		return []map[string]any{}
	}
}

func boundedRows(rows []map[string]any, limit int) ([]map[string]any, bool) {
	if len(rows) <= limit {
		return rows, false
	}
	return rows[:limit], true
}

func limitRows(rows []map[string]any, limit int) []map[string]any {
	if len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func asMap(value any) map[string]any {
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}
	return nil
}

func nested(value map[string]any, keys ...string) any {
	var current any = value
	for _, key := range keys {
		current = asMap(current)[key]
	}
	return current
}

func text(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func number(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case uint64:
		return float64(typed)
	case uint:
		return float64(typed)
	case json.Number:
		parsed, _ := strconv.ParseFloat(string(typed), 64)
		return parsed
	default:
		return 0
	}
}

func optionalNumber(value any) (float64, bool) {
	var parsed float64
	switch typed := value.(type) {
	case float64:
		parsed = typed
	case float32:
		parsed = float64(typed)
	case int:
		parsed = float64(typed)
	case int64:
		parsed = float64(typed)
	case int32:
		parsed = float64(typed)
	case uint64:
		parsed = float64(typed)
	case uint:
		parsed = float64(typed)
	case json.Number:
		value, err := strconv.ParseFloat(string(typed), 64)
		if err != nil {
			return 0, false
		}
		parsed = value
	default:
		return 0, false
	}
	if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, false
	}
	return parsed, true
}

func microseconds(value any) (int64, bool) {
	parsed, ok := optionalNumber(value)
	if !ok || math.Trunc(parsed) != parsed ||
		parsed < float64(math.MinInt64) || parsed >= float64(math.MaxInt64) {
		return 0, false
	}
	return int64(parsed), true
}

func canonicalHost(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		value = parsed.Host
	}
	value = strings.TrimSuffix(strings.ToLower(value), ".")
	return value
}

func overlapDuration(aStart, aEnd, bStart, bEnd time.Time) float64 {
	start := aStart
	if bStart.After(start) {
		start = bStart
	}
	end := aEnd
	if bEnd.Before(end) {
		end = bEnd
	}
	if !end.After(start) {
		return 0
	}
	return float64(end.Sub(start)) / float64(time.Millisecond)
}

func hasTimedHTTP(rows []httpTransaction) bool {
	for _, row := range rows {
		if row.HasTime {
			return true
		}
	}
	return false
}

func httpAlignmentGrade(rows []httpTransaction) string {
	if hasTimedHTTP(rows) {
		return "aligned"
	}
	if len(rows) > 0 {
		return "duration_only"
	}
	return "none"
}

func confidenceForHTTP(rows []httpTransaction) string {
	if hasTimedHTTP(rows) {
		return "high"
	}
	if len(rows) > 0 {
		return "low"
	}
	return "none"
}

func strongestConfidence(rows []map[string]any) string {
	best := "none"
	rank := map[string]int{"none": 0, "low": 1, "medium": 2, "high": 3}
	for _, row := range rows {
		value := text(row["confidence"])
		if rank[value] > rank[best] {
			best = value
		}
	}
	return best
}

func allRowsAligned(rows []map[string]any) bool {
	if len(rows) == 0 {
		return false
	}
	for _, row := range rows {
		if text(row["alignment_grade"]) != "aligned" || row["clock_compared"] != true {
			return false
		}
	}
	return true
}

func countAlignment(rows []sourceDiagnostic, grade string) int {
	total := 0
	for _, row := range rows {
		if row.Source != "http_capture" && row.AlignmentGrade == grade {
			total++
		}
	}
	return total
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func round(value float64, places int) float64 {
	factor := math.Pow10(places)
	return math.Round(value*factor) / factor
}

func mergedSourceFiles(results ...map[string]any) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, result := range results {
		if result == nil {
			continue
		}
		values, _ := result["source_files"].([]any)
		for _, value := range values {
			path := text(value)
			if path != "" && !seen[path] {
				seen[path] = true
				out = append(out, path)
			}
		}
		if values, ok := result["source_files"].([]string); ok {
			for _, path := range values {
				if path != "" && !seen[path] {
					seen[path] = true
					out = append(out, path)
				}
			}
		}
	}
	return out
}

func provenanceRows(results ...map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, result := range results {
		if result == nil {
			continue
		}
		out = append(out, map[string]any{
			"result_type":    text(result["type"]),
			"created_at":     text(result["created_at"]),
			"schema_version": text(nested(result, "metadata", "schema_version")),
			"source_files":   result["source_files"],
		})
	}
	return out
}
