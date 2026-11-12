package metricmodel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"unsafe"
)

// QueryIndex owns a private model copy and its derived intervals. Window queries
// retain predecessor/reset context and the same canonical series/time ordering.
// Neither input mutation nor a returned page can mutate the retained index.
type QueryIndex struct {
	rows   []Interval
	series []Series
	spans  map[string][2]int
	charge int64
}

func PrepareQueries(ctx context.Context, set Set) (*QueryIndex, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	raw, e := json.Marshal(set)
	if e != nil {
		return nil, e
	}
	if len(raw) > MaxBytes {
		return nil, budget("model_bytes", len(raw), MaxBytes)
	}
	var owned Set
	if e = Strict(raw, &owned); e != nil {
		return nil, e
	}
	rows, e := Intervals(ctx, owned)
	if e != nil {
		return nil, e
	}
	out := &QueryIndex{rows: rows, spans: map[string][2]int{}, charge: 256}
	for _, s := range owned.Series {
		out.series = append(out.series, Series{ID: s.ID})
		out.charge += 128 + int64(len(s.ID))
	}
	for i, row := range rows {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		span, ok := out.spans[row.SeriesID]
		if !ok {
			span[0] = i
		}
		span[1] = i + 1
		out.spans[row.SeriesID] = span
		// Conservative structural admission units, not measured RSS. Charge shared
		// strings/buckets repeatedly rather than relying on decoder aliasing.
		out.charge += int64(unsafe.Sizeof(row)) + int64(len(row.ID)+len(row.SeriesID)+len(row.Name)+len(row.Unit)+len(row.Kind)+len(row.Temporality)+len(row.Clock)+len(row.Special)+len(row.RateMeaning)+len(row.Status)+len(row.PointID)+len(row.PreviousPointID))
		for _, v := range []*string{row.ObservedStart, row.Start, row.End, row.Timestamp, row.Value, row.Delta, row.Rate} {
			if v != nil {
				out.charge += 32 + int64(len(*v))
			}
		}
		for _, r := range row.Refs {
			out.charge += 64 + int64(len(r.SourceID)+len(r.Position))
		}
		for _, h := range []*Histogram{row.ObservedHistogram, row.Histogram} {
			if h != nil {
				out.charge += 512
				for _, values := range [][]string{h.Bounds, h.Counts} {
					for _, v := range values {
						out.charge += 32 + int64(len(v))
					}
				}
				for _, v := range []*string{h.Sum, h.Min, h.Max, h.ZeroThreshold} {
					if v != nil {
						out.charge += 32 + int64(len(*v))
					}
				}
				for _, b := range []*Buckets{h.Positive, h.Negative} {
					if b != nil {
						out.charge += 64
						for _, v := range b.Counts {
							out.charge += 32 + int64(len(v))
						}
					}
				}
			}
		}
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}
func (i *QueryIndex) Charge() int64 { return i.charge }

func (i *QueryIndex) Query(ctx context.Context, digest string, q Query) (Page, error) {
	out := Page{Version: Version, ResultSHA256: digest, WindowPolicy: "end_timestamp_selection; intervals are not clipped or redistributed", Rows: []Interval{}}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if len(digest) != 64 || q.Limit < 1 || q.Limit > 200 || len(q.Cursor) > 2048 {
		return out, fmt.Errorf("METRIC_QUERY_INVALID")
	}
	q.Selection.SeriesIDs = append([]string(nil), q.Selection.SeriesIDs...)
	if q.Selection.From != nil {
		q.Selection.From = Pointer(*q.Selection.From)
	}
	if q.Selection.To != nil {
		q.Selection.To = Pointer(*q.Selection.To)
	}
	out.Selection = q.Selection
	identity := q
	identity.Cursor = ""
	identity.Selection.SeriesIDs = append([]string(nil), q.Selection.SeriesIDs...)
	sort.Strings(identity.Selection.SeriesIDs)
	out.QuerySHA256 = Hash(identity)
	offset := 0
	if q.Cursor != "" {
		var c struct {
			Result, Query string
			Offset        int
		}
		raw, e := base64.RawURLEncoding.DecodeString(q.Cursor)
		if e != nil || json.Unmarshal(raw, &c) != nil || c.Result != digest || c.Query != out.QuerySHA256 || c.Offset < 0 {
			return out, fmt.Errorf("METRIC_CURSOR_STALE")
		}
		offset = c.Offset
	}
	if e := (Set{Series: i.series}).ValidateSelection(q.Selection); e != nil {
		return out, e
	}
	selected := map[string]bool{}
	for _, id := range q.Selection.SeriesIDs {
		selected[id] = true
	}
	from, fk := Time(q.Selection.From)
	to, tk := Time(q.Selection.To)
	spans := [][2]int{}
	for _, series := range i.series {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if !selected[series.ID] {
			continue
		}
		span, ok := i.spans[series.ID]
		if !ok {
			continue
		}
		lo, hi := span[0], span[1]
		if fk || tk {
			lo += sort.Search(hi-lo, func(k int) bool { return i.rows[lo+k].End != nil })
			if fk {
				base := lo
				lo += sort.Search(hi-lo, func(k int) bool { v, _ := Time(i.rows[base+k].End); return v >= from })
			}
			if tk {
				base := lo
				hi = lo + sort.Search(hi-lo, func(k int) bool { v, _ := Time(i.rows[base+k].End); return v >= to })
			}
		}
		spans = append(spans, [2]int{lo, hi})
		out.Total += hi - lo
	}
	if offset > out.Total {
		return out, fmt.Errorf("METRIC_CURSOR_STALE")
	}
	skip := offset
	rowBytes := 0
	for _, span := range spans {
		if skip >= span[1]-span[0] {
			skip -= span[1] - span[0]
			continue
		}
		for at := span[0] + skip; at < span[1] && len(out.Rows) < q.Limit; at++ {
			if e := ctx.Err(); e != nil {
				return Page{}, e
			}
			raw, e := json.Marshal(i.rows[at])
			if e != nil {
				return Page{}, e
			}
			rowBytes += len(raw)
			if rowBytes > 1<<20 {
				return Page{}, fmt.Errorf("METRIC_QUERY_BUDGET: response exceeds 1 MiB; reduce page size")
			}
			// Decode only bounded outgoing rows: pointers/refs/histograms never escape.
			var copy Interval
			if e = json.Unmarshal(raw, &copy); e != nil {
				return Page{}, e
			}
			out.Rows = append(out.Rows, copy)
		}
		skip = 0
		if len(out.Rows) == q.Limit {
			break
		}
	}
	end := offset + len(out.Rows)
	if end < out.Total {
		raw, _ := json.Marshal(struct {
			Result, Query string
			Offset        int
		}{digest, out.QuerySHA256, end})
		out.Next = base64.RawURLEncoding.EncodeToString(raw)
	}
	raw, e := json.Marshal(out)
	if e != nil {
		return Page{}, e
	}
	if len(raw) > 1<<20 {
		return Page{}, fmt.Errorf("METRIC_QUERY_BUDGET: response exceeds 1 MiB; reduce page size")
	}
	return out, ctx.Err()
}
