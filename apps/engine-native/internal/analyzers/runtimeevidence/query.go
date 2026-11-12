package runtimeevidence

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/symbolinfo"
)

type Query struct {
	StackID string `json:"stack_id,omitempty"`
	FrameID string `json:"frame_id,omitempty"`
	Kind    string `json:"kind"`
	Entity  string `json:"entity_id,omitempty"`
	CPU     string `json:"cpu_observation_id,omitempty"`
	Stack   string `json:"stack_observation_id,omitempty"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit"`
}
type Page struct {
	ResultSHA string            `json:"result_sha256"`
	QuerySHA  string            `json:"query_sha256"`
	Total     int               `json:"total"`
	Next      string            `json:"next"`
	Rows      []json.RawMessage `json:"rows"`
}

func QuerySet(ctx context.Context, s r.Set, digest string, q Query) (Page, error) {
	out := Page{ResultSHA: digest, Rows: []json.RawMessage{}}
	if !r.Digest(digest) || q.Limit < 1 || q.Limit > 100 || !r.Text(q.Entity) || !r.Text(q.StackID) || !r.Text(q.FrameID) || !r.Text(q.CPU) || !r.Text(q.Stack) || len(q.Cursor) > 2048 || !r.In(q.Kind, "observations", "panic_snapshot", "handle_snapshot", "task_snapshot", "thread_snapshot", "allocation", "event_loop", "cpu", "gc", "access", "database", "entities", "processes", "sources", "frames", "stacks", "relations", "profile_refs", "plan_nodes", "symbol_evidence", "cpu_link") {
		return out, fmt.Errorf("RUNTIME_QUERY_INVALID")
	}
	identity := q
	identity.Cursor = ""
	out.QuerySHA = r.Hash(identity)
	type cursor struct {
		Result string `json:"result"`
		Query  string `json:"query"`
		Offset int    `json:"offset"`
	}
	offset := 0
	if q.Cursor != "" {
		raw, e := base64.RawURLEncoding.DecodeString(q.Cursor)
		if e != nil {
			return out, fmt.Errorf("RUNTIME_CURSOR_INVALID")
		}
		var c cursor
		if e = r.Strict(raw, &c, 2048); e != nil || c.Result != digest || c.Query != out.QuerySHA || c.Offset < 0 {
			return out, fmt.Errorf("RUNTIME_CURSOR_STALE")
		}
		offset = c.Offset
	}
	add := func(value any) {
		i := out.Total
		out.Total++
		if i >= offset && len(out.Rows) < q.Limit {
			raw, _ := json.Marshal(value)
			out.Rows = append(out.Rows, raw)
		}
	}
	wantedStacks := map[string]bool{}
	wantedFrames := map[string]bool{}
	frameStacks := map[string]bool{}
	for _, v := range s.Observations {
		if q.Entity == "" || v.Entity == q.Entity {
			if v.Stack != "" {
				wantedStacks[v.Stack] = true
			}
		}
	}
	for _, v := range s.Stacks {
		if (q.Entity == "" || wantedStacks[v.ID]) && (q.StackID == "" || q.StackID == v.ID) {
			for _, id := range v.Frames {
				if id == q.FrameID {
					frameStacks[v.ID] = true
				}
				wantedFrames[id] = true
			}
		}
	}
	frameMatch := func(id string) bool {
		return (q.Entity == "" && q.StackID == "" || wantedFrames[id]) && (q.FrameID == "" || q.FrameID == id)
	}
	switch q.Kind {
	case "cpu_link":
		if q.CPU == "" || q.Stack == "" {
			return out, fmt.Errorf("RUNTIME_CPU_PAIR_REQUIRED")
		}
		var cpu, stack r.Observation
		for _, v := range s.Observations {
			if v.ID == q.CPU {
				cpu = v
			}
			if v.ID == q.Stack {
				stack = v
			}
		}
		if cpu.ID == "" || stack.ID == "" {
			return out, fmt.Errorf("RUNTIME_OBSERVATION_MISSING")
		}
		add(r.Correlate(s, cpu, stack))
	case "entities":
		for _, v := range s.Entities {
			if q.Entity == "" || v.ID == q.Entity {
				add(v)
			}
		}
	case "processes":
		for _, v := range s.Processes {
			add(v)
		}
	case "sources":
		for _, v := range s.Sources {
			add(v)
		}
	case "frames":
		for _, v := range s.Frames {
			if frameMatch(v.ID) {
				add(v)
			}
		}
	case "symbol_evidence":
		for _, v := range s.Frames {
			if !frameMatch(v.ID) {
				continue
			}
			if out.Total >= offset && len(out.Rows) < q.Limit {
				add(symbolinfo.Describe(v.Symbol, v.File, v.Language))
			} else {
				out.Total++
			}
		}
	case "stacks":
		for _, v := range s.Stacks {
			if (q.Entity == "" || wantedStacks[v.ID]) && (q.StackID == "" || q.StackID == v.ID) {
				add(v)
			}
		}
	case "relations":
		for _, v := range s.Relations {
			if q.Entity == "" || v.From == q.Entity || v.To == q.Entity {
				add(v)
			}
		}
	case "profile_refs":
		for _, v := range s.Profiles {
			if q.Entity == "" || v.Entity == q.Entity {
				add(v)
			}
		}
	case "plan_nodes":
		for _, v := range s.Plans {
			add(v)
		}
	default:
		for i, v := range s.Observations {
			if i%128 == 0 {
				if e := ctx.Err(); e != nil {
					return out, e
				}
			}
			if (q.Kind == "observations" || v.Kind == q.Kind) && (q.Entity == "" || v.Entity == q.Entity) && (q.StackID == "" || q.StackID == v.Stack) && (q.FrameID == "" || frameStacks[v.Stack]) {
				add(v)
			}
		}
	}
	if offset > out.Total {
		return out, fmt.Errorf("RUNTIME_CURSOR_OFFSET_INVALID")
	}
	if end := offset + len(out.Rows); end < out.Total {
		raw, _ := json.Marshal(cursor{digest, out.QuerySHA, end})
		out.Next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return out, ctx.Err()
}
