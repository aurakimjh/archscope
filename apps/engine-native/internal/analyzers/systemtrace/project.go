package systemtrace

import (
	"context"
	"encoding/json"
	"fmt"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"

	"strings"
)

type Projection struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Unit string `json:"unit"`
}

func Projections(s m.Set) []Projection {
	out := []Projection{}
	for _, d := range s.Capture.Events {
		if d.Kind == "sample" {
			out = append(out, Projection{"samples/" + d.Name, "sample_count", "count"})
			if d.PeriodKind != "" {
				out = append(out, Projection{"period/" + d.Name, d.PeriodKind, d.PeriodUnit})
			}
		}
		if d.Kind == "block_io" || d.Kind == "syscall" || d.Kind == "network" {
			out = append(out, Projection{"duration/" + d.Name, "duration", "nanoseconds"})
		}
	}
	scheduler, wakeup := false, false
	for _, d := range s.Capture.Events {
		scheduler = scheduler || d.Kind == "sched_switch"
		wakeup = wakeup || d.Kind == "wakeup"
	}
	if scheduler {
		out = append(out, Projection{"off_cpu", "duration", "nanoseconds"}, Projection{"on_cpu", "duration", "nanoseconds"})
	}
	if scheduler || wakeup {
		out = append(out, Projection{"runqueue", "duration", "nanoseconds"})
	}
	return out
}
func Project(ctx context.Context, s m.Set, metricID string, entityIDs []string) (p.ProfileSet, error) {
	var result p.ProfileSet
	if e := s.Validate(ctx); e != nil {
		return result, e
	}
	var metric Projection
	for _, v := range Projections(s) {
		if v.ID == metricID {
			metric = v
		}
	}
	if metric.ID == "" {
		return result, fmt.Errorf("SYSTEM_PROJECTION_UNSUPPORTED: select a sample, period or qualified duration; aggregate histograms have no event stacks")
	}
	allowed := map[string]bool{}
	for _, e := range s.Capture.Entities {
		allowed[e.ID] = len(entityIDs) == 0
	}
	for _, id := range entityIDs {
		if _, ok := allowed[id]; !ok {
			return result, fmt.Errorf("SYSTEM_ENTITY_SELECTION_INVALID")
		}
		allowed[id] = true
	}
	selectedCapture := s.Capture
	selectedCapture.Entities = nil
	for _, e := range s.Capture.Entities {
		if allowed[e.ID] {
			selectedCapture.Entities = append(selectedCapture.Entities, e)
		}
	}
	capture, _ := json.Marshal(struct {
		Capture    m.Capture `json:"system_capture"`
		Source     string    `json:"source_sha256"`
		Projection string    `json:"projection"`
	}{selectedCapture, s.SourceSHA256, metricID})
	if len(capture) > 64<<10 {
		return result, fmt.Errorf("SYSTEM_PROJECTION_CONTEXT_BUDGET: select fewer entity lifetimes")
	}
	result = p.ProfileSet{SchemaVersion: p.Version, Producer: s.Capture.Tool, ProducerVersion: s.Capture.ToolVersion, Format: "system-projection.v1", SourceHashes: []string{s.SourceSHA256}, CaptureOptions: capture, Frames: []p.Frame{}, Stacks: []p.Stack{}, Metrics: []p.Metric{{ID: metric.ID, Kind: metric.Kind, Unit: metric.Unit, Scale: "1", SourceType: metricID, SourceUnit: metric.Unit}}, Profiles: []p.Profile{}, Quality: p.Quality{Lost: s.Capture.Lost, Unknown: s.Capture.UnwindFailures, ClockConfidence: "declared", ConversionLoss: []string{"File-only projection; source/clock/lifetime declarations are not authenticated.", "Sample count and period stay separate; no frequency/multiplex conversion.", "Scheduler duration is qualified endpoint evidence, not lock contention or request latency.", "Unqualified/incomplete intervals are excluded from the numeric profile; inspect system interval rows.", "Selected observation costs can overlap or exceed walltime; phase definitions stay separate from request latency.", "Frames retain module/address/context identity; no implicit symbol download or demangling."}}}
	if s.Capture.Clock == "unknown" {
		result.Quality.ClockConfidence = "unavailable"
		result.Quality.ConversionLoss = append(result.Quality.ConversionLoss, "Clock is unknown: sample timestamps are retained in the parent evidence and omitted from the aggregate projection.")
	}
	profiles := map[string]int{}
	entity := map[string]m.Entity{}
	for _, e := range s.Capture.Entities {
		entity[e.ID] = e
	}
	frames, stacks := map[string]bool{}, map[string]bool{}
	add := func(r m.Record, weight string, interval *string, stack []m.Frame) error {
		if !allowed[r.Entity] {
			return nil
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		index, ok := profiles[r.Entity]
		if !ok {
			index = len(result.Profiles)
			profiles[r.Entity] = index
			e := entity[r.Entity]
			result.Profiles = append(result.Profiles, p.Profile{ID: "profile-" + m.Hash([]any{s.Capture.ID, e}), Process: s.Capture.BootID + ":" + s.Capture.Namespace + ":" + e.PID + ":" + e.ID, Thread: e.TID, Worker: e.ID, Clock: "relative", TimeUnit: "nanoseconds", TimeScale: "1", Samples: []p.Sample{}})
		}
		path := []string{}
		if len(stack) == 0 {
			stack = []m.Frame{{Symbol: "[unattributed]", Context: "unknown"}}
		}
		for _, f := range stack {
			symbol := f.Symbol
			if symbol == "" || symbol == "[unknown]" {
				symbol = "[unknown@" + f.Address + "]"
			}
			frame := p.Frame{Symbol: symbol + " @[" + f.Address + ";" + f.Context + "]", Display: symbol, Module: f.Module, Runtime: "native", Native: true, Kernel: f.Context == "kernel"}
			frame.ID = p.FrameID(frame)
			if !frames[frame.ID] {
				result.Frames = append(result.Frames, frame)
				frames[frame.ID] = true
			}
			path = append(path, frame.ID)
		}
		id := p.StackID(path)
		if !stacks[id] {
			result.Stacks = append(result.Stacks, p.Stack{ID: id, Frames: path})
			stacks[id] = true
		}
		labels := map[string][]string{"event": {r.Name}, "state": {r.State}, "context": {r.Context}, "entity_id": {r.Entity}}
		for key, value := range r.Fields {
			labels["field."+key] = []string{value}
		}
		if r.CPU != nil {
			labels["cpu"] = []string{*r.CPU}
		}
		if r.Period != nil {
			labels["period"] = []string{*r.Period}
		}
		timestamp := r.Time
		if s.Capture.Clock == "unknown" {
			result.Profiles[index].Clock = "aggregate"
			timestamp = nil
			interval = nil
		}
		labels["source_clock"] = []string{s.Capture.Clock}
		result.Profiles[index].Samples = append(result.Profiles[index].Samples, p.Sample{StackID: id, MetricID: metricID, Weight: weight, Timestamp: timestamp, Interval: interval, Labels: labels, SourceRef: fmt.Sprintf("line:%d", r.Line)})
		return nil
	}
	if metricID == "off_cpu" || metricID == "on_cpu" || metricID == "runqueue" {
		intervals, e := Intervals(ctx, s)
		if e != nil {
			return result, e
		}
		records := map[string]m.Record{}
		for _, r := range s.Records {
			records[r.ID] = r
		}
		for _, span := range intervals {
			if span.Kind != metricID || span.Duration == nil {
				continue
			}
			r := records[span.StartRef]
			stack := []m.Frame{}
			if metricID == "off_cpu" {
				stack = r.Stack
			}
			if e := add(r, *span.Duration, span.Duration, stack); e != nil {
				return result, e
			}
		}
	} else {
		for _, r := range s.Records {
			if r.Kind == "sample" && metricID == "samples/"+r.Name {
				if e := add(r, "1", nil, r.Stack); e != nil {
					return result, e
				}
			}
			if r.Kind == "sample" && metricID == "period/"+r.Name && r.Period != nil {
				if e := add(r, *r.Period, nil, r.Stack); e != nil {
					return result, e
				}
			}
			if r.Kind == "duration" && metricID == "duration/"+r.Name {
				one := s
				one.Records = []m.Record{r}
				spans, e := Intervals(ctx, one)
				if e != nil {
					return result, e
				}
				if len(spans) > 0 && spans[0].Duration != nil {
					if e := add(r, *spans[0].Duration, spans[0].Duration, r.Stack); e != nil {
						return result, e
					}
				}
			}
		}
	}
	if len(result.Profiles) == 0 {
		return result, fmt.Errorf("SYSTEM_PROJECTION_UNAVAILABLE: no observations qualify for selected metric/entity; inspect loss/clock/lifetime and interval status")
	}
	unresolved := 0
	for _, frame := range result.Frames {
		if strings.HasPrefix(frame.Symbol, "[unknown@") {
			unresolved++
		}
	}
	result.Quality.ConversionLoss = append(result.Quality.ConversionLoss, fmt.Sprintf("%d distinct unresolved symbol labels; symbol-resolution ratio is not certified.", unresolved))
	if e := result.Validate(); e != nil {
		return result, e
	}
	return result, ctx.Err()
}
