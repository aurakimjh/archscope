// Package gputrace computes interval views, never claims hardware utilization.
package gputrace

import (
	"context"
	"encoding/json"
	"fmt"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	"math/big"
	"sort"
	"strconv"
)

type EventView struct {
	g.Event
	Source     string  `json:"source_id"`
	Clock      string  `json:"clock_id"`
	Rank       *string `json:"rank"`
	Duration   string  `json:"duration_ns"`
	Self       *string `json:"observed_self_ns"`
	SelfStatus string  `json:"self_status"`
	Parent     string  `json:"observed_parent_event"`
}
type Busy struct {
	Source      string `json:"source_id"`
	Clock       string `json:"clock_id"`
	Device      string `json:"device_id"`
	KernelCount string `json:"kernel_count"`
	KernelSum   string `json:"kernel_duration_sum_ns"`
	Busy        string `json:"kernel_interval_union_ns"`
	From        string `json:"event_extent_from_ns"`
	To          string `json:"event_extent_to_ns"`
	Extent      string `json:"event_extent_ns"`
	Meaning     string `json:"meaning"`
}
type Relation struct {
	Source   string `json:"source_id"`
	From     string `json:"from_event"`
	To       string `json:"to_event"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}
type Projection struct {
	Events    []EventView
	Busy      []Busy
	Relations []Relation
}

func number(s string) *big.Int { v, _ := new(big.Int).SetString(s, 10); return v }
func ordered(events []g.Event) {
	sort.Slice(events, func(i, j int) bool {
		if c := g.Cmp(events[i].From, events[j].From); c != 0 {
			return c < 0
		}
		if c := g.Cmp(events[i].To, events[j].To); c != 0 {
			return c > 0
		}
		return events[i].ID < events[j].ID
	})
}
func union(events []g.Event) string {
	if len(events) == 0 {
		return "0"
	}
	ordered(events)
	start, end := events[0].From, events[0].To
	total := new(big.Int)
	for _, v := range events[1:] {
		if g.Cmp(v.From, end) <= 0 {
			if g.Cmp(v.To, end) > 0 {
				end = v.To
			}
		} else {
			total.Add(total, number(g.Difference(end, start)))
			start, end = v.From, v.To
		}
	}
	return total.Add(total, number(g.Difference(end, start))).String()
}
func Project(ctx context.Context, s g.Set) (Projection, error) {
	out := Projection{Events: []EventView{}, Busy: []Busy{}, Relations: []Relation{}}
	sources := map[string]g.Source{}
	tracks := map[string]g.Track{}
	perTrack := map[string][]g.Event{}
	gpu := map[string][]g.Event{}
	api := map[string][]g.Event{}
	flows := map[string][]g.Event{}
	for _, v := range s.Sources {
		sources[v.ID] = v
	}
	for _, t := range s.Tracks {
		tracks[t.ID] = t
	}
	eventIndex := map[string]int{}
	for _, v := range s.Events {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		t := tracks[v.Track]
		src := sources[t.Source]
		eventIndex[v.ID] = len(out.Events)
		out.Events = append(out.Events, EventView{Event: v, Source: src.ID, Clock: src.Clock, Rank: src.Rank, Duration: g.Difference(v.To, v.From), SelfStatus: "not_cpu_operator"})
		perTrack[v.Track] = append(perTrack[v.Track], v)
		if v.Kind == "gpu_kernel" {
			key := t.Source + "/" + t.Device
			gpu[key] = append(gpu[key], v)
		}
		if v.Kind == "cuda_api" && v.Correlation != "" && v.Correlation != "0" {
			key := g.Hash([]string{t.Source, t.Process, v.Correlation})
			api[key] = append(api[key], v)
		}
		if v.Kind == "flow" {
			key := g.Hash([]string{t.Source, v.Flow})
			flows[key] = append(flows[key], v)
		}
	}
	keys := []string{}
	for k := range gpu {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		events := gpu[key]
		sum := new(big.Int)
		lo, hi := events[0].From, events[0].To
		for _, v := range events {
			sum.Add(sum, number(g.Difference(v.To, v.From)))
			if g.Cmp(v.From, lo) < 0 {
				lo = v.From
			}
			if g.Cmp(v.To, hi) > 0 {
				hi = v.To
			}
		}
		t := tracks[events[0].Track]
		src := sources[t.Source]
		out.Busy = append(out.Busy, Busy{Source: src.ID, Clock: src.Clock, Device: t.Device, KernelCount: strconv.Itoa(len(events)), KernelSum: sum.String(), Busy: union(append([]g.Event(nil), events...)), From: lo, To: hi, Extent: g.Difference(hi, lo), Meaning: "observed kernel-only extent; union across streams; no hardware utilization or CPU/GPU sum"})
	}
	for id, events := range perTrack {
		if tracks[id].Kind != "cpu" {
			continue
		}
		ordered(events)
		stack := []g.Event{}
		children := map[string][]g.Event{}
		parents := map[string]string{}
		bad := false
		for _, v := range events {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			if v.From == v.To {
				continue
			}
			for len(stack) > 0 && g.Cmp(stack[len(stack)-1].To, v.From) <= 0 {
				stack = stack[:len(stack)-1]
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				if g.Cmp(v.To, parent.To) > 0 || v.From == parent.From && v.To == parent.To {
					bad = true
				} else {
					parents[v.ID] = parent.ID
					children[parent.ID] = append(children[parent.ID], v)
				}
			}
			stack = append(stack, v)
		}
		for _, v := range events {
			i := eventIndex[v.ID]
			if bad {
				out.Events[i].SelfStatus = "ambiguous_crossing_or_equal_intervals"
				continue
			}
			self := g.Difference(out.Events[i].Duration, union(children[v.ID]))
			out.Events[i].Self = &self
			out.Events[i].SelfStatus = "observed_same_track_child_union; no source parent proof"
			out.Events[i].Parent = parents[v.ID]
		}
	}
	for _, v := range s.Events {
		if !g.In(v.Kind, "gpu_kernel", "memcpy") || v.Correlation == "" {
			continue
		}
		t := tracks[v.Track]
		matches := api[g.Hash([]string{t.Source, t.Process, v.Correlation})]
		r := Relation{Source: t.Source, To: v.ID, Kind: "cuda_launch", Evidence: "same source + serialized process + correlation ID"}
		if len(matches) == 1 {
			r.From = matches[0].ID
			r.Status = "explicit_correlation"
		} else if len(matches) == 0 {
			r.Status = "unresolved_launch"
		} else {
			r.Status = "ambiguous_launch"
		}
		out.Relations = append(out.Relations, r)
	}
	flowKeys := []string{}
	for k := range flows {
		flowKeys = append(flowKeys, k)
	}
	sort.Strings(flowKeys)
	for _, key := range flowKeys {
		var start, end []g.Event
		for _, v := range flows[key] {
			if v.Phase == "s" {
				start = append(start, v)
			} else {
				end = append(end, v)
			}
		}
		r := Relation{Source: tracks[flows[key][0].Track].Source, Kind: "explicit_flow", Evidence: "source-scoped flow endpoints; no name-based join", Status: "orphan_or_ambiguous_flow"}
		if len(start) == 1 && len(end) == 1 {
			r.From = start[0].ID
			r.To = end[0].ID
			r.Status = "explicit_flow_endpoints"
			if g.Cmp(start[0].From, end[0].From) > 0 {
				r.Status = "reversed_flow_endpoints"
			}
		}
		out.Relations = append(out.Relations, r)
	}
	return out, ctx.Err()
}

type Query struct {
	Kind   string  `json:"kind"`
	Source string  `json:"source_id,omitempty"`
	Track  string  `json:"track_id,omitempty"`
	Rank   *string `json:"rank,omitempty"`
	From   *string `json:"from_ns,omitempty"`
	To     *string `json:"to_ns,omitempty"`
	Cursor string  `json:"cursor,omitempty"`
	Limit  int     `json:"limit"`
}
type Page struct {
	ResultSHA string            `json:"result_sha256"`
	QuerySHA  string            `json:"query_sha256"`
	Rows      []json.RawMessage `json:"rows"`
	Next      string            `json:"next"`
	Total     int               `json:"total"`
}

func QuerySet(ctx context.Context, s g.Set, p Projection, digest string, q Query) (Page, error) {
	out := Page{ResultSHA: digest, Rows: []json.RawMessage{}}
	if len(digest) != 64 || q.Limit < 1 || q.Limit > 100 || !g.In(q.Kind, "events", "tracks", "sources", "busy", "relations", "summaries") || !g.Text(q.Source) || !g.Text(q.Track) || q.Rank != nil && !g.Uint(*q.Rank) || (q.From == nil) != (q.To == nil) || q.From != nil && (!g.Uint(*q.From) || !g.Uint(*q.To) || g.Cmp(*q.From, *q.To) >= 0) {
		return out, fmt.Errorf("GPU_TRACE_QUERY_INVALID")
	}
	if q.Kind != "events" && (q.Track != "" || q.From != nil) {
		return out, fmt.Errorf("GPU_TRACE_FILTER_NOT_APPLICABLE")
	}
	if q.From != nil && q.Source == "" {
		return out, fmt.Errorf("GPU_TRACE_SOURCE_CLOCK_REQUIRED")
	}
	selector := q
	selector.Cursor = ""
	out.QuerySHA = g.Hash([]any{digest, selector})
	offset, e := decodeCursor(q.Cursor, out.QuerySHA)
	if e != nil {
		return out, e
	}
	sources := map[string]g.Source{}
	for _, v := range s.Sources {
		sources[v.ID] = v
	}
	if q.Source != "" && sources[q.Source].ID == "" {
		return out, fmt.Errorf("GPU_TRACE_SOURCE_SELECTOR_UNKNOWN")
	}
	if q.Track != "" {
		known := false
		for _, t := range s.Tracks {
			known = known || t.ID == q.Track
		}
		if !known {
			return out, fmt.Errorf("GPU_TRACE_TRACK_SELECTOR_UNKNOWN")
		}
	}
	matches := func(source string) bool {
		v := sources[source]
		return (q.Source == "" || q.Source == v.ID) && (q.Rank == nil || v.Rank != nil && *q.Rank == *v.Rank)
	}
	charge := 0
	var failure error
	add := func(v any) {
		i := out.Total
		out.Total++
		if i >= offset && len(out.Rows) < q.Limit {
			b, _ := json.Marshal(v)
			charge += len(b)
			if charge > 256<<10 {
				failure = fmt.Errorf("GPU_TRACE_PAGE_BYTES_BUDGET: reduce limit")
				return
			}
			out.Rows = append(out.Rows, b)
		}
	}
	switch q.Kind {
	case "events":
		for _, v := range p.Events {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			if matches(v.Source) && (q.Track == "" || q.Track == v.Track) && (q.From == nil || g.Intersects(v.From, v.To, *q.From, *q.To)) {
				add(v)
			}
		}
	case "tracks":
		for _, v := range s.Tracks {
			if matches(v.Source) {
				add(v)
			}
		}
	case "sources":
		for _, v := range s.Sources {
			if matches(v.ID) {
				add(v)
			}
		}
	case "busy":
		for _, v := range p.Busy {
			if matches(v.Source) {
				add(v)
			}
		}
	case "relations":
		for _, v := range p.Relations {
			if matches(v.Source) {
				add(v)
			}
		}
	case "summaries":
		for _, v := range s.Summaries {
			if matches(v.Source) {
				add(v)
			}
		}
	}
	if failure != nil {
		return Page{}, failure
	}
	if offset > out.Total {
		return out, fmt.Errorf("GPU_TRACE_CURSOR_OFFSET")
	}
	if end := offset + len(out.Rows); end < out.Total {
		out.Next = encodeCursor(out.QuerySHA, end)
	}
	return out, ctx.Err()
}
