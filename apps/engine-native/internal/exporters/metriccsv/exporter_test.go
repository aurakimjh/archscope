package metriccsv

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"os"
	"strings"
	"testing"
)

func TestExactIntervalsHistogramsAndCoordinates(t *testing.T) {
	ctx := context.Background()
	for _, file := range []string{"snapshots.json", "sdk-python-1.38.0.otlp.json"} {
		t.Run(file, func(t *testing.T) {
			raw, e := os.ReadFile("../../parsers/metrichistory/testdata/" + file)
			if e != nil {
				t.Fatal(e)
			}
			set, e := p.ParseBytes(ctx, raw, file, p.Options{})
			if e != nil {
				t.Fatal(e)
			}
			selection := m.Selection{}
			for _, s := range set.Series {
				selection.SeriesIDs = append(selection.SeriesIDs, s.ID)
			}
			body, e := Encode(ctx, set, selection, strings.Repeat("a", 64))
			if e != nil {
				t.Fatal(e)
			}
			again, e := Encode(ctx, set, selection, strings.Repeat("a", 64))
			if e != nil || !bytes.Equal(body, again) {
				t.Fatal("nondeterministic", e)
			}
			records, e := csv.NewReader(bytes.NewReader(body)).ReadAll()
			if e != nil {
				t.Fatal(e)
			}
			rows, e := m.SelectRows(ctx, set, selection)
			if e != nil || len(records) != len(rows)+1 {
				t.Fatal(e)
			}
			columns := map[string]int{}
			for i, h := range records[0] {
				columns[h] = i
			}
			for i, row := range rows {
				for field, want := range map[string]string{"value": optional(row.Value), "delta": optional(row.Delta), "rate": optional(row.Rate), "start_ns": optional(row.Start), "end_ns": optional(row.End), "status": row.Status, "refs_json": object(row.Refs), "observed_histogram_json": object(row.ObservedHistogram), "interval_histogram_json": object(row.Histogram)} {
					if records[i+1][columns[field]] != want {
						t.Fatalf("row%d field%s", i, field)
					}
				}
				var refs []m.Ref
				if json.Unmarshal([]byte(records[i+1][columns["refs_json"]]), &refs) != nil || len(refs) == 0 {
					t.Fatal("coordinates absent")
				}
			}
			// Half-open window selects whole intervals, including their outside predecessor.
			selection.From = rows[0].End
			selection.To = rows[len(rows)-1].End
			if selection.From != nil && selection.To != nil && *selection.From != *selection.To {
				if _, e = Encode(ctx, set, selection, strings.Repeat("b", 64)); e != nil {
					t.Fatal(e)
				}
			}
			cancel, c := context.WithCancel(ctx)
			c()
			if _, e = Encode(cancel, set, selection, strings.Repeat("a", 64)); e == nil {
				t.Fatal("cancel ignored")
			}
		})
	}
}
func TestFreeTextFormulaGuardAndBound(t *testing.T) {
	for _, s := range []string{"=HYPERLINK(\"x\")", " +cmd", "-cmd", "@SUM(1)", "\t=cmd", "\r\ntext"} {
		if text(s) != "'"+s {
			t.Fatal(s)
		}
	}
	if text("a,b\nquoted") != "a,b\nquoted" {
		t.Fatal("benign text altered")
	}
	// The writer refuses the complete export, including buffered tail bytes.
	var b boundedBuffer
	block := bytes.Repeat([]byte{'x'}, 1<<20)
	for i := 0; i < 64; i++ {
		if _, e := b.Write(block); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := b.Write([]byte{'x'}); e == nil || b.Len() != MaxBytes {
		t.Fatal("output silently truncated")
	}
}
