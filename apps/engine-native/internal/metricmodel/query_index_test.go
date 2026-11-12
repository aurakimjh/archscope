package metricmodel_test

import (
	"context"
	"encoding/json"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestPreparedMetricQueriesMatchStatelessExactPages(t *testing.T) {
	ctx := context.Background()
	for _, file := range []string{"metric-history-set.json", "system-db-postgres-model.json", "system-db-linux-ss-model.json"} {
		t.Run(file, func(t *testing.T) {
			raw, e := os.ReadFile("../catalog/contracts/" + file)
			if e != nil {
				t.Fatal(e)
			}
			set, e := m.Decode(raw)
			if e != nil {
				t.Fatal(e)
			}
			index, e := m.PrepareQueries(ctx, set)
			if e != nil {
				t.Fatal(e)
			}
			ids := []string{}
			for _, s := range set.Series {
				ids = append(ids, s.ID)
			}
			for _, selected := range [][]string{ids, ids[:1]} {
				for _, limit := range []int{1, 25, 200} {
					for _, window := range []bool{false, true} {
						q := m.Query{Selection: m.Selection{SeriesIDs: selected}, Limit: limit}
						if window {
							q.Selection.From = m.Pointer("0")
							q.Selection.To = set.Points[len(set.Points)-1].End
						}
						for page := 0; page < 200; page++ {
							before, e := m.QuerySet(ctx, set, strings.Repeat("a", 64), q)
							after, err := index.Query(ctx, strings.Repeat("a", 64), q)
							if e != nil || err != nil {
								t.Fatal(e, err)
							}
							a, _ := json.Marshal(before)
							b, _ := json.Marshal(after)
							if string(a) != string(b) {
								t.Fatalf("prepared/stateless mismatch limit%d window%v page%d", limit, window, page)
							}
							if after.Next == "" {
								break
							}
							q.Cursor = after.Next
						}
					}
				}
			}
		})
	}
}
func TestPreparedMetricSnapshotIsolationHistogramAndContext(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("../parsers/metrichistory/testdata/sdk-python-1.38.0.otlp.json")
	if e != nil {
		t.Fatal(e)
	}
	set, e := p.ParseBytes(ctx, raw, "sdk.json", p.Options{})
	if e != nil {
		t.Fatal(e)
	}
	index, e := m.PrepareQueries(ctx, set)
	if e != nil {
		t.Fatal(e)
	}
	ids := []string{}
	for _, s := range set.Series {
		ids = append(ids, s.ID)
	}
	q := m.Query{Selection: m.Selection{SeriesIDs: ids}, Limit: 25}
	digest := strings.Repeat("b", 64)
	baseline, e := m.QuerySet(ctx, set, digest, q)
	if e != nil {
		t.Fatal(e)
	}
	baselineBytes, _ := json.Marshal(baseline)
	baseline = m.Page{}
	json.Unmarshal(baselineBytes, &baseline)
	page, e := index.Query(ctx, digest, q)
	if e != nil || !reflect.DeepEqual(baseline, page) {
		t.Fatal("histogram mismatch", e)
	}
	for _, row := range page.Rows {
		if row.Value != nil {
			*row.Value = "0"
		}
		if row.Histogram != nil && len(row.Histogram.Counts) > 0 {
			row.Histogram.Counts[0] = "99"
		}
		if row.ObservedHistogram != nil {
			row.ObservedHistogram.Count = "999"
		}
		if len(row.Refs) > 0 {
			row.Refs[0].Position = "mutated"
		}
	}
	page.Selection.SeriesIDs[0] = "mutated"
	for j := range set.Points {
		if set.Points[j].Value != nil {
			*set.Points[j].Value = "12345"
		}
	}
	// The caller query's slice also remains isolated from the response.
	if q.Selection.SeriesIDs[0] == "mutated" {
		t.Fatal("query aliased output")
	}
	again, e := index.Query(ctx, digest, q)
	if e != nil || !reflect.DeepEqual(baseline, again) {
		t.Fatal("snapshot mutated", e)
	}
	var wg sync.WaitGroup
	for j := 0; j < 8; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, e := index.Query(ctx, digest, q)
			if e != nil || !reflect.DeepEqual(baseline, got) {
				t.Error("concurrent mismatch", e)
			}
			got.Rows[0].Refs[0].Position = "private"
		}()
	}
	wg.Wait()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = index.Query(cancelled, digest, q); e == nil {
		t.Fatal("query cancellation ignored")
	}
	if _, e = m.PrepareQueries(cancelled, set); e == nil {
		t.Fatal("prepare cancellation ignored")
	}
}
