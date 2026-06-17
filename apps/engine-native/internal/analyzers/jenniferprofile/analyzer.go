// Package jenniferprofile is the MVP1 analyzer that consumes
// JenniferTransactionProfile records emitted by
// internal/parsers/jenniferprofile and produces an AnalysisResult
// envelope: per-profile body metrics, header-vs-body validation,
// and aggregated counts.
//
// MSA grouping (GUID-level call matching, network gap, signature
// stats, parallelism) is layered on top in MVP2-MVP4.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] jenniferprofile 분석기 — Jennifer APM Profile Export 의
// 트랜잭션 프로파일 분석기. 8개 .go 파일로 분리:
//
//   - analyzer.go         : 진입점 + Build (이 파일).
//   - aggregator.go       : 본문 이벤트 → 비용 ledger 합산.
//   - msa.go              : §13-§18 GUID 그룹핑 파이프라인.
//   - msa_match.go        : caller-callee 매칭 알고리즘 + 점수.
//   - msa_parallelism.go  : §16.7 외부호출 병렬도 분석.
//   - msa_stats.go        : §19-§21 timeline signature 분포 통계.
//   - html_report.go      : 자기 충족 HTML 보고서 렌더러.
//
// MVP 단계
//
//	MVP1 : per-profile metrics + header/body validation (이 파일).
//	MVP2 : GUID 단위 MSA 그룹핑 + EXTERNAL_CALL caller↔callee 매칭 +
//	       NETWORK_GAP 계산.
//	MVP3 : Timeline Signature 통계(같은 호출 구조의 분포).
//	MVP4 : 외부호출 병렬도(parallelism) + HTML 보고서.
//
// 알고리즘 흐름 (Build)
//  1. 각 FileResult 의 모든 profile 을 순회하면서:
//     • AggregateBody : SQL/FETCH/2PC/EXTERNAL_CALL/CONNECTION_ACQUIRE
//     누적 합 산출.
//     • header vs body 검증 : header 의 사전 집계 메트릭과 body
//     합이 tolerance(기본 1ms) 안에 들어오는지. 벗어나면 warning.
//     • profileRows + rowsBySource 누적.
//  2. buildGuidGroups (msa.go) : 같은 correlation key (GUID 또는
//     TXID 폴백) 를 가진 profile 을 묶고, caller-callee 매칭/network
//     gap 계산.
//  3. 결과 envelope 채움 — summary(총수/오류/경고), tables(profile/
//     group), series(필요 시), metadata(diagnostics, errors, warnings).
//
// parity 주의
//   - header_body_tolerance_ms 의 0 입력은 기본값(1ms) 으로 fallback —
//     Python 의 None 처리와 동치.
//   - orderedCounter / sort 정렬 키가 Python 과 byte 동일.
package jenniferprofile

import (
	"fmt"
	"sort"
	"strings"

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
	// CustomAnalysisRules define user-visible roll-up cards. They can
	// match whole profile URLs, method/event names, or EXTERNAL_CALL URLs.
	CustomAnalysisRules []models.JenniferCustomAnalysisRule
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
	incompleteProfiles := 0
	fullProfileCount := 0
	rowsBySource := []map[string]any{}
	profileRows := []map[string]any{}
	fileErrors := []map[string]any{}
	networkPrepRows := []map[string]any{}
	slowSQLRows := []map[string]any{}
	methodHotspotRows := []map[string]any{}
	// hotspotsByTXID feeds the per-GUID-group roll-up below so the MSA
	// timeline can show the slowest pure methods across the whole group.
	hotspotsByTXID := map[string][]models.JenniferMethodHotspot{}

	type aggregate struct {
		BodyMetrics      models.JenniferBodyMetrics
		ResponseTimeMs   int64
		ResponseTimeN    int
		ProfileCount     int
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
		fileIncompleteProfiles := 0
		for _, p := range file.Profiles {
			if p.Body.CapacityExceeded {
				fileIncompleteProfiles++
			}
		}
		rowsBySource = append(rowsBySource, map[string]any{
			"source_file":                file.SourceFile,
			"declared_transaction_count": file.DeclaredTransactionCount,
			"detected_transaction_count": file.DetectedTransactionCount,
			"profile_count":              len(file.Profiles),
			"incomplete_profile_count":   fileIncompleteProfiles,
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

			if p.Body.CapacityExceeded {
				incompleteProfiles++
			}
			isFull := len(p.Errors) == 0 && !p.Body.CapacityExceeded
			if isFull {
				fullProfileCount++
				totals.FullProfileCount++
			}
			if p.Header.ResponseTimeMs != nil && !p.Body.CapacityExceeded {
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

			hotspots := MethodHotspotsWithCustomRules(p, DefaultMethodHotspotLimit, opts.CustomAnalysisRules)
			p.MethodHotspots = hotspots
			if len(hotspots) > 0 {
				hotspotsByTXID[p.Header.TXID] = hotspots
				methodHotspotRows = append(methodHotspotRows, methodHotspotsToRows(p, hotspots)...)
			}

			profileRows = append(profileRows, profileToRow(p, metrics))
			networkPrepRows = append(networkPrepRows, networkPrepMethodsToRows(p, metrics)...)
			slowSQLRows = append(slowSQLRows, slowSQLEventsToRows(p)...)
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
	unprofiledGroupRows := []map[string]any{}
	totalEdges := 0
	totalMatched := 0
	totalUnmatched := 0
	totalUnprofiledMs := 0
	totalGapMs := 0
	signatureExcludedGroups := 0
	for i := range guidGroups {
		// Stamp each group's signature so guid_group rows can
		// reference the parent signature hash.
		guidGroups[i].Metrics.GUID = guidGroups[i].GUID
		if guidGroups[i].ExcludedFromSignatureStats {
			signatureExcludedGroups++
		}
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
		groupRow := guidGroupToRowWithSignature(g, sig)
		groupRow["method_hotspots"] = groupMethodHotspotRows(g, hotspotsByTXID)
		guidGroupRows = append(guidGroupRows, groupRow)
		totalEdges += len(g.Edges)
		totalMatched += g.MatchedEdgeCount
		totalUnmatched += g.UnmatchedEdgeCount
		totalUnprofiledMs += g.Metrics.TotalUnprofiledExternalCallMs
		for _, group := range g.UnprofiledExternalCallGroups {
			unprofiledGroupRows = append(unprofiledGroupRows, unprofiledExternalCallGroupToRow(group))
		}
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
		"total_files":                len(files),
		"total_profiles":             totalProfiles,
		"incomplete_profile_count":   incompleteProfiles,
		"full_profile_count":         fullProfileCount,
		"total_errors":               totalErrors,
		"total_warnings":             totalWarnings,
		"avg_response_time_ms":       avgInt(totals.ResponseTimeMs, totals.ResponseTimeN),
		"sql_execute_cum_ms":         totals.BodyMetrics.SQLExecuteCumMs,
		"check_query_cum_ms":         totals.BodyMetrics.CheckQueryCumMs,
		"two_pc_cum_ms":              totals.BodyMetrics.TwoPCCumMs,
		"fetch_cum_ms":               totals.BodyMetrics.FetchCumMs,
		"fetch_total_rows":           totals.BodyMetrics.FetchTotalRows,
		"external_call_cum_ms":       totals.BodyMetrics.ExternalCallCumMs,
		"external_call_count":        totals.BodyMetrics.ExternalCallCount,
		"network_prep_cum_ms":        totals.BodyMetrics.NetworkPrepCumMs,
		"network_prep_method_cum_ms": totals.BodyMetrics.NetworkPrepMethodCumMs,
		"network_prep_method_count":  totals.BodyMetrics.NetworkPrepMethodCount,
		"connection_acquire_cum_ms":  totals.BodyMetrics.ConnectionAcquireCumMs,
		// MVP2 MSA roll-up.
		"guid_group_count":                  len(guidGroups),
		"matched_external_call_count":       totalMatched,
		"unmatched_external_call_count":     totalUnmatched,
		"total_unprofiled_external_call_ms": totalUnprofiledMs,
		"total_network_gap_cum_ms":          totalGapMs,
		// MVP3 signature aggregation.
		"unique_signature_count":           len(signatureStats),
		"signature_excluded_group_count":   signatureExcludedGroups,
		"signature_excluded_profile_count": incompleteProfiles,
	}
	result.Series = map[string]any{
		"file_summary":                 rowsBySource,
		"guid_groups":                  guidGroupRows,
		"signature_statistics":         signatureRows,
		"service_call_network_summary": serviceCallNetworkSummaryRows(guidGroups),
	}
	result.Tables = map[string]any{
		"profiles":                        profileRows,
		"file_errors":                     fileErrors,
		"msa_edges":                       allEdgeRows,
		"unmatched_external_calls":        unmatchedRows,
		"unprofiled_external_call_groups": unprofiledGroupRows,
		"network_prep_methods":            networkPrepRows,
		"slow_sql_events":                 slowSQLRows,
		"method_hotspots":                 methodHotspotRows,
		"custom_rule_stats":               customRuleStats(fileBuckets, opts.CustomAnalysisRules),
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
			"caller_application":       e.CallerApplication,
			"callee_application":       e.CalleeApplication,
			"occurrence_index":         e.OccurrenceIndex,
			"external_call_elapsed_ms": metricStatsToMap(e.ExternalCallElapsedStats),
			"callee_response_time_ms":  metricStatsToMap(e.CalleeResponseTimeStats),
			"raw_network_gap_ms":       metricStatsToMap(e.RawNetworkGapStats),
			"adjusted_network_gap_ms":  metricStatsToMap(e.AdjustedNetworkGapStats),
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

type networkTimeGroup struct {
	Key   string
	Label string
	Index int
}

// classifyNetworkTimeGroup maps the observed network gap into a stable
// latency band. The thresholds intentionally separate the single-digit
// internal-call group from the double-digit gateway/external-call group.
func classifyNetworkTimeGroup(ms float64) networkTimeGroup {
	switch {
	case ms < 5:
		return networkTimeGroup{Key: "near_0_4_ms", Label: "0-4 ms", Index: 0}
	case ms < 10:
		return networkTimeGroup{Key: "internal_5_9_ms", Label: "5-9 ms", Index: 1}
	case ms < 20:
		return networkTimeGroup{Key: "cross_service_10_19_ms", Label: "10-19 ms", Index: 2}
	case ms < 50:
		return networkTimeGroup{Key: "gateway_external_20_49_ms", Label: "20-49 ms", Index: 3}
	case ms < 100:
		return networkTimeGroup{Key: "remote_50_99_ms", Label: "50-99 ms", Index: 4}
	default:
		return networkTimeGroup{Key: "slow_remote_ge_100_ms", Label: ">=100 ms", Index: 5}
	}
}

func serviceCallNetworkSummaryRows(groups []models.JenniferGuidGroup) []map[string]any {
	type bucket struct {
		caller    string
		callee    string
		count     int
		elapsed   []float64
		calleeRT  []float64
		network   []float64
		guidSet   map[string]bool
		guidOrder []string
	}

	buckets := map[string]*bucket{}
	order := []string{}
	for _, group := range groups {
		for _, edge := range group.Edges {
			if edge.MatchStatus != models.JenniferMatchOK {
				continue
			}
			key := edge.CallerApplication + "->" + edge.CalleeApplication
			b, ok := buckets[key]
			if !ok {
				b = &bucket{
					caller:  edge.CallerApplication,
					callee:  edge.CalleeApplication,
					guidSet: map[string]bool{},
				}
				buckets[key] = b
				order = append(order, key)
			}
			b.count++
			b.elapsed = append(b.elapsed, float64(edge.ExternalCallElapsedMs))
			if edge.CalleeResponseTimeMs != nil {
				b.calleeRT = append(b.calleeRT, float64(*edge.CalleeResponseTimeMs))
			}
			if edge.AdjustedNetworkGapMs != nil {
				b.network = append(b.network, float64(*edge.AdjustedNetworkGapMs))
			}
			if group.GUID != "" && !b.guidSet[group.GUID] {
				b.guidSet[group.GUID] = true
				b.guidOrder = append(b.guidOrder, group.GUID)
			}
		}
	}

	rows := make([]map[string]any, 0, len(order))
	for _, key := range order {
		b := buckets[key]
		elapsedStats := computeMetricStats(b.elapsed)
		calleeStats := computeMetricStats(b.calleeRT)
		networkStats := computeMetricStats(b.network)
		group := classifyNetworkTimeGroup(networkStats.Avg)
		rows = append(rows, map[string]any{
			"caller_application":           b.caller,
			"callee_application":           b.callee,
			"call_count":                   b.count,
			"guid_count":                   len(b.guidOrder),
			"sample_guids":                 b.guidOrder,
			"external_call_elapsed_ms":     metricStatsToMap(elapsedStats),
			"callee_response_time_ms":      metricStatsToMap(calleeStats),
			"network_gap_ms":               metricStatsToMap(networkStats),
			"network_time_group":           group.Key,
			"network_time_group_label":     group.Label,
			"network_time_group_index":     group.Index,
			"network_time_group_basis":     "avg_network_gap_ms",
			"avg_network_gap_ms":           networkStats.Avg,
			"p95_network_gap_ms":           networkStats.P95,
			"max_network_gap_ms":           networkStats.Max,
			"total_network_gap_ms":         sumFloat64(b.network),
			"avg_external_call_elapsed_ms": elapsedStats.Avg,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ai := rows[i]["network_time_group_index"].(int)
		aj := rows[j]["network_time_group_index"].(int)
		if ai != aj {
			return ai > aj
		}
		ati := rows[i]["total_network_gap_ms"].(float64)
		atj := rows[j]["total_network_gap_ms"].(float64)
		if ati != atj {
			return ati > atj
		}
		ci := rows[i]["caller_application"].(string)
		cj := rows[j]["caller_application"].(string)
		if ci != cj {
			return ci < cj
		}
		return rows[i]["callee_application"].(string) < rows[j]["callee_application"].(string)
	})
	return rows
}

func sumFloat64(values []float64) float64 {
	var total float64
	for _, v := range values {
		total += v
	}
	return total
}

// guidGroupToRow projects a JenniferGuidGroup into the
// `series.guid_groups` row shape consumed by the renderer.
func guidGroupToRow(g models.JenniferGuidGroup) map[string]any {
	rootRT := any(nil)
	if g.RootResponseTimeMs != nil {
		rootRT = *g.RootResponseTimeMs
	}
	rootBodyStartMs := any(nil)
	if g.RootBodyStartMs != nil {
		rootBodyStartMs = *g.RootBodyStartMs
	}
	graph := make([]map[string]any, 0, len(g.CallGraph))
	for _, edge := range g.CallGraph {
		graph = append(graph, map[string]any{
			"caller_txid":              edge.CallerTXID,
			"caller_application":       edge.CallerApplication,
			"callee_txid":              edge.CalleeTXID,
			"callee_application":       edge.CalleeApplication,
			"external_call_elapsed_ms": edge.ExternalCallElapsedMs,
			"callee_response_time_ms":  edge.CalleeResponseTimeMs,
			"network_gap_ms":           edge.NetworkGapMs,
			"caller_event_start_ms":    edge.CallerEventStartMs,
			"callee_body_start_ms":     edge.CalleeBodyStartMs,
		})
	}
	return map[string]any{
		"guid":                          g.GUID,
		"profile_count":                 g.ProfileCount,
		"incomplete_profile_count":      g.IncompleteProfileCount,
		"excluded_from_signature_stats": g.ExcludedFromSignatureStats,
		"profile_txids":                 g.ProfileTXIDs,
		"root_txid":                     g.RootTXID,
		"root_application":              g.RootApplication,
		"root_response_time_ms":         rootRT,
		"root_body_start_ms":            rootBodyStartMs,
		"matched_edge_count":            g.MatchedEdgeCount,
		"unmatched_edge_count":          g.UnmatchedEdgeCount,
		"validation_status":             g.ValidationStatus,
		"warnings":                      g.Warnings,
		"call_graph":                    graph,
		"metrics": map[string]any{
			"profile_count":                     g.Metrics.ProfileCount,
			"matched_external_call_count":       g.Metrics.MatchedExternalCallCount,
			"unmatched_external_call_count":     g.Metrics.UnmatchedExternalCallCount,
			"total_external_call_cumulative_ms": g.Metrics.TotalExternalCallCumulativeMs,
			"total_external_call_wall_time_ms":  g.Metrics.TotalExternalCallWallTimeMs,
			"total_unprofiled_external_call_ms": g.Metrics.TotalUnprofiledExternalCallMs,
			"total_network_gap_cumulative_ms":   g.Metrics.TotalNetworkGapCumulativeMs,
			"total_sql_execute_ms":              g.Metrics.TotalSqlExecuteMs,
			"total_check_query_ms":              g.Metrics.TotalCheckQueryMs,
			"total_two_pc_ms":                   g.Metrics.TotalTwoPcMs,
			"total_fetch_ms":                    g.Metrics.TotalFetchMs,
			"total_fetch_rows":                  g.Metrics.TotalFetchRows,
			"total_connection_acquire_ms":       g.Metrics.TotalConnectionAcquireMs,
			"max_external_call_concurrency":     g.Metrics.MaxExternalCallConcurrency,
			"external_call_parallelism_ratio":   g.Metrics.ExternalCallParallelismRatio,
			"group_execution_mode":              string(g.Metrics.GroupExecutionMode),
			"response_time_breakdown": map[string]any{
				"root_response_time_ms":       g.Metrics.ResponseTimeBreakdown.RootResponseTimeMs,
				"sql_execute_ms":              g.Metrics.ResponseTimeBreakdown.SQLExecuteMs,
				"check_query_ms":              g.Metrics.ResponseTimeBreakdown.CheckQueryMs,
				"two_pc_ms":                   g.Metrics.ResponseTimeBreakdown.TwoPCMs,
				"fetch_ms":                    g.Metrics.ResponseTimeBreakdown.FetchMs,
				"network_call_ms":             g.Metrics.ResponseTimeBreakdown.NetworkCallMs,
				"unprofiled_external_call_ms": g.Metrics.ResponseTimeBreakdown.UnprofiledExternalCallMs,
				"network_prep_ms":             g.Metrics.ResponseTimeBreakdown.NetworkPrepMs,
				"connection_acquire_ms":       g.Metrics.ResponseTimeBreakdown.ConnectionAcquireMs,
				"custom_slices":               responseTimeCustomSlicesToRows(g.Metrics.ResponseTimeBreakdown.CustomSlices),
				"method_time_ms":              g.Metrics.ResponseTimeBreakdown.MethodTimeMs,
				"method_time_ratio":           g.Metrics.ResponseTimeBreakdown.MethodTimeRatio,
				"coverage":                    g.Metrics.ResponseTimeBreakdown.Coverage,
				"negative_method_time":        g.Metrics.ResponseTimeBreakdown.NegativeMethodTime,
			},
		},
	}
}

func responseTimeCustomSlicesToRows(slices []models.JenniferResponseTimeCustomSlice) []map[string]any {
	if len(slices) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(slices))
	for _, slice := range slices {
		rows = append(rows, map[string]any{
			"id":              slice.ID,
			"label":           slice.Label,
			"group":           slice.Group,
			"source":          slice.Source,
			"value_ms":        slice.ValueMs,
			"count":           slice.Count,
			"matched_txids":   slice.MatchedTXIDs,
			"matched_samples": slice.MatchedSamples,
		})
	}
	return rows
}

// edgeToRow projects an edge into a flat row for `tables.msa_edges`.
func edgeToRow(e models.JenniferExternalCallEdge) map[string]any {
	row := map[string]any{
		"guid":                     e.GUID,
		"caller_txid":              e.CallerTXID,
		"caller_application":       e.CallerApplication,
		"external_call_sequence":   e.ExternalCallSequence,
		"external_call_url":        e.ExternalCallURL,
		"external_call_target":     e.ExternalCallTarget,
		"external_call_protocol":   e.ExternalCallProtocol,
		"external_call_client":     e.ExternalCallClient,
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

func unprofiledExternalCallGroupToRow(g models.JenniferUnprofiledExternalCallGroup) map[string]any {
	return map[string]any{
		"guid":               g.GUID,
		"caller_application": g.CallerApplication,
		"target":             g.Target,
		"protocol":           g.Protocol,
		"client":             g.Client,
		"match_status":       string(g.MatchStatus),
		"count":              g.Count,
		"total_elapsed_ms":   g.TotalElapsedMs,
		"avg_elapsed_ms":     g.AvgElapsedMs,
		"max_elapsed_ms":     g.MaxElapsedMs,
		"caller_txids":       g.CallerTXIDs,
		"external_call_urls": g.ExternalCallURLs,
	}
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
		"txid":            p.Header.TXID,
		"guid":            p.Header.GUID,
		"application":     p.Header.Application,
		"start_time":      p.Header.StartTime,
		"is_full_profile": len(p.Errors) == 0 && !p.Body.CapacityExceeded,
		"is_incomplete":   p.Body.CapacityExceeded,
		"errors":          errors,
		"warnings":        warnings,
		"body_metrics": map[string]any{
			"sql_execute_cum_ms":         m.SQLExecuteCumMs,
			"sql_execute_count":          m.SQLExecuteCount,
			"check_query_cum_ms":         m.CheckQueryCumMs,
			"check_query_count":          m.CheckQueryCount,
			"two_pc_cum_ms":              m.TwoPCCumMs,
			"two_pc_count":               m.TwoPCCount,
			"fetch_cum_ms":               m.FetchCumMs,
			"fetch_count":                m.FetchCount,
			"fetch_total_rows":           m.FetchTotalRows,
			"external_call_cum_ms":       m.ExternalCallCumMs,
			"external_call_count":        m.ExternalCallCount,
			"network_prep_method_cum_ms": m.NetworkPrepMethodCumMs,
			"network_prep_method_count":  m.NetworkPrepMethodCount,
			"network_prep_cum_ms":        m.NetworkPrepCumMs,
			"connection_acquire_cum_ms":  m.ConnectionAcquireCumMs,
			"connection_acquire_count":   m.ConnectionAcquireCount,
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

func networkPrepMethodsToRows(p models.JenniferTransactionProfile, m models.JenniferBodyMetrics) []map[string]any {
	rows := make([]map[string]any, 0, len(m.NetworkPrepMethods))
	for _, method := range m.NetworkPrepMethods {
		calls := make([]map[string]any, 0, len(method.IncludedExternalCalls))
		urls := make([]string, 0, len(method.IncludedExternalCalls))
		for _, call := range method.IncludedExternalCalls {
			urls = append(urls, call.ExternalURL)
			calls = append(calls, map[string]any{
				"event_no":        call.EventNo,
				"event_start":     call.EventStart,
				"external_url":    call.ExternalURL,
				"elapsed_ms":      call.ElapsedMs,
				"start_offset_ms": ptrIntOrNil(call.StartOffsetMs),
				"end_offset_ms":   ptrIntOrNil(call.EndOffsetMs),
			})
		}
		rows = append(rows, map[string]any{
			"source_file":                  p.SourceFile,
			"guid":                         p.Header.GUID,
			"txid":                         p.Header.TXID,
			"application":                  p.Header.Application,
			"event_no":                     method.EventNo,
			"event_start":                  method.EventStart,
			"raw_message":                  method.RawMessage,
			"method_elapsed_ms":            method.MethodElapsedMs,
			"start_offset_ms":              ptrIntOrNil(method.StartOffsetMs),
			"end_offset_ms":                ptrIntOrNil(method.EndOffsetMs),
			"external_call_count":          method.ExternalCallCount,
			"external_call_cum_ms":         method.ExternalCallCumMs,
			"network_prep_ms":              method.NetworkPrepMs,
			"external_call_over_method_ms": method.ExternalCallOverMethodMs,
			"suspicious":                   method.Suspicious,
			"warnings":                     method.Warnings,
			"external_call_urls":           urls,
			"included_external_calls":      calls,
		})
	}
	return rows
}

func slowSQLEventsToRows(p models.JenniferTransactionProfile) []map[string]any {
	rows := []map[string]any{}
	for _, ev := range p.Body.Events {
		if !isSQLExecuteEvent(ev.EventType) || ev.ElapsedMs == nil {
			continue
		}
		rows = append(rows, map[string]any{
			"source_file":     p.SourceFile,
			"guid":            p.Header.GUID,
			"txid":            p.Header.TXID,
			"application":     p.Header.Application,
			"event_no":        ev.EventNo,
			"event_start":     ev.EventStart,
			"event_type":      string(ev.EventType),
			"elapsed_ms":      *ev.ElapsedMs,
			"raw_message":     ev.RawMessage,
			"sql_text":        strings.Join(ev.DetailLines, "\n"),
			"start_offset_ms": ptrIntOrNil(ev.StartOffsetMs),
		})
	}
	return rows
}

func isSQLExecuteEvent(t models.JenniferEventType) bool {
	return t == models.JenniferEventSQLExecute ||
		t == models.JenniferEventSQLUpdate ||
		t == models.JenniferEventSQLQuery
}

// methodHotspotToRow projects one ranked method into the renderer row
// shape consumed by the MSA timeline's method-hotspot panel.
func methodHotspotToRow(h models.JenniferMethodHotspot) map[string]any {
	return map[string]any{
		"method":           h.Method,
		"application":      h.Application,
		"txid":             h.TXID,
		"guid":             h.GUID,
		"self_time_ms":     h.SelfTimeMs,
		"total_elapsed_ms": h.TotalElapsedMs,
		"calls":            h.Calls,
		"max_self_ms":      h.MaxSelfMs,
		"avg_self_ms":      h.AvgSelfMs,
		"self_ratio":       h.SelfRatio,
		"child_method_ms":  h.ChildMethodMs,
		"sql_ms":           h.SqlMs,
		"external_ms":      h.ExternalMs,
		"other_ms":         h.OtherMs,
	}
}

// methodHotspotsToRows emits the per-profile `tables.method_hotspots`
// rows; the renderer re-aggregates them by drilldown scope.
func methodHotspotsToRows(p models.JenniferTransactionProfile, hotspots []models.JenniferMethodHotspot) []map[string]any {
	rows := make([]map[string]any, 0, len(hotspots))
	for _, h := range hotspots {
		row := methodHotspotToRow(h)
		row["source_file"] = p.SourceFile
		rows = append(rows, row)
	}
	return rows
}

// groupMethodHotspotRows rolls per-profile hotspots up into a group
// Top-N (by application+method) for the GUID-group row.
func groupMethodHotspotRows(g models.JenniferGuidGroup, byTXID map[string][]models.JenniferMethodHotspot) []map[string]any {
	var collected []models.JenniferMethodHotspot
	for _, txid := range g.ProfileTXIDs {
		collected = append(collected, byTXID[txid]...)
	}
	rolled := RollUpMethodHotspots(collected, g.GUID, DefaultMethodHotspotLimit)
	rows := make([]map[string]any, 0, len(rolled))
	for _, h := range rolled {
		rows = append(rows, methodHotspotToRow(h))
	}
	return rows
}

func ptrIntOrNil(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
