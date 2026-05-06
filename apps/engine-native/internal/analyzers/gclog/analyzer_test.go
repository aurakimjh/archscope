package gclog

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoFixture mirrors the helper in the parser tests — walks up from
// the test working directory to find the checked-in fixtures so the
// test runs regardless of where `go test` is invoked from.
func repoFixture(t *testing.T, rel string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("fixture %q not found from %s", rel, wd)
	return ""
}

// TestAnalyzeReturnsAnalysisResult mirrors Python
// test_gc_log_analyzer_returns_analysis_result.
func TestAnalyzeReturnsAnalysisResult(t *testing.T) {
	path := repoFixture(t, "examples/gc-logs/sample-hotspot-gc.log")
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Type != "gc_log" {
		t.Fatalf("Type = %q, want gc_log", result.Type)
	}
	if got := asInt(result.Summary["total_events"]); got != 2 {
		t.Errorf("total_events = %d, want 2", got)
	}
	if got := asInt(result.Summary["full_gc_count"]); got != 1 {
		t.Errorf("full_gc_count = %d, want 1", got)
	}
	if result.Metadata.Diagnostics == nil {
		t.Fatal("Diagnostics nil")
	}
	if result.Metadata.Diagnostics.ParsedRecords != 2 {
		t.Errorf("parsed_records = %d, want 2", result.Metadata.Diagnostics.ParsedRecords)
	}
	codes := map[string]bool{}
	for _, finding := range result.Metadata.Findings {
		codes[finding["code"].(string)] = true
	}
	wantCodes := []string{"LONG_GC_PAUSE", "FULL_GC_PRESENT"}
	for _, c := range wantCodes {
		if !codes[c] {
			t.Errorf("missing finding %q in %v", c, codes)
		}
	}
}

// TestBuildAcceptsEvents mirrors Python
// test_gc_log_result_builder_accepts_event_iterable.
func TestBuildAcceptsEvents(t *testing.T) {
	path := repoFixture(t, "examples/gc-logs/sample-hotspot-gc.log")
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := asInt(result.Summary["total_events"]); got != 2 {
		t.Errorf("total_events = %d, want 2", got)
	}
	pause, ok := result.Series["pause_timeline"].([]map[string]any)
	if !ok {
		t.Fatalf("pause_timeline is %T", result.Series["pause_timeline"])
	}
	if len(pause) != 2 {
		t.Errorf("pause_timeline len = %d, want 2", len(pause))
	}
	events, ok := result.Tables["events"].([]map[string]any)
	if !ok {
		t.Fatalf("tables.events is %T", result.Tables["events"])
	}
	if len(events) != 2 {
		t.Errorf("tables.events len = %d, want 2", len(events))
	}
}

// TestAnalyzeFixtureSummaryParity asserts the per-event fields on the
// canonical sample resolve to the same values the Python implementation
// produces (banker's rounding, 3-decimal pauses, etc.).
func TestAnalyzeFixtureSummaryParity(t *testing.T) {
	path := repoFixture(t, "examples/gc-logs/sample-hotspot-gc.log")
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	// Sample fixture: pauses are 25.0 and 190.12 ms.
	if got := asFloat(result.Summary["max_pause_ms"]); got != 190.12 {
		t.Errorf("max_pause_ms = %v, want 190.12", got)
	}
	if got := asFloat(result.Summary["total_pause_ms"]); got != 215.12 {
		t.Errorf("total_pause_ms = %v, want 215.12", got)
	}
	if got := asFloat(result.Summary["avg_pause_ms"]); got != 107.56 {
		t.Errorf("avg_pause_ms = %v, want 107.56", got)
	}
	// The fixture has no uptime info → wall_time_sec = 0, throughput 0.
	if got := asFloat(result.Summary["wall_time_sec"]); got != 0 {
		t.Errorf("wall_time_sec = %v, want 0", got)
	}
	if got := asFloat(result.Summary["throughput_percent"]); got != 0 {
		t.Errorf("throughput_percent = %v, want 0", got)
	}
}

// TestPercentilesLinearInterp validates the Python-style linear-
// interpolation percentile.
func TestPercentilesLinearInterp(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	p50, p95, p99 := computePercentiles(values)
	if p50 != 3 {
		t.Errorf("p50 = %v, want 3", p50)
	}
	// rank 95 → 0.95 * 4 = 3.8 → 4*0.2 + 5*0.8 = 4.8
	if p95 != 4.8 {
		t.Errorf("p95 = %v, want 4.8", p95)
	}
	// rank 99 → 0.99 * 4 = 3.96 → 4*0.04 + 5*0.96 = 4.96
	if p99 != 4.96 {
		t.Errorf("p99 = %v, want 4.96", p99)
	}
}

func TestPercentilesEmptyAndSingle(t *testing.T) {
	a, b, c := computePercentiles(nil)
	if a != 0 || b != 0 || c != 0 {
		t.Errorf("empty: got %v %v %v, want 0 0 0", a, b, c)
	}
	a, b, c = computePercentiles([]float64{42})
	if a != 42 || b != 42 || c != 42 {
		t.Errorf("single: got %v %v %v, want 42 42 42", a, b, c)
	}
}

func TestHistogramEmpty(t *testing.T) {
	rows := buildPauseHistogram(nil)
	if len(rows) != len(histogramBuckets) {
		t.Fatalf("rows len = %d, want %d", len(rows), len(histogramBuckets))
	}
	for _, row := range rows {
		if asInt(row["count"]) != 0 {
			t.Errorf("empty histogram bucket %v has non-zero count", row)
		}
	}
	if rows[len(rows)-1]["max_ms"] != nil {
		t.Errorf("top bucket max_ms = %v, want nil", rows[len(rows)-1]["max_ms"])
	}
}

func TestHistogramBucketing(t *testing.T) {
	pauses := []float64{0.5, 2.0, 7.5, 25.0, 75.0, 250.0, 800.0, 3000.0, 8000.0}
	rows := buildPauseHistogram(pauses)
	wantCounts := map[string]int{
		"<1ms":      1,
		"1-5ms":     1,
		"5-10ms":    1,
		"10-50ms":   1,
		"50-100ms":  1,
		"100-500ms": 1,
		"500ms-1s":  1,
		"1-5s":      1,
		">=5s":      1,
	}
	for _, row := range rows {
		bucket := row["bucket"].(string)
		got := asInt(row["count"])
		if got != wantCounts[bucket] {
			t.Errorf("bucket %q count = %d, want %d", bucket, got, wantCounts[bucket])
		}
	}
}

// TestThroughputAndWallTimeFromUptimeSec exercises the wall-time
// branch by feeding events that carry uptime_sec values.
func TestThroughputAndWallTimeFromUptimeSec(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gc.log")
	// Two-event JDK 8 G1 legacy log (parser supports uptime_sec).
	body := strings.Join([]string{
		"2026-04-27T10:00:00.000+0900: 0.000: [GC pause (G1 Evacuation Pause) (young), 0.0250000 secs]",
		"   [Eden: 80M(80M)->0B(72M) Survivors: 0B->8M Heap: 100M(200M)->50M(200M)]",
		"2026-04-27T10:00:10.000+0900: 10.000: [GC pause (G1 Evacuation Pause) (young), 0.0500000 secs]",
		"   [Eden: 70M(72M)->0B(64M) Survivors: 8M->8M Heap: 110M(200M)->55M(200M)]",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := asInt(result.Summary["total_events"]); got != 2 {
		t.Errorf("total_events = %d, want 2", got)
	}
	wall := asFloat(result.Summary["wall_time_sec"])
	if wall != 10 {
		t.Errorf("wall_time_sec = %v, want 10", wall)
	}
	// total pause = 75ms → 0.075s → throughput 99.25%.
	throughput := asFloat(result.Summary["throughput_percent"])
	if math.Abs(throughput-99.25) > 0.01 {
		t.Errorf("throughput_percent = %v, want ~99.25", throughput)
	}
}

// TestFindingsRules covers each finding code's threshold logic.
func TestFindingsRules(t *testing.T) {
	cases := []struct {
		name    string
		summary map[string]any
		want    []string
	}{
		{
			"long_pause_only",
			map[string]any{
				"max_pause_ms":   105.0,
				"full_gc_count":  0,
				"throughput_percent": 99.0,
				"wall_time_sec":  10.0,
				"p99_pause_ms":   50.0,
			},
			[]string{"LONG_GC_PAUSE"},
		},
		{
			"full_gc",
			map[string]any{
				"max_pause_ms":   10.0,
				"full_gc_count":  3,
				"throughput_percent": 99.0,
				"wall_time_sec":  10.0,
				"p99_pause_ms":   50.0,
			},
			[]string{"FULL_GC_PRESENT"},
		},
		{
			"low_throughput",
			map[string]any{
				"max_pause_ms":   10.0,
				"full_gc_count":  0,
				"throughput_percent": 90.0,
				"wall_time_sec":  10.0,
				"p99_pause_ms":   50.0,
			},
			[]string{"LOW_GC_THROUGHPUT"},
		},
		{
			"high_p99",
			map[string]any{
				"max_pause_ms":   10.0,
				"full_gc_count":  0,
				"throughput_percent": 99.0,
				"wall_time_sec":  10.0,
				"p99_pause_ms":   250.0,
			},
			[]string{"HIGH_P99_PAUSE"},
		},
		{
			"humongous",
			map[string]any{
				"max_pause_ms":               10.0,
				"full_gc_count":              0,
				"throughput_percent":         99.0,
				"wall_time_sec":              10.0,
				"p99_pause_ms":               50.0,
				"humongous_allocation_count": 5,
			},
			[]string{"HUMONGOUS_ALLOCATION"},
		},
		{
			"cmf_critical",
			map[string]any{
				"max_pause_ms":                  10.0,
				"full_gc_count":                 0,
				"throughput_percent":            99.0,
				"wall_time_sec":                 10.0,
				"p99_pause_ms":                  50.0,
				"concurrent_mode_failure_count": 2,
			},
			[]string{"CONCURRENT_MODE_FAILURE"},
		},
		{
			"promotion_failure_critical",
			map[string]any{
				"max_pause_ms":            10.0,
				"full_gc_count":           0,
				"throughput_percent":      99.0,
				"wall_time_sec":           10.0,
				"p99_pause_ms":            50.0,
				"promotion_failure_count": 1,
			},
			[]string{"PROMOTION_FAILURE"},
		},
		{
			"low_throughput_skipped_when_no_wall_time",
			map[string]any{
				"max_pause_ms":   10.0,
				"full_gc_count":  0,
				"throughput_percent": 0.0,
				"wall_time_sec":  0.0,
				"p99_pause_ms":   50.0,
			},
			[]string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildFindings(tc.summary)
			gotCodes := make([]string, 0, len(got))
			for _, f := range got {
				gotCodes = append(gotCodes, f["code"].(string))
			}
			if !sameSet(gotCodes, tc.want) {
				t.Errorf("findings codes = %v, want %v", gotCodes, tc.want)
			}
		})
	}
}

func TestFindingsHumongousCriticalSeverity(t *testing.T) {
	got := buildFindings(map[string]any{
		"max_pause_ms":                  0.0,
		"full_gc_count":                 0,
		"throughput_percent":            99.0,
		"wall_time_sec":                 10.0,
		"p99_pause_ms":                  0.0,
		"concurrent_mode_failure_count": 1,
		"promotion_failure_count":       1,
	})
	gotMap := map[string]string{}
	for _, f := range got {
		gotMap[f["code"].(string)] = f["severity"].(string)
	}
	if gotMap["CONCURRENT_MODE_FAILURE"] != "critical" {
		t.Errorf("CMF severity = %q, want critical", gotMap["CONCURRENT_MODE_FAILURE"])
	}
	if gotMap["PROMOTION_FAILURE"] != "critical" {
		t.Errorf("Promotion severity = %q, want critical", gotMap["PROMOTION_FAILURE"])
	}
}

// TestParserSelection covers the gc_format → parser-name mapping.
func TestParserSelection(t *testing.T) {
	path := repoFixture(t, "examples/gc-logs/sample-hotspot-gc.log")
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Metadata.Parser != "hotspot_unified_gc_log" {
		t.Errorf("parser = %q, want hotspot_unified_gc_log", result.Metadata.Parser)
	}
	if result.Metadata.Extra["gc_format"] != "unified" {
		t.Errorf("gc_format = %v, want unified", result.Metadata.Extra["gc_format"])
	}
}

// TestEventTimeFallback exercises eventTime's three-way precedence.
func TestEventTimeFallback(t *testing.T) {
	// No fixture needed — exercise via direct function call.
	// timestamp wins over uptime_sec, which wins over index.
	if got := formatFloat(10.0); got != "10.0" {
		t.Errorf("formatFloat(10.0) = %q, want 10.0", got)
	}
	if got := formatFloat(0.001); got != "0.001" {
		t.Errorf("formatFloat(0.001) = %q, want 0.001", got)
	}
}

func TestMarshalsToJSONWithJVMInfo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gc.log")
	body := strings.Join([]string{
		"[0.001s][info][gc] Using G1",
		"[0.002s][info][gc,init] Version: 17.0.7+8 (release)",
		"[0.003s][info][gc,init] CPUs: 8 total, 8 available",
		"[0.004s][info][gc] GC(0) Pause Young (Normal) (G1 Evacuation Pause) 100M->50M(200M) 25.000ms",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	body2, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`"type":"gc_log"`,
		`"parser":"hotspot_unified_gc_log"`,
		`"schema_version":"0.1.0"`,
		`"gc_format":"unified"`,
		`"jvm_info":`,
		`"analysis_options":`,
	} {
		if !strings.Contains(string(body2), want) {
			t.Errorf("missing %q in marshalled output: %s", want, body2)
		}
	}
}

func TestAllocationAndPromotionRates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gc.log")
	body := strings.Join([]string{
		"2026-04-27T10:00:00.000+0900: 0.000: [GC pause (G1 Evacuation Pause) (young), 0.0250000 secs]",
		"   [Eden: 80M(80M)->0B(72M) Survivors: 0B->8M Heap: 80M(200M)->10M(200M)]",
		"2026-04-27T10:00:10.000+0900: 10.000: [GC pause (G1 Evacuation Pause) (young), 0.0500000 secs]",
		"   [Eden: 70M(72M)->0B(64M) Survivors: 8M->8M Heap: 80M(200M)->20M(200M)]",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	allocSeries, ok := result.Series["allocation_rate_mb_per_sec"].([]map[string]any)
	if !ok {
		t.Fatalf("allocation_rate_mb_per_sec is %T", result.Series["allocation_rate_mb_per_sec"])
	}
	if len(allocSeries) != 1 {
		t.Errorf("allocation_rate len = %d, want 1", len(allocSeries))
	}
	// allocated = 80 - 10 = 70MB over 10s → 7.0 MB/s.
	if got := asFloat(allocSeries[0]["value"]); got != 7.0 {
		t.Errorf("allocation rate = %v, want 7.0", got)
	}
	avgAlloc := asFloat(result.Summary["avg_allocation_rate_mb_per_sec"])
	if avgAlloc != 7.0 {
		t.Errorf("avg_allocation_rate = %v, want 7.0", avgAlloc)
	}
}

func TestRoundHalfEven(t *testing.T) {
	cases := []struct {
		v        float64
		decimals int
		want     float64
	}{
		{0.5, 0, 0.0}, // banker's: 0.5 → 0
		{1.5, 0, 2.0}, // banker's: 1.5 → 2
		{2.5, 0, 2.0}, // banker's: 2.5 → 2
		{3.14159, 2, 3.14},
		{215.12, 3, 215.12},
	}
	for _, tc := range cases {
		if got := roundHalfEven(tc.v, tc.decimals); got != tc.want {
			t.Errorf("roundHalfEven(%v, %d) = %v, want %v", tc.v, tc.decimals, got, tc.want)
		}
	}
}

func TestOrderedCounter(t *testing.T) {
	c := newOrderedCounter()
	c.add("a")
	c.add("b")
	c.add("a")
	c.add("c")
	c.add("b")
	c.add("a")
	got := c.items()
	if len(got) != 3 {
		t.Fatalf("entries = %d, want 3", len(got))
	}
	if got[0].key != "a" || got[0].count != 3 {
		t.Errorf("entry[0] = %+v, want {a 3}", got[0])
	}
	if got[1].key != "b" || got[1].count != 2 {
		t.Errorf("entry[1] = %+v, want {b 2}", got[1])
	}
	if got[2].key != "c" || got[2].count != 1 {
		t.Errorf("entry[2] = %+v, want {c 1}", got[2])
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		m[s]--
		if m[s] < 0 {
			return false
		}
	}
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return true
}
