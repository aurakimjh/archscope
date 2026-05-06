package reportdiff

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// sampleResult builds a minimal access-log-shaped AnalysisResult dict
// (already projected to map[string]any) so tests don't have to worry
// about encoding/json round-tripping.
func sampleResult(t *testing.T, opts ...func(map[string]any)) map[string]any {
	t.Helper()
	r := map[string]any{
		"type":         "access_log",
		"source_files": []any{"sample.log"},
		"created_at":   "2026-04-29T10:00:00Z",
		"summary": map[string]any{
			"total_requests":  100.0,
			"avg_response_ms": 250.0,
			"format_label":    "nginx_combined", // non-numeric — must be skipped
		},
		"series": map[string]any{},
		"tables": map[string]any{},
		"charts": map[string]any{},
		"metadata": map[string]any{
			"parser":         "nginx_combined",
			"schema_version": "0.1.0",
			"findings":       []any{},
		},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// TestDiffSameResultZeroDelta verifies that diffing a result against
// itself yields zero changed metrics and no finding delta — every
// metric row has delta == 0 and change_percent == 0.
func TestDiffSameResultZeroDelta(t *testing.T) {
	r := sampleResult(t)

	report, err := Diff(r, r, "")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	summary := report["summary"].(map[string]any)
	if summary["changed_metrics"].(int) != 0 {
		t.Errorf("changed_metrics = %v, want 0", summary["changed_metrics"])
	}
	if summary["finding_delta"].(int) != 0 {
		t.Errorf("finding_delta = %v, want 0", summary["finding_delta"])
	}
	if summary["before_findings"].(int) != 0 {
		t.Errorf("before_findings = %v, want 0", summary["before_findings"])
	}

	rows := report["series"].(map[string]any)["summary_metric_deltas"].([]map[string]any)
	if len(rows) != 2 {
		t.Fatalf("metric rows = %d, want 2 (numeric metrics only)", len(rows))
	}
	for _, row := range rows {
		if row["delta"].(float64) != 0 {
			t.Errorf("row %s delta = %v, want 0", row["metric"], row["delta"])
		}
	}
}

// TestDiffNumericChangeCaptured exercises the per-metric numeric
// diff: before/after/delta/change_percent should all be present and
// match Python's rounding (delta = 6 dp, change_percent = 4 dp).
func TestDiffNumericChangeCaptured(t *testing.T) {
	before := sampleResult(t)
	after := sampleResult(t, func(r map[string]any) {
		s := r["summary"].(map[string]any)
		s["total_requests"] = 150.0      // +50 / +50%
		s["avg_response_ms"] = 200.12345 // -49.87655 / -19.95062%
	})

	report, err := Diff(before, after, "")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	summary := report["summary"].(map[string]any)
	if summary["changed_metrics"].(int) != 2 {
		t.Errorf("changed_metrics = %v, want 2", summary["changed_metrics"])
	}

	rows := report["series"].(map[string]any)["summary_metric_deltas"].([]map[string]any)
	if len(rows) != 2 {
		t.Fatalf("metric rows = %d, want 2", len(rows))
	}

	rowByMetric := map[string]map[string]any{}
	for _, row := range rows {
		rowByMetric[row["metric"].(string)] = row
	}

	tr := rowByMetric["total_requests"]
	if tr == nil {
		t.Fatal("total_requests row missing")
	}
	if tr["before"].(float64) != 100 {
		t.Errorf("total_requests.before = %v", tr["before"])
	}
	if tr["after"].(float64) != 150 {
		t.Errorf("total_requests.after = %v", tr["after"])
	}
	if tr["delta"].(float64) != 50 {
		t.Errorf("total_requests.delta = %v", tr["delta"])
	}
	if tr["change_percent"].(float64) != 50 {
		t.Errorf("total_requests.change_percent = %v, want 50", tr["change_percent"])
	}

	avg := rowByMetric["avg_response_ms"]
	if avg == nil {
		t.Fatal("avg_response_ms row missing")
	}
	gotDelta := avg["delta"].(float64)
	wantDelta := -49.87655
	if math.Abs(gotDelta-wantDelta) > 1e-9 {
		t.Errorf("avg_response_ms.delta = %v, want %v", gotDelta, wantDelta)
	}
	gotPct := avg["change_percent"].(float64)
	wantPct := -19.9506 // 4 dp
	if math.Abs(gotPct-wantPct) > 1e-4 {
		t.Errorf("avg_response_ms.change_percent = %v, want ≈ %v", gotPct, wantPct)
	}
}

// TestDiffSkipsNonNumericAndOneSidedMetrics replicates Python's
// numeric-only filter: string metrics, missing-on-one-side metrics,
// and bool metrics are silently skipped.
func TestDiffSkipsNonNumericAndOneSidedMetrics(t *testing.T) {
	before := map[string]any{
		"summary": map[string]any{
			"requests":       100.0,
			"only_in_before": 1.0,
			"label":          "before",
		},
	}
	after := map[string]any{
		"summary": map[string]any{
			"requests":      120.0,
			"only_in_after": 2.0,
			"label":         "after",
		},
	}

	report, err := Diff(before, after, "")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	rows := report["series"].(map[string]any)["summary_metric_deltas"].([]map[string]any)
	if len(rows) != 1 {
		t.Fatalf("metric rows = %d, want 1 (only `requests` is numeric on both sides)", len(rows))
	}
	if rows[0]["metric"] != "requests" {
		t.Errorf("metric = %v, want requests", rows[0]["metric"])
	}
}

// TestDiffChangePercentNullWhenBeforeZero — Python returns None when
// the before value is exactly 0; we emit nil so JSON marshals null.
func TestDiffChangePercentNullWhenBeforeZero(t *testing.T) {
	before := map[string]any{"summary": map[string]any{"errors": 0.0}}
	after := map[string]any{"summary": map[string]any{"errors": 5.0}}

	report, err := Diff(before, after, "")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	rows := report["series"].(map[string]any)["summary_metric_deltas"].([]map[string]any)
	if len(rows) != 1 {
		t.Fatalf("metric rows = %d", len(rows))
	}
	if rows[0]["change_percent"] != nil {
		t.Errorf("change_percent = %v, want nil (Python None)", rows[0]["change_percent"])
	}

	// Confirm JSON renders as null.
	body, err := Marshal(before, after)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), `"change_percent": null`) {
		t.Errorf("expected null change_percent in JSON; body:\n%s", body)
	}
}

// TestDiffFindingCountComparison exercises the finding-count diff:
// added findings increment after_findings, removed findings decrement.
func TestDiffFindingCountComparison(t *testing.T) {
	before := sampleResult(t, func(r map[string]any) {
		r["metadata"].(map[string]any)["findings"] = []any{
			map[string]any{"severity": "high", "code": "F001", "message": "boom"},
			map[string]any{"severity": "medium", "code": "F002", "message": "warn"},
		}
	})
	after := sampleResult(t, func(r map[string]any) {
		// F001 removed, F003 added — count goes 2 → 2 with churn,
		// but we only diff counts, so finding_delta should be 0.
		r["metadata"].(map[string]any)["findings"] = []any{
			map[string]any{"severity": "medium", "code": "F002", "message": "warn"},
			map[string]any{"severity": "low", "code": "F003", "message": "info"},
		}
	})

	report, err := Diff(before, after, "")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	summary := report["summary"].(map[string]any)
	if summary["before_findings"].(int) != 2 {
		t.Errorf("before_findings = %v, want 2", summary["before_findings"])
	}
	if summary["after_findings"].(int) != 2 {
		t.Errorf("after_findings = %v, want 2", summary["after_findings"])
	}
	if summary["finding_delta"].(int) != 0 {
		t.Errorf("finding_delta = %v, want 0 (count-only diff)", summary["finding_delta"])
	}

	// Now an actual count change.
	after2 := sampleResult(t, func(r map[string]any) {
		r["metadata"].(map[string]any)["findings"] = []any{
			map[string]any{"severity": "high", "code": "F001"},
			map[string]any{"severity": "medium", "code": "F002"},
			map[string]any{"severity": "low", "code": "F003"},
		}
	})
	report2, err := Diff(before, after2, "")
	if err != nil {
		t.Fatalf("Diff (added): %v", err)
	}
	summary2 := report2["summary"].(map[string]any)
	if summary2["finding_delta"].(int) != 1 {
		t.Errorf("finding_delta = %v, want 1 (one added)", summary2["finding_delta"])
	}

	// Removal case.
	after3 := sampleResult(t, func(r map[string]any) {
		r["metadata"].(map[string]any)["findings"] = []any{}
	})
	report3, err := Diff(before, after3, "")
	if err != nil {
		t.Fatalf("Diff (removed): %v", err)
	}
	summary3 := report3["summary"].(map[string]any)
	if summary3["finding_delta"].(int) != -2 {
		t.Errorf("finding_delta = %v, want -2 (both removed)", summary3["finding_delta"])
	}

	// finding_count_comparison rows.
	rows := report3["series"].(map[string]any)["finding_count_comparison"].([]map[string]any)
	if len(rows) != 2 {
		t.Fatalf("finding_count_comparison rows = %d, want 2", len(rows))
	}
	if rows[0]["side"] != "before" || rows[0]["finding_count"].(int) != 2 {
		t.Errorf("rows[0] = %v", rows[0])
	}
	if rows[1]["side"] != "after" || rows[1]["finding_count"].(int) != 0 {
		t.Errorf("rows[1] = %v", rows[1])
	}
}

// TestDiffDifferentResultTypes — Python doesn't error on different
// `type` fields; it just records them in summary.before_type /
// summary.after_type. We follow Python.
func TestDiffDifferentResultTypes(t *testing.T) {
	before := map[string]any{
		"type":    "access_log",
		"summary": map[string]any{"requests": 100.0},
	}
	after := map[string]any{
		"type":    "thread_dump",
		"summary": map[string]any{"requests": 100.0}, // unrelated metric, but matches name
	}

	report, err := Diff(before, after, "comparison")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	summary := report["summary"].(map[string]any)
	if summary["before_type"] != "access_log" {
		t.Errorf("before_type = %v", summary["before_type"])
	}
	if summary["after_type"] != "thread_dump" {
		t.Errorf("after_type = %v", summary["after_type"])
	}
	if summary["label"] != "comparison" {
		t.Errorf("label = %v, want comparison", summary["label"])
	}
}

// TestDiffEmptyLabelLeavesNil — passing label="" matches Python's
// `label=None` default and should marshal as JSON null.
func TestDiffEmptyLabelLeavesNil(t *testing.T) {
	before := sampleResult(t)
	after := sampleResult(t)
	report, err := Diff(before, after, "")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	summary := report["summary"].(map[string]any)
	if summary["label"] != nil {
		t.Errorf("label = %v, want nil", summary["label"])
	}

	body, _ := Marshal(before, after)
	if !strings.Contains(string(body), `"label": null`) {
		t.Errorf("expected null label in JSON; body:\n%s", body)
	}
}

// TestDiffAcceptsTypedAnalysisResult confirms the `toMap` round-trip
// works for the Go `models.AnalysisResult` struct (which has a
// custom Metadata.MarshalJSON), so callers don't have to convert
// to map[string]any themselves.
func TestDiffAcceptsTypedAnalysisResult(t *testing.T) {
	before := models.New("access_log", "nginx_combined")
	before.Summary["requests"] = 100
	before.AddFinding("info", "F001", "ok", nil)

	after := models.New("access_log", "nginx_combined")
	after.Summary["requests"] = 120
	after.AddFinding("info", "F001", "ok", nil)
	after.AddFinding("high", "F002", "boom", nil)

	report, err := Diff(before, after, "ci-run")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	summary := report["summary"].(map[string]any)
	if summary["before_findings"].(int) != 1 {
		t.Errorf("before_findings = %v, want 1", summary["before_findings"])
	}
	if summary["after_findings"].(int) != 2 {
		t.Errorf("after_findings = %v, want 2", summary["after_findings"])
	}
	if summary["finding_delta"].(int) != 1 {
		t.Errorf("finding_delta = %v, want 1", summary["finding_delta"])
	}
	if summary["changed_metrics"].(int) != 1 {
		t.Errorf("changed_metrics = %v, want 1", summary["changed_metrics"])
	}
	if summary["label"] != "ci-run" {
		t.Errorf("label = %v, want ci-run", summary["label"])
	}
}

// TestWriteRoundTrip writes the diff to disk and verifies the JSON
// round-trips back to the same shape. Also confirms parent dirs are
// created and the output ends with a trailing newline (T-340 parity).
func TestWriteRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "deeply", "nested", "diff.json")

	before := sampleResult(t)
	after := sampleResult(t, func(r map[string]any) {
		r["summary"].(map[string]any)["total_requests"] = 200.0
	})

	if err := Write(out, before, after); err != nil {
		t.Fatalf("Write: %v", err)
	}

	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasSuffix(string(body), "\n") {
		t.Errorf("output should end with newline")
	}

	var loaded map[string]any
	if err := json.Unmarshal(body, &loaded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if loaded["type"] != "comparison_report" {
		t.Errorf("type = %v, want comparison_report", loaded["type"])
	}
	metadata := loaded["metadata"].(map[string]any)
	if metadata["report_kind"] != "before_after_diff" {
		t.Errorf("report_kind = %v", metadata["report_kind"])
	}
	if metadata["schema_version"] != "0.1.0" {
		t.Errorf("schema_version = %v", metadata["schema_version"])
	}
}

// TestBuildComparisonReportFromFiles mirrors the Python public API:
// reads two JSON files from disk and emits the comparison report.
func TestBuildComparisonReportFromFiles(t *testing.T) {
	tmp := t.TempDir()
	beforePath := filepath.Join(tmp, "before.json")
	afterPath := filepath.Join(tmp, "after.json")

	beforeBody, _ := json.Marshal(sampleResult(t))
	afterBody, _ := json.Marshal(sampleResult(t, func(r map[string]any) {
		r["summary"].(map[string]any)["total_requests"] = 250.0
	}))

	if err := os.WriteFile(beforePath, beforeBody, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(afterPath, afterBody, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := BuildComparisonReport(beforePath, afterPath, "release-2026-05")
	if err != nil {
		t.Fatalf("BuildComparisonReport: %v", err)
	}

	files := report["source_files"].([]string)
	if len(files) != 2 || files[0] != beforePath || files[1] != afterPath {
		t.Errorf("source_files = %v", files)
	}
	summary := report["summary"].(map[string]any)
	if summary["label"] != "release-2026-05" {
		t.Errorf("label = %v", summary["label"])
	}
	if summary["changed_metrics"].(int) != 1 {
		t.Errorf("changed_metrics = %v, want 1", summary["changed_metrics"])
	}
	metadata := report["metadata"].(map[string]any)
	if metadata["before_created_at"] != "2026-04-29T10:00:00Z" {
		t.Errorf("before_created_at = %v", metadata["before_created_at"])
	}
}

// TestBuildComparisonReportMissingFile surfaces a clear error when an
// input file doesn't exist (Python would raise FileNotFoundError).
func TestBuildComparisonReportMissingFile(t *testing.T) {
	_, err := BuildComparisonReport("/nonexistent/before.json", "/nonexistent/after.json", "")
	if err == nil {
		t.Fatal("expected error for missing files")
	}
}

// TestMarshalIs2SpaceIndented asserts the wire format matches T-340.
func TestMarshalIs2SpaceIndented(t *testing.T) {
	body, err := Marshal(map[string]any{}, map[string]any{})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), "  \"type\": \"comparison_report\"") {
		t.Errorf("expected 2-space indent; got:\n%s", body)
	}
}

// TestMetricRowsSortedByKey — Python sorts the metric union; ensure
// we do too so the output is byte-stable.
func TestMetricRowsSortedByKey(t *testing.T) {
	before := map[string]any{
		"summary": map[string]any{"z_metric": 1.0, "a_metric": 1.0, "m_metric": 1.0},
	}
	after := map[string]any{
		"summary": map[string]any{"z_metric": 2.0, "a_metric": 2.0, "m_metric": 2.0},
	}

	report, err := Diff(before, after, "")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	rows := report["series"].(map[string]any)["summary_metric_deltas"].([]map[string]any)
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	wantOrder := []string{"a_metric", "m_metric", "z_metric"}
	for i, w := range wantOrder {
		if rows[i]["metric"] != w {
			t.Errorf("rows[%d].metric = %v, want %s", i, rows[i]["metric"], w)
		}
	}
}
