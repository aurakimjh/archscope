package inferencemodel

import (
	"context"
	"fmt"
	"strings"

	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
)

// Declaration is an explicit interpretation of an untyped range series, never
// a rewrite of the original model or evidence of remote counter provenance.
type Declaration struct {
	Unit         string  `json:"unit"`
	Kind         string  `json:"kind"`
	Temporality  string  `json:"temporality"`
	Monotonic    bool    `json:"monotonic"`
	Start        *string `json:"start_ns,omitempty"`
	ExpectedStep *string `json:"expected_step_ns,omitempty"`
}

func EffectiveSeries(signal Signal, series m.Series) (m.Series, error) {
	if signal.Declaration == nil {
		return series, nil
	}
	d := signal.Declaration
	if !strings.HasPrefix(series.Origin, "prometheus-range-json:") || series.Kind != "gauge" || series.Unit != "" {
		return series, fmt.Errorf("INFERENCE_DECLARATION_SOURCE: only original untyped Prometheus range series accept declared metadata")
	}
	if !text(d.Unit) || d.Unit == "" || d.Kind != "gauge" && d.Kind != "sum" || d.Kind == "gauge" && (d.Temporality != "unspecified" || d.Monotonic) || d.Kind == "sum" && (d.Temporality != "cumulative" || !d.Monotonic) {
		return series, fmt.Errorf("INFERENCE_DECLARATION_INVALID: gauge/unspecified or monotonic cumulative sum required; raw range buckets are not assembled into histograms")
	}
	for _, p := range []*string{d.Start, d.ExpectedStep} {
		if p != nil {
			if _, e := m.Uint(*p); e != nil {
				return series, e
			}
		}
	}
	if d.ExpectedStep != nil && *d.ExpectedStep == "0" {
		return series, fmt.Errorf("INFERENCE_DECLARATION_STEP_INVALID")
	}
	series.Unit = d.Unit
	series.Kind = d.Kind
	series.Temporality = d.Temporality
	series.Monotonic = d.Monotonic
	if d.ExpectedStep != nil {
		series.ExpectedStep = d.ExpectedStep
	}
	series.Origin += ":inference-declaration:" + m.Hash(d)
	series.ID = m.SeriesID(series)
	return series, nil
}
func EffectiveMetrics(ctx context.Context, set Set) (m.Set, map[string]string, error) {
	out := set.Metrics
	out.Series = append([]m.Series(nil), set.Metrics.Series...)
	out.Points = append([]m.Point(nil), set.Metrics.Points...)
	signals := map[string]Signal{}
	for _, v := range set.Signals {
		signals[v.SeriesID] = v
	}
	mapping := map[string]string{}
	for i, series := range out.Series {
		v, e := EffectiveSeries(signals[series.ID], series)
		if e != nil {
			return out, nil, e
		}
		mapping[series.ID] = v.ID
		out.Series[i] = v
	}
	for i, p := range out.Points {
		signal := signals[p.SeriesID]
		p.SeriesID = mapping[p.SeriesID]
		if signal.Declaration != nil && signal.Declaration.Start != nil {
			p.Start = signal.Declaration.Start
		}
		p.ID = m.PointID(p)
		out.Points[i] = p
	}
	return out, mapping, out.Validate(ctx)
}

// SelectSeries retains full predecessor/reset context for every chosen series.
func SelectSeries(ctx context.Context, set Set, ids []string) (Set, error) {
	if len(ids) == 0 {
		return set, nil
	}
	selection := m.Selection{SeriesIDs: ids}
	if e := set.Metrics.ValidateSelection(selection); e != nil {
		return set, e
	}
	keep := map[string]bool{}
	for _, id := range ids {
		keep[id] = true
	}
	out := set
	out.Metrics = set.Metrics
	out.Metrics.Series = []m.Series{}
	out.Metrics.Points = []m.Point{}
	out.Metrics.Duplicates = 0
	out.Signals = []Signal{}
	out.Quality = []Quality{}
	points := map[string]bool{}
	for _, v := range set.Metrics.Series {
		if keep[v.ID] {
			out.Metrics.Series = append(out.Metrics.Series, v)
		}
	}
	for _, v := range set.Metrics.Points {
		if keep[v.SeriesID] {
			out.Metrics.Points = append(out.Metrics.Points, v)
			out.Metrics.Duplicates += len(v.Refs) - 1
			points[v.ID] = true
		}
	}
	for _, v := range set.Signals {
		if keep[v.SeriesID] {
			out.Signals = append(out.Signals, v)
		}
	}
	for _, v := range set.Quality {
		if points[v.PointID] {
			out.Quality = append(out.Quality, v)
		}
	}
	return out, out.Validate(ctx)
}

// SelectedDeclarations travels with a materialized view so its interpretation
// can be inspected without recovering or reinterpreting the parent result.
func SelectedDeclarations(set Set, ids []string) map[string]*Declaration {
	out := map[string]*Declaration{}
	keep := map[string]bool{}
	for _, id := range ids {
		keep[id] = true
	}
	for _, v := range set.Signals {
		if keep[v.ID] && v.Declaration != nil {
			out[v.ID] = v.Declaration
		}
	}
	return out
}
