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
	if groupFailed {
		group.ValidationStatus = "GROUP_FAILED"
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
	for _, p := range b.profiles {
		bm := AggregateBody(&p)
		m.TotalSqlExecuteMs += bm.SQLExecuteCumMs
		m.TotalCheckQueryMs += bm.CheckQueryCumMs
		m.TotalTwoPcMs += bm.TwoPCCumMs
		m.TotalFetchMs += bm.FetchCumMs
		m.TotalFetchRows += bm.FetchTotalRows
		m.TotalConnectionAcquireMs += bm.ConnectionAcquireCumMs
	}
	for _, e := range edges {
		m.TotalExternalCallCumulativeMs += e.ExternalCallElapsedMs
		if e.AdjustedNetworkGapMs != nil {
			m.TotalNetworkGapCumulativeMs += *e.AdjustedNetworkGapMs
		}
	}
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
