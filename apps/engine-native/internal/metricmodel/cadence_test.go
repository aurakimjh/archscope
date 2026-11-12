package metricmodel_test

import (
	"context"
	"encoding/json"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"strings"
	"testing"
)

func TestRangeCadenceCompatibilityAndWindowContext(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"g"},"values":[[4,"40"],[0,"10"],[1,"20"],[4,"41"],[5,"NaN"],[7,"70"]]}]}}`)
	set, err := p.ParseBytes(ctx, raw, "range.json", p.Options{Query: json.RawMessage(`{"query":"g","step_ns":"1000000000"}`)})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := m.Intervals(ctx, set)
	if err != nil {
		t.Fatal(err)
	}
	gaps := 0
	for _, r := range rows {
		if r.Gap {
			gaps++
		}
		if r.Rate != nil || r.Delta != nil {
			t.Fatal("gauge rate invented")
		}
	}
	if gaps != 2 {
		t.Fatalf("gaps=%d: %+v", gaps, rows)
	}
	if rows[2].Status != "conflicting_timestamp" || rows[4].Status != "non_finite" {
		t.Fatal(rows)
	}
	// A window retains the predecessor outside the window and the exact observation.
	q := m.Query{Selection: m.Selection{SeriesIDs: []string{set.Series[0].ID}, From: m.Pointer("7000000000")}, Limit: 25}
	page, err := m.QuerySet(ctx, set, strings.Repeat("a", 64), q)
	if err != nil || len(page.Rows) != 1 || !page.Rows[0].Gap || *page.Rows[0].Value != "70" {
		t.Fatal(page, err)
	}
	index, err := m.PrepareQueries(ctx, set)
	if err != nil {
		t.Fatal(err)
	}
	cached, err := index.Query(ctx, strings.Repeat("a", 64), q)
	if err != nil {
		t.Fatal(err)
	}
	x, _ := json.Marshal(page)
	y, _ := json.Marshal(cached)
	if string(x) != string(y) {
		t.Fatal("prepared mismatch")
	}
	// Saved original-normalizer projections keep their original interpretation.
	legacy := set
	legacy.Normalizer = m.Normalizer
	old, err := m.Intervals(ctx, legacy)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range old {
		if r.Gap {
			t.Fatal("legacy reinterpreted")
		}
	}
	for _, model := range []m.Set{legacy, set} {
		result, err := a.Build(ctx, model, q.Selection, "range.json")
		if err != nil {
			t.Fatal(err)
		}
		if err = a.Validate(result); err != nil {
			t.Fatal(err)
		}
	}
	merged, err := m.Merge(ctx, []m.Set{legacy, set})
	if err != nil || merged.Normalizer != set.Normalizer {
		t.Fatal("merge downgraded", err)
	}
	// Cadence is optional and never inferred from sample spacing.
	without, err := p.ParseBytes(ctx, raw, "range.json", p.Options{Query: json.RawMessage(`{"query":"g"}`)})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := m.Intervals(ctx, without)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range plain {
		if r.Gap {
			t.Fatal("invented cadence")
		}
	}
}
