package traceimport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	l "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
)

type LLMQuery struct {
	Kind      string `json:"kind"`
	TraceID   string `json:"trace_id,omitempty"`
	RecordID  string `json:"record_id,omitempty"`
	Namespace string `json:"request_namespace,omitempty"`
	Logical   string `json:"logical_request_id,omitempty"`
	Cursor    string `json:"cursor,omitempty"`
	Limit     int    `json:"limit"`
}
type LLMPage struct {
	Version   string            `json:"version"`
	ResultSHA string            `json:"result_sha256"`
	QuerySHA  string            `json:"query_sha256"`
	Total     int               `json:"total"`
	Next      string            `json:"next"`
	Rows      []json.RawMessage `json:"rows"`
}

func QueryLLM(ctx context.Context, s l.Set, digest string, q LLMQuery) (LLMPage, error) {
	return queryLLM(ctx, s, nil, digest, q)
}

// QueryLLMProjected is for internal callers holding a validated immutable model
// and its corresponding digest-bound projection. It never reparses source files.
func QueryLLMProjected(ctx context.Context, s l.Set, p LLMProjection, digest string, q LLMQuery) (LLMPage, error) {
	return queryLLM(ctx, s, &p, digest, q)
}
func queryLLM(ctx context.Context, s l.Set, prepared *LLMProjection, digest string, q LLMQuery) (LLMPage, error) {
	out := LLMPage{Version: l.Version, ResultSHA: digest, Rows: []json.RawMessage{}}
	if len(digest) != 64 || q.Limit < 1 || q.Limit > 100 || len(q.Cursor) > 2048 || !l.In(q.Kind, "spans", "records", "relations", "attempts", "logical_requests", "events", "contexts") || !l.Text(q.TraceID) || !l.Text(q.RecordID) || !l.Text(q.Namespace) || !l.Text(q.Logical) {
		return out, fmt.Errorf("LLMTRACE_QUERY_INVALID")
	}
	identity := q
	identity.Cursor = ""
	out.QuerySHA = l.Hash(identity)
	type cursor struct {
		Result string `json:"result"`
		Query  string `json:"query"`
		Offset int    `json:"offset"`
	}
	offset := 0
	if q.Cursor != "" {
		raw, e := base64.RawURLEncoding.DecodeString(q.Cursor)
		if e != nil {
			return out, fmt.Errorf("LLMTRACE_CURSOR_INVALID")
		}
		var c cursor
		if e = l.Strict(raw, &c, 2048); e != nil || c.Result != digest || c.Query != out.QuerySHA || c.Offset < 0 {
			return out, fmt.Errorf("LLMTRACE_CURSOR_STALE")
		}
		offset = c.Offset
	}
	var p LLMProjection
	if prepared == nil {
		var e error
		p, e = ProjectLLM(ctx, s)
		if e != nil {
			return out, e
		}
	} else {
		p = *prepared
	}
	spans := map[string]l.Span{}
	for _, v := range s.Spans {
		spans[v.ID] = v
	}
	match := func(id string) bool {
		v, ok := spans[id]
		return ok && (q.TraceID == "" || v.TraceID == q.TraceID) && (q.RecordID == "" || id == q.RecordID) && (q.Namespace == "" || v.RequestNamespace == q.Namespace) && (q.Logical == "" || v.LogicalRequest == q.Logical)
	}
	matchedContexts := map[string]bool{}
	for _, v := range s.Spans {
		if match(v.ID) {
			matchedContexts[v.ContextID] = true
		}
	}
	matchedRequests := map[string]bool{}
	for _, v := range s.Spans {
		if match(v.ID) && v.LogicalRequest != "" {
			matchedRequests[requestKey(v)] = true
		}
	}
	add := func(v any) {
		i := out.Total
		out.Total++
		if i >= offset && len(out.Rows) < q.Limit {
			raw, _ := json.Marshal(v)
			out.Rows = append(out.Rows, raw)
		}
	}
	switch q.Kind {
	case "contexts":
		for _, v := range s.Contexts {
			if matchedContexts[v.ID] {
				add(v)
			}
		}
	case "spans":
		for _, v := range p.Spans {
			if match(v.ID) {
				add(v)
			}
		}
	case "records":
		for _, v := range s.Spans {
			if match(v.ID) {
				add(v)
			}
		}
	case "relations":
		for _, v := range p.Relations {
			if match(v.Record) {
				add(v)
			}
		}
	case "events":
		for _, v := range s.Events {
			if match(v.Span) {
				add(v)
			}
		}
	case "attempts":
		for _, v := range p.Attempts {
			var ids []string
			json.Unmarshal([]byte(v.Records), &ids)
			ok := false
			for _, id := range ids {
				ok = ok || match(id)
			}
			if ok {
				add(v)
			}
		}
	case "logical_requests":
		for _, v := range p.Requests {
			if (q.Namespace == "" || v.Namespace == q.Namespace) && (q.Logical == "" || v.ID == q.Logical) {
				if q.TraceID != "" || q.RecordID != "" {
					found := matchedRequests[l.Hash([]string{v.Namespace, v.ID})]
					if !found {
						continue
					}
				}
				add(v)
			}
		}
	}
	if offset > out.Total {
		return out, fmt.Errorf("LLMTRACE_CURSOR_OFFSET_INVALID: offset=%d", offset)
	}
	end := offset + len(out.Rows)
	if end < out.Total {
		raw, _ := json.Marshal(cursor{digest, out.QuerySHA, end})
		out.Next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return out, ctx.Err()
}
