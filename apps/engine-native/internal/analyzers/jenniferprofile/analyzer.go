// Package jenniferprofile is the MVP1 analyzer that consumes
// JenniferTransactionProfile records emitted by
// internal/parsers/jenniferprofile and produces an AnalysisResult
// envelope: per-profile body metrics, header-vs-body validation,
// and aggregated counts.
//
// MSA grouping (GUID-level call matching, network gap, signature
// stats, parallelism) is layered on top in MVP2-MVP4.
package jenniferprofile

import (
	"fmt"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/jenniferprofile"
)

const (
	ResultType    = "jennifer_profile"
	ParserName    = "jennifer_profile_export"
	SchemaVersion = "0.1.0"

	// HeaderBodyToleranceMs is the per-§12.2 default 1ms drift we
	// allow before promoting a mismatch to a warning.
	HeaderBodyToleranceMs = 1
)

// Options is the analyzer-level knob set.
type Options struct {
	// FallbackCorrelationToTxid mirrors the parser-side flag — kept
	// here for symmetry and forwarding.
	FallbackCorrelationToTxid bool
	// HeaderBodyToleranceMs overrides the default 1ms tolerance on
	// header vs body cumulative checks.
	HeaderBodyToleranceMs int
	// NetworkPrepPatterns are forwarded to the parser's classifier;
	// METHOD lines whose message contains any of these substrings
	// (case-insensitive) become NETWORK_PREP_METHOD events. Empty
	// means "use built-in defaults" (IntegrationUtil.sendToService).
	NetworkPrepPatterns []string
	// EventCategoryPatterns extends the event classifier. Keys are
	// JenniferEventType values; values are case-insensitive substrings.
	// User patterns are applied to METHOD/UNKNOWN events only — they
	// can't override well-known matches like EXTERNAL_CALL or FETCH.
	EventCategoryPatterns map[string][]string
}

// parserOpts projects the analyzer-level options onto the parser's
// option struct so classifier customization (network-prep patterns,
// extra event-category rules) flows end-to-end.
func (o Options) parserOpts() jenniferprofile.Options {
	return jenniferprofile.Options{
		FallbackCorrelationToTxid: o.FallbackCorrelationToTxid,
		NetworkPrepPatterns:       o.NetworkPrepPatterns,
		EventCategoryPatterns:     o.EventCategoryPatterns,
	}
}

// AnalyzeFile parses + analyses a single Jennifer profile export.
func AnalyzeFile(path string, opts Options) (models.AnalysisResult, error) {
	parsed, err := jenniferprofile.ParseFile(path, opts.parserOpts())
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return Build([]jenniferprofile.FileResult{parsed}, opts), nil
}

// AnalyzeFiles parses + analyses multiple files, joining their
// profiles into a single AnalysisResult.
func AnalyzeFiles(paths []string, opts Options) (models.AnalysisResult, error) {
	results := make([]jenniferprofile.FileResult, 0, len(paths))
	for _, p := range paths {
		parsed, err := jenniferprofile.ParseFile(p, opts.parserOpts())
		if err != nil {
			return models.AnalysisResult{}, fmt.Errorf("%s: %w", p, err)
		}
		results = append(results, parsed)
	}
	return Build(results, opts), nil
}

// Build assembles the AnalysisResult from already-parsed file
// results — kept separate from AnalyzeFile so tests / future
// streaming callers can feed records from elsewhere.
func Build(files []jenniferprofile.FileResult, opts Options) models.AnalysisResult {
	tolerance := opts.HeaderBodyToleranceMs
	if tolerance <= 0 {
		tolerance = HeaderBodyToleranceMs
	}

	totalProfiles := 0
	totalErrors := 0
	totalWarnings := 0
	fullProfileCount := 0
	rowsBySource := []map[string]any{}
	profileRows := []map[string]any{}
	fileErrors := []map[string]any{}

	type aggregate struct {
		BodyMetrics     models.JenniferBodyMetrics
		ResponseTimeMs  int64
		ResponseTimeN   int
		ProfileCount    int
		FullProfileCount int
	}
	totals := aggregate{}

	sources := []string{}
	// fileBuckets passes profiles down to the MSA pipeline once
	// validation has already been applied — this keeps the MSA
	// matcher seeing the warning-tagged profiles and avoids a
	// second walk over every event.
	fileBuckets := make([]jenniferFileBucket, 0, len(files))
	for _, file := range files {
		if file.SourceFile != "" {
			sources = append(sources, file.SourceFile)
		}
		rowsBySource = append(rowsBySource, map[string]any{
			"source_file":                 file.SourceFile,
			"declared_transaction_count":  file.DeclaredTransactionCount,
			"detected_transaction_count":  file.DetectedTransactionCount,
			"profile_count":               len(file.Profiles),
		})
		for _, fe := range file.FileErrors {
			fileErrors = append(fileErrors, map[string]any{
				"source_file": file.SourceFile,
				"code":        fe.Code,
				"message":     fe.Message,
			})
		}
		mutated := make([]models.JenniferTransactionProfile, 0, len(file.Profiles))
		for _, p := range file.Profiles {
			totalProfiles++
			metrics := AggregateBody(&p)
			validateHeaderVsBody(&p, metrics, tolerance)

			isFull := len(p.Errors) == 0
			if isFull {
				fullProfileCount++
				totals.FullProfileCount++
			}
			if p.Header.ResponseTimeMs != nil {
				totals.ResponseTimeMs += int64(*p.Header.ResponseTimeMs)
				totals.ResponseTimeN++
			}
			totals.BodyMetrics.SQLExecuteCumMs += metrics.SQLExecuteCumMs
			totals.BodyMetrics.SQLExecuteCount += metrics.SQLExecuteCount
			totals.BodyMetrics.CheckQueryCumMs += metrics.CheckQueryCumMs
			totals.BodyMetrics.CheckQueryCount += metrics.CheckQueryCount
			totals.BodyMetrics.TwoPCCumMs += metrics.TwoPCCumMs
			totals.BodyMetrics.TwoPCCount += metrics.TwoPCCount
			totals.BodyMetrics.FetchCumMs += metrics.FetchCumMs
			totals.BodyMetrics.FetchCount += metrics.FetchCount
			totals.BodyMetrics.FetchTotalRows += metrics.FetchTotalRows
			totals.BodyMetrics.ExternalCallCumMs += metrics.ExternalCallCumMs
			totals.BodyMetrics.ExternalCallCount += metrics.ExternalCallCount
			totals.BodyMetrics.ConnectionAcquireCumMs += metrics.ConnectionAcquireCumMs
			totals.BodyMetrics.ConnectionAcquireCount += metrics.ConnectionAcquireCount
			totals.BodyMetrics.NetworkPrepMethodCumMs += metrics.NetworkPrepMethodCumMs
			totals.BodyMetrics.NetworkPrepMethodCount += metrics.NetworkPrepMethodCount
			totals.BodyMetrics.NetworkPrepCumMs += metrics.NetworkPrepCumMs

			totalErrors += len(p.Errors)
			totalWarnings += len(p.Warnings)

			profileRows = append(profileRows, profileToRow(p, metrics))
			mutated = append(mutated, p)
		}
		fileBuckets = append(fileBuckets, jenniferFileBucket{
			sourceFile: file.SourceFile,
			profiles:   mutated,
		})
	}

	// MVP2 — group profiles by GUID, run external-call matching +
	// network-gap + root + call-graph for each group.
	// MVP3 — fold same-shape MSA structures into Timeline Signatures
	// and emit per-signature distribution stats.
	guidGroups := buildGuidGroups(fileBuckets, opts)
	guidGroupRows := make([]map[string]any, 0, len(guidGroups))
	allEdgeRows := []map[string]any{}
	unmatchedRows := []map[string]any{}
	totalEdges := 0
	totalMatched := 0
	totalUnmatched := 0
	totalGapMs := 0
	for i := range guidGroups {
		// Stamp each group's signature so guid_group rows can
		// reference the parent signature hash.
		guidGroups[i].Metrics.GUID = guidGroups[i].GUID
	}
	signatureStats := aggregateSignatureStats(guidGroups)
	signatureRows := make([]map[string]any, 0, len(signatureStats))
	for _, s := range signatureStats {
		signatureRows = append(signatureRows, signatureStatsToRow(s))
	}
	signatureByHash := map[string]models.JenniferTimelineSignature{}
	for _, s := range signatureStats {
		signatureByHash[s.Signature.Hash] = s.Signature
	}
	for _, g := range guidGroups {
		// Recompute the per-group signature so the guid_groups
		// row can carry its hash. Cheap (≈O(edges)).
		sig := computeTimelineSignature(g)
		guidGroupRows = append(guidGroupRows, guidGroupToRowWithSignature(g, sig))
		totalEdges += len(g.Edges)
		totalMatched += g.MatchedEdgeCount
		totalUnmatched += g.UnmatchedEdgeCount
		for _, e := range g.Edges {
			allEdgeRows = append(allEdgeRows, edgeToRow(e))
			if e.MatchStatus != models.JenniferMatchOK {
				unmatchedRows = append(unmatchedRows, edgeToRow(e))
			}
			if e.AdjustedNetworkGapMs != nil {
				totalGapMs += *e.AdjustedNetworkGapMs
			}
		}
	}

	result := models.New(ResultType, ParserName)
	result.SourceFiles = sources
	result.Summary = map[string]any{
		"total_files":         len(files),
		"total_profiles":      totalProfiles,
		"full_profile_count":  fullProfileCount,
		"total_errors":        totalErrors,
		"total_warnings":      totalWarnings,
		"avg_response_time_ms": avgInt(totals.ResponseTimeMs, totals.ResponseTimeN),
		"sql_execute_cum_ms":  totals.BodyMetrics.SQLExecuteCumMs,
		"check_query_cum_ms":  totals.BodyMetrics.CheckQueryCumMs,
		"two_pc_cum_ms":       totals.BodyMetrics.TwoPCCumMs,
		"fetch_cum_ms":        totals.BodyMetrics.FetchCumMs,
		"fetch_total_rows":    totals.BodyMetrics.FetchTotalRows,
		"external_call_cum_ms": totals.BodyMetrics.ExternalCallCumMs,
		"external_call_count":  totals.BodyMetrics.ExternalCallCount,
		"network_prep_cum_ms":         totals.BodyMetrics.NetworkPrepCumMs,
		"network_prep_method_cum_ms":  totals.BodyMetrics.NetworkPrepMethodCumMs,
		"network_prep_method_count":   totals.BodyMetrics.NetworkPrepMethodCount,
		"connection_acquire_cum_ms": totals.BodyMetrics.ConnectionAcquireCumMs,
		// MVP2 MSA roll-up.
		"guid_group_count":             len(guidGroups),
		"matched_external_call_count":  totalMatched,
		"unmatched_external_call_count": totalUnmatched,
		"total_network_gap_cum_ms":     totalGapMs,
		// MVP3 signature aggregation.
		"unique_signature_count": len(signatureStats),
	}
	result.Series = map[string]any{
		"file_summary":          rowsBySource,
		"guid_groups":           guidGroupRows,
		"signature_statistics":  signatureRows,
	}
	result.Tables = map[string]any{
		"profiles":                   profileRows,
		"file_errors":                fileErrors,
		"msa_edges":                  allEdgeRows,
		"unmatched_external_calls":   unmatchedRows,
	}
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = diagnostics.New(ParserName)
	return result
}

// guidGroupToRowWithSignature is the MVP3 variant — same layout as
// guidGroupToRow but stamped with the group's parent signature
// hash so the renderer can link rows back to the
// `series.signature_statistics` table.
func guidGroupToRowWithSignature(g models.JenniferGuidGroup, sig models.JenniferTimelineSignature) map[string]any {
	row := guidGroupToRow(g)
	row["signature_hash"] = sig.Hash
	row["signature_version"] = sig.Version
	return row
}

// signatureStatsToRow projects a JenniferSignatureStats into the
// `series.signature_statistics` row shape.
func signatureStatsToRow(s models.JenniferSignatureStats) map[string]any {
	metrics := map[string]any{}
	for k, v := range s.Metrics {
		metrics[k] = metricStatsToMap(v)
	}
	edges := make([]map[string]any, 0, len(s.Edges))
	for _, e := range s.Edges {
		edges = append(edges, map[string]any{
			"caller_application":         e.CallerApplication,
			"callee_application":         e.CalleeApplication,
			"occurrence_index":           e.OccurrenceIndex,
			"external_call_elapsed_ms":   metricStatsToMap(e.ExternalCallElapsedStats),
			"callee_response_time_ms":    metricStatsToMap(e.CalleeResponseTimeStats),
			"raw_network_gap_ms":         metricStatsToMap(e.RawNetworkGapStats),
			"adjusted_network_gap_ms":    metricStatsToMap(e.AdjustedNetworkGapStats),
		})
	}
	return map[string]any{
		"signature_hash":      s.Signature.Hash,
		"signature_version":   s.Signature.Version,
		"canonical_signature": s.Signature.Canonical,
		"root_application":    s.Signature.RootApplication,
		"edge_count":          s.Signature.EdgeCount,
		"sample_count":        s.SampleCount,
		"guids":               s.GUIDs,
		"metrics":             metrics,
		"edges":               edges,
	}
}

// metricStatsToMap renders a JenniferMetricStats as a JSON-friendly
// map. We expose all 9 fields (count + 8 distribution numbers) so
// the renderer can pick the columns it wants.
func metricStatsToMap(m models.JenniferMetricStats) map[string]any {
	return map[string]any{
		"count":  m.Count,
		"min":    m.Min,
		"avg":    m.Avg,
		"p50":    m.P50,
		"p90":    m.P90,
		"p95":    m.P95,
		"p99":    m.P99,
		"max":    m.Max,
		"stddev": m.Stddev,
	}
}

// guidGroupToRow projects a JenniferGuidGroup into the
// `series.guid_groups` row shape consumed by the renderer.
func guidGroupToRow(g models.JenniferGuidGroup) map[string]any {
	rootRT := any(nil)
	if g.RootResponseTimeMs != nil {
		rootRT = *g.RootResponseTimeMs
	}
	graph := make([]map[string]any, 0, len(g.CallGraph))
	for _, edge := range g.CallGraph {
		graph = append(graph, map[string]any{
			"caller_txid":               edge.CallerTXID,
			"caller_application":        edge.CallerApplication,
			"callee_txid":               edge.CalleeTXID,
			"callee_application":        edge.CalleeApplication,
			"external_call_elapsed_ms":  edge.ExternalCallElapsedMs,
			"callee_response_time_ms":   edge.CalleeResponseTimeMs,
			"network_gap_ms":            edge.NetworkGapMs,
			"caller_event_start_ms":     edge.CallerEventStartMs,
			"callee_body_start_ms":      edge.CalleeBodyStartMs,
		})
	}
	return map[string]any{
		"guid":                  g.GUID,
		"profile_count":         g.ProfileCount,
		"profile_txids":         g.ProfileTXIDs,
		"root_txid":             g.RootTXID,
		"root_application":      g.RootApplication,
		"root_response_time_ms": rootRT,
		"matched_edge_count":    g.MatchedEdgeCount,
		"unmatched_edge_count":  g.UnmatchedEdgeCount,
		"validation_status":     g.ValidationStatus,
		"call_graph":            graph,
		"metrics": map[string]any{
			"profile_count":                    g.Metrics.ProfileCount,
			"matched_external_call_count":      g.Metrics.MatchedExternalCallCount,
			"unmatched_external_call_count":    g.Metrics.UnmatchedExternalCallCount,
			"total_external_call_cumulative_ms": g.Metrics.TotalExternalCallCumulativeMs,
			"total_external_call_wall_time_ms": g.Metrics.TotalExternalCallWallTimeMs,
			"total_network_gap_cumulative_ms":  g.Metrics.TotalNetworkGapCumulativeMs,
			"total_sql_execute_ms":             g.Metrics.TotalSqlExecuteMs,
			"total_check_query_ms":             g.Metrics.TotalCheckQueryMs,
			"total_two_pc_ms":                  g.Metrics.TotalTwoPcMs,
			"total_fetch_ms":                   g.Metrics.TotalFetchMs,
			"total_fetch_rows":                 g.Metrics.TotalFetchRows,
			"total_connection_acquire_ms":      g.Metrics.TotalConnectionAcquireMs,
			"max_external_call_concurrency":    g.Metrics.MaxExternalCallConcurrency,
			"external_call_parallelism_ratio":  g.Metrics.ExternalCallParallelismRatio,
			"group_execution_mode":             string(g.Metrics.GroupExecutionMode),
		},
	}
}

// edgeToRow projects an edge into a flat row for `tables.msa_edges`.
func edgeToRow(e models.JenniferExternalCallEdge) map[string]any {
	row := map[string]any{
		"guid":                     e.GUID,
		"caller_txid":              e.CallerTXID,
		"caller_application":       e.CallerApplication,
		"external_call_sequence":   e.ExternalCallSequence,
		"external_call_url":        e.ExternalCallURL,
		"external_call_elapsed_ms": e.ExternalCallElapsedMs,
		"match_status":             string(e.MatchStatus),
		"caller_event_start_ms":    e.CallerEventStartMs,
		"callee_body_start_ms":     e.CalleeBodyStartMs,
	}
	if e.CalleeTXID != "" {
		row["callee_txid"] = e.CalleeTXID
		row["callee_application"] = e.CalleeApplication
	}
	if e.CalleeResponseTimeMs != nil {
		row["callee_response_time_ms"] = *e.CalleeResponseTimeMs
	}
	if e.RawNetworkGapMs != nil {
		row["raw_network_gap_ms"] = *e.RawNetworkGapMs
	}
	if e.AdjustedNetworkGapMs != nil {
		row["adjusted_network_gap_ms"] = *e.AdjustedNetworkGapMs
	}
	if e.MatchScore > 0 {
		row["match_score"] = e.MatchScore
	}
	if len(e.Warnings) > 0 {
		row["warnings"] = e.Warnings
	}
	return row
}

// avgInt returns nil-friendly average so 0/0 becomes JSON null.
func avgInt(total int64, n int) any {
	if n <= 0 {
		return nil
	}
	return float64(total) / float64(n)
}

// profileToRow projects a parsed profile + body metrics into the
// `tables.profiles` row shape consumed by the renderer.
func profileToRow(p models.JenniferTransactionProfile, m models.JenniferBodyMetrics) map[string]any {
	errors := make([]map[string]any, 0, len(p.Errors))
	for _, e := range p.Errors {
		errors = append(errors, map[string]any{"code": e.Code, "message": e.Message})
	}
	warnings := make([]map[string]any, 0, len(p.Warnings))
	for _, w := range p.Warnings {
		warnings = append(warnings, map[string]any{"code": w.Code, "message": w.Message})
	}
	row := map[string]any{
		"txid":              p.Header.TXID,
		"guid":              p.Header.GUID,
		"application":       p.Header.Application,
		"start_time":        p.Header.StartTime,
		"is_full_profile":   len(p.Errors) == 0,
		"errors":            errors,
		"warnings":          warnings,
		"body_metrics": map[string]any{
			"sql_execute_cum_ms":        m.SQLExecuteCumMs,
			"sql_execute_count":         m.SQLExecuteCount,
			"check_query_cum_ms":        m.CheckQueryCumMs,
			"check_query_count":         m.CheckQueryCount,
			"two_pc_cum_ms":             m.TwoPCCumMs,
			"two_pc_count":              m.TwoPCCount,
			"fetch_cum_ms":              m.FetchCumMs,
			"fetch_count":               m.FetchCount,
			"fetch_total_rows":          m.FetchTotalRows,
			"external_call_cum_ms":      m.ExternalCallCumMs,
			"external_call_count":       m.ExternalCallCount,
			"network_prep_method_cum_ms": m.NetworkPrepMethodCumMs,
			"network_prep_method_count":  m.NetworkPrepMethodCount,
			"network_prep_cum_ms":        m.NetworkPrepCumMs,
			"connection_acquire_cum_ms": m.ConnectionAcquireCumMs,
			"connection_acquire_count":  m.ConnectionAcquireCount,
		},
		"header": map[string]any{
			"response_time_ms":      ptrIntOrNil(p.Header.ResponseTimeMs),
			"sql_time_ms":           ptrIntOrNil(p.Header.SQLTimeMs),
			"sql_count":             ptrIntOrNil(p.Header.SQLCount),
			"external_call_time_ms": ptrIntOrNil(p.Header.ExternalCallMs),
			"fetch_time_ms":         ptrIntOrNil(p.Header.FetchTimeMs),
			"cpu_time_ms":           ptrIntOrNil(p.Header.CPUTimeMs),
			"http_status_code":      ptrIntOrNil(p.Header.HTTPStatusCode),
			"domain":                p.Header.Domain,
			"instance":              p.Header.Instance,
		},
	}
	return row
}

func ptrIntOrNil(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
