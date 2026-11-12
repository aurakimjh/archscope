package execution_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/execution"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/execution"
)

func TestR19SixtyThousandRowsExactCompactDistributionAndSavedCompatibility(t *testing.T) {
	ctx := context.Background()
	options, err := p.DecodeOptions(fixture(t, "jmeter-options.json"))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("timeStamp,elapsed,label,success\n")
	for i := 0; i < 60000; i++ {
		fmt.Fprintf(&b, "%d,%d,group-%d,%t\n", 1700000000000+int64(i), i%1000, i%3, i%10 != 0)
	}
	set, err := p.ParseBytes(ctx, []byte(b.String()), options)
	if err != nil {
		t.Fatal(err)
	}
	if set.Version != e.DistributionVersion || len(set.Samples) != 0 || len(set.Distributions) != 3 {
		t.Fatal("not compact")
	}
	for _, d := range set.Distributions {
		if d.Count != 20000 || d.Errors != 2000 || len(d.Values) != 1000 {
			t.Fatal(d.Count, d.Errors, len(d.Values))
		}
	}
	want := map[string]string{"min": "0", "max": "999/1000", "mean": "999/2000", "p50": "499/1000", "p90": "899/1000", "p95": "949/1000", "p99": "989/1000"}
	for _, m := range set.Metrics {
		if m.Name == "sample_latency" && *m.Value != want[m.Statistic] {
			t.Fatal(m)
		}
	}
	result, err := a.Build(ctx, set, "sanitized.csv")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(result)
	if len(raw) > 1500000 {
		t.Fatal("result still repeats per sample", len(raw))
	}
	if err = a.Validate(result); err != nil {
		t.Fatal(err)
	}
	page, err := a.QuerySet(ctx, set, strings.Repeat("a", 64), a.Query{Kind: "distributions", Limit: 25})
	if err != nil || page.Total != 3000 || len(page.Rows) != 25 || page.Next == "" {
		t.Fatal(page.Total, err)
	}
	set.Distributions[0].Values[0].Count++
	if set.Validate(ctx) == nil {
		t.Fatal("forged distribution")
	}
	// Existing saved 1.0.0/0.1.0 remains the exact old projection.
	legacy, err := e.Decode(ctx, fixture(t, "jmeter-set.json"))
	if err != nil {
		t.Fatal(err)
	}
	old, err := a.Build(ctx, legacy, "legacy")
	if err != nil || old.Metadata.SchemaVersion != "0.1.0" {
		t.Fatal(err)
	}
	if _, ok := old.Tables["distributions"]; ok {
		t.Fatal("old projection changed")
	}
}

func TestR19StreamingRowAndDistinctBinLimits(t *testing.T) {
	o, _ := p.DecodeOptions(fixture(t, "jmeter-options.json"))
	ctx := context.Background()
	header := "timeStamp,elapsed,label,success\n"
	row := "1700000000000,1,synthetic,true\n"
	raw := []byte(header + strings.Repeat(row, e.MaxJTLRows+1))
	_, err := p.ParseBytes(ctx, raw, o)
	if err == nil || !strings.Contains(err.Error(), "rows=1000001 limit=1000000") || !strings.Contains(err.Error(), "percentiles cannot be combined") {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString(header)
	for i := 0; i <= e.MaxJTLBins; i++ {
		fmt.Fprintf(&b, "1700000000000,%d,synthetic,true\n", i)
	}
	_, err = p.ParseBytes(ctx, []byte(b.String()), o)
	if err == nil || !strings.Contains(err.Error(), "distinct_values=100001 limit=100000") {
		t.Fatal(err)
	}
}
