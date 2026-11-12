package runtimeevidence

import (
	"context"
	"encoding/json"
	"fmt"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
)

// AllocationProfile projects one entity and one source snapshot only. Multiple
// captures are never added together; the provided live rows need not cover the
// whole process. Reuse the existing exact profile tree and visual pipelines.
func AllocationProfile(ctx context.Context, s r.Set, entity, source, metric string) (p.ProfileSet, p.Selection, error) {
	out := p.ProfileSet{}
	selection := p.Selection{}
	if entity == "" || source == "" || !r.In(metric, "live_bytes", "live_blocks") {
		return out, selection, fmt.Errorf("RUNTIME_ALLOCATION_SELECTION_REQUIRED")
	}
	sourceSHA := ""
	for _, v := range s.Sources {
		if v.ID == source {
			sourceSHA = v.SHA
		}
	}
	if sourceSHA == "" {
		return out, selection, fmt.Errorf("RUNTIME_SOURCE_MISSING")
	}
	unit, kind := "bytes", "bytes"
	if metric == "live_blocks" {
		unit, kind = "count", "count"
	}
	rows := []p.Sample{}
	stacks := map[string]bool{}
	captureTime := ""
	timeSet := false
	for _, v := range s.Observations {
		if e := ctx.Err(); e != nil {
			return out, selection, e
		}
		if v.Entity == entity && v.Ref.Source == source && v.Kind == "allocation" && v.Metric == metric {
			if v.Basis != "snapshot" {
				return out, selection, fmt.Errorf("RUNTIME_ALLOCATION_SNAPSHOT_REQUIRED")
			}
			time := "unknown"
			if v.Time != nil {
				time = *v.Time
			}
			if timeSet && time != captureTime {
				return out, selection, fmt.Errorf("RUNTIME_ALLOCATION_MULTIPLE_SNAPSHOTS: split the source snapshot")
			}
			captureTime = time
			timeSet = true
			if v.Value == nil {
				continue
			}
			if v.Stack == "" {
				return out, selection, fmt.Errorf("RUNTIME_ALLOCATION_STACK_REQUIRED")
			}
			stacks[v.Stack] = true
			rows = append(rows, p.Sample{StackID: v.Stack, MetricID: metric, Weight: *v.Value, SourceRef: v.ID})
		}
	}
	if len(rows) == 0 {
		return out, selection, fmt.Errorf("RUNTIME_ALLOCATION_ROWS_UNAVAILABLE")
	}
	capture, _ := json.Marshal(map[string]string{"entity_id": entity, "source_id": source, "metric": metric, "snapshot_time_ns": captureTime, "semantics": "provided live traced allocation rows; not cumulative allocation or whole-process retained heap"})
	out = p.ProfileSet{SchemaVersion: p.Version, Producer: "runtime allocation snapshot projection", ProducerVersion: "1.0.0", Format: "runtime-allocation-snapshot", SourceHashes: []string{sourceSHA}, CaptureOptions: capture, Frames: []p.Frame{}, Stacks: []p.Stack{}, Metrics: []p.Metric{{ID: metric, Kind: kind, Unit: unit, Scale: "1", SourceType: "tracemalloc_" + metric, SourceUnit: unit}}, Profiles: []p.Profile{{ID: "allocation-snapshot", Process: entity, Clock: "aggregate", Samples: rows}}, Quality: p.Quality{ClockConfidence: "unavailable", ConversionLoss: []string{"Provided live traced rows only; not whole-process heap coverage, retained size or cumulative allocation rate.", "No event chronology is reconstructed from an allocation snapshot."}}}
	frames := map[string]bool{}
	for _, v := range s.Stacks {
		if stacks[v.ID] {
			out.Stacks = append(out.Stacks, v)
			for _, id := range v.Frames {
				frames[id] = true
			}
		}
	}
	for _, v := range s.Frames {
		if frames[v.ID] {
			out.Frames = append(out.Frames, v)
		}
	}
	selection = p.Selection{ProfileIDs: []string{"allocation-snapshot"}, MetricID: metric}
	return out, selection, out.Validate()
}
