package metrichistory_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
)

var ctx = context.Background()

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	body, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return body
}
func parse(t *testing.T, body []byte, options p.Options) m.Set {
	t.Helper()
	set, e := p.ParseBytes(ctx, body, "synthetic.json", options)
	if e != nil {
		t.Fatal(e)
	}
	return set
}
func selected(set m.Set) m.Selection {
	selection := m.Selection{}
	for _, series := range set.Series {
		selection.SeriesIDs = append(selection.SeriesIDs, series.ID)
	}
	return selection
}
func rows(t *testing.T, set m.Set) []m.Interval {
	t.Helper()
	rows, e := m.Intervals(ctx, set)
	if e != nil {
		t.Fatal(e)
	}
	return rows
}
func TestPinnedSDKAndImmutableRoundTrip(t *testing.T) {
	set := parse(t, fixture(t, "sdk-python-1.38.0.otlp.json"), p.Options{Producer: "opentelemetry-python", ProducerVersion: "1.38.0"})
	if len(set.Series) != 4 || len(set.Points) != 8 {
		t.Fatalf("inventory %d/%d", len(set.Series), len(set.Points))
	}
	var large, explicit, exponential, signed bool
	derived := 0
	for _, point := range set.Points {
		if point.Value != nil && *point.Value == "9007199254740993" {
			large = true
		}
	}
	for _, series := range set.Series {
		if series.Scope.Name != "synthetic.instrumentation" || series.Scope.Version != "1.0.0" || series.Scope.SchemaURL == "" || !bytes.Contains(series.Resource, []byte("synthetic-checkout")) {
			t.Fatal("identity lost", series)
		}
		if series.Kind == "histogram" {
			explicit = true
		}
		if series.Kind == "exponential_histogram" {
			exponential = true
		}
	}
	for _, row := range rows(t, set) {
		if row.Rate != nil {
			derived++
		}
		if row.RateMeaning == "signed_change_per_second" && row.Delta != nil && *row.Delta == "-1" {
			signed = true
		}
	}
	if !large || !explicit || !exponential || !signed || derived != 4 {
		t.Fatalf("SDK coverage %v %v %v %v %d", large, explicit, exponential, signed, derived)
	}
	body, _ := json.MarshalIndent(set, "", "  ")
	reopened, e := m.Decode(body)
	if e != nil {
		t.Fatal(e)
	}
	result, e := a.Build(ctx, reopened, selected(set), "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	snapshot, _ := json.MarshalIndent(result, "", "  ")
	var decoded = result
	if e = json.Unmarshal(snapshot, &decoded); e != nil {
		t.Fatal(e)
	}
	if e = a.Validate(decoded); e != nil {
		t.Fatal(e)
	}
	decoded.Summary["point_count"] = "9"
	if a.Validate(decoded) == nil {
		t.Fatal("forged derived result accepted")
	}
}
func TestSnapshotExactResetGapOrderAndWindow(t *testing.T) {
	set := parse(t, fixture(t, "snapshots.json"), p.Options{})
	got := rows(t, set)
	if len(got) != 4 || got[1].Delta == nil || *got[1].Delta != "7" || *got[1].Rate != "7" || got[2].Status != "reset" || !got[2].Gap || *got[2].Rate != "3" {
		t.Fatalf("intervals %+v", got)
	}
	selection := selected(set)
	selection.From = m.Pointer("2000000000")
	selection.To = m.Pointer("4000000000")
	page, e := m.QuerySet(ctx, set, strings.Repeat("a", 64), m.Query{Selection: selection, Limit: 1})
	if e != nil || len(page.Rows) != 1 || *page.Rows[0].Start != "1000000000" || *page.Rows[0].Rate != "7" {
		t.Fatalf("unclipped selection %+v %v", page, e)
	}
	// Preserve original order as provenance, but calculate on event time.
	set.Points[0], set.Points[1] = set.Points[1], set.Points[0]
	got = rows(t, set)
	if !got[0].Reordered || *got[1].Delta != "7" {
		t.Fatal("event order", got)
	}
}
func TestProvenanceDedupAndFormatPopulations(t *testing.T) {
	set := parse(t, fixture(t, "snapshots.json"), p.Options{})
	merged, e := m.Merge(ctx, []m.Set{set, set})
	if e != nil || len(merged.Points) != 4 || merged.Duplicates != 4 || len(merged.Points[0].Refs) != 2 {
		t.Fatalf("dedup %+v %v", merged, e)
	}
	// Re-merging a deduplicated model preserves its receipt and every original ref.
	merged, e = m.Merge(ctx, []m.Set{merged})
	if e != nil || merged.Duplicates != 4 || len(merged.Points[0].Refs) != 2 {
		t.Fatalf("dedup replay %+v %v", merged, e)
	}
	other := parse(t, append(fixture(t, "snapshots.json"), '\n'), p.Options{})
	merged, e = m.Merge(ctx, []m.Set{set, other})
	if e != nil || len(merged.Points) != 8 || merged.Duplicates != 0 {
		t.Fatal("different export bytes auto deduplicated", e)
	}
	query := fixture(t, "prometheus-query.json")
	prom := parse(t, fixture(t, "prometheus-range.json"), p.Options{Query: query})
	merged, e = m.Merge(ctx, []m.Set{set, prom})
	if e != nil || len(merged.Series) != 2 {
		t.Fatal("cross-format populations merged", e)
	}
	for _, row := range rows(t, prom) {
		if row.Rate != nil {
			t.Fatal("query-result counter inferred")
		}
	}
	if _, e = p.ParseBytes(ctx, fixture(t, "prometheus-range.json"), "range.json", p.Options{}); e == nil {
		t.Fatal("query provenance omitted")
	}
}
func otlp(points string, kind string) []byte {
	return []byte(`{"resourceMetrics":[{"scopeMetrics":[{"metrics":[{"name":"x","unit":"count","` + kind + `":{"aggregationTemporality":2,"isMonotonic":true,"dataPoints":` + points + `}}]}]}]}`)
}
func TestAmbiguityResetsUnknownAndNoRecord(t *testing.T) {
	set := parse(t, otlp(`[{"startTimeUnixNano":"1","timeUnixNano":"1000000000","asInt":"5"},{"startTimeUnixNano":"1","timeUnixNano":"2000000000","asInt":"6"},{"startTimeUnixNano":"1","timeUnixNano":"2000000000","asInt":"7"},{"startTimeUnixNano":"1","timeUnixNano":"3000000000","asInt":"8"},{"startTimeUnixNano":"1","timeUnixNano":"4000000000","asInt":"1"},{"timeUnixNano":"5000000000","flags":1}]`, "sum"), p.Options{})
	got := rows(t, set)
	for _, i := range []int{1, 2, 3, 4, 5} {
		if got[i].Rate != nil {
			t.Fatal("invented rate", i, got[i])
		}
	}
	if got[1].Status != "conflicting_timestamp" || got[3].Status != "ambiguous_predecessor" || got[4].Status != "counter_reset_unannounced" || got[5].Status != "no_recorded_value" {
		t.Fatal(got)
	}
	set = parse(t, otlp(`[{"asInt":"3"}]`, "gauge"), p.Options{})
	got = rows(t, set)
	if got[0].Timestamp != nil || got[0].Status != "timestamp_unavailable" {
		t.Fatal("invented timestamp", got)
	}
}
func TestDeltaIntervalsAndHistogramSeparation(t *testing.T) {
	body := bytes.ReplaceAll(otlp(`[{"startTimeUnixNano":"1000000000","timeUnixNano":"3000000000","asInt":"8"},{"startTimeUnixNano":"2000000000","timeUnixNano":"4000000000","asInt":"6"},{"startTimeUnixNano":"5000000000","timeUnixNano":"6000000000","asInt":"2"}]`, "sum"), []byte(`"aggregationTemporality":2`), []byte(`"aggregationTemporality":1`))
	got := rows(t, parse(t, body, p.Options{}))
	if got[0].Rate != nil || got[1].Rate != nil || got[2].Rate == nil || *got[2].Rate != "2" || !got[2].Gap {
		t.Fatal("overlap/gap", got)
	}
	set := parse(t, otlp(`[{"startTimeUnixNano":"1","timeUnixNano":"1000000000","count":"2","bucketCounts":["1","1"],"explicitBounds":["5"]},{"startTimeUnixNano":"1","timeUnixNano":"2000000000","count":"5","bucketCounts":["2","3"],"explicitBounds":["5"]}]`, "histogram"), p.Options{})
	got = rows(t, set)
	if got[0].ObservedHistogram == nil || got[1].Histogram == nil || got[1].Histogram.Count != "3" || got[1].Histogram.Counts[1] != "2" || *got[1].Rate != "3" {
		t.Fatal("bucket delta", got)
	}
	bad := bytes.Replace(otlp(`[{"timeUnixNano":"100","count":"3","bucketCounts":["1","1"],"explicitBounds":["5"]}]`, "histogram"), []byte("unused"), nil, 1)
	if _, e := p.ParseBytes(ctx, bad, "bad.json", p.Options{}); e == nil {
		t.Fatal("bucket/count mismatch accepted")
	}
	exp := otlp(`[{"startTimeUnixNano":"1","timeUnixNano":"1000000000","count":"4","scale":0,"zeroCount":"1","positive":{"offset":-1,"bucketCounts":["1"]},"negative":{"offset":1,"bucketCounts":["2"]}},{"startTimeUnixNano":"1","timeUnixNano":"2000000000","count":"8","scale":0,"zeroCount":"2","positive":{"offset":-1,"bucketCounts":["2"]},"negative":{"offset":1,"bucketCounts":["4"]}}]`, "exponentialHistogram")
	got = rows(t, parse(t, exp, p.Options{}))
	if got[1].Histogram == nil || got[1].Histogram.ZeroCount != "1" || got[1].Histogram.Negative.Counts[0] != "2" || got[1].Histogram.Positive.Offset != -1 {
		t.Fatal("exp buckets", got)
	}
	exp = bytes.Replace(exp, []byte(`"scale":0`), []byte(`"scale":1`), 1)
	got = rows(t, parse(t, exp, p.Options{}))
	if got[1].Rate != nil || got[1].Status != "histogram_schema_changed" {
		t.Fatal("schema silently reaggregated", got)
	}
}
func TestStrictOTLPBoundsCursorAndGzip(t *testing.T) {
	for _, body := range [][]byte{otlp(`[{"asInt":"9223372036854775808"}]`, "sum"), otlp(`[{"asInt":"1","asDouble":2}]`, "sum"), []byte(`{"resourceMetrics":[],"resourceMetrics":[]}`), bytes.Replace(otlp(`[{"asInt":"1"}]`, "sum"), []byte(`"aggregationTemporality":2`), []byte(`"aggregationTemporality":"CUMULATIVE"`), 1)} {
		if _, e := p.ParseBytes(ctx, body, "bad.json", p.Options{}); e == nil {
			t.Fatal("invalid OTLP accepted", string(body))
		}
	}
	body := fixture(t, "snapshots.json")
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	z.Write(body)
	z.Close()
	set := parse(t, compressed.Bytes(), p.Options{})
	plain := parse(t, body, p.Options{})
	if set.Sources[0].SHA256 == plain.Sources[0].SHA256 || len(set.Points) != 4 {
		t.Fatal("compressed identity lost")
	}
	q := m.Query{Selection: selected(set), Limit: 1}
	page, e := m.QuerySet(ctx, set, strings.Repeat("a", 64), q)
	if e != nil || page.Next == "" {
		t.Fatal(e)
	}
	q.Cursor = page.Next
	q.Selection.From = m.Pointer("2000000000")
	if _, e = m.QuerySet(ctx, set, strings.Repeat("a", 64), q); e == nil {
		t.Fatal("changed query cursor accepted")
	}
	q.Selection = selected(set)
	if _, e = m.QuerySet(ctx, set, strings.Repeat("b", 64), q); e == nil {
		t.Fatal("changed result cursor accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = p.ParseBytes(cancelled, body, "cancelled.json", p.Options{}); e == nil {
		t.Fatal("cancel ignored")
	}
}

func TestEpochZeroAndIdentityBoundaries(t *testing.T) {
	set := parse(t, []byte(`{"schema_version":"1.0.0","snapshots":[{"time_unix_nano":"0","metrics":[{"name":"x","unit":"count","kind":"sum","temporality":"delta","monotonic":true,"start_time_unix_nano":"0","value":"1"}]},{"time_unix_nano":"1000000000","metrics":[{"name":"x","unit":"count","kind":"sum","temporality":"delta","monotonic":true,"start_time_unix_nano":"0","value":"2"}]}]}`), p.Options{})
	got := rows(t, set)
	if got[0].Timestamp == nil || *got[0].Timestamp != "1970-01-01T00:00:00Z" || got[0].Rate != nil || got[1].Rate == nil || *got[1].Rate != "2" {
		t.Fatal(got)
	}
	raw := otlp(`[{"timeUnixNano":"0","asInt":"1"}]`, "gauge")
	got = rows(t, parse(t, raw, p.Options{}))
	if got[0].Timestamp != nil {
		t.Fatal("OTLP unknown zero timestamp inferred")
	}
	// Point attributes, scope and resource each remain independent series identity.
	source := []byte(`{"resourceMetrics":[{"resource":{"attributes":[{"key":"role","value":{"stringValue":"client"}}]},"scopeMetrics":[{"scope":{"name":"a","version":"1"},"schemaUrl":"https://example.invalid/a","metrics":[{"name":"x","gauge":{"dataPoints":[{"asDouble":"NaN","attributes":[{"key":"worker","value":{"intValue":"1"}}]},{"asDouble":"Infinity","attributes":[{"key":"worker","value":{"intValue":"2"}}]}]}}]}]},{"resource":{"attributes":[{"key":"role","value":{"stringValue":"server"}}]},"scopeMetrics":[{"scope":{"name":"a","version":"2"},"metrics":[{"name":"x","gauge":{"dataPoints":[{"asInt":"1"}]}}]}]}]}`)
	set = parse(t, source, p.Options{})
	if len(set.Series) != 3 || len(set.Points) != 3 {
		t.Fatal("client/server or attrs merged")
	}
	bad := bytes.Replace(source, []byte(`"asInt":"1"`), []byte(`"asInt":"1","exemplars":[{"traceId":"invalid"}]`), 1)
	if _, e := p.ParseBytes(ctx, bad, "bad.json", p.Options{}); e == nil {
		t.Fatal("invalid exemplar trace hex accepted")
	}
}

func TestMissingEndAndDedupClaimsRemainConservative(t *testing.T) {
	set := parse(t, otlp(`[{"startTimeUnixNano":"1","asInt":"3"}]`, "sum"), p.Options{})
	got := rows(t, set)
	if got[0].End != nil || got[0].ObservedStart == nil || got[0].Rate != nil {
		t.Fatal(got)
	}
	set.Duplicates = 1
	if set.Validate(ctx) == nil {
		t.Fatal("forged dedup count accepted")
	}
	a := parse(t, fixture(t, "snapshots.json"), p.Options{})
	merged, e := m.Merge(ctx, []m.Set{a, a})
	if e != nil {
		t.Fatal(e)
	}
	merged.Sources[1].SHA256 = strings.Repeat("b", 64)
	if merged.Validate(ctx) == nil {
		t.Fatal("unsupported provenance merge accepted")
	}
}
