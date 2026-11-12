package execution

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
)

type Query struct {
	Kind     string `json:"kind"`
	Name     string `json:"metric_name,omitempty"`
	GroupSHA string `json:"group_sha256,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
	Limit    int    `json:"limit"`
}
type Page struct {
	ResultSHA string            `json:"result_sha256"`
	QuerySHA  string            `json:"query_sha256"`
	Rows      []json.RawMessage `json:"rows"`
	Next      string            `json:"next"`
	Total     int               `json:"total"`
}
type cursor struct {
	Identity string `json:"identity"`
	Offset   int    `json:"offset"`
}

func readCursor(raw, identity string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	if len(raw) > 1024 {
		return 0, fmt.Errorf("EXECUTION_CURSOR_BUDGET")
	}
	body, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, err
	}
	var c cursor
	if err = e.Strict(body, &c, 1024); err != nil || c.Identity != identity || c.Offset < 0 {
		return 0, fmt.Errorf("EXECUTION_CURSOR_STALE")
	}
	return c.Offset, nil
}
func next(identity string, offset int) string {
	raw, _ := json.Marshal(cursor{identity, offset})
	return base64.RawURLEncoding.EncodeToString(raw)
}
func QuerySet(ctx context.Context, s e.Set, digest string, q Query) (Page, error) {
	out := Page{ResultSHA: digest, Rows: []json.RawMessage{}}
	if !e.Digest(digest) || q.Limit < 1 || q.Limit > 100 || !e.In(q.Kind, "metrics", "samples", "contexts", "sources", "crash", "crash_frames") || !e.Text(q.Name) || q.GroupSHA != "" && !e.Digest(q.GroupSHA) {
		return out, fmt.Errorf("EXECUTION_QUERY_INVALID")
	}
	if q.Name != "" && q.Kind != "metrics" || q.GroupSHA != "" && !e.In(q.Kind, "metrics", "samples") {
		return out, fmt.Errorf("EXECUTION_FILTER_NOT_APPLICABLE")
	}
	identity := q
	identity.Cursor = ""
	out.QuerySHA = e.Hash([]any{digest, identity})
	offset, err := readCursor(q.Cursor, out.QuerySHA)
	if err != nil {
		return out, err
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
	case "metrics":
		for _, v := range s.Metrics {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			if (q.Name == "" || v.Name == q.Name) && (q.GroupSHA == "" || v.GroupSHA == q.GroupSHA) {
				add(v)
			}
		}
	case "samples":
		for _, v := range s.Samples {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			if q.GroupSHA == "" || v.GroupSHA == q.GroupSHA {
				add(v)
			}
		}
	case "contexts":
		if s.Context != nil {
			add(*s.Context)
		}
	case "sources":
		add(s.Source)
	case "crash":
		if s.Crash != nil {
			v := *s.Crash
			v.Frames = []e.Frame{}
			add(v)
		}
	case "crash_frames":
		if s.Crash != nil {
			for _, v := range s.Crash.Frames {
				add(v)
			}
		}
	}
	if offset > out.Total {
		return out, fmt.Errorf("EXECUTION_CURSOR_OFFSET_INVALID")
	}
	if end := offset + len(out.Rows); end < out.Total {
		out.Next = next(out.QuerySHA, end)
	}
	return out, ctx.Err()
}

type CompareQuery struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
}

func CompareSets(ctx context.Context, a, b e.Set, leftSHA, rightSHA string, q CompareQuery) (e.Comparison, error) {
	identity := e.Hash([]any{"execution-comparison.v1", leftSHA, rightSHA, q.Limit})
	offset, err := readCursor(q.Cursor, identity)
	if err != nil {
		return e.Comparison{}, err
	}
	out, err := e.Compare(ctx, a, b, leftSHA, rightSHA, offset, q.Limit)
	if err != nil {
		return out, err
	}
	out.QuerySHA = identity
	if out.Status != "compatible_declarations" && q.Cursor != "" {
		return out, fmt.Errorf("EXECUTION_COMPARISON_CURSOR_INCOMPATIBLE")
	}
	if end := offset + len(out.Rows); end < out.Total {
		out.Next = next(identity, end)
	}
	return out, ctx.Err()
}
