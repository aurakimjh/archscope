package jenniferprofile

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func drillFixture() models.AnalysisResult {
	r := models.New(ResultType, ParserName)
	profile := func(g, id, app string, rt, sql int) map[string]any {
		return map[string]any{"guid": g, "txid": id, "application": app, "header": map[string]any{"response_time_ms": rt}, "body_metrics": map[string]any{"sql_execute_cum_ms": sql}, "is_full_profile": true}
	}
	edge := func(caller, callee string, gap, elapsed int) map[string]any {
		return map[string]any{"caller_txid": caller, "callee_txid": callee, "network_gap_ms": gap, "external_call_elapsed_ms": elapsed}
	}
	group := func(g string, ids []string, graph []map[string]any) map[string]any {
		return map[string]any{"guid": g, "signature_hash": "sig", "root_txid": ids[0], "root_application": "/root", "profile_txids": ids, "root_response_time_ms": 1000, "call_graph": graph, "metrics": map[string]any{"max_external_call_concurrency": 99, "response_time_breakdown": map[string]any{"root_response_time_ms": 1000, "sql_execute_ms": 999}}}
	}
	r.Tables["profiles"] = []map[string]any{profile("g1", "a", "/root", 1000, 100), profile("g1", "b", "/middle", 200, 20), profile("g1", "c", "/leaf", 80, 30), profile("g1", "d", "/sibling", 50, 90), profile("g2", "b", "/middle", 400, 40), profile("g2", "b2", "/middle", 300, 60), profile("g3", "z", "/other", 99, 9)}
	r.Series["guid_groups"] = []map[string]any{group("g1", []string{"a", "b", "c", "d"}, []map[string]any{edge("a", "b", 5, 250), edge("b", "c", 10, 100), edge("a", "d", 8, 90)}), group("g2", []string{"b", "b2"}, nil), group("g3", []string{"z"}, nil)}
	r.Tables["msa_edges"] = []map[string]any{{"guid": "g1", "caller_txid": "b", "match_status": "UNMATCHED", "external_call_elapsed_ms": 15}, {"guid": "g1", "caller_txid": "a", "match_status": "UNMATCHED", "external_call_elapsed_ms": 800}}
	r.Tables["slow_sql_events"] = []map[string]any{{"guid": "g1", "txid": "b", "sql_text": "chosen"}, {"guid": "g1", "txid": "a", "sql_text": "parent"}, {"guid": "g2", "txid": "b", "sql_text": "other-guid"}}
	return r
}

func TestDrilldownSubtreeCostAndIsolation(t *testing.T) {
	source := drillFixture()
	before, _ := json.Marshal(source)
	projected, err := ProjectDrilldown(source, DrilldownOptions{Mode: "single", GUID: "g1", TXID: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if projected.Summary["total_profiles"] != 2 {
		t.Fatal(projected.Summary)
	}
	group := getMapSlice(projected.Series, "guid_groups")[0]
	metrics := getMap(group, "metrics")
	bd := getMap(metrics, "response_time_breakdown")
	for key, want := range map[string]float64{"root_response_time_ms": 200, "sql_execute_ms": 50, "network_call_ms": 10, "unprofiled_external_call_ms": 15, "method_time_ms": 125, "coverage": 0.375} {
		if bd[key] != want {
			t.Errorf("%s = %v, want %v", key, bd[key], want)
		}
	}
	if metrics["max_external_call_concurrency"] != nil {
		t.Fatal("parent concurrency leaked")
	}
	rows := getMapSlice(projected.Tables, "slow_sql_events")
	if len(rows) != 1 || rows[0]["sql_text"] != "chosen" {
		t.Fatal(rows)
	}
	if len(getMapSlice(projected.Tables, "msa_edges")) != 2 {
		t.Fatal(projected.Tables["msa_edges"])
	}
	group["root_application"] = "changed"
	rows[0]["sql_text"] = "changed"
	after, _ := json.Marshal(source)
	if string(before) != string(after) {
		t.Fatal("projection mutated source")
	}
}

func TestDrilldownAverageRepresentativeAndDenominator(t *testing.T) {
	r, err := ProjectDrilldown(drillFixture(), DrilldownOptions{Mode: "average", SignatureHash: "sig", Application: "/middle"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Summary["sample_count"] != 2 || r.Summary["candidate_group_count"] != 3 || r.Summary["excluded_group_count"] != 1 {
		t.Fatal(r.Summary)
	}
	avg := getMap(r.Series, "drilldown_average_breakdown")
	if avg["root_response_time_ms"] != float64(300) || avg["sql_execute_ms"] != float64(45) {
		t.Fatal(avg)
	}
	if len(getMapSlice(r.Tables, "profiles")) != 3 || len(getMapSlice(r.Tables, "slow_sql_events")) != 2 {
		t.Fatal(r.Tables)
	}
}

func TestDrilldownWholeAndJSONParity(t *testing.T) {
	source := drillFixture()
	opts := DrilldownOptions{Mode: "single", GUID: "g1"}
	direct, err := ProjectDrilldown(source, opts)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(source)
	var decoded models.AnalysisResult
	json.Unmarshal(raw, &decoded)
	wire, err := ProjectDrilldown(decoded, opts)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(direct)
	b, _ := json.Marshal(wire)
	if string(a) != string(b) {
		t.Fatal("native/JSON projection differ")
	}
	if !reflect.DeepEqual(getMap(getMapSlice(direct.Series, "guid_groups")[0], "metrics"), getMap(getMapSlice(decoded.Series, "guid_groups")[0], "metrics")) {
		t.Fatal("whole group metrics changed")
	}
}

func TestDrilldownCyclesZeroAndOvercoverage(t *testing.T) {
	source := drillFixture()
	groups := getMapSlice(source.Series, "guid_groups")
	graph := getMapSlice(groups[0], "call_graph")
	groups[0]["call_graph"] = append(graph, map[string]any{"caller_txid": "c", "callee_txid": "b", "network_gap_ms": 500})
	r, err := ProjectDrilldown(source, DrilldownOptions{GUID: "g1", TXID: "b"})
	if err != nil {
		t.Fatal(err)
	}
	bd := getMap(getMap(getMapSlice(r.Series, "guid_groups")[0], "metrics"), "response_time_breakdown")
	if bd["method_time_ms"] != float64(0) || bd["negative_method_time"] != true || bd["coverage"].(float64) <= 1 {
		t.Fatal(bd)
	}
	getMap(getMapSlice(source.Tables, "profiles")[1], "header")["response_time_ms"] = 0
	r, err = ProjectDrilldown(source, DrilldownOptions{GUID: "g1", TXID: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = json.Marshal(r); err != nil {
		t.Fatal("nonfinite zero denominator", err)
	}
}

func TestDrilldownInvalidSelectionsAndDuplicateIdentity(t *testing.T) {
	for _, opts := range []DrilldownOptions{{GUID: "missing"}, {GUID: "g1", TXID: "missing"}, {Mode: "bad", GUID: "g1"}, {Mode: "average", SignatureHash: "sig", Application: "missing"}, {Mode: "average", SignatureHash: "sig", TXID: "b"}, {GUID: "g1", Application: "/middle"}} {
		if _, err := ProjectDrilldown(drillFixture(), opts); err == nil {
			t.Fatal(opts)
		}
	}
	r := drillFixture()
	r.Tables["profiles"] = append(getMapSlice(r.Tables, "profiles"), getMapSlice(r.Tables, "profiles")[1])
	if _, err := ProjectDrilldown(r, DrilldownOptions{GUID: "g1"}); err == nil {
		t.Fatal("duplicate TXID accepted")
	}
	projection, _ := ProjectDrilldown(drillFixture(), DrilldownOptions{GUID: "g1"})
	if _, err := ProjectDrilldown(projection, DrilldownOptions{GUID: "g1"}); err == nil {
		t.Fatal("nested projection accepted")
	}
}

func TestDrilldownStandaloneAndHTMLScope(t *testing.T) {
	r := drillFixture()
	p := getMapSlice(r.Tables, "profiles")[0]
	p["guid"] = ""
	g := getMapSlice(r.Series, "guid_groups")[0]
	g["guid"] = "standalone:a"
	g["profile_txids"] = []string{"a"}
	g["call_graph"] = []map[string]any{}
	projected, err := ProjectDrilldown(r, DrilldownOptions{GUID: "standalone:a", TXID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(getMapSlice(projected.Tables, "profiles")) != 1 {
		t.Fatal(projected.Tables)
	}
	report := RenderHTMLReport(projected)
	if !strings.Contains(report, "Drilldown scope") || !strings.Contains(report, "1.0.0") {
		t.Fatal("report has no scope disclosure")
	}
}

func TestDrilldownAverageExcludesIncompleteAndKeepsTiesStable(t *testing.T) {
	source := drillFixture()
	getMapSlice(source.Series, "guid_groups")[2]["excluded_from_signature_stats"] = true
	rows := getMapSlice(source.Tables, "profiles")
	getMap(rows[5], "header")["response_time_ms"] = 400
	r, err := ProjectDrilldown(source, DrilldownOptions{Mode: "average", SignatureHash: "sig", Application: "/middle"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Summary["candidate_group_count"] != 2 || r.Summary["sample_count"] != 2 {
		t.Fatal(r.Summary)
	}
	scopes := getMapSlice(r.Series, "drilldown_scopes")
	if scopes[1]["selected_txid"] != "b" {
		t.Fatal("tie selected a later profile")
	}
}

func TestDrilldownWeightedAverageEdges(t *testing.T) {
	edges := []map[string]any{{"caller_application": "a", "callee_application": "b", "call_count": float64(2), "external_call_elapsed_ms": float64(100), "callee_response_time_ms": float64(80), "network_gap_ms": float64(20)}, {"caller_application": "a", "callee_application": "b", "external_call_elapsed_ms": float64(40), "callee_response_time_ms": float64(30), "network_gap_ms": float64(10)}}
	row := drillAverageEdges(edges)[0]
	if row["call_count"] != float64(3) || row["external_call_elapsed_ms"] != float64(80) || row["network_gap_ms"] != float64(17) || row["timeline_kind"] != "synthetic_average" {
		t.Fatal(row)
	}
}

func TestDrilldownDesktopCompactionAndCLIHaveIdenticalResults(t *testing.T) {
	compact := drillFixture()
	full := drillFixture()
	duplicate := map[string]any{"guid": "g1", "caller_txid": "b", "callee_txid": "c", "match_status": "MATCHED", "network_gap_ms": 10, "external_call_elapsed_ms": 100}
	full.Tables["msa_edges"] = append(getMapSlice(full.Tables, "msa_edges"), duplicate)
	full.CreatedAt = compact.CreatedAt
	a, err := ProjectDrilldown(compact, DrilldownOptions{GUID: "g1", TXID: "b"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := ProjectDrilldown(full, DrilldownOptions{GUID: "g1", TXID: "b"})
	if err != nil {
		t.Fatal(err)
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	if string(x) != string(y) {
		t.Fatal("desktop-compacted and full CLI input produced different evidence")
	}
}

func TestDrilldownRowLimitFailsWithoutTruncation(t *testing.T) {
	r := drillFixture()
	r.Tables["profiles"] = make([]map[string]any, 100001)
	if _, err := ProjectDrilldown(r, DrilldownOptions{GUID: "g1"}); err == nil {
		t.Fatal("oversized input accepted")
	}
}
