package jfr

import (
	"testing"
	"time"

	jfrparser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/jfr"
)

// fixturePath points at the same examples/jfr fixture the Python tests
// use so cross-engine parity is observable.
const fixturePath = "../../../../../examples/jfr/sample-jfr-print.json"

func TestAnalyzeFixtureSummary(t *testing.T) {
	result, err := Analyze(fixturePath, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Type != ResultType {
		t.Fatalf("Type = %q, want %q", result.Type, ResultType)
	}
	if got := getInt(result.Summary, "event_count"); got != 3 {
		t.Errorf("event_count = %d, want 3", got)
	}
	if got := getInt(result.Summary, "event_count_total"); got != 3 {
		t.Errorf("event_count_total = %d, want 3", got)
	}
	// gc_pause_total_ms == 42.5: only jdk.GCPhasePause matches
	// _gc_events ("GC" substring) and has duration 42.5 ms.
	if got := getFloat(result.Summary, "gc_pause_total_ms"); got != 42.5 {
		t.Errorf("gc_pause_total_ms = %v, want 42.5", got)
	}
	if got := getInt(result.Summary, "blocked_thread_events"); got != 1 {
		t.Errorf("blocked_thread_events = %d, want 1", got)
	}
	if got := result.Summary["selected_mode"]; got != "all" {
		t.Errorf("selected_mode = %v, want all", got)
	}
}

func TestAnalyzeMetadataAndNotableEvents(t *testing.T) {
	result, err := Analyze(fixturePath, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Metadata.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version = %q, want %q", result.Metadata.SchemaVersion, SchemaVersion)
	}
	if result.Metadata.Parser != ParserName {
		t.Errorf("parser = %q, want %q", result.Metadata.Parser, ParserName)
	}
	notable := result.Tables["notable_events"].([]map[string]any)
	if len(notable) != 3 {
		t.Fatalf("notable_events length = %d, want 3", len(notable))
	}
	// Sorted by duration desc — first entry is jdk.GCPhasePause (42.5 ms).
	if got := notable[0]["event_type"]; got != "jdk.GCPhasePause" {
		t.Errorf("notable[0].event_type = %v, want jdk.GCPhasePause", got)
	}
	if got := notable[0]["evidence_ref"]; got != "jfr:event:1" {
		t.Errorf("notable[0].evidence_ref = %v, want jfr:event:1", got)
	}
	if got := notable[0]["sampling_type"]; got != "exact" {
		t.Errorf("notable[0].sampling_type = %v, want exact", got)
	}
	// Metadata extras carry mode + window companions.
	if got := result.Metadata.Extra["selected_mode"]; got != "all" {
		t.Errorf("metadata.selected_mode = %v, want all", got)
	}
	if got := result.Metadata.Extra["jfr_command_version"]; got != "external" {
		t.Errorf("metadata.jfr_command_version = %v, want external", got)
	}
	availableModes, ok := result.Metadata.Extra["available_modes"].([]string)
	if !ok {
		t.Fatalf("available_modes type = %T, want []string", result.Metadata.Extra["available_modes"])
	}
	if !contains(availableModes, "all") {
		t.Errorf("available_modes missing 'all': %v", availableModes)
	}
	if !contains(availableModes, "gc") {
		t.Errorf("available_modes missing 'gc': %v", availableModes)
	}
	if !contains(availableModes, "lock") {
		t.Errorf("available_modes missing 'lock': %v", availableModes)
	}
	supported, ok := result.Metadata.Extra["supported_modes"].([]string)
	if !ok {
		t.Fatalf("supported_modes type = %T, want []string", result.Metadata.Extra["supported_modes"])
	}
	if supported[0] != "all" {
		t.Errorf("supported_modes[0] = %q, want all", supported[0])
	}
	// Window not requested → both inputs nil.
	w, ok := result.Metadata.Extra["filter_window"].(map[string]any)
	if !ok {
		t.Fatalf("filter_window type = %T", result.Metadata.Extra["filter_window"])
	}
	if w["from"] != nil || w["to"] != nil {
		t.Errorf("filter_window from/to should be nil, got %+v", w)
	}
}

func TestModeFilter(t *testing.T) {
	// Mode 'gc' should restrict to jdk.GCPhasePause only — count=1.
	result, err := Analyze(fixturePath, Options{Mode: "gc"})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := getInt(result.Summary, "event_count"); got != 1 {
		t.Errorf("event_count under mode=gc: %d, want 1", got)
	}
	if got := getInt(result.Summary, "event_count_total"); got != 3 {
		t.Errorf("event_count_total under mode=gc: %d, want 3", got)
	}
	if got := result.Summary["selected_mode"]; got != "gc" {
		t.Errorf("selected_mode = %v, want gc", got)
	}
	// Unknown mode falls through to "all".
	result, err = Analyze(fixturePath, Options{Mode: "definitely-not-real"})
	if err != nil {
		t.Fatalf("Analyze unknown mode: %v", err)
	}
	if result.Summary["selected_mode"] != "all" {
		t.Errorf("unknown mode → selected_mode = %v, want all", result.Summary["selected_mode"])
	}
}

func TestMinDurationFilter(t *testing.T) {
	result, err := Analyze(fixturePath, Options{MinDurationMS: ptrFloat(20)})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	// CPUTimeSample has no duration (0) → filtered out.
	// ThreadPark = 12 ms → filtered out.
	// GCPhasePause = 42.5 ms → kept.
	if got := getInt(result.Summary, "event_count"); got != 1 {
		t.Errorf("event_count >= 20ms: %d, want 1", got)
	}
}

func TestEventsByTypeOrdering(t *testing.T) {
	events := []jfrparser.Event{
		{EventType: "jdk.A"},
		{EventType: "jdk.B"},
		{EventType: "jdk.B"},
		{EventType: "jdk.A"},
		{EventType: "jdk.C"},
	}
	rows := eventsByType(events)
	// Tie 2/2 → ascending event_type.
	if rows[0]["event_type"] != "jdk.A" {
		t.Errorf("rows[0] = %v, want jdk.A first (tie-broken ascending)", rows[0])
	}
	if rows[2]["event_type"] != "jdk.C" {
		t.Errorf("rows[2] = %v, want jdk.C last", rows[2])
	}
}

func TestHeatmapStripBucketing(t *testing.T) {
	// Three timestamps spread across ~5 seconds — falls in the
	// "<= 30s → 0.5s buckets" branch.
	events := []jfrparser.Event{
		{EventType: "x", Time: "2026-04-30T01:00:00.000000+00:00"},
		{EventType: "x", Time: "2026-04-30T01:00:02.500000+00:00"},
		{EventType: "x", Time: "2026-04-30T01:00:05.000000+00:00"},
	}
	strip := buildHeatmapStrip(events)
	if got := strip["bucket_seconds"]; got != 0.5 {
		t.Errorf("bucket_seconds = %v, want 0.5", got)
	}
	buckets := strip["buckets"].([]map[string]any)
	// 5s / 0.5s + 1 = 11 buckets.
	if len(buckets) != 11 {
		t.Errorf("buckets length = %d, want 11", len(buckets))
	}
	// First and last buckets receive a hit; middle one too.
	if buckets[0]["count"] != 1 {
		t.Errorf("buckets[0].count = %v, want 1", buckets[0]["count"])
	}
	if buckets[10]["count"] != 1 {
		t.Errorf("buckets[10].count = %v, want 1", buckets[10]["count"])
	}
}

func TestHeatmapStripEmpty(t *testing.T) {
	// All events have unparseable times → empty heatmap shape.
	events := []jfrparser.Event{{EventType: "x", Time: ""}}
	strip := buildHeatmapStrip(events)
	if got := strip["bucket_seconds"]; got != 0.0 {
		t.Errorf("empty bucket_seconds = %v, want 0.0", got)
	}
	if strip["start_time"] != nil || strip["end_time"] != nil {
		t.Errorf("empty start/end should be nil, got %+v / %+v", strip["start_time"], strip["end_time"])
	}
	if got := strip["max_count"]; got != 0 {
		t.Errorf("empty max_count = %v, want 0", got)
	}
}

func TestSupportedModes(t *testing.T) {
	modes := SupportedModes()
	if modes[0] != "all" {
		t.Errorf("SupportedModes()[0] = %q, want all", modes[0])
	}
	// The remainder is sorted.
	for i := 2; i < len(modes); i++ {
		if modes[i-1] > modes[i] {
			t.Errorf("SupportedModes not sorted after 'all': %v", modes)
			break
		}
	}
	// Sanity: 'nativemem' is in the set.
	if !contains(modes, "nativemem") {
		t.Errorf("SupportedModes missing 'nativemem': %v", modes)
	}
}

func TestRoundHalfEven(t *testing.T) {
	// Banker's rounding: 0.5 → 0, 1.5 → 2.
	if got := roundHalfEven(0.5, 0); got != 0 {
		t.Errorf("roundHalfEven(0.5, 0) = %v, want 0", got)
	}
	if got := roundHalfEven(1.5, 0); got != 2 {
		t.Errorf("roundHalfEven(1.5, 0) = %v, want 2", got)
	}
	if got := roundHalfEven(2.5, 0); got != 2 {
		t.Errorf("roundHalfEven(2.5, 0) = %v, want 2", got)
	}
	if got := roundHalfEven(0.125, 2); got != 0.12 {
		t.Errorf("roundHalfEven(0.125, 2) = %v, want 0.12", got)
	}
}

func TestParseTimeInputRelative(t *testing.T) {
	start := mustParse(t, "2026-04-30T01:00:00+00:00")
	end := mustParse(t, "2026-04-30T01:00:30+00:00")
	got := parseTimeInput("+10s", &start, &end)
	if got == nil {
		t.Fatal("parseTimeInput returned nil for +10s")
	}
	if !got.Equal(start.Add(10_000_000_000)) {
		t.Errorf("+10s = %v, want %v", got, start.Add(10_000_000_000))
	}
	got = parseTimeInput("-5s", &start, &end)
	if got == nil {
		t.Fatal("parseTimeInput returned nil for -5s")
	}
	if !got.Equal(end.Add(-5_000_000_000)) {
		t.Errorf("-5s = %v, want %v", got, end.Add(-5_000_000_000))
	}
}

func TestParseTimeInputISO(t *testing.T) {
	got := parseTimeInput("2026-04-30T01:00:05+00:00", nil, nil)
	if got == nil {
		t.Fatal("ISO parse returned nil")
	}
	want := mustParse(t, "2026-04-30T01:00:05+00:00")
	if !got.Equal(want) {
		t.Errorf("ISO parse = %v, want %v", got, want)
	}
}

func TestParseTimeInputHMS(t *testing.T) {
	start := mustParse(t, "2026-04-30T01:00:00+00:00")
	got := parseTimeInput("02:30:15", &start, nil)
	if got == nil {
		t.Fatal("HMS parse returned nil")
	}
	want := mustParse(t, "2026-04-30T02:30:15+00:00")
	if !got.Equal(want) {
		t.Errorf("HMS parse = %v, want %v", got, want)
	}
}

func TestWindowFilter(t *testing.T) {
	events := []jfrparser.Event{
		{EventType: "a", Time: "2026-04-30T01:00:00+00:00"},
		{EventType: "b", Time: "2026-04-30T01:00:05+00:00"},
		{EventType: "c", Time: "2026-04-30T01:00:10+00:00"},
	}
	result := Build(events, "/tmp/fake.json", jfrparser.SourceInfo{SourceFormat: "json"}, nil, Options{
		FromTime: "2026-04-30T01:00:03+00:00",
		ToTime:   "2026-04-30T01:00:08+00:00",
	})
	if got := getInt(result.Summary, "event_count"); got != 1 {
		t.Errorf("event_count windowed = %d, want 1", got)
	}
	w := result.Metadata.Extra["filter_window"].(map[string]any)
	if w["from"] != "2026-04-30T01:00:03+00:00" {
		t.Errorf("filter_window.from = %v", w["from"])
	}
	if w["effective_start"] == nil {
		t.Errorf("filter_window.effective_start should not be nil")
	}
}

func TestWindowKeepsEventsWithoutTimestamp(t *testing.T) {
	// Python: events with unparseable timestamps stay in the result
	// rather than being silently dropped.
	events := []jfrparser.Event{
		{EventType: "a", Time: "2026-04-30T01:00:00+00:00"},
		{EventType: "b", Time: ""},
		{EventType: "c", Time: "2026-04-30T01:00:30+00:00"},
	}
	result := Build(events, "/tmp/fake.json", jfrparser.SourceInfo{SourceFormat: "json"}, nil, Options{
		FromTime: "2026-04-30T01:00:10+00:00",
		ToTime:   "2026-04-30T01:00:20+00:00",
	})
	// Event a filtered out, b kept (no timestamp), c filtered out.
	if got := getInt(result.Summary, "event_count"); got != 1 {
		t.Errorf("event_count = %d, want 1 (kept the no-timestamp event)", got)
	}
}

func TestNanosecondTimestampsTreatedAsUnparseable(t *testing.T) {
	// Python 3.9 fromisoformat rejects 9-digit fractional seconds —
	// the Go port matches that contract so heatmap input is empty.
	events := []jfrparser.Event{
		{EventType: "x", Time: "2026-04-30T01:00:00.123456789Z"},
	}
	strip := buildHeatmapStrip(events)
	if got := strip["bucket_seconds"]; got != 0.0 {
		t.Errorf("nanosecond → bucket_seconds = %v, want 0.0 (treated as unparseable)", got)
	}
}

func TestEventsOverTime(t *testing.T) {
	events := []jfrparser.Event{
		{EventType: "z", Time: "2026-01-01T00:00:00Z"},
		{EventType: "a", Time: "2026-01-01T00:00:00Z"},
		{EventType: "a", Time: "2026-01-01T00:00:00Z"},
		{EventType: "z", Time: "2026-01-01T00:00:01Z"},
	}
	rows := eventsOverTime(events)
	if len(rows) != 3 {
		t.Fatalf("eventsOverTime rows = %d, want 3", len(rows))
	}
	// Sorted lexically by (time, event_type).
	if rows[0]["time"] != "2026-01-01T00:00:00Z" || rows[0]["event_type"] != "a" {
		t.Errorf("rows[0] = %+v, want time/00 a", rows[0])
	}
	if rows[0]["count"] != 2 {
		t.Errorf("rows[0].count = %v, want 2", rows[0]["count"])
	}
	if rows[2]["time"] != "2026-01-01T00:00:01Z" || rows[2]["event_type"] != "z" {
		t.Errorf("rows[2] = %+v, want time/01 z", rows[2])
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func getInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func getFloat(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

func contains(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

func ptrFloat(v float64) *float64 { return &v }

func mustParse(t *testing.T, value string) time.Time {
	t.Helper()
	res, ok := parseISOLike(value)
	if !ok {
		t.Fatalf("parseISOLike(%q) failed", value)
	}
	return res
}
