// [한글] msa.go — §13-§18 MSA 그룹핑 파이프라인의 본체.
//
// 입력
//
//	buildGuidGroups 가 받는 jenniferFileBucket 슬라이스는 모든 파일에서
//	파싱된 트랜잭션 프로파일들을 한 데 모은 형태.
//
// 처리 흐름
//  1. Bucket 형성 — correlation key(GUID 또는 옵션 fallback 시 TXID)
//     별로 profile 묶음. 빈 key 는 skip(상위 분석기가 diagnostic 으로
//     보고).
//  2. 각 bucket 에 대해 §15 caller-callee 매칭 (msa_match.go).
//  3. §16 network gap 계산 (raw / adjusted).
//  4. §17 root profile 추론 (들어오는 외부호출 매치가 없는 profile).
//  5. §18 call graph 구축 — DAG 형태의 (caller_txid → callee_txid)
//     엣지 리스트.
//  6. §16.7 parallelism (msa_parallelism.go) 호출.
//  7. §10 group validation status 결정 — 어느 한 profile 이라도 FULL
//     validation 실패면 GROUP_FAILED.
//
// 결정론
//
//	bucket 출력 순서는 keyOrder 에 보존된 입력 순서. parity gate 가
//	바이트 단위로 비교 가능하도록 정렬은 명시적 sort 만 사용.
//
// 결과
//
//	[]models.JenniferGuidGroup — 분석기의 tables.guid_groups 로 직결.
package jenniferprofile

import (
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// guidGroupBucket is the working-set bag of profiles we walk
// through the §13-§18 pipeline. The exposed result is
// models.JenniferGuidGroup; this stays internal so the matcher /
// graph builder can lean on it without re-coercing field types.
type guidGroupBucket struct {
	guid     string
	profiles []models.JenniferTransactionProfile
}

// buildGuidGroups runs the full §13-§18 MSA pipeline against every
// FileResult and returns one models.JenniferGuidGroup per distinct
// GUID. Profiles missing GUID (and not opted into the TXID
// fallback) bubble up through the diagnostics path on the analyzer
// side; this function silently skips them.
func buildGuidGroups(files []jenniferFileBucket, opts Options) []models.JenniferGuidGroup {
	buckets := map[string]*guidGroupBucket{}
	keyOrder := []string{}

	for _, file := range files {
		for _, p := range file.profiles {
			key := correlationKey(p, opts)
			if key == "" {
				continue
			}
			b, ok := buckets[key]
			if !ok {
				b = &guidGroupBucket{guid: key}
				buckets[key] = b
				keyOrder = append(keyOrder, key)
			}
			b.profiles = append(b.profiles, p)
		}
	}

	out := make([]models.JenniferGuidGroup, 0, len(keyOrder))
	for _, k := range keyOrder {
		b := buckets[k]
		out = append(out, runMSAForGroup(b, opts))
	}
	return out
}

// jenniferFileBucket is the analyzer-side view of one FileResult —
// kept tiny so the analyzer can pass profiles down without
// importing the parser package's type from here.
type jenniferFileBucket struct {
	sourceFile string
	profiles   []models.JenniferTransactionProfile
}

// correlationKey honors §8.4 — GUID by default; falls back to TXID
// when the parser-level fallback flag is on.
func correlationKey(p models.JenniferTransactionProfile, opts Options) string {
	if p.Header.GUID != "" {
		return p.Header.GUID
	}
	if opts.FallbackCorrelationToTxid {
		return p.Header.TXID
	}
	return ""
}

// runMSAForGroup executes the §14-§18 stages against one bucket
// and returns the rolled-up models.JenniferGuidGroup.
func runMSAForGroup(b *guidGroupBucket, opts Options) models.JenniferGuidGroup {
	group := models.JenniferGuidGroup{
		GUID:         b.guid,
		ProfileCount: len(b.profiles),
		Edges:        []models.JenniferExternalCallEdge{},
		CallGraph:    []models.JenniferCallGraphEdge{},
	}
	for _, p := range b.profiles {
		group.ProfileTXIDs = append(group.ProfileTXIDs, p.Header.TXID)
		if p.Body.CapacityExceeded {
			group.IncompleteProfileCount++
		}
	}
	sort.Strings(group.ProfileTXIDs)

	// §10 group-level validation: any profile with errors fails
	// the whole group in STRICT_MODE.
	groupFailed := false
	for _, p := range b.profiles {
		if len(p.Errors) > 0 {
			groupFailed = true
			group.Errors = append(group.Errors, p.Errors...)
		}
	}
	if group.IncompleteProfileCount > 0 {
		group.ExcludedFromSignatureStats = true
		group.Warnings = append(group.Warnings, models.JenniferProfileIssue{
			Code:    "GROUP_EXCLUDED_FROM_SIGNATURE_STATS",
			Message: "At least one profile in this GUID group was truncated by Jennifer's 5MB export capacity; signature averages exclude this group",
		})
	}
	if groupFailed {
		group.ValidationStatus = "GROUP_FAILED"
	} else if group.ExcludedFromSignatureStats {
		group.ValidationStatus = "GROUP_INCOMPLETE"
	} else {
		group.ValidationStatus = "OK"
	}

	edges := matchExternalCalls(b)
	group.Edges = edges
	for _, e := range edges {
		switch e.MatchStatus {
		case models.JenniferMatchOK:
			group.MatchedEdgeCount++
		default:
			group.UnmatchedEdgeCount++
		}
	}
	group.UnprofiledExternalCallGroups = groupUnprofiledExternalCalls(edges)

	// §17 root profile detection runs against the matched edges
	// only — unmatched edges don't contribute inbound state.
	rootIdx := detectRootProfile(b.profiles, edges)
	if rootIdx >= 0 {
		root := &b.profiles[rootIdx]
		group.RootTXID = root.Header.TXID
		group.RootApplication = root.Header.Application
		if root.Header.ResponseTimeMs != nil {
			rt := *root.Header.ResponseTimeMs
			group.RootResponseTimeMs = &rt
		}
	}

	// §18 call graph + cycle detection.
	group.CallGraph = buildCallGraph(edges)
	if cycle := detectCycle(group.CallGraph); cycle {
		group.Warnings = append(group.Warnings, models.JenniferProfileIssue{
			Code:    "CALL_GRAPH_CYCLE_DETECTED",
			Message: "GUID call graph contains a cycle",
		})
	}

	group.Metrics = computeGroupMetrics(b, edges, group)
	return group
}

// applyNetworkGap fills the raw + adjusted gap on a matched edge
// per §15. Negative raw values are clamped to 0 for the adjusted
// metric and tagged with NEGATIVE_NETWORK_GAP_ADJUSTED.
func applyNetworkGap(edge *models.JenniferExternalCallEdge) {
	if edge.CalleeResponseTimeMs == nil {
		return
	}
	raw := edge.ExternalCallElapsedMs - *edge.CalleeResponseTimeMs
	rawCopy := raw
	edge.RawNetworkGapMs = &rawCopy
	if raw < 0 {
		zero := 0
		edge.AdjustedNetworkGapMs = &zero
		edge.Warnings = append(edge.Warnings, "NEGATIVE_NETWORK_GAP_ADJUSTED")
		return
	}
	edge.AdjustedNetworkGapMs = &rawCopy
}

// detectRootProfile applies §17 priority order:
//  1. Profile NOT referenced as a callee in any matched edge.
//  2. Earliest START_TIME (header field; lex-sortable ISO format).
//  3. Largest RESPONSE_TIME on tie.
//
// Returns -1 when no candidate exists (group has zero profiles).
func detectRootProfile(profiles []models.JenniferTransactionProfile, edges []models.JenniferExternalCallEdge) int {
	if len(profiles) == 0 {
		return -1
	}
	calleeSet := map[string]bool{}
	for _, e := range edges {
		if e.MatchStatus == models.JenniferMatchOK && e.CalleeTXID != "" {
			calleeSet[e.CalleeTXID] = true
		}
	}
	candidates := []int{}
	for i := range profiles {
		if !calleeSet[profiles[i].Header.TXID] {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		// Cycle / pathological group: pick the largest RESPONSE_TIME.
		candidates = make([]int, len(profiles))
		for i := range profiles {
			candidates[i] = i
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a := &profiles[candidates[i]]
		b := &profiles[candidates[j]]
		if a.Header.StartTime != b.Header.StartTime {
			return a.Header.StartTime < b.Header.StartTime
		}
		ar := 0
		if a.Header.ResponseTimeMs != nil {
			ar = *a.Header.ResponseTimeMs
		}
		br := 0
		if b.Header.ResponseTimeMs != nil {
			br = *b.Header.ResponseTimeMs
		}
		return ar > br
	})
	return candidates[0]
}

// buildCallGraph projects matched edges into a flat directed-edge
// list keyed by caller-TXID → callee-TXID (§18).
func buildCallGraph(edges []models.JenniferExternalCallEdge) []models.JenniferCallGraphEdge {
	out := make([]models.JenniferCallGraphEdge, 0, len(edges))
	for _, e := range edges {
		if e.MatchStatus != models.JenniferMatchOK {
			continue
		}
		gap := 0
		if e.AdjustedNetworkGapMs != nil {
			gap = *e.AdjustedNetworkGapMs
		}
		callee := 0
		if e.CalleeResponseTimeMs != nil {
			callee = *e.CalleeResponseTimeMs
		}
		out = append(out, models.JenniferCallGraphEdge{
			CallerTXID:            e.CallerTXID,
			CallerApplication:     e.CallerApplication,
			CalleeTXID:            e.CalleeTXID,
			CalleeApplication:     e.CalleeApplication,
			ExternalCallElapsedMs: e.ExternalCallElapsedMs,
			CalleeResponseTimeMs:  callee,
			NetworkGapMs:          gap,
			CallerEventStartMs:    e.CallerEventStartMs,
			CalleeBodyStartMs:     e.CalleeBodyStartMs,
		})
	}
	return out
}

// detectCycle reports whether the call graph contains a directed
// cycle. We DFS each node tracking on-stack vs done; a back-edge
// onto an on-stack ancestor is a cycle.
func detectCycle(graph []models.JenniferCallGraphEdge) bool {
	if len(graph) == 0 {
		return false
	}
	adj := map[string][]string{}
	nodes := map[string]bool{}
	for _, e := range graph {
		adj[e.CallerTXID] = append(adj[e.CallerTXID], e.CalleeTXID)
		nodes[e.CallerTXID] = true
		nodes[e.CalleeTXID] = true
	}
	const (
		unseen  = 0
		onstack = 1
		done    = 2
	)
	color := map[string]int{}
	for n := range nodes {
		color[n] = unseen
	}
	var dfs func(node string) bool
	dfs = func(node string) bool {
		color[node] = onstack
		for _, next := range adj[node] {
			switch color[next] {
			case onstack:
				return true
			case unseen:
				if dfs(next) {
					return true
				}
			}
		}
		color[node] = done
		return false
	}
	for n := range nodes {
		if color[n] == unseen {
			if dfs(n) {
				return true
			}
		}
	}
	return false
}

// computeGroupMetrics rolls every matched edge + every body metric
// up into the §20.1 single-instance summary, plus the §16.7
// parallelism columns added in MVP4.
func computeGroupMetrics(b *guidGroupBucket, edges []models.JenniferExternalCallEdge, group models.JenniferGuidGroup) models.JenniferGuidMetrics {
	m := models.JenniferGuidMetrics{
		GUID:                       group.GUID,
		RootApplication:            group.RootApplication,
		RootResponseTimeMs:         group.RootResponseTimeMs,
		ProfileCount:               group.ProfileCount,
		MatchedExternalCallCount:   group.MatchedEdgeCount,
		UnmatchedExternalCallCount: group.UnmatchedEdgeCount,
	}
	totalNetworkPrep := 0
	for _, p := range b.profiles {
		bm := AggregateBody(&p)
		m.TotalSqlExecuteMs += bm.SQLExecuteCumMs
		m.TotalCheckQueryMs += bm.CheckQueryCumMs
		m.TotalTwoPcMs += bm.TwoPCCumMs
		m.TotalFetchMs += bm.FetchCumMs
		m.TotalFetchRows += bm.FetchTotalRows
		m.TotalConnectionAcquireMs += bm.ConnectionAcquireCumMs
		totalNetworkPrep += bm.NetworkPrepCumMs
	}
	for _, e := range edges {
		m.TotalExternalCallCumulativeMs += e.ExternalCallElapsedMs
		if e.MatchStatus != models.JenniferMatchOK {
			m.TotalUnprofiledExternalCallMs += e.ExternalCallElapsedMs
		}
		if e.AdjustedNetworkGapMs != nil {
			m.TotalNetworkGapCumulativeMs += *e.AdjustedNetworkGapMs
		}
	}
	// Response time decomposition (the "where did the time go" view).
	// We use NetworkGap (= external_call_elapsed - callee_response)
	// for "network call time" so we don't double-count the callee's
	// own work that's already attributed to its own SQL / FETCH /
	// etc. fields aggregated above.
	rootResp := 0
	if group.RootResponseTimeMs != nil {
		rootResp = *group.RootResponseTimeMs
	}
	bd := models.JenniferResponseTimeBreakdown{
		RootResponseTimeMs:       rootResp,
		SQLExecuteMs:             m.TotalSqlExecuteMs,
		CheckQueryMs:             m.TotalCheckQueryMs,
		TwoPCMs:                  m.TotalTwoPcMs,
		FetchMs:                  m.TotalFetchMs,
		NetworkCallMs:            m.TotalNetworkGapCumulativeMs,
		UnprofiledExternalCallMs: m.TotalUnprofiledExternalCallMs,
		NetworkPrepMs:            totalNetworkPrep,
		ConnectionAcquireMs:      m.TotalConnectionAcquireMs,
	}
	covered := bd.SQLExecuteMs + bd.CheckQueryMs + bd.TwoPCMs + bd.FetchMs +
		bd.NetworkCallMs + bd.UnprofiledExternalCallMs + bd.NetworkPrepMs + bd.ConnectionAcquireMs
	if rootResp > 0 {
		methodMs := rootResp - covered
		if methodMs < 0 {
			bd.NegativeMethodTime = true
			methodMs = 0
		}
		bd.MethodTimeMs = methodMs
		bd.MethodTimeRatio = float64(methodMs) / float64(rootResp)
		bd.Coverage = float64(covered) / float64(rootResp)
	}
	m.ResponseTimeBreakdown = bd
	// §16.7 parallelism — ALWAYS computed even for sequential
	// inputs so the renderer can show ratio=1.0 / mode=SEQUENTIAL
	// uniformly. Cumulative ms stays sourced from the edge sum
	// above (counts events even when offset parsing fails); wall-
	// clock + concurrency come from offset-based intervals.
	_, wall, conc, _, mode, perProfile := computeGroupParallelism(b.profiles)
	m.TotalExternalCallWallTimeMs = wall
	m.MaxExternalCallConcurrency = conc
	if wall > 0 {
		m.ExternalCallParallelismRatio = float64(m.TotalExternalCallCumulativeMs) / float64(wall)
	} else {
		m.ExternalCallParallelismRatio = 1
	}
	m.GroupExecutionMode = mode
	m.ProfileParallelism = perProfile
	return m
}

func groupUnprofiledExternalCalls(edges []models.JenniferExternalCallEdge) []models.JenniferUnprofiledExternalCallGroup {
	type groupKey struct {
		guid     string
		caller   string
		target   string
		protocol string
		client   string
		status   models.JenniferMatchStatus
	}
	type groupState struct {
		group   models.JenniferUnprofiledExternalCallGroup
		txidSet map[string]bool
		urlSet  map[string]bool
	}

	states := map[groupKey]*groupState{}
	order := []groupKey{}
	for _, edge := range edges {
		if edge.MatchStatus == models.JenniferMatchOK {
			continue
		}
		target := edge.ExternalCallTarget
		if target == "" {
			target = normalizeExternalTargetForGrouping(edge.ExternalCallURL)
		}
		key := groupKey{
			guid:     edge.GUID,
			caller:   edge.CallerApplication,
			target:   target,
			protocol: edge.ExternalCallProtocol,
			client:   edge.ExternalCallClient,
			status:   edge.MatchStatus,
		}
		state, ok := states[key]
		if !ok {
			state = &groupState{
				group: models.JenniferUnprofiledExternalCallGroup{
					GUID:              edge.GUID,
					CallerApplication: edge.CallerApplication,
					Target:            target,
					Protocol:          edge.ExternalCallProtocol,
					Client:            edge.ExternalCallClient,
					MatchStatus:       edge.MatchStatus,
				},
				txidSet: map[string]bool{},
				urlSet:  map[string]bool{},
			}
			states[key] = state
			order = append(order, key)
		}
		state.group.Count++
		state.group.TotalElapsedMs += edge.ExternalCallElapsedMs
		if edge.ExternalCallElapsedMs > state.group.MaxElapsedMs {
			state.group.MaxElapsedMs = edge.ExternalCallElapsedMs
		}
		if edge.CallerTXID != "" && !state.txidSet[edge.CallerTXID] {
			state.txidSet[edge.CallerTXID] = true
			state.group.CallerTXIDs = append(state.group.CallerTXIDs, edge.CallerTXID)
		}
		if edge.ExternalCallURL != "" && !state.urlSet[edge.ExternalCallURL] {
			state.urlSet[edge.ExternalCallURL] = true
			state.group.ExternalCallURLs = append(state.group.ExternalCallURLs, edge.ExternalCallURL)
		}
	}
	out := make([]models.JenniferUnprofiledExternalCallGroup, 0, len(order))
	for _, key := range order {
		group := states[key].group
		if group.Count > 0 {
			group.AvgElapsedMs = float64(group.TotalElapsedMs) / float64(group.Count)
		}
		out = append(out, group)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TotalElapsedMs != out[j].TotalElapsedMs {
			return out[i].TotalElapsedMs > out[j].TotalElapsedMs
		}
		if out[i].CallerApplication != out[j].CallerApplication {
			return out[i].CallerApplication < out[j].CallerApplication
		}
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		return out[i].MatchStatus < out[j].MatchStatus
	})
	return out
}
