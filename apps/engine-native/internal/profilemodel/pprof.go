package profilemodel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	pp "github.com/google/pprof/profile"
)

// ExportPprof requires an explicit integer tick. Both the tick and original
// metric/selection are embedded in Comments; a scaled export uses unit=ticks,
// never falsely labels integer ticks as the original duration/count unit.
func ExportPprof(ctx context.Context, s ProfileSet, selection Selection, tickScale string) ([]byte, error) {
	tick, e := Number(tickScale)
	if e != nil || tick.Sign() <= 0 {
		return nil, fmt.Errorf("explicit pprof tick scale required")
	}
	metric, ok := s.Metric(selection.MetricID)
	if !ok {
		return nil, fmt.Errorf("unknown selected metric")
	}
	scale, e := Number(metric.Scale)
	if e != nil {
		return nil, e
	}
	unit := metric.Unit
	if tick.Cmp(big.NewRat(1, 1)) != 0 {
		unit = "ticks"
	}
	profile := &pp.Profile{SampleType: []*pp.ValueType{{Type: metric.Kind, Unit: unit}}, DefaultSampleType: metric.Kind}
	note, _ := json.Marshal(map[string]any{"schema_version": Version, "metric": metric, "tick_scale": tickScale, "selection": selection, "source_sha256": s.SourceHashes, "scope": "selected_metric_and_profiles_only", "frames": s.Frames, "quality": s.Quality})
	if len(note) > 8<<20 {
		return nil, budget("pprof identity appendix budget exceeded")
	}
	profile.Comments = []string{"profile_set_projection:" + string(note)}
	frames := append([]Frame(nil), s.Frames...)
	sort.Slice(frames, func(i, j int) bool { return frames[i].ID < frames[j].ID })
	locations := map[string]*pp.Location{}
	for index, frame := range frames {
		line := int64(0)
		if frame.Line != nil {
			line = *frame.Line
		}
		id := uint64(index + 1)
		function := &pp.Function{ID: id, Name: frame.Display, SystemName: frame.Symbol, Filename: frame.File, StartLine: line}
		profile.Function = append(profile.Function, function)
		location := &pp.Location{ID: id, Line: []pp.Line{{Function: function, Line: line}}}
		if frame.Module != "" || frame.BuildID != "" {
			mapping := &pp.Mapping{ID: id, File: frame.Module, BuildID: frame.BuildID}
			profile.Mapping = append(profile.Mapping, mapping)
			location.Mapping = mapping
		}
		profile.Location = append(profile.Location, location)
		locations[frame.ID] = location
	}
	e = s.each(ctx, selection, func(p Profile, sample Sample, path []string, weight *big.Rat) error {
		value := new(big.Rat).Quo(new(big.Rat).Mul(weight, scale), tick)
		if !value.IsInt() || !value.Num().IsInt64() {
			return fmt.Errorf("pprof int64 projection would lose precision or range")
		}
		row := &pp.Sample{Value: []int64{value.Num().Int64()}, Label: map[string][]string{}}
		for key, values := range sample.Labels {
			row.Label[key] = append([]string(nil), values...)
		}
		for key, text := range map[string]string{"archscope.profile_id": p.ID, "archscope.process": p.Process, "archscope.thread": p.Thread, "archscope.worker": p.Worker, "archscope.source_ref": sample.SourceRef, "archscope.clock": p.Clock, "archscope.time_unit": p.TimeUnit, "archscope.time_scale": p.TimeScale} {
			if _, collision := row.Label[key]; collision {
				return fmt.Errorf("pprof reserved provenance label collision")
			}
			if text != "" {
				row.Label[key] = []string{text}
			}
		}
		for i := len(path) - 1; i >= 0; i-- {
			row.Location = append(row.Location, locations[path[i]])
		}
		// Event time/interval are strings, preserving exact relative/absolute context.
		for key, value := range map[string]*string{"archscope.timestamp": sample.Timestamp, "archscope.interval": sample.Interval} {
			if _, collision := row.Label[key]; collision {
				return fmt.Errorf("pprof reserved time label collision")
			}
			if (key == "archscope.timestamp" && sample.TimestampNull) || (key == "archscope.interval" && sample.IntervalNull) {
				row.Label[key] = []string{"null"}
			}
			if value != nil {
				row.Label[key] = []string{*value}
			}
		}
		profile.Sample = append(profile.Sample, row)
		return nil
	})
	if e != nil {
		return nil, e
	}
	if len(profile.Sample) == 0 {
		return nil, fmt.Errorf("pprof projection unavailable: no selected observations")
	}
	if e = profile.CheckValid(); e != nil {
		return nil, e
	}
	var out bytes.Buffer
	e = profile.Write(cappedPprofWriter{&out})
	return out.Bytes(), e
}

type cappedPprofWriter struct{ buffer *bytes.Buffer }

func (w cappedPprofWriter) Write(body []byte) (int, error) {
	if len(body) > 64<<20-w.buffer.Len() {
		return 0, budget("pprof output byte budget exceeded")
	}
	return w.buffer.Write(body)
}
