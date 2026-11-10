package jenniferprofile

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// DrilldownVersion versions the selection and cumulative-cost semantics, not
// the parser. The enclosing result remains the common AnalysisResult contract.
const DrilldownVersion = "1.0.0"

// DrilldownOptions selects one GUID/TXID or one signature/application. Empty
// TXID/application means the whole group. An average selects the slowest
// matching application profile once per GUID; ties preserve input order.
type DrilldownOptions struct {
	Mode          string `json:"mode"`
	GUID          string `json:"guid,omitempty"`
	TXID          string `json:"txid,omitempty"`
	SignatureHash string `json:"signature_hash,omitempty"`
	Application   string `json:"application,omitempty"`
}

// ProjectDrilldown consumes an existing result only: no file reparse, parser
// mutation or exporter logic. JSON normalization detaches every nested map and
// supports both native analyzer rows and a JSON/Wails round trip identically.
// [한글] 선택 → GUID별 하위 호출 탐색 → 비용 재합산 → 공통 결과 계약 순서다.
// 누적 비용은 병렬 실행 때문에 응답시간을 넘을 수 있으므로 비율을 숨기거나
// 합을 강제로 줄이지 않는다. 재계산하지 않은 상위 통계도 복사하지 않는다.
func ProjectDrilldown(source models.AnalysisResult, opts DrilldownOptions) (models.AnalysisResult, error) {
	fail := func(message string) (models.AnalysisResult, error) {
		return models.AnalysisResult{}, fmt.Errorf("Jennifer drilldown: %s", message)
	}
	if source.Type != ResultType {
		return fail("requires a jennifer_profile result")
	}
	if source.Metadata.Extra["drilldown"] != nil {
		return fail("select from the original result, not an existing projection")
	}
	if opts.Mode == "" {
		opts.Mode = "single"
	}
	if opts.Mode == "single" {
		if opts.GUID == "" || opts.SignatureHash != "" || opts.Application != "" {
			return fail("single mode requires GUID and optional TXID only")
		}
	} else if opts.Mode == "average" {
		if opts.SignatureHash == "" || opts.GUID != "" || opts.TXID != "" {
			return fail("average mode requires signature_hash and optional application only")
		}
	} else {
		return fail("mode must be single or average")
	}
	if len(getMapSlice(source.Tables, "profiles")) > 100000 || len(getMapSlice(source.Series, "guid_groups")) > 10000 || len(getMapSlice(source.Tables, "msa_edges")) > 500000 {
		return fail("input exceeds drilldown row limits")
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		return models.AnalysisResult{}, err
	}
	if len(encoded) > 64<<20 {
		return fail("input exceeds 64 MiB")
	}
	var input models.AnalysisResult
	if err = json.Unmarshal(encoded, &input); err != nil {
		return models.AnalysisResult{}, err
	}
	groups := getMapSlice(input.Series, "guid_groups")
	graphCount := 0
	for _, group := range groups {
		graphCount += len(getMapSlice(group, "call_graph"))
	}
	if graphCount > 500000 {
		return fail("input exceeds 500000 call-graph edges")
	}
	profiles := getMapSlice(input.Tables, "profiles")
	byGUID := map[string][]map[string]any{}
	orphanByTXID := map[string][]map[string]any{}
	for _, p := range profiles {
		guid := drillText(p["guid"])
		if guid != "" {
			byGUID[guid] = append(byGUID[guid], p)
		} else {
			id := drillText(p["txid"])
			orphanByTXID[id] = append(orphanByTXID[id], p)
		}
	}
	edgesByGUID := map[string][]map[string]any{}
	for _, e := range getMapSlice(input.Tables, "msa_edges") {
		g := drillText(e["guid"])
		edgesByGUID[g] = append(edgesByGUID[g], e)
	}
	out := models.New(ResultType, input.Metadata.Parser)
	out.CreatedAt = input.CreatedAt
	out.SourceFiles = append([]string{}, input.SourceFiles...)
	out.Metadata.SchemaVersion = input.Metadata.SchemaVersion
	scopes := []map[string]any{}
	scopedGroups := []map[string]any{}
	selectedProfiles := []map[string]any{}
	selectedEdges := []map[string]any{}
	identities := map[string]bool{}
	candidateCount := 0
	seenGUID := map[string]bool{}
	for _, group := range groups {
		guid := drillText(group["guid"])
		if opts.Mode == "single" && guid != opts.GUID {
			continue
		}
		if opts.Mode == "average" && drillText(group["signature_hash"]) != opts.SignatureHash {
			continue
		}
		if opts.Mode == "average" && group["excluded_from_signature_stats"] == true {
			continue
		}
		if guid == "" || seenGUID[guid] {
			return fail("missing or duplicate GUID group identity")
		}
		seenGUID[guid] = true
		candidateCount++
		rows := append([]map[string]any{}, byGUID[guid]...)
		for _, id := range drillStrings(group["profile_txids"]) {
			rows = append(rows, orphanByTXID[id]...)
		}
		target := opts.TXID
		if opts.Mode == "average" && opts.Application != "" {
			var representative map[string]any
			for _, row := range rows {
				if strings.TrimSpace(drillText(row["application"])) == opts.Application && (representative == nil || drillResponse(row) > drillResponse(representative)) {
					representative = row
				}
			}
			if representative == nil {
				continue
			}
			target = drillText(representative["txid"])
			if target == "" {
				return fail("selected application has no TXID")
			}
		}
		scope, scopeErr := projectGroup(group, rows, edgesByGUID[guid], target)
		if scopeErr != nil {
			return models.AnalysisResult{}, scopeErr
		}
		scopes = append(scopes, scope)
		scopedGroups = append(scopedGroups, scope["group"].(map[string]any))
		for _, row := range scope["profile_rows"].([]map[string]any) {
			selectedProfiles = append(selectedProfiles, row)
			identities[drillIdentity(row)] = true
		}
		selectedEdges = append(selectedEdges, scope["service_network_edges"].([]map[string]any)...)
	}
	if candidateCount == 0 {
		return fail("selected GUID/signature was not found")
	}
	if len(scopes) == 0 {
		return fail("selected application was not found in this signature")
	}
	// SQL/method tables are joined by GUID+TXID, never by TXID alone. Empty
	// GUID standalone rows remain distinct from identically named remote TXIDs.
	selectedTables := []string{"slow_sql_events", "network_prep_methods", "method_hotspots"}
	for _, key := range selectedTables {
		rows := []map[string]any{}
		for _, row := range getMapSlice(input.Tables, key) {
			if identities[drillIdentity(row)] {
				rows = append(rows, row)
			}
		}
		out.Tables[key] = rows
	}
	scopesByIdentity := map[string][]map[string]any{}
	for _, scope := range scopes {
		scope["slow_sql_rows"] = []map[string]any{}
		for _, p := range scope["profile_rows"].([]map[string]any) {
			key := drillIdentity(p)
			scopesByIdentity[key] = append(scopesByIdentity[key], scope)
		}
	}
	for _, row := range out.Tables["slow_sql_events"].([]map[string]any) {
		for _, scope := range scopesByIdentity[drillIdentity(row)] {
			scope["slow_sql_rows"] = append(scope["slow_sql_rows"].([]map[string]any), row)
		}
	}
	out.Tables["profiles"] = selectedProfiles
	out.Tables["msa_edges"] = drillUniqueEdges(selectedEdges)
	out.Series["guid_groups"] = scopedGroups
	out.Series["drilldown_scopes"] = scopes
	actualTimeline := []map[string]any{}
	for _, scope := range scopes {
		actualTimeline = append(actualTimeline, scope["timeline_edges"].([]map[string]any)...)
	}
	out.Series["drilldown_average_timeline_edges"] = drillAverageEdges(drillUniqueEdges(actualTimeline))
	// Uniform group weighting matches the UI: each GUID contributes once,
	// including zero-duration records. The denominator is always explicit.
	average := map[string]any{}
	sums := map[string]float64{}
	for _, g := range scopedGroups {
		for key, value := range getMap(getMap(g, "metrics"), "response_time_breakdown") {
			if number, ok := value.(float64); ok {
				sums[key] += number
			}
		}
	}
	for key, sum := range sums {
		if strings.HasSuffix(key, "_ms") {
			average[key] = sum / float64(len(scopes))
		}
	}
	out.Series["drilldown_average_breakdown"] = average
	incomplete, full, errorsCount, warningsCount := 0, 0, 0, 0
	for _, p := range selectedProfiles {
		if p["is_incomplete"] == true {
			incomplete++
		}
		if p["is_full_profile"] == true {
			full++
		}
		errorsCount += len(getMapSlice(p, "errors"))
		warningsCount += len(getMapSlice(p, "warnings"))
	}
	out.Summary = map[string]any{"total_profiles": len(selectedProfiles), "guid_group_count": len(scopes), "sample_count": len(scopes), "candidate_group_count": candidateCount, "excluded_group_count": candidateCount - len(scopes), "incomplete_profile_count": incomplete, "full_profile_count": full, "total_errors": errorsCount, "total_warnings": warningsCount, "avg_response_time_ms": average["root_response_time_ms"]}
	out.Metadata.Extra["drilldown"] = map[string]any{"version": DrilldownVersion, "selection": opts, "sample_count": len(scopes), "candidate_group_count": candidateCount, "excluded_group_count": candidateCount - len(scopes), "representative_policy": "slowest_matching_profile_per_guid_input_order_tie", "cost_semantics": "cumulative_not_critical_path", "omitted_parent_statistics": []string{"signature_statistics", "file_summary", "custom_rule_stats", "unprofiled_external_call_groups", "parallelism"}}
	return out, nil
}

func projectGroup(group map[string]any, profiles, edges []map[string]any, target string) (map[string]any, error) {
	byID := map[string]map[string]any{}
	for _, p := range profiles {
		id := drillText(p["txid"])
		if id == "" {
			continue
		}
		if byID[id] != nil {
			return nil, fmt.Errorf("Jennifer drilldown: ambiguous duplicate TXID %q in selected group", id)
		}
		byID[id] = p
	}
	graph := getMapSlice(group, "call_graph")
	whole := target == ""
	if !whole && byID[target] == nil {
		return nil, fmt.Errorf("Jennifer drilldown: selected TXID %q was not found in selected GUID", target)
	}
	wanted := map[string]bool{}
	order := []string{}
	if whole {
		for _, p := range profiles {
			id := drillText(p["txid"])
			if id != "" && !wanted[id] {
				wanted[id] = true
				order = append(order, id)
			}
		}
	} else {
		adjacency := map[string][]string{}
		for _, e := range graph {
			caller, callee := drillText(e["caller_txid"]), drillText(e["callee_txid"])
			if caller != "" && callee != "" {
				adjacency[caller] = append(adjacency[caller], callee)
			}
		}
		stack := []string{target}
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if wanted[id] {
				continue
			}
			wanted[id] = true
			order = append(order, id)
			for _, child := range adjacency[id] {
				if !wanted[child] {
					stack = append(stack, child)
				}
			}
		}
	}
	rows := []map[string]any{}
	for _, p := range profiles {
		if whole || wanted[drillText(p["txid"])] {
			rows = append(rows, p)
		}
	}
	matched := []map[string]any{}
	for _, e := range graph {
		if whole || (wanted[drillText(e["caller_txid"])] && wanted[drillText(e["callee_txid"])]) {
			e["guid"] = group["guid"]
			e["match_status"] = "MATCHED"
			matched = append(matched, e)
		}
	}
	unmatched := []map[string]any{}
	for _, e := range edges {
		caller, status := drillText(e["caller_txid"]), drillText(e["match_status"])
		if !whole && !wanted[caller] {
			continue
		}
		if status != "" && status != "MATCHED" {
			unmatched = append(unmatched, e)
		}
	}
	timeline := drillUniqueEdges(append(append([]map[string]any{}, matched...), unmatched...))
	// Matched evidence has one canonical representation in call_graph. The
	// renderer compacts duplicate MATCHED rows out of msa_edges before IPC;
	// consuming those duplicates here would make CLI and desktop disagree.
	service := timeline
	selectedApp := drillText(group["root_application"])
	if !whole {
		selectedApp = drillText(byID[target]["application"])
		root := drillResponse(byID[target])
		metrics, breakdown := map[string]any{}, map[string]any{"root_response_time_ms": root}
		costs := []struct{ body, metric, part string }{{"sql_execute_cum_ms", "total_sql_execute_ms", "sql_execute_ms"}, {"check_query_cum_ms", "total_check_query_ms", "check_query_ms"}, {"two_pc_cum_ms", "total_two_pc_ms", "two_pc_ms"}, {"fetch_cum_ms", "total_fetch_ms", "fetch_ms"}, {"connection_acquire_cum_ms", "total_connection_acquire_ms", "connection_acquire_ms"}, {"network_prep_cum_ms", "", "network_prep_ms"}, {"servlet_dispatch_cum_ms", "", "servlet_dispatch_ms"}}
		covered := float64(0)
		for _, cost := range costs {
			sum := float64(0)
			for _, p := range rows {
				sum += drillNonnegative(getMap(p, "body_metrics")[cost.body])
			}
			breakdown[cost.part] = sum
			covered += sum
			if cost.metric != "" {
				metrics[cost.metric] = sum
			}
		}
		fetchRows := float64(0)
		for _, p := range rows {
			fetchRows += drillNonnegative(getMap(p, "body_metrics")["fetch_total_rows"])
		}
		metrics["total_fetch_rows"] = fetchRows
		network, external, unprofiled := float64(0), float64(0), float64(0)
		for _, e := range matched {
			network += drillNonnegative(e["network_gap_ms"])
			external += drillNonnegative(e["external_call_elapsed_ms"])
		}
		for _, e := range unmatched {
			unprofiled += drillNonnegative(e["external_call_elapsed_ms"])
		}
		covered += network + unprofiled
		breakdown["network_call_ms"], breakdown["unprofiled_external_call_ms"] = network, unprofiled
		breakdown["method_time_ms"] = math.Max(0, root-covered)
		breakdown["method_time_ratio"], breakdown["coverage"] = float64(0), float64(0)
		if root > 0 {
			breakdown["method_time_ratio"] = math.Max(0, root-covered) / root
			breakdown["coverage"] = covered / root
		}
		breakdown["negative_method_time"] = covered > root
		metrics["profile_count"], metrics["matched_external_call_count"], metrics["unmatched_external_call_count"] = len(rows), len(matched), len(unmatched)
		metrics["total_external_call_cumulative_ms"], metrics["total_unprofiled_external_call_ms"], metrics["total_network_gap_cumulative_ms"] = external+unprofiled, unprofiled, network
		metrics["response_time_breakdown"] = breakdown
		// Keep only identity/provenance fields from the parent. Parent hotspots,
		// custom costs and parallelism do not describe this selected subtree.
		scoped := map[string]any{}
		for _, key := range []string{"guid", "signature_hash", "signature_version"} {
			scoped[key] = group[key]
		}
		scoped["root_txid"], scoped["root_application"], scoped["root_response_time_ms"] = target, selectedApp, root
		if target == drillText(group["root_txid"]) {
			scoped["root_body_start_ms"] = group["root_body_start_ms"]
		}
		scoped["profile_txids"], scoped["profile_count"], scoped["matched_edge_count"], scoped["unmatched_edge_count"], scoped["call_graph"], scoped["metrics"] = order, len(rows), len(matched), len(unmatched), matched, metrics
		group = scoped
	}
	networkByID := map[string]float64{}
	for _, e := range matched {
		id := drillText(e["caller_txid"])
		if id != "" {
			networkByID[id] += drillNumber(e["network_gap_ms"])
		}
	}
	return map[string]any{"group": group, "profile_rows": rows, "timeline_edges": timeline, "topology_edges": matched, "service_network_edges": service, "network_time_by_txid": networkByID, "selected_value": map[bool]string{true: "__whole__", false: target}[whole], "selected_application": selectedApp, "selected_txid": target, "is_whole_transaction": whole}, nil
}

func drillText(value any) string         { s, _ := value.(string); return strings.TrimSpace(s) }
func drillNumber(value any) float64      { n, _ := value.(float64); return n }
func drillNonnegative(value any) float64 { return math.Max(0, math.Floor(drillNumber(value)+0.5)) }
func drillResponse(row map[string]any) float64 {
	return drillNonnegative(getMap(row, "header")["response_time_ms"])
}
func drillStrings(value any) []string {
	out := []string{}
	values, _ := value.([]any)
	for _, v := range values {
		if text := drillText(v); text != "" {
			out = append(out, text)
		}
	}
	return out
}
func drillIdentity(row map[string]any) string {
	return drillText(row["guid"]) + "\x00" + drillText(row["txid"])
}
func drillUniqueEdges(edges []map[string]any) []map[string]any {
	out := []map[string]any{}
	seen := map[string]bool{}
	for _, e := range edges {
		parts := []any{e["timeline_kind"], e["match_status"], e["guid"], e["caller_txid"], e["callee_txid"], e["external_call_sequence"], e["caller_application"], e["callee_application"], e["external_call_target"], e["external_call_url"], e["caller_event_start_ms"], e["external_call_elapsed_ms"]}
		if parts[0] == nil {
			parts[0] = "external_call"
		}
		if parts[1] == nil {
			parts[1] = "MATCHED"
		}
		key, _ := json.Marshal(parts)
		if !seen[string(key)] {
			seen[string(key)] = true
			out = append(out, e)
		}
	}
	return out
}

// [한글] 평균 타임라인은 실제 시각이 아니라 호출 쌍별 평균 길이를 순서대로
// 배치한 도식이다. call_count로 가중하고 실제 흐름과 구분할 수 있게 표시한다.
func drillAverageEdges(edges []map[string]any) []map[string]any {
	type pair struct {
		caller, callee                    string
		count, elapsed, response, network float64
	}
	states := []*pair{}
	byPair := map[[2]string]*pair{}
	for _, edge := range edges {
		caller, callee := drillText(edge["caller_application"]), drillText(edge["callee_application"])
		if callee == "" {
			callee = drillText(edge["external_call_target"])
		}
		if callee == "" {
			callee = drillText(edge["external_call_url"])
		}
		if caller == "" || callee == "" {
			continue
		}
		key := [2]string{caller, callee}
		state := byPair[key]
		if state == nil {
			state = &pair{caller: caller, callee: callee}
			byPair[key] = state
			states = append(states, state)
		}
		count := math.Max(1, drillNonnegative(edge["call_count"]))
		state.count += count
		state.elapsed += math.Max(0, drillNumber(edge["external_call_elapsed_ms"])) * count
		state.response += math.Max(0, drillNumber(edge["callee_response_time_ms"])) * count
		gap := edge["network_gap_ms"]
		if gap == nil {
			gap = edge["adjusted_network_gap_ms"]
		}
		state.network += math.Max(0, drillNumber(gap)) * count
	}
	sort.SliceStable(states, func(i, j int) bool { return states[i].elapsed > states[j].elapsed })
	out := []map[string]any{}
	cursor := float64(0)
	for i, state := range states {
		elapsed := math.Floor(state.elapsed/state.count + 0.5)
		start := cursor
		cursor += elapsed
		if elapsed <= 0 {
			continue
		}
		out = append(out, map[string]any{"caller_application": state.caller, "callee_application": state.callee, "caller_txid": fmt.Sprintf("avg-drill-%d-caller", i), "callee_txid": fmt.Sprintf("avg-drill-%d-callee", i), "external_call_elapsed_ms": elapsed, "callee_response_time_ms": math.Floor(state.response/state.count + 0.5), "network_gap_ms": math.Floor(state.network/state.count + 0.5), "caller_event_start_ms": start, "match_status": "MATCHED", "call_count": state.count, "timeline_kind": "synthetic_average"})
	}
	return out
}
