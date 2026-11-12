package runtimeevidence

import (
	"encoding/json"
	"fmt"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	"regexp"
	"strconv"
)

type nodeReport struct {
	Header struct {
		Version     int         `json:"reportVersion"`
		NodeVersion string      `json:"nodejsVersion"`
		PID         json.Number `json:"processId"`
		Thread      json.Number `json:"threadId"`
	} `json:"header"`
	JavaScript struct {
		Stack []string `json:"stack"`
	} `json:"javascriptStack"`
	Native []struct {
		Symbol string `json:"symbol"`
	} `json:"nativeStack"`
	Libuv []struct {
		Type   string `json:"type"`
		Active bool   `json:"is_active"`
	} `json:"libuv"`
	Workers []json.RawMessage `json:"workers"`
}
type nodeExport struct {
	Schema    string          `json:"schema"`
	Report    json.RawMessage `json:"report"`
	Time      *string         `json:"time_ns"`
	EventLoop []struct {
		Thread      string      `json:"thread_id"`
		From        string      `json:"from_ns"`
		To          string      `json:"to_ns"`
		Mean        json.Number `json:"delay_mean_ns"`
		P95         json.Number `json:"delay_p95_ns"`
		Max         json.Number `json:"delay_max_ns"`
		Utilization json.Number `json:"utilization_ratio"`
	} `json:"event_loop"`
}

var jsFrame = regexp.MustCompile(`^\s*at\s+(.*?)\s+\((.*):(\d+):(\d+)\)\s*$`)

func (b *builder) node(raw []byte, wrapped bool) error {
	if e := b.process(); e != nil {
		return e
	}
	time := b.options.Time
	var export nodeExport
	if wrapped {
		if e := r.Strict(raw, &export); e != nil {
			return e
		}
		if export.Schema != "node-runtime-export.v1" {
			return fmt.Errorf("RUNTIME_NODE_EXPORT_SCHEMA")
		}
		raw = export.Report
		time = export.Time
	}
	type item struct {
		raw  json.RawMessage
		path string
	}
	queue := []item{{raw, "/report"}}
	count := 0
	ids := map[string]string{}
	for len(queue) > 0 {
		if e := b.ctx.Err(); e != nil {
			return e
		}
		current := queue[0]
		queue = queue[1:]
		var v nodeReport
		if e := decode(current.raw, &v); e != nil {
			return e
		}
		if v.Header.Version != 5 || v.Header.NodeVersion == "" || v.Header.PID.String() != b.options.Process.PID {
			return fmt.Errorf("RUNTIME_NODE_REPORT_CONTEXT: reportVersion 5 and matching declared PID required")
		}
		if e := b.version("Node.js diagnostic report", v.Header.NodeVersion); e != nil {
			return e
		}
		thread := v.Header.Thread.String()
		if thread == "" {
			return fmt.Errorf("RUNTIME_NODE_THREAD_ID_REQUIRED")
		}
		if ids[thread] != "" {
			return fmt.Errorf("RUNTIME_NODE_DUPLICATE_WORKER_ID")
		}
		kind, name := "worker", "worker "+thread
		if thread == "0" {
			kind, name = "event_loop", "main event loop"
		}
		id := b.entity(kind, thread, name, "", "none")
		ids[thread] = id
		frames := []frame{}
		for _, line := range v.JavaScript.Stack {
			if x := jsFrame.FindStringSubmatch(line); x != nil {
				n, e := strconv.ParseInt(x[3], 10, 64)
				if e != nil {
					return e
				}
				frames = append(frames, frame{Symbol: x[1], File: x[2], Line: &n, Language: "JavaScript"})
			}
		}
		if e := b.add(r.Observation{Entity: id, Kind: "thread_snapshot", State: "unknown", Basis: "snapshot", Time: time, Stack: b.stack(frames), Ref: r.Ref{Position: current.path + "/javascriptStack"}}); e != nil {
			return e
		}
		native := []frame{}
		for _, f := range v.Native {
			native = append(native, frame{Symbol: f.Symbol, Language: "native"})
		}
		if len(native) > 0 {
			if e := b.add(r.Observation{Entity: id, Kind: "thread_snapshot", State: "unknown", Basis: "snapshot", Time: time, Stack: b.stack(native), Ref: r.Ref{Position: current.path + "/nativeStack (reporting thread, not a worker pool)"}}); e != nil {
				return e
			}
		}
		// Handle presence is inventory, never a blocked/waiting-state observation.
		for i, h := range v.Libuv {
			state := "inactive"
			if h.Active {
				state = "active"
			}
			if e := b.add(r.Observation{Entity: id, Kind: "handle_snapshot", State: state, Basis: "snapshot", Time: time, Population: "libuv handle inventory: " + h.Type, Ref: r.Ref{Position: fmt.Sprintf("%s/libuv/%d", current.path, i)}}); e != nil {
				return e
			}
		}
		count++
		if count > 128 || len(queue)+len(v.Workers) > 128 {
			return fmt.Errorf("RUNTIME_NODE_WORKER_BUDGET")
		}
		for i, child := range v.Workers {
			queue = append(queue, item{child, fmt.Sprintf("%s/workers/%d", current.path, i)})
		}
	}
	for i, x := range export.EventLoop {
		id := ids[x.Thread]
		if id == "" {
			return fmt.Errorf("RUNTIME_EVENT_LOOP_WORKER_MISSING")
		}
		for _, field := range []struct {
			name, unit, basis string
			value             json.Number
		}{{"delay_mean_ns", "nanoseconds", "distribution_statistic", x.Mean}, {"delay_p95_ns", "nanoseconds", "distribution_statistic", x.P95}, {"delay_max_ns", "nanoseconds", "distribution_statistic", x.Max}, {"utilization_ratio", "ratio", "interval", x.Utilization}} {
			value, e := number(field.value)
			if e != nil {
				return e
			}
			if e = b.add(r.Observation{Entity: id, Kind: "event_loop", Metric: field.name, Value: value, Unit: field.unit, Basis: field.basis, From: &x.From, To: &x.To, Time: time, Ref: r.Ref{Position: fmt.Sprintf("/event_loop/%d/%s", i, field.name)}}); e != nil {
				return e
			}
		}
	}
	return nil
}
