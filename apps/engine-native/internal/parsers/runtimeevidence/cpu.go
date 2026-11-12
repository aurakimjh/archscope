package runtimeevidence

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	"io"
)

func (b *builder) cpuCSV(raw []byte) error {
	if e := b.process(); e != nil {
		return e
	}
	if b.options.Basis != "interval_average" && b.options.Basis != "lifetime_average" {
		return fmt.Errorf("RUNTIME_CPU_BASIS_REQUIRED: ps lifetime average and top interval percentages are different")
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = 3
	head, e := reader.Read()
	if e != nil || len(head) != 3 || head[0] != "pid" || head[1] != "tid" || head[2] != "cpu_percent" {
		return fmt.Errorf("RUNTIME_CPU_FIELDS_REQUIRED: pid,tid,cpu_percent")
	}
	for line := 2; ; line++ {
		fields, e := reader.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if fields[0] != b.options.Process.PID {
			return fmt.Errorf("RUNTIME_CPU_PROCESS_MISMATCH")
		}
		v, e := number(json.Number(fields[2]))
		if e != nil {
			return e
		}
		id := ""
		for _, entity := range b.options.Entities {
			tid, err := r.NativeID(entity)
			if err == nil && entity.Process == b.options.Process.ID && entity.Kind == "os_thread" && tid == fields[1] {
				if id != "" {
					return fmt.Errorf("RUNTIME_CPU_ENTITY_AMBIGUOUS")
				}
				id = entity.ID
			}
		}
		if id == "" {
			id = b.entity("os_thread", fields[1], "OS thread "+fields[1], fields[1], "os_tid_decimal")
		}
		if e = b.add(r.Observation{Entity: id, Kind: "cpu", Metric: "cpu_usage_percent", Value: v, Unit: "percent", Basis: b.options.Basis, Time: b.options.Time, From: b.options.From, To: b.options.To, Ref: r.Ref{Position: fmt.Sprintf("line:%d", line)}}); e != nil {
			return e
		}
	}
	return nil
}

// Explicit bundle export reuses the existing registered dump grammars. Raw
// parsing is done by the caller's selected plugin; no mixed-runtime identity join.
func (b *builder) threads(raw []byte) error {
	if e := b.process(); e != nil {
		return e
	}
	var v models.ThreadDumpBundle
	if e := decode(raw, &v); e != nil {
		return e
	}
	if v.SourceFormat == "" || len(v.Snapshots) > 4096 {
		return fmt.Errorf("RUNTIME_THREAD_EXPORT_INVALID")
	}
	if b.source.Producer == "" {
		b.source.Producer = v.SourceFormat
	}
	for i, s := range v.Snapshots {
		runtimeID := s.SnapshotID
		if s.ThreadID != nil {
			runtimeID = *s.ThreadID
		}
		native := ""
		encoding := "none"
		if value, ok := s.Metadata["native_tid"].(string); ok {
			native = value
			encoding = "jvm_nid_hex"
		}
		kind, obsKind := "runtime_thread", "thread_snapshot"
		if virtual, _ := s.Metadata["is_virtual_thread"].(bool); virtual {
			kind, obsKind = "async_task", "task_snapshot"
			native, encoding = "", "none"
		}
		id := b.entity(kind, runtimeID, s.ThreadName, native, encoding)
		observed := "unknown"
		if v, ok := s.Metadata["observed_state"].(string); ok && r.In(v, "unknown", "RUNNABLE", "BLOCKED", "WAITING", "TIMED_WAITING", "NEW", "TERMINATED") {
			observed = v
		}
		frames := []frame{}
		for _, f := range s.StackFrames {
			file := ""
			if f.File != nil {
				file = *f.File
			}
			var at *int64
			if f.Line != nil {
				x := int64(*f.Line)
				at = &x
			}
			frames = append(frames, frame{Symbol: f.Function, File: file, Line: at})
		}
		if e := b.add(r.Observation{Entity: id, Kind: obsKind, State: observed, Inferred: string(s.State), InferenceBasis: "legacy parser classification; not an observed CPU state", Basis: "snapshot", Time: b.options.Time, Stack: b.stack(frames), Ref: r.Ref{Position: fmt.Sprintf("%s/snapshots/%d", b.prefix, i)}}); e != nil {
			return e
		}
	}
	return nil
}
