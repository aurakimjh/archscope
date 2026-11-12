package runtimeevidence

import (
	"encoding/json"
	"fmt"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
)

// Explicit safe JSON export of stdlib APIs. This is not pickle/pstats/memray.
type pythonExport struct {
	Schema  string  `json:"schema"`
	Version string  `json:"python_version"`
	PID     string  `json:"pid"`
	Time    *string `json:"time_ns"`
	Tasks   []struct {
		ID    string  `json:"id"`
		Name  string  `json:"name"`
		State string  `json:"state"`
		Stack []frame `json:"stack"`
	} `json:"tasks"`
	Workers []struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		State    string  `json:"state"`
		NativeID string  `json:"native_tid"`
		Stack    []frame `json:"stack"`
	} `json:"workers"`
	Allocations []struct {
		Bytes  json.Number `json:"size_bytes"`
		Blocks json.Number `json:"blocks"`
		Trace  []frame     `json:"traceback"`
	} `json:"allocations"`
	GC []struct {
		From  string `json:"from_ns"`
		To    string `json:"to_ns"`
		Phase string `json:"phase"`
	} `json:"gc_events"`
}

func (b *builder) python(raw []byte) error {
	if e := b.process(); e != nil {
		return e
	}
	var v pythonExport
	if e := r.Strict(raw, &v); e != nil {
		return e
	}
	if v.Schema != "python-runtime-export.v1" || v.PID != b.options.Process.PID {
		return fmt.Errorf("RUNTIME_PYTHON_CONTEXT_MISMATCH")
	}
	if e := b.version("CPython asyncio/tracemalloc JSON exporter", v.Version); e != nil {
		return e
	}
	if len(v.Tasks)+len(v.Workers) > 4096 || len(v.Allocations) > r.MaxObservations {
		return fmt.Errorf("RUNTIME_PYTHON_EXPORT_BUDGET")
	}
	seen := map[string]bool{}
	for i, x := range v.Tasks {
		if x.ID == "" || seen["task:"+x.ID] {
			return fmt.Errorf("RUNTIME_TASK_ID_INVALID")
		}
		seen["task:"+x.ID] = true
		id := b.entity("async_task", x.ID, x.Name, "", "none")
		if e := b.add(r.Observation{Entity: id, Kind: "task_snapshot", Basis: "snapshot", State: x.State, Time: v.Time, Stack: b.stack(x.Stack), Ref: r.Ref{Position: fmt.Sprintf("/tasks/%d", i)}}); e != nil {
			return e
		}
	}
	for i, x := range v.Workers {
		if x.ID == "" || seen["worker:"+x.ID] {
			return fmt.Errorf("RUNTIME_WORKER_ID_INVALID")
		}
		seen["worker:"+x.ID] = true
		encoding := "none"
		if x.NativeID != "" {
			encoding = "os_tid_decimal"
		}
		id := b.entity("runtime_thread", x.ID, x.Name, x.NativeID, encoding)
		if e := b.add(r.Observation{Entity: id, Kind: "thread_snapshot", Basis: "snapshot", State: x.State, Time: v.Time, Stack: b.stack(x.Stack), Ref: r.Ref{Position: fmt.Sprintf("/workers/%d", i)}}); e != nil {
			return e
		}
	}
	id := b.entity("process", "process", b.options.Process.ID, "", "none")
	for i, x := range v.Allocations {
		for _, metric := range []struct {
			name, unit string
			value      json.Number
		}{{"live_bytes", "bytes", x.Bytes}, {"live_blocks", "count", x.Blocks}} {
			value, e := number(metric.value)
			if e != nil {
				return e
			}
			if e = b.add(r.Observation{Entity: id, Kind: "allocation", Metric: metric.name, Value: value, Unit: metric.unit, Basis: "snapshot", Time: v.Time, Stack: b.stack(x.Trace), Ref: r.Ref{Position: fmt.Sprintf("/allocations/%d/%s", i, metric.name)}}); e != nil {
				return e
			}
		}
	}
	for i, x := range v.GC {
		if x.Phase != "pause" && x.Phase != "concurrent" {
			return fmt.Errorf("RUNTIME_GC_PHASE_REQUIRED")
		}
		if e := b.add(r.Observation{Entity: id, Kind: "gc", Metric: x.Phase + "_ns", Unit: "nanoseconds", Basis: "observed_interval", From: &x.From, To: &x.To, Ref: r.Ref{Position: fmt.Sprintf("/gc_events/%d", i)}}); e != nil {
			return e
		}
	}
	return nil
}
