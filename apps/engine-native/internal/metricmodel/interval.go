package metricmodel

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"time"
)

type Interval struct {
	ObservedStart     *string    `json:"observed_start_ns"`
	ObservedHistogram *Histogram `json:"observed_histogram,omitempty"`
	ID                string     `json:"id"`
	SeriesID          string     `json:"series_id"`
	Name              string     `json:"name"`
	Unit              string     `json:"unit"`
	Kind              string     `json:"kind"`
	Temporality       string     `json:"temporality"`
	Start             *string    `json:"start_ns"`
	End               *string    `json:"end_ns"`
	Timestamp         *string    `json:"time"`
	Clock             string     `json:"clock"`
	Value             *string    `json:"value"`
	Special           string     `json:"special,omitempty"`
	Delta             *string    `json:"delta"`
	Rate              *string    `json:"rate"`
	RateMeaning       string     `json:"rate_meaning"`
	Histogram         *Histogram `json:"histogram,omitempty"`
	Status            string     `json:"status"`
	Gap               bool       `json:"gap"`
	Reordered         bool       `json:"reordered"`
	PointID           string     `json:"point_id"`
	PreviousPointID   string     `json:"previous_point_id,omitempty"`
	Refs              []Ref      `json:"refs"`
}

func Pointer(s string) *string { return &s }
func Timestamp(ns *string) *string {
	n, ok := Time(ns)
	if !ok {
		return nil
	}
	value := time.Unix(int64(n/1_000_000_000), int64(n%1_000_000_000)).UTC().Format(time.RFC3339Nano)
	return &value
}
func elapsed(start, end *string) (*big.Rat, bool) {
	a, ak := Time(start)
	b, bk := Time(end)
	if !ak || !bk || a >= b {
		return nil, false
	}
	return new(big.Rat).SetFrac(new(big.Int).SetUint64(b-a), big.NewInt(1_000_000_000)), true
}
func integerDelta(a, b string) (string, error) {
	x, e := Uint(a)
	if e != nil {
		return "", e
	}
	y, e := Uint(b)
	if e != nil {
		return "", e
	}
	x.Sub(x, y)
	if x.Sign() < 0 {
		return "", fmt.Errorf("counter_decreased")
	}
	return x.String(), nil
}
func histogramDelta(current, previous *Histogram, kind string) (*Histogram, error) {
	if current == nil || previous == nil {
		return nil, fmt.Errorf("histogram_unavailable")
	}
	out := *current
	out.Min, out.Max = nil, nil
	count, e := integerDelta(current.Count, previous.Count)
	if e != nil {
		return nil, e
	}
	out.Count = count
	out.Sum = nil
	if current.Sum != nil && previous.Sum != nil {
		a, e := Number(*current.Sum)
		if e != nil {
			return nil, e
		}
		b, e := Number(*previous.Sum)
		if e != nil {
			return nil, e
		}
		out.Sum = Pointer(Text(new(big.Rat).Sub(a, b)))
	}
	if kind == "histogram" {
		if Hash(current.Bounds) != Hash(previous.Bounds) || len(current.Counts) != len(previous.Counts) {
			return nil, fmt.Errorf("histogram_schema_changed")
		}
		out.Counts = []string{}
		for i, value := range current.Counts {
			delta, e := integerDelta(value, previous.Counts[i])
			if e != nil {
				return nil, e
			}
			out.Counts = append(out.Counts, delta)
		}
	} else {
		if current.Scale == nil || previous.Scale == nil || *current.Scale != *previous.Scale || Hash(current.ZeroThreshold) != Hash(previous.ZeroThreshold) {
			return nil, fmt.Errorf("histogram_schema_changed")
		}
		out.ZeroCount, e = integerDelta(current.ZeroCount, previous.ZeroCount)
		if e != nil {
			return nil, e
		}
		subtract := func(a, b *Buckets) (*Buckets, error) {
			if a == nil && b == nil {
				return nil, nil
			}
			values := map[int64]string{}
			if a != nil {
				for i, v := range a.Counts {
					values[int64(a.Offset)+int64(i)] = v
				}
			}
			if b != nil {
				for i, v := range b.Counts {
					idx := int64(b.Offset) + int64(i)
					current := values[idx]
					if current == "" {
						current = "0"
					}
					delta, e := integerDelta(current, v)
					if e != nil {
						return nil, e
					}
					values[idx] = delta
				}
			}
			if len(values) == 0 {
				return &Buckets{Counts: []string{}}, nil
			}
			min, max := int64(1<<62), int64(-1<<62)
			for idx := range values {
				if idx < min {
					min = idx
				}
				if idx > max {
					max = idx
				}
			}
			if max-min+1 > MaxBuckets || min < -2147483648 || max > 2147483647 {
				return nil, fmt.Errorf("histogram_index_span_limit")
			}
			out := &Buckets{Offset: int32(min), Counts: []string{}}
			for i := min; i <= max; i++ {
				v := values[i]
				if v == "" {
					v = "0"
				}
				out.Counts = append(out.Counts, v)
			}
			return out, nil
		}
		out.Positive, e = subtract(current.Positive, previous.Positive)
		if e != nil {
			return nil, e
		}
		out.Negative, e = subtract(current.Negative, previous.Negative)
		if e != nil {
			return nil, e
		}
	}
	if _, e = validateHistogram(kind, &out); e != nil {
		return nil, e
	}
	return &out, nil
}
func derive(row *Interval, point Point, previous *Point, series Series) {
	if _, ok := Time(point.End); !ok {
		row.Status = "timestamp_unavailable"
		return
	}
	if point.Flags&1 != 0 {
		row.Status = "no_recorded_value"
		return
	}
	if point.Flags&^uint32(1) != 0 {
		row.Status = "unsupported_data_point_flags"
		return
	}
	if point.Special != "" {
		row.Status = "non_finite"
		return
	}
	if series.Kind == "gauge" {
		row.Status = "observed_gauge"
		return
	}
	if series.Temporality == "unspecified" {
		row.Status = "temporality_unspecified"
		return
	}
	if series.Temporality == "cumulative" {
		if previous == nil {
			row.Status = "first_cumulative_point"
			return
		}
		if previous.Flags != 0 || previous.Special != "" {
			row.Status = "previous_unavailable"
			return
		}
		row.PreviousPointID = previous.ID
		end, _ := Time(point.End)
		before, bk := Time(previous.End)
		if !bk || end <= before {
			row.Status = "duplicate_or_ambiguous_timestamp"
			return
		}
		start, sk := Time(point.Start)
		priorStart, pk := Time(previous.Start)
		if !sk || !pk {
			row.Status = "start_time_unavailable"
			return
		}
		if start != priorStart {
			if start < before {
				row.Status = "lifetime_overlap"
				return
			}
			if start >= end {
				row.Status = "unknown_start_reset"
				return
			}
			row.Status = "reset"
			row.Gap = start > before
			row.Start = point.Start
			if series.Kind == "sum" {
				row.Delta = point.Value
			} else {
				row.Histogram = point.Histogram
				row.Delta = Pointer(point.Histogram.Count)
			}
		} else {
			row.Start = previous.End
			row.Status = "derived"
			if series.ExpectedStep != nil {
				step, _ := Time(series.ExpectedStep)
				row.Gap = end-before > step
				if row.Gap {
					row.Status = "gap_average"
				}
			}
			if series.Kind == "sum" {
				if point.Value == nil || previous.Value == nil {
					row.Status = "value_unavailable"
					return
				}
				a, _ := Number(*point.Value)
				b, _ := Number(*previous.Value)
				delta := new(big.Rat).Sub(a, b)
				if series.Monotonic && delta.Sign() < 0 {
					row.Status = "counter_reset_unannounced"
					return
				}
				row.Delta = Pointer(Text(delta))
			} else {
				hist, e := histogramDelta(point.Histogram, previous.Histogram, series.Kind)
				if e != nil {
					row.Status = e.Error()
					return
				}
				row.Histogram = hist
				row.Delta = Pointer(hist.Count)
			}
		}
	} else {
		row.Status = "derived_delta"
		if series.Kind == "sum" {
			row.Delta = point.Value
		} else {
			row.Histogram = point.Histogram
			row.Delta = Pointer(point.Histogram.Count)
		}
	}
	span, ok := elapsed(row.Start, row.End)
	if !ok {
		row.Status = "interval_unavailable"
		row.Delta = nil
		row.Histogram = nil
		return
	}
	if row.Delta != nil {
		delta, _ := Number(*row.Delta)
		row.Rate = Pointer(new(big.Rat).Quo(delta, span).RatString())
		row.RateMeaning = "increment_per_second"
		if series.Kind == "sum" && !series.Monotonic {
			row.RateMeaning = "signed_change_per_second"
		}
		if series.Kind != "sum" {
			row.RateMeaning = "observations_per_second"
		}
	}
}
func Intervals(ctx context.Context, set Set) ([]Interval, error) {
	if e := set.Validate(ctx); e != nil {
		return nil, e
	}
	out := []Interval{}
	groups := map[string][]Point{}
	reordered := map[string]bool{}
	last := map[string]uint64{}
	for _, p := range set.Points {
		if end, ok := Time(p.End); ok {
			if end < last[p.SeriesID] {
				reordered[p.ID] = true
			}
			last[p.SeriesID] = end
		}
		groups[p.SeriesID] = append(groups[p.SeriesID], p)
	}
	for _, series := range set.Series {
		points := groups[series.ID]
		sort.SliceStable(points, func(i, j int) bool {
			a, ak := Time(points[i].End)
			b, bk := Time(points[j].End)
			if ak != bk {
				return !ak
			}
			if a != b {
				return a < b
			}
			return points[i].ID < points[j].ID
		})
		startIndex := len(out)
		for i, p := range points {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			row := Interval{ObservedStart: p.Start, ObservedHistogram: p.Histogram, ID: "interval-" + p.ID, SeriesID: series.ID, Name: series.Name, Unit: series.Unit, Kind: series.Kind, Temporality: series.Temporality, Start: p.Start, End: p.End, Timestamp: Timestamp(p.End), Clock: "unix", Value: p.Value, Special: p.Special, PointID: p.ID, Refs: append([]Ref(nil), p.Refs...), Reordered: reordered[p.ID]}
			var previous *Point
			if i > 0 {
				previous = &points[i-1]
			}
			derive(&row, p, previous, series)
			if set.Normalizer != Normalizer && series.Kind == "gauge" && series.ExpectedStep != nil && previous != nil {
				end, ok := Time(p.End)
				before, known := Time(previous.End)
				step, _ := Time(series.ExpectedStep)
				// A gap describes observed cadence, independently of value availability.
				row.Gap = ok && known && end > before && end-before > step
			}
			out = append(out, row)
		}
		// Conflicting equal timestamps never select an arbitrary writer/predecessor.
		for i := startIndex; i < len(out); i++ {
			if i > startIndex && out[i].End != nil && out[i-1].End != nil && *out[i].End == *out[i-1].End {
				for _, j := range []int{i - 1, i} {
					out[j].Status = "conflicting_timestamp"
					out[j].Delta = nil
					out[j].Rate = nil
					out[j].Histogram = nil
				}
				if i+1 < len(out) && series.Temporality == "cumulative" {
					out[i+1].Status = "ambiguous_predecessor"
					out[i+1].Delta = nil
					out[i+1].Rate = nil
					out[i+1].Histogram = nil
				}
			}
		}
		if series.Temporality == "delta" {
			indices := []int{}
			for i := startIndex; i < len(out); i++ {
				if _, ok := elapsed(out[i].Start, out[i].End); ok {
					indices = append(indices, i)
				}
			}
			sort.Slice(indices, func(i, j int) bool {
				a, _ := Time(out[indices[i]].Start)
				b, _ := Time(out[indices[j]].Start)
				if a != b {
					return a < b
				}
				return indices[i] < indices[j]
			})
			var maximum uint64
			maxIndex := -1
			var previousEnd uint64
			for _, index := range indices {
				a, _ := Time(out[index].Start)
				b, _ := Time(out[index].End)
				if maxIndex >= 0 && a < maximum {
					for _, j := range []int{maxIndex, index} {
						out[j].Status = "overlap"
						out[j].Delta = nil
						out[j].Rate = nil
						out[j].Histogram = nil
					}
				}
				if previousEnd != 0 && a > previousEnd {
					out[index].Gap = true
				}
				if b > maximum {
					maximum = b
					maxIndex = index
				}
				if b > previousEnd {
					previousEnd = b
				}
			}
		}
	}
	return out, nil
}
func Ns(value uint64) *string { return Pointer(strconv.FormatUint(value, 10)) }
