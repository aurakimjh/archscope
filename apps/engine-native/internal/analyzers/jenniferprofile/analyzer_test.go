package jenniferprofile

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/jenniferprofile"
)

const testFixture = "../../parsers/jenniferprofile/testdata/sample_two_tx.txt"

// callerElapsedMs / calleeResponseMs are tiny option-builders used
// by the inline-fixture tests to keep call sites readable.
type callerElapsedMs int
type calleeResponseMs int

// buildTwoProfileFixture synthesises a minimal Jennifer profile
// export with one caller (TX1, /prod/order/create) and one callee
// (TX2, /dev/svc/a) sharing one GUID. Used to drive matcher edge
// cases (negative gap, scoring tiebreaks) without polluting the
// canonical testdata file.
func buildTwoProfileFixture(callerElapsed callerElapsedMs, calleeRT calleeResponseMs) jenniferprofile.FileResult {
	body := fmt.Sprintf(`---------------------------------------------------------------------------------------------------------------------
Total Transaction : 2
---------------------------------------------------------------------------------------------------------------------



TXID : 100                                                       DOMAIN (ID) : caller (1)
RESPONSE_TIME : 200                                              GUID : G1
EXTERNAL_CALL_TIME : %[1]d                                       USER_AGENT : test
APPLICATION : /prod/order/create

---------------------------------------------------------------------------------------------------------------------
[ No.][ START_TIME ][  GAP][CPU_T]
---------------------------------------------------------------------------------------------------------------------
[0000][10:00:00 000][    0][    0] START
[0001][10:00:00 010][    0][    0] EXTERNAL_CALL [HTTP] APACHE_HTTP_CLIENT_V5 ( url=/dev/svc/a) [%[1]dms]
[    ][10:00:00 100][    0][    0] END
---------------------------------------------------------------------------------------------------------------------
                TOTAL[%[1]d][   0]



TXID : 200                                                       DOMAIN (ID) : callee (2)
RESPONSE_TIME : %[2]d                                            GUID : G1
APPLICATION : /dev/svc/a

---------------------------------------------------------------------------------------------------------------------
[ No.][ START_TIME ][  GAP][CPU_T]
---------------------------------------------------------------------------------------------------------------------
[0000][10:00:00 020][    0][    0] START
[    ][10:00:00 100][    0][    0] END
---------------------------------------------------------------------------------------------------------------------
                TOTAL[%[2]d][   0]
`, int(callerElapsed), int(calleeRT))
	return jenniferprofile.ParseString(body, jenniferprofile.Options{})
}

// Acceptance #4 — header.EXTERNAL_CALL_TIME == sum(body.EXTERNAL_CALL.elapsed).
// Sample profile #1: header EXTERNAL_CALL_TIME=173, body has one
// EXTERNAL_CALL with elapsed=173 → no warning emitted.
func TestAnalyzeFile_ExternalCallSumMatches(t *testing.T) {
	res, err := AnalyzeFile(filepath.Join(testFixture), Options{})
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}
	if got := res.Type; got != ResultType {
		t.Errorf("type = %q, want %q", got, ResultType)
	}
	if res.Summary["external_call_count"].(int) != 1 {
		t.Errorf("external_call_count = %v, want 1", res.Summary["external_call_count"])
	}
	if res.Summary["external_call_cum_ms"].(int) != 173 {
		t.Errorf("external_call_cum_ms = %v, want 173", res.Summary["external_call_cum_ms"])
	}
}

// Acceptance #8 — SQL / Check Query / 2PC / Fetch counts don't
// double-add. Sample profile #1 has:
//   - 1 CHECK_QUERY (`select 1 from dual`)
//   - 3 TWO_PC (xa_start, xa_end, xa_prepare)
//   - 2 SQL_EXECUTE (UPDATE INSERT JOBS, QUERY SELECT a,b,c)
//   - 1 FETCH
// SQL_EXECUTE_GENERIC should NOT include the check-query or 2PC
// rows.
func TestAnalyzeFile_NoDoubleCounting(t *testing.T) {
	res, err := AnalyzeFile(filepath.Join(testFixture), Options{})
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}
	rows := res.Tables["profiles"].([]map[string]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	bm := rows[0]["body_metrics"].(map[string]any)
	if got := bm["sql_execute_count"].(int); got != 2 {
		t.Errorf("sql_execute_count = %d, want 2 (UPDATE+QUERY only)", got)
	}
	if got := bm["check_query_count"].(int); got != 1 {
		t.Errorf("check_query_count = %d, want 1", got)
	}
	if got := bm["two_pc_count"].(int); got != 3 {
		t.Errorf("two_pc_count = %d, want 3 (start/end/prepare)", got)
	}
	if got := bm["fetch_count"].(int); got != 1 {
		t.Errorf("fetch_count = %d, want 1", got)
	}
	if got := bm["fetch_total_rows"].(int); got != 1 {
		t.Errorf("fetch_total_rows = %d, want 1", got)
	}
	// 2 SQL_EXECUTE × 2503ms = 5006
	if got := bm["sql_execute_cum_ms"].(int); got != 5006 {
		t.Errorf("sql_execute_cum_ms = %d, want 5006", got)
	}
	// 1 CHECK_QUERY × 1ms = 1
	if got := bm["check_query_cum_ms"].(int); got != 1 {
		t.Errorf("check_query_cum_ms = %d, want 1", got)
	}
}

// Header-vs-body validation — sample profile #1 SQL_COUNT=5 in header
// but body has 6 sql-class events (1 check-query + 3 xa + 2
// sql-execute) → SQL_COUNT_MISMATCH warning expected.
// EXTERNAL_CALL_TIME and FETCH_TIME match exactly so no warnings
// from those.
func TestAnalyzeFile_HeaderBodyMismatch(t *testing.T) {
	res, err := AnalyzeFile(filepath.Join(testFixture), Options{})
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}
	rows := res.Tables["profiles"].([]map[string]any)
	warns := rows[0]["warnings"].([]map[string]any)
	gotCodes := map[string]bool{}
	for _, w := range warns {
		gotCodes[w["code"].(string)] = true
	}
	if !gotCodes["SQL_COUNT_MISMATCH"] {
		t.Errorf("expected SQL_COUNT_MISMATCH warning, got %v", warns)
	}
	if gotCodes["EXTERNAL_CALL_TIME_MISMATCH"] {
		t.Errorf("did not expect EXTERNAL_CALL_TIME_MISMATCH (header=173 == body sum=173)")
	}
	if gotCodes["FETCH_TIME_MISMATCH"] {
		t.Errorf("did not expect FETCH_TIME_MISMATCH (header=20 == body=20)")
	}
}

// Acceptance #9 — profiles sharing a GUID are grouped together.
// The fixture's two TXIDs share guid 2026050908316771PCMONLxtz8x0000001
// → exactly one GUID group with profile_count=2.
func TestAnalyzeFile_GuidGrouping(t *testing.T) {
	res, err := AnalyzeFile(testFixture, Options{})
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}
	if got := res.Summary["guid_group_count"].(int); got != 1 {
		t.Errorf("guid_group_count = %d, want 1", got)
	}
	groups := res.Series["guid_groups"].([]map[string]any)
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	g := groups[0]
	if got := g["profile_count"].(int); got != 2 {
		t.Errorf("profile_count = %d, want 2", got)
	}
	if got := g["guid"].(string); got != "2026050908316771PCMONLxtz8x0000001" {
		t.Errorf("guid = %q", got)
	}
}

// Acceptance #10 — EXTERNAL_CALL.url == Callee.APPLICATION matches.
// TX1 has EXTERNAL_CALL url=/dev/common/rule/testrule and TX2 has
// APPLICATION=/dev/common/rule/testrule, so the matcher should
// produce 1 matched edge (TX1 → TX2).
func TestAnalyzeFile_ExternalCallMatched(t *testing.T) {
	res, err := AnalyzeFile(testFixture, Options{})
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}
	if got := res.Summary["matched_external_call_count"].(int); got != 1 {
		t.Errorf("matched_external_call_count = %d, want 1", got)
	}
	if got := res.Summary["unmatched_external_call_count"].(int); got != 0 {
		t.Errorf("unmatched_external_call_count = %d, want 0", got)
	}
	edges := res.Tables["msa_edges"].([]map[string]any)
	if len(edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(edges))
	}
	e := edges[0]
	if got := e["match_status"].(string); got != "MATCHED" {
		t.Errorf("match_status = %q", got)
	}
	if got := e["callee_application"].(string); got != "/dev/common/rule/testrule" {
		t.Errorf("callee_application = %q", got)
	}
}

// Acceptance #11 — Network Gap = caller.elapsed - callee.responseTime.
// TX1.EXTERNAL_CALL.elapsed=173, TX2.RESPONSE_TIME=84, so gap=89.
func TestAnalyzeFile_NetworkGap(t *testing.T) {
	res, err := AnalyzeFile(testFixture, Options{})
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}
	if got := res.Summary["total_network_gap_cum_ms"].(int); got != 89 {
		t.Errorf("total_network_gap_cum_ms = %d, want 89", got)
	}
	edges := res.Tables["msa_edges"].([]map[string]any)
	e := edges[0]
	if got := e["raw_network_gap_ms"].(int); got != 89 {
		t.Errorf("raw_network_gap_ms = %d, want 89", got)
	}
	if got := e["adjusted_network_gap_ms"].(int); got != 89 {
		t.Errorf("adjusted_network_gap_ms = %d, want 89", got)
	}
}

// §17 root profile detection — TX1 is not a callee anywhere, TX2 is.
// Root therefore picks TX1.
func TestAnalyzeFile_RootDetection(t *testing.T) {
	res, err := AnalyzeFile(testFixture, Options{})
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}
	groups := res.Series["guid_groups"].([]map[string]any)
	g := groups[0]
	if got := g["root_txid"].(string); got != "922846047875413046" {
		t.Errorf("root_txid = %q, want TX1 (922846047875413046)", got)
	}
	if got := g["root_application"].(string); got != "/prod/common/rule/testrule" {
		t.Errorf("root_application = %q", got)
	}
}

// §15.2 negative gap — when callee.responseTime > caller.elapsed the
// raw value goes negative; adjusted is clamped to 0 and the edge
// carries NEGATIVE_NETWORK_GAP_ADJUSTED.
func TestAnalyzeFile_NegativeGapAdjusted(t *testing.T) {
	src := buildTwoProfileFixture(callerElapsedMs(50), calleeResponseMs(100))
	res := Build([]jenniferprofile.FileResult{src}, Options{})
	edges := res.Tables["msa_edges"].([]map[string]any)
	if len(edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(edges))
	}
	e := edges[0]
	raw := e["raw_network_gap_ms"].(int)
	adj := e["adjusted_network_gap_ms"].(int)
	if raw != -50 {
		t.Errorf("raw = %d, want -50", raw)
	}
	if adj != 0 {
		t.Errorf("adjusted = %d, want 0", adj)
	}
	warns, _ := e["warnings"].([]string)
	hasFlag := false
	for _, w := range warns {
		if w == "NEGATIVE_NETWORK_GAP_ADJUSTED" {
			hasFlag = true
			break
		}
	}
	if !hasFlag {
		t.Errorf("expected NEGATIVE_NETWORK_GAP_ADJUSTED in warnings, got %v", warns)
	}
}

// Acceptance #12 — multiple GUID groups sharing the same call
// structure fold into ONE signature; per-metric stats expose count,
// avg, and p95 across the GUIDs. Three synthetic invocations of
// `/prod/order/create → /dev/svc/a` with elapsed=100/200/300 give
// avg=200 and p95=300 (nearest-rank: ceil(0.95 * 3) - 1 = 2).
func TestAnalyzeFile_SignatureStats(t *testing.T) {
	files := []jenniferprofile.FileResult{
		buildSignatureFixture("G1", 100, 50),
		buildSignatureFixture("G2", 200, 100),
		buildSignatureFixture("G3", 300, 150),
	}
	res := Build(files, Options{})

	if got := res.Summary["unique_signature_count"].(int); got != 1 {
		t.Errorf("unique_signature_count = %d, want 1", got)
	}
	if got := res.Summary["guid_group_count"].(int); got != 3 {
		t.Errorf("guid_group_count = %d, want 3", got)
	}

	sigs := res.Series["signature_statistics"].([]map[string]any)
	if len(sigs) != 1 {
		t.Fatalf("signatures = %d, want 1", len(sigs))
	}
	sig := sigs[0]
	if got := sig["sample_count"].(int); got != 3 {
		t.Errorf("sample_count = %d, want 3", got)
	}
	guids := sig["guids"].([]string)
	if len(guids) != 3 {
		t.Errorf("guids = %d, want 3", len(guids))
	}
	if got := sig["root_application"].(string); got != "/prod/order/create" {
		t.Errorf("root_application = %q", got)
	}

	metrics := sig["metrics"].(map[string]any)
	ext := metrics["total_external_call_cumulative_ms"].(map[string]any)
	if got := ext["count"].(int); got != 3 {
		t.Errorf("ext count = %d, want 3", got)
	}
	if got := ext["avg"].(float64); got != 200 {
		t.Errorf("ext avg = %v, want 200", got)
	}
	// nearest-rank p95 over [100,200,300]: ceil(0.95*3)=3 → idx=2 → 300
	if got := ext["p95"].(float64); got != 300 {
		t.Errorf("ext p95 = %v, want 300", got)
	}
	if got := ext["max"].(float64); got != 300 {
		t.Errorf("ext max = %v, want 300", got)
	}

	// Edge stats: three observations of caller→callee edge with
	// gaps 50/100/150 → avg=100, p95=150.
	edges := sig["edges"].([]map[string]any)
	if len(edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(edges))
	}
	gap := edges[0]["adjusted_network_gap_ms"].(map[string]any)
	if got := gap["count"].(int); got != 3 {
		t.Errorf("gap count = %d, want 3", got)
	}
	if got := gap["avg"].(float64); got != 100 {
		t.Errorf("gap avg = %v, want 100", got)
	}
	if got := gap["p95"].(float64); got != 150 {
		t.Errorf("gap p95 = %v, want 150", got)
	}
}

// buildSignatureFixture is the inline-fixture helper used by the
// signature-stats test. callerElapsed and gapMs are the controllable
// knobs — callee responseTime is derived as callerElapsed - gapMs
// so the matcher always pairs cleanly.
func buildSignatureFixture(guid string, callerElapsed int, gapMs int) jenniferprofile.FileResult {
	calleeRT := callerElapsed - gapMs
	body := fmt.Sprintf(`---------------------------------------------------------------------------------------------------------------------
Total Transaction : 2
---------------------------------------------------------------------------------------------------------------------



TXID : 100%[1]s                                                  DOMAIN (ID) : caller (1)
RESPONSE_TIME : %[2]d                                            GUID : %[1]s
EXTERNAL_CALL_TIME : %[2]d                                       USER_AGENT : test
APPLICATION : /prod/order/create

---------------------------------------------------------------------------------------------------------------------
[ No.][ START_TIME ][  GAP][CPU_T]
---------------------------------------------------------------------------------------------------------------------
[0000][10:00:00 000][    0][    0] START
[0001][10:00:00 010][    0][    0] EXTERNAL_CALL [HTTP] APACHE_HTTP_CLIENT_V5 ( url=/dev/svc/a) [%[2]dms]
[    ][10:00:00 100][    0][    0] END
---------------------------------------------------------------------------------------------------------------------
                TOTAL[%[2]d][   0]



TXID : 200%[1]s                                                  DOMAIN (ID) : callee (2)
RESPONSE_TIME : %[3]d                                            GUID : %[1]s
APPLICATION : /dev/svc/a

---------------------------------------------------------------------------------------------------------------------
[ No.][ START_TIME ][  GAP][CPU_T]
---------------------------------------------------------------------------------------------------------------------
[0000][10:00:00 020][    0][    0] START
[    ][10:00:00 100][    0][    0] END
---------------------------------------------------------------------------------------------------------------------
                TOTAL[%[3]d][   0]
`, guid, callerElapsed, calleeRT)
	return jenniferprofile.ParseString(body, jenniferprofile.Options{})
}

// Acceptance #13 — overlapping External Call intervals are detected
// as PARALLEL. Two calls 0~200ms and 50~250ms (one caller) overlap;
// the profile-level mode flips to PARALLEL and maxConcurrency=2.
func TestAnalyzeFile_ParallelCallsDetected(t *testing.T) {
	src := buildParallelFixture()
	res := Build([]jenniferprofile.FileResult{src}, Options{})
	groups := res.Series["guid_groups"].([]map[string]any)
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	m := groups[0]["metrics"].(map[string]any)
	if got := m["group_execution_mode"].(string); got != "PARALLEL" {
		t.Errorf("group_execution_mode = %q, want PARALLEL", got)
	}
	if got := m["max_external_call_concurrency"].(int); got != 2 {
		t.Errorf("max_concurrency = %d, want 2", got)
	}
}

// Acceptance #14 — for parallel calls, cumulative_ms and
// wall_time_ms diverge. 0~200 + 50~250 = cumulative 400 ms but
// wall-clock = 250 ms (union). Parallelism ratio = 400/250 = 1.6.
func TestAnalyzeFile_CumulativeVsWallClockSeparated(t *testing.T) {
	src := buildParallelFixture()
	res := Build([]jenniferprofile.FileResult{src}, Options{})
	groups := res.Series["guid_groups"].([]map[string]any)
	m := groups[0]["metrics"].(map[string]any)
	cum := m["total_external_call_cumulative_ms"].(int)
	wall := m["total_external_call_wall_time_ms"].(int)
	if cum != 400 {
		t.Errorf("cumulative = %d, want 400", cum)
	}
	if wall != 250 {
		t.Errorf("wall-clock = %d, want 250", wall)
	}
	ratio := m["external_call_parallelism_ratio"].(float64)
	if ratio < 1.59 || ratio > 1.61 {
		t.Errorf("parallelism_ratio = %v, want ~1.6", ratio)
	}
}

// Sequential baseline — when calls don't overlap, cumulative ==
// wall-clock and parallelism ratio is exactly 1.0.
func TestAnalyzeFile_SequentialBaseline(t *testing.T) {
	res, err := AnalyzeFile(testFixture, Options{})
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}
	groups := res.Series["guid_groups"].([]map[string]any)
	m := groups[0]["metrics"].(map[string]any)
	if got := m["group_execution_mode"].(string); got != "SEQUENTIAL" {
		t.Errorf("execution_mode = %q, want SEQUENTIAL", got)
	}
	if got := m["external_call_parallelism_ratio"].(float64); got != 1.0 {
		t.Errorf("parallelism_ratio = %v, want 1.0", got)
	}
}

// buildParallelFixture synthesises one Jennifer-shape transaction
// with two overlapping EXTERNAL_CALL events (0~200ms and 50~250ms)
// + a callee profile so the matcher pairs cleanly. The callee
// matches both — the matcher's one-to-one rule will pick one and
// leave the other UNMATCHED, which is fine; we're testing that the
// PARALLEL detection runs on the body intervals regardless.
func buildParallelFixture() jenniferprofile.FileResult {
	body := `---------------------------------------------------------------------------------------------------------------------
Total Transaction : 2
---------------------------------------------------------------------------------------------------------------------



TXID : C1                                                        DOMAIN (ID) : caller (1)
RESPONSE_TIME : 300                                              GUID : GP1
EXTERNAL_CALL_TIME : 400                                         USER_AGENT : test
APPLICATION : /prod/order/create

---------------------------------------------------------------------------------------------------------------------
[ No.][ START_TIME ][  GAP][CPU_T]
---------------------------------------------------------------------------------------------------------------------
[0000][10:00:00 000][    0][    0] START
[0001][10:00:00 000][    0][    0] EXTERNAL_CALL [HTTP] APACHE_HTTP_CLIENT_V5 ( url=/dev/svc/a) [200ms]
[0002][10:00:00 050][    0][    0] EXTERNAL_CALL [HTTP] APACHE_HTTP_CLIENT_V5 ( url=/dev/svc/b) [200ms]
[    ][10:00:00 250][    0][    0] END
---------------------------------------------------------------------------------------------------------------------
                TOTAL[ 250][   0]



TXID : C2                                                        DOMAIN (ID) : callee (2)
RESPONSE_TIME : 200                                              GUID : GP1
APPLICATION : /dev/svc/a

---------------------------------------------------------------------------------------------------------------------
[ No.][ START_TIME ][  GAP][CPU_T]
---------------------------------------------------------------------------------------------------------------------
[0000][10:00:00 000][    0][    0] START
[    ][10:00:00 200][    0][    0] END
---------------------------------------------------------------------------------------------------------------------
                TOTAL[ 200][   0]
`
	return jenniferprofile.ParseString(body, jenniferprofile.Options{})
}

// Multi-file batch — Build accepts multiple FileResult and folds
// counts together.
func TestAnalyzeFiles_BatchSums(t *testing.T) {
	parsed1, err := jenniferprofile.ParseFile(testFixture, jenniferprofile.Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	res := Build([]jenniferprofile.FileResult{parsed1, parsed1}, Options{})
	if got := res.Summary["total_files"].(int); got != 2 {
		t.Errorf("total_files = %d, want 2", got)
	}
	if got := res.Summary["total_profiles"].(int); got != 4 {
		t.Errorf("total_profiles = %d, want 4 (2 files × 2 profiles)", got)
	}
	if got := res.Summary["external_call_count"].(int); got != 2 {
		t.Errorf("external_call_count = %d, want 2 (one per file)", got)
	}
}
