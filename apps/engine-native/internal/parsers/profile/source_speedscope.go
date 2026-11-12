package profile

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"math/big"
	"strings"
)

type scopeFile struct {
	Schema   string `json:"$schema"`
	Exporter string `json:"exporter"`
	Shared   struct {
		Frames []struct {
			Name   string `json:"name"`
			File   string `json:"file"`
			Line   *int64 `json:"line"`
			Column *int64 `json:"col"`
		} `json:"frames"`
	} `json:"shared"`
	Profiles []scopeProfile `json:"profiles"`
}
type scopeProfile struct {
	Type    string        `json:"type"`
	Name    string        `json:"name"`
	Unit    string        `json:"unit"`
	Start   json.Number   `json:"startValue"`
	End     json.Number   `json:"endValue"`
	Samples [][]int       `json:"samples"`
	Weights []json.Number `json:"weights"`
	Events  []struct {
		Type  string      `json:"type"`
		At    json.Number `json:"at"`
		Frame *int        `json:"frame"`
	} `json:"events"`
}

func (b *sourceBuilder) speedscope(body []byte, options SourceOptions) error {
	var file scopeFile
	if e := wirejson.DecodeWithLimits(body, &file, wirejson.ResultLimits(SourceMaxBytes, 256)); e != nil {
		return e
	}
	if file.Schema != "https://www.speedscope.app/file-format-schema.json" || len(file.Profiles) == 0 || len(file.Profiles) > 1024 || len(file.Shared.Frames) > profilemodel.MaxFrames {
		return fmt.Errorf("PROFILE_SPEEDSCOPE_INVALID: schema/profiles/frames")
	}
	b.contract.FormatVersion = "speedscope.schema.v1"
	if file.Exporter != "" {
		parts := strings.SplitN(file.Exporter, "@", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("PROFILE_EXPORTER_INVALID")
		}
		if options.Producer != "" && options.Producer != parts[0] || options.ProducerVersion != "" && options.ProducerVersion != parts[1] {
			return fmt.Errorf("PROFILE_PRODUCER_CONFLICT")
		}
		b.set.Producer, b.set.ProducerVersion = parts[0], parts[1]
		b.contract.ProducerBasis = "file_exporter"
	}
	raw, _ := json.Marshal(file.Exporter)
	b.contract.Attributes["exporter"] = raw
	frames := []string{}
	for _, f := range file.Shared.Frames {
		if f.Name == "" || f.Line != nil && *f.Line < 0 || f.Column != nil && *f.Column < 0 {
			return fmt.Errorf("PROFILE_FRAME_INVALID")
		}
		module := ""
		if f.Column != nil {
			module = fmt.Sprintf("source-column:%d", *f.Column)
		}
		frames = append(frames, b.frame(profilemodel.Frame{Symbol: f.Name, Display: f.Name, File: f.File, Line: f.Line, Module: module}))
		if b.failure != nil {
			return b.failure
		}
	}
	metricIndex := map[string]int{}
	for index, p := range file.Profiles {
		start, e := number(p.Start)
		if e != nil {
			return e
		}
		end, e := number(p.End)
		if e != nil || end.Cmp(start) < 0 {
			return fmt.Errorf("PROFILE_TIME_INVALID: end precedes start")
		}
		unit, kind := p.Unit, "duration"
		switch unit {
		case "none":
			unit = "count"
			kind = "count"
		case "bytes":
			kind = "bytes"
		case "seconds", "milliseconds", "microseconds", "nanoseconds":
		default:
			return fmt.Errorf("PROFILE_UNIT_UNSUPPORTED")
		}
		if p.Type == "evented" && (kind == "count" || kind == "bytes") {
			return fmt.Errorf("PROFILE_EVENT_UNIT_UNSUPPORTED: evented clock must be time")
		}
		metricID := p.Type + "-" + unit
		if _, ok := metricIndex[metricID]; !ok {
			metricIndex[metricID] = len(b.set.Metrics)
			b.set.Metrics = append(b.set.Metrics, profilemodel.Metric{ID: metricID, Kind: kind, Unit: unit, Scale: "1"})
			b.contract.Metrics = append(b.contract.Metrics, profilemodel.SourceMetric{ID: metricID, Type: p.Type, Unit: p.Unit})
		}
		id := fmt.Sprintf("profile-%d", index)
		profile := profilemodel.Profile{ID: id, Worker: p.Name, Clock: "aggregate", Samples: []profilemodel.Sample{}}
		if p.Type == "evented" {
			profile.Clock = "relative"
			profile.TimeUnit = unit
			profile.TimeScale = "1"
		}
		b.set.Profiles = append(b.set.Profiles, profile)
		b.contract.Profiles = append(b.contract.Profiles, profilemodel.SourceProfile{ID: id, Name: p.Name, Type: p.Type, Start: profilemodel.Text(start), End: profilemodel.Text(end), Unit: p.Unit})
		path := func(stack []int) ([]string, error) {
			ids := []string{}
			if len(stack) > profilemodel.MaxDepth {
				return nil, fmt.Errorf("PROFILE_DEPTH_LIMIT")
			}
			for _, j := range stack {
				if j < 0 || j >= len(frames) {
					return nil, fmt.Errorf("PROFILE_FRAME_REFERENCE_INVALID")
				}
				ids = append(ids, frames[j])
			}
			return ids, nil
		}
		switch p.Type {
		case "sampled":
			if p.Samples == nil || p.Weights == nil || len(p.Samples) != len(p.Weights) {
				return fmt.Errorf("PROFILE_WEIGHTS_REQUIRED: one exact weight per stack")
			}
			for j, stack := range p.Samples {
				weight, e := number(p.Weights[j])
				if e != nil || weight.Sign() < 0 {
					return fmt.Errorf("PROFILE_WEIGHT_INVALID")
				}
				ids, e := path(stack)
				if e != nil {
					return e
				}
				if e = b.sample(index, ids, metricID, profilemodel.Text(weight), nil, nil, nil, fmt.Sprintf("profile:%d/sample:%d", index, j)); e != nil {
					return e
				}
			}
		case "evented":
			if p.Events == nil {
				return fmt.Errorf("PROFILE_EVENTS_REQUIRED")
			}
			stack := []int{}
			previous := new(big.Rat).Set(start)
			for j, event := range p.Events {
				at, e := number(event.At)
				if e != nil || at.Cmp(previous) < 0 || at.Cmp(end) > 0 || event.Frame == nil || *event.Frame < 0 || *event.Frame >= len(frames) {
					return fmt.Errorf("PROFILE_EVENT_ORDER_INVALID")
				}
				if event.Type != "O" && event.Type != "C" {
					return fmt.Errorf("PROFILE_EVENT_TYPE_INVALID")
				}
				if len(stack) > 0 || at.Cmp(previous) > 0 {
					ids, e := path(stack)
					if e != nil {
						return e
					}
					weight := new(big.Rat).Sub(at, previous)
					if e = b.sample(index, ids, metricID, profilemodel.Text(weight), numberText(previous), numberText(weight), nil, fmt.Sprintf("profile:%d/event:%d", index, j)); e != nil {
						return e
					}
				}
				if event.Type == "O" {
					stack = append(stack, *event.Frame)
					if len(stack) > profilemodel.MaxDepth {
						return fmt.Errorf("PROFILE_DEPTH_LIMIT")
					}
				} else {
					if len(stack) == 0 || stack[len(stack)-1] != *event.Frame {
						return fmt.Errorf("PROFILE_EVENT_UNBALANCED")
					}
					stack = stack[:len(stack)-1]
				}
				previous.Set(at)
			}
			if len(stack) != 0 {
				return fmt.Errorf("PROFILE_EVENT_UNBALANCED")
			}
			if end.Cmp(previous) > 0 {
				weight := new(big.Rat).Sub(end, previous)
				if e = b.sample(index, nil, metricID, profilemodel.Text(weight), numberText(previous), numberText(weight), nil, fmt.Sprintf("profile:%d/unattributed-tail", index)); e != nil {
					return e
				}
			}
		default:
			return fmt.Errorf("PROFILE_TYPE_UNSUPPORTED")
		}
	}
	var capture map[string]json.RawMessage
	json.Unmarshal(b.set.CaptureOptions, &capture)
	if _, ok := capture["source_profile_bounds"]; ok {
		return fmt.Errorf("PROFILE_CAPTURE_RESERVED_KEY")
	}
	capture["source_profile_bounds"], _ = json.Marshal(b.contract.Profiles)
	b.set.CaptureOptions, _ = json.Marshal(capture)
	b.set.Quality.ConversionLoss = append(b.set.Quality.ConversionLoss, "sampled weights remain aggregate observations; evented gaps are unattributed elapsed intervals; no CPU-time inference")
	return nil
}
