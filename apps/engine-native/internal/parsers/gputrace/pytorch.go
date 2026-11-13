package gputrace

import (
	"encoding/json"
	"fmt"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	pp "github.com/aurakimjh/archscope/apps/engine-native/internal/producerpolicy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"math/big"
	"strconv"
	"strings"
)

func scalar(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}
func ns(raw json.RawMessage) (string, error) {
	s := scalar(raw)
	if len(s) > 128 || strings.ContainsAny(s, "eE") {
		return "", fmt.Errorf("GPU_TRACE_TIME_NUMBER_INVALID")
	}
	n, e := m.Number(s)
	if e != nil || n.Sign() < 0 {
		return "", fmt.Errorf("GPU_TRACE_TIME_NUMBER_INVALID")
	}
	n.Mul(n, big.NewRat(1000, 1))
	if !n.IsInt() || !g.Uint(n.Num().String()) {
		return "", fmt.Errorf("GPU_TRACE_SUBNANOSECOND_OR_OVERFLOW")
	}
	return n.Num().String(), nil
}
func (b *builder) pytorch(raw []byte) error {
	version, qualification, err := pp.Select(b.source.Version, "2.15.0", b.o.AllowUnverifiedVersion)
	if err != nil {
		return fmt.Errorf("GPU_TRACE_TORCH_VERSION: %w", err)
	}
	b.source.Version, b.source.Qualification = version, qualification
	if qualification != "" {
		b.set.Warnings = append(b.set.Warnings, pp.Limitation)
	}
	var input struct {
		Schema      json.Number `json:"schemaVersion"`
		Distributed *struct {
			Backend string      `json:"backend"`
			Rank    json.Number `json:"rank"`
			World   json.Number `json:"world_size"`
		} `json:"distributedInfo"`
		Events []struct {
			Phase    string                     `json:"ph"`
			Category string                     `json:"cat"`
			Name     string                     `json:"name"`
			PID      json.RawMessage            `json:"pid"`
			TID      json.RawMessage            `json:"tid"`
			Time     json.RawMessage            `json:"ts"`
			Duration json.RawMessage            `json:"dur"`
			ID       json.RawMessage            `json:"id"`
			ID2      map[string]json.RawMessage `json:"id2"`
			Scope    string                     `json:"scope"`
			Args     map[string]json.RawMessage `json:"args"`
		} `json:"traceEvents"`
	}
	if e := wirejson.DecodeWithLimits(raw, &input, wirejson.ResultLimits(64<<20, 128)); e != nil {
		return e
	}
	if input.Schema.String() != "1" || input.Events == nil {
		return fmt.Errorf("GPU_TRACE_TORCH_SCHEMA_REQUIRED: schemaVersion1 and traceEvents array required")
	}
	b.source.Producer = "pytorch"
	b.source.Schema = "kineto.chrome.schema1"
	if input.Distributed != nil {
		rank, world := input.Distributed.Rank.String(), input.Distributed.World.String()
		b.source.Rank = &rank
		b.source.WorldSize = &world
		b.source.Backend = input.Distributed.Backend
	}
	if e := b.scope(); e != nil {
		return e
	}
	ignored := 0
	for i, v := range input.Events {
		if e := b.ctx.Err(); e != nil {
			return e
		}
		kind := ""
		switch v.Category {
		case "cpu_op":
			kind = "cpu"
		case "cuda_runtime", "cuda_driver":
			kind = "cuda_api"
		case "kernel":
			kind = "gpu_kernel"
		case "gpu_memcpy", "gpu_memset":
			kind = "memcpy"
		case "user_annotation":
			kind = "annotation"
		}
		if g.In(v.Phase, "s", "f") {
			kind = "flow"
		} else if kind != "" && v.Phase != "X" {
			return fmt.Errorf("GPU_TRACE_TORCH_PHASE_UNSUPPORTED at /traceEvents/%d", i)
		}
		if kind == "" {
			ignored++
			continue
		}
		from, e := ns(v.Time)
		if e != nil {
			return fmt.Errorf("GPU_TRACE_TORCH_TIME at /traceEvents/%d", i)
		}
		to := from
		if v.Phase == "X" {
			d, e := ns(v.Duration)
			if e != nil {
				return fmt.Errorf("GPU_TRACE_TORCH_DURATION at /traceEvents/%d", i)
			}
			a, _ := new(big.Int).SetString(from, 10)
			n, _ := new(big.Int).SetString(d, 10)
			to = a.Add(a, n).String()
			if !g.Uint(to) {
				return fmt.Errorf("GPU_TRACE_TORCH_TIME_OVERFLOW")
			}
		}
		pid, tid := scalar(v.PID), scalar(v.TID)
		if !g.Known(pid) || !g.Known(tid) {
			return fmt.Errorf("GPU_TRACE_TORCH_PROCESS_REQUIRED at /traceEvents/%d", i)
		}
		name := v.Name
		if kind == "annotation" {
			name = "annotation-sha256:" + Digest([]byte(v.Name))
			if b.source.Backend == "gloo" && g.In(v.Name, "gloo:all_reduce", "gloo:all_gather", "gloo:broadcast", "gloo:reduce", "gloo:barrier") {
				kind = "communication"
				name = v.Name
			}
		}
		if !g.Text(name) {
			return fmt.Errorf("GPU_TRACE_TORCH_NAME_BUDGET at /traceEvents/%d", i)
		}
		t := g.Track{Kind: kind, Process: pid, Thread: tid}
		event := g.Event{Name: name, From: from, To: to, Correlation: scalar(v.Args["correlation"]), External: scalar(v.Args["External id"]), Position: "traceEvents/" + strconv.Itoa(i)}
		if g.In(kind, "gpu_kernel", "memcpy") {
			index := scalar(v.Args["device"])
			t.DeviceIndex = index
			t.Device = b.o.DeviceMap[index]
			if t.Device == "" {
				return fmt.Errorf("GPU_TRACE_DEVICE_MAPPING_REQUIRED at /traceEvents/%d", i)
			}
			t.Context = scalar(v.Args["context"])
			if t.Context == "" {
				t.Context = "producer-not-exported"
			}
			t.Stream = scalar(v.Args["stream"])
			if t.Stream == "" {
				return fmt.Errorf("GPU_TRACE_STREAM_REQUIRED")
			}
		}
		if kind == "flow" {
			flowID, scope := scalar(v.ID), "legacy-process"
			qualifier := pid
			if len(v.ID2) > 0 {
				if len(v.ID2) != 1 {
					return fmt.Errorf("GPU_TRACE_FLOW_ID_AMBIGUOUS")
				}
				if raw, ok := v.ID2["global"]; ok {
					flowID = scalar(raw)
					scope = "global"
					qualifier = ""
				} else if raw, ok := v.ID2["local"]; ok {
					flowID = scalar(raw)
					scope = "local"
				} else {
					return fmt.Errorf("GPU_TRACE_FLOW_ID_UNSUPPORTED")
				}
			}
			if !g.Known(flowID) {
				return fmt.Errorf("GPU_TRACE_FLOW_ID_REQUIRED")
			}
			event.Name = "explicit-flow"
			event.Flow = g.Hash([]string{scope, qualifier, v.Scope, v.Category, flowID})
			event.Phase = v.Phase
		}
		if e := b.add(t, event); e != nil {
			return e
		}
	}
	if len(b.set.Events) == 0 {
		return fmt.Errorf("GPU_TRACE_NO_SUPPORTED_EVENTS")
	}
	b.set.Warnings = append(b.set.Warnings, fmt.Sprintf("PyTorch2.15 schema1 ts/dur are microseconds; displayTimeUnit is display only. Exact integer nanoseconds retained. Ignored metadata/unsupported categories=%d; arbitrary args, paths, host_name and user annotation names excluded.", ignored), "CPU and GPU intervals stay separate. Source clocks/ranks stay isolated; explicit clock mapping metadata is audit-only in this release. Producer IDs/metadata are declarations, not authenticated workload identity. Actual PyTorch fixture qualifies CPU/Gloo only; CUDA shapes have structural tests, not native PyTorch CUDA qualification.")
	return nil
}
