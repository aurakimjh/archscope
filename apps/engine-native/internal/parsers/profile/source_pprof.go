package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	pp "github.com/google/pprof/profile"
	"strconv"
)

func (b *sourceBuilder) pprof(body []byte) error {
	p, e := pp.ParseUncompressed(body)
	if e != nil {
		return e
	}
	if e = p.CheckValid(); e != nil {
		return e
	}
	if len(p.SampleType) == 0 || len(p.SampleType) > 64 || len(p.Location) > profilemodel.MaxFrames {
		return fmt.Errorf("PROFILE_PPROF_INVALID: value/location budget")
	}
	b.contract.FormatVersion = "profile.proto.v1"
	metrics := []profilemodel.Metric{}
	for i, t := range p.SampleType {
		if t == nil {
			return fmt.Errorf("PROFILE_PPROF_VALUE_TYPE_INVALID")
		}
		kind := ""
		unit := t.Unit
		switch unit {
		case "count", "samples", "objects":
			kind = "sample_count"
			unit = "count"
		case "bytes":
			kind = "bytes"
		case "nanoseconds", "microseconds", "milliseconds", "seconds":
			kind = "duration"
			if t.Type == "cpu" {
				kind = "cpu_time"
			} else if t.Type == "wall" {
				kind = "wall_time"
			}
		case "cycles":
			kind = "cycles"
		default:
			return fmt.Errorf("PROFILE_PPROF_UNIT_UNSUPPORTED: %s", t.Unit)
		}
		id := fmt.Sprintf("pprof-%d", i)
		m := profilemodel.Metric{ID: id, Kind: kind, Unit: unit, Scale: "1"}
		for _, sample := range p.Sample {
			if sample.Value[i] < 0 {
				m.Kind = "signed_difference"
				m.BaseKind = kind
				m.Signed = true
				break
			}
		}
		metrics = append(metrics, m)
		b.contract.Metrics = append(b.contract.Metrics, profilemodel.SourceMetric{ID: id, Type: t.Type, Unit: t.Unit})
	}
	b.set.Metrics = metrics
	frameByLocation := map[uint64][]string{}
	for _, location := range p.Location {
		ids := []string{}
		module, build := "", ""
		if location.Mapping != nil {
			module = location.Mapping.File
			build = location.Mapping.BuildID
		}
		if len(location.Line) == 0 {
			symbol := fmt.Sprintf("[address:0x%x]", location.Address)
			ids = append(ids, b.frame(profilemodel.Frame{Symbol: symbol, Display: symbol, Module: module, BuildID: build, Native: true}))
		} else {
			// pprof locations and inline lines are leaf-first; canonical stacks are root-first.
			for j := len(location.Line) - 1; j >= 0; j-- {
				line := location.Line[j]
				if line.Function == nil {
					return fmt.Errorf("PROFILE_PPROF_FUNCTION_INVALID")
				}
				symbol := line.Function.SystemName
				if symbol == "" {
					symbol = line.Function.Name
				}
				display := line.Function.Name
				if display == "" {
					display = symbol
				}
				if symbol == "" {
					symbol = fmt.Sprintf("[function:%d]", line.Function.ID)
					display = symbol
				}
				var at *int64
				if line.Line > 0 {
					copy := line.Line
					at = &copy
				}
				ids = append(ids, b.frame(profilemodel.Frame{Symbol: symbol, Display: display, Module: module, BuildID: build, File: line.Function.Filename, Line: at, Inline: len(location.Line) > 1}))
			}
		}
		if b.failure != nil {
			return b.failure
		}
		frameByLocation[location.ID] = ids
	}
	groups := map[string]int{}
	for i, sample := range p.Sample {
		identity := map[string]any{}
		for _, key := range []string{"process", "pid", "thread", "thread_id", "tid", "worker", "rank"} {
			if v, ok := sample.Label[key]; ok {
				identity[key] = v
			}
			if v, ok := sample.NumLabel[key]; ok {
				values := []string{}
				for _, n := range v {
					values = append(values, strconv.FormatInt(n, 10))
				}
				identity["numeric."+key] = values
				identity["numeric_unit."+key] = sample.NumUnit[key]
			}
		}
		keyBytes, _ := json.Marshal(identity)
		key := string(keyBytes)
		group, ok := groups[key]
		if !ok {
			if len(b.set.Profiles) >= 1024 {
				return fmt.Errorf("PROFILE_PROFILE_LIMIT")
			}
			group = len(b.set.Profiles)
			groups[key] = group
			id := "profile-0"
			name := "pprof aggregate"
			if len(identity) > 0 {
				sum := sha256.Sum256(keyBytes)
				id = "profile-" + hex.EncodeToString(sum[:8])
				name = key
			}
			profile := profilemodel.Profile{ID: id, Worker: name, Clock: "aggregate", Samples: []profilemodel.Sample{}}
			b.set.Profiles = append(b.set.Profiles, profile)
			b.contract.Profiles = append(b.contract.Profiles, profilemodel.SourceProfile{ID: id, Name: name, Type: "pprof"})
		}
		ids := []string{}
		for j := len(sample.Location) - 1; j >= 0; j-- {
			ids = append(ids, frameByLocation[sample.Location[j].ID]...)
		}
		labels := map[string][]string{}
		for key, v := range sample.Label {
			labels[key] = append([]string(nil), v...)
		}
		for key, v := range sample.NumLabel {
			numeric := []string{}
			for _, n := range v {
				numeric = append(numeric, strconv.FormatInt(n, 10))
			}
			reserved := "pprof.numeric." + key
			if _, collision := labels[reserved]; collision {
				return fmt.Errorf("PROFILE_LABEL_COLLISION")
			}
			labels[reserved] = numeric
			unitsKey := "pprof.numeric_unit." + key
			if _, collision := labels[unitsKey]; collision {
				return fmt.Errorf("PROFILE_LABEL_COLLISION")
			}
			labels[unitsKey] = append([]string(nil), sample.NumUnit[key]...)
		}
		for j, m := range metrics {
			if e = b.sample(group, ids, m.ID, strconv.FormatInt(sample.Value[j], 10), nil, nil, labels, fmt.Sprintf("pprof.sample:%d/value:%d", i, j)); e != nil {
				return e
			}
		}
	}
	if len(b.set.Profiles) == 0 {
		b.set.Profiles = []profilemodel.Profile{{ID: "profile-0", Worker: "pprof aggregate", Clock: "aggregate", Samples: []profilemodel.Sample{}}}
		b.contract.Profiles = []profilemodel.SourceProfile{{ID: "profile-0", Name: "pprof aggregate", Type: "pprof"}}
	}
	// Retain period metadata verbatim; values already have explicit units, so the
	// sampling period is never multiplied into Sample.Value a second time.
	metadata := map[string]any{"period": strconv.FormatInt(p.Period, 10), "period_type": p.PeriodType, "time_nanos": strconv.FormatInt(p.TimeNanos, 10), "duration_nanos": strconv.FormatInt(p.DurationNanos, 10), "default_sample_type": p.DefaultSampleType, "comments": p.Comments, "locations": p.Location, "mappings": p.Mapping, "functions": p.Function, "numeric_labels": "pprof.numeric.* and pprof.numeric_unit.* preserve value/unit arrays"}
	raw, _ := json.Marshal(metadata)
	b.contract.Attributes["pprof"] = raw
	var options map[string]json.RawMessage
	json.Unmarshal(b.set.CaptureOptions, &options)
	for key, value := range map[string]any{"source_period": strconv.FormatInt(p.Period, 10), "source_period_type": p.PeriodType, "source_duration": strconv.FormatInt(p.DurationNanos, 10)} {
		if _, exists := options[key]; exists {
			return fmt.Errorf("PROFILE_CAPTURE_RESERVED_KEY")
		}
		options[key], _ = json.Marshal(value)
	}
	b.set.CaptureOptions, _ = json.Marshal(options)
	return nil
}
