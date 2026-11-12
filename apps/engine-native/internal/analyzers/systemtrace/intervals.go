package systemtrace

import (
	"context"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"sort"
)

type Interval struct {
	Availability string  `json:"availability"`
	ID           string  `json:"id"`
	Kind         string  `json:"kind"`
	Entity       string  `json:"entity_id"`
	Start        *string `json:"start_ns"`
	End          *string `json:"end_ns"`
	Duration     *string `json:"duration_ns"`
	Status       string  `json:"status"`
	State        string  `json:"state"`
	StartRef     string  `json:"start_ref"`
	EndRef       string  `json:"end_ref"`
	CPU          *string `json:"cpu"`
}
type pending struct {
	record  *m.Record
	invalid bool
}

// Intervals pairs only within one declared capture/clock/entity lifetime. Missing,
// duplicate or lossy sequences remain rows with null duration, never imputation.
func Intervals(ctx context.Context, s m.Set) ([]Interval, error) {
	out := []Interval{}
	defs := map[string]string{}
	entities := map[string]m.Entity{}
	for _, d := range s.Capture.Events {
		defs[d.Name] = d.Kind
	}
	for _, e := range s.Capture.Entities {
		entities[e.ID] = e
	}
	emit := func(kind string, a, b *m.Record, status string) {
		r := Interval{Kind: kind, Status: status}
		if a != nil {
			r.Start = a.Time
			r.StartRef = a.ID
			r.Entity = a.Entity
			r.State = a.State
			r.CPU = a.CPU
		}
		if b != nil {
			r.End = b.Time
			r.EndRef = b.ID
			r.Entity = b.Entity
			if r.CPU == nil {
				r.CPU = b.CPU
			}
		}
		if a != nil && b != nil && status == "paired_observation" {
			switch {
			case s.Capture.Clock == "unknown":
				r.Status = "clock_unqualified"
			case a.Kind == "duration":
				v := m.Sub(*b.Time, *a.Time)
				r.Duration = &v
				r.Status = "observed_duration"
			case s.Capture.Lost == nil:
				r.Status = "loss_unknown"
			case *s.Capture.Lost != "0":
				r.Status = "loss_affected"
			case s.Capture.BootID == "unknown" || s.Capture.Namespace == "unknown" || entities[r.Entity].ProcessStart == nil:
				r.Status = "entity_lifetime_unqualified"
			case kind == "on_cpu" && (a.CPU == nil || b.CPU == nil || *a.CPU != *b.CPU):
				r.Status = "cpu_discontinuity"
			default:
				v := m.Sub(*b.Time, *a.Time)
				r.Duration = &v
			}
		}
		r.Availability = "unavailable"
		if r.Duration != nil {
			r.Availability = "observed"
		}
		r.ID = "interval-" + m.Hash(r)
		out = append(out, r)
	}
	byEntity := map[string][]*m.Record{}
	for i := range s.Records {
		r := &s.Records[i]
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		switch r.Kind {
		case "duration":
			end := *r
			end.Time = r.End
			emit(defs[r.Name], r, &end, "paired_observation")
		case "sched_out", "sched_in", "wakeup":
			byEntity[r.Entity] = append(byEntity[r.Entity], r)
		}
	}
	ids := make([]string, 0, len(byEntity))
	for id := range byEntity {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		records := byEntity[id]
		sort.SliceStable(records, func(i, j int) bool { return m.Cmp(*records[i].Time, *records[j].Time) < 0 })
		var on, off, wake pending
		flush := func(p *pending, kind, status string) {
			if p.record != nil {
				emit(kind, p.record, nil, status)
			}
			*p = pending{}
		}
		for i := 0; i < len(records); {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			j := i + 1
			for j < len(records) && *records[j].Time == *records[i].Time {
				j++
			}
			if j-i > 1 {
				flush(&on, "on_cpu", "ambiguous_sequence")
				flush(&off, "off_cpu", "ambiguous_sequence")
				flush(&wake, "runqueue", "ambiguous_sequence")
				for _, r := range records[i:j] {
					emit(r.Kind, r, nil, "ambiguous_timestamp")
				}
				i = j
				continue
			}
			r := records[i]
			i++
			pair := func(kind string, p pending, end *m.Record) {
				status := "paired_observation"
				if p.invalid {
					status = "ambiguous_sequence"
				}
				emit(kind, p.record, end, status)
			}
			switch r.Kind {
			case "sched_out":
				bad := off.record != nil
				flush(&off, "off_cpu", "unpaired_start")
				if on.record != nil {
					pair("on_cpu", on, r)
				} else {
					emit("on_cpu", nil, r, "unpaired_end")
				}
				on = pending{}
				flush(&wake, "runqueue", "unpaired_start")
				off = pending{r, bad}
			case "sched_in":
				bad := on.record != nil
				flush(&on, "on_cpu", "unpaired_start")
				if off.record != nil {
					pair("off_cpu", off, r)
				} else {
					emit("off_cpu", nil, r, "unpaired_end")
				}
				if wake.record != nil {
					pair("runqueue", wake, r)
				} else if off.record != nil && off.record.State == "runnable" {
					pair("runqueue", off, r)
				}
				off = pending{}
				wake = pending{}
				on = pending{r, bad}
			case "wakeup":
				bad := wake.record != nil || on.record != nil
				if wake.record != nil {
					flush(&wake, "runqueue", "ambiguous_sequence")
				}
				wake = pending{r, bad}
			}
		}
		flush(&on, "on_cpu", "unpaired_start")
		flush(&off, "off_cpu", "unpaired_start")
		flush(&wake, "runqueue", "unpaired_start")
	}
	return out, ctx.Err()
}
