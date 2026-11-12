package metricmodel_test

import (
	"context"
	"encoding/json"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"testing"
)

func TestCrossExportObservationCohorts(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"gauge", "cumulative", "delta", "histogram"} {
		t.Run(kind, func(t *testing.T) {
			metric := `"name":"x","kind":"sum","temporality":"` + kind + `","monotonic":true,"start_time_unix_nano":"0","value":"9007199254740993"`
			if kind == "gauge" {
				metric = `"name":"x","kind":"gauge","value":"9007199254740993"`
			}
			if kind == "histogram" {
				metric = `"name":"x","kind":"histogram","temporality":"cumulative","start_time_unix_nano":"0","histogram":{"count":"3","sum":"7","min":null,"max":null,"bounds":["2"],"counts":["1","2"]}`
			}
			raw := []byte(`{"schema_version":"1.0.0","snapshots":[{"time_unix_nano":"1000000000","metrics":[{` + metric + `}]},{"time_unix_nano":"2000000000","metrics":[{` + metric + `}]}]}`)
			first, e := p.ParseBytes(ctx, raw, "a.json", p.Options{})
			if e != nil {
				t.Fatal(e)
			}
			second, e := p.ParseBytes(ctx, append(raw, '\n'), "b.json", p.Options{})
			if e != nil {
				t.Fatal(e)
			}
			// Only the first instant is repeated; a later cumulative row must remain ambiguous.
			second.Points = second.Points[:1]
			merged, e := m.Merge(ctx, []m.Set{first, second})
			if e != nil {
				t.Fatal(e)
			}
			rows, e := m.Intervals(ctx, merged)
			if e != nil {
				t.Fatal(e)
			}
			if len(merged.Points) != 3 || merged.Duplicates != 0 || rows[0].Status != "duplicate_observation" || rows[1].Status != "duplicate_observation" {
				t.Fatal(rows)
			}
			for _, row := range rows[:2] {
				if row.Rate != nil || row.Delta != nil || row.Histogram != nil || len(row.Refs) != 1 {
					t.Fatal(row)
				}
			}
			if kind == "cumulative" || kind == "histogram" {
				if rows[2].Status != "ambiguous_predecessor" || rows[2].Rate != nil {
					t.Fatal(rows[2])
				}
			}
			legacy := merged
			legacy.Normalizer = m.CadenceNormalizer
			old, e := m.Intervals(ctx, legacy)
			if e != nil {
				t.Fatal(e)
			}
			if kind != "delta" && old[0].Status != "conflicting_timestamp" {
				t.Fatal("legacy changed", old)
			}
			// A conflicting third writer makes the complete cohort conflicting.
			changed := merged.Points[0]
			changed.Refs = []m.Ref{{SourceID: changed.Refs[0].SourceID, Position: "different"}}
			if changed.Histogram != nil {
				body, _ := json.Marshal(changed.Histogram)
				var h m.Histogram
				json.Unmarshal(body, &h)
				h.Sum = m.Pointer("8")
				changed.Histogram = &h
			} else {
				changed.Value = m.Pointer("9007199254740994")
			}
			changed.ID = m.PointID(changed)
			merged.Points = append(merged.Points, changed)
			conflict, e := m.Intervals(ctx, merged)
			if e != nil {
				t.Fatal(e)
			}
			for _, row := range conflict[:3] {
				if row.Status != "conflicting_timestamp" || row.Rate != nil {
					t.Fatal(conflict)
				}
			}
		})
	}
}
