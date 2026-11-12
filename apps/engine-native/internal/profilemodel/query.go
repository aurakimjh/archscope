package profilemodel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"unicode/utf8"
)

const QueryVersion = "1.0.0"

type Query struct {
	Selection Selection `json:"selection"`
	View      string    `json:"view"`
	FrameID   string    `json:"frame_id,omitempty"`
	Text      string    `json:"text,omitempty"`
	Cursor    string    `json:"cursor,omitempty"`
	Limit     int       `json:"limit"`
	TimeFrom  *string   `json:"time_from,omitempty"`
	TimeTo    *string   `json:"time_to,omitempty"`
}
type QueryRow struct {
	Depth        int      `json:"depth"`
	LayoutX      *float64 `json:"layout_x,omitempty"`
	LayoutWidth  *float64 `json:"layout_width,omitempty"`
	ID           string   `json:"id"`
	FrameID      string   `json:"frame_id,omitempty"`
	Name         string   `json:"name,omitempty"`
	Inclusive    string   `json:"inclusive"`
	Self         string   `json:"self"`
	ProfileID    string   `json:"profile_id,omitempty"`
	SourceRef    string   `json:"source_ref,omitempty"`
	SourceSHA256 []string `json:"source_sha256,omitempty"`
	Path         []string `json:"path,omitempty"`
	Timestamp    *string  `json:"timestamp,omitempty"`
	Interval     *string  `json:"interval,omitempty"`
}
type QueryPage struct {
	Availability string     `json:"availability"`
	Reason       string     `json:"reason,omitempty"`
	Untimed      int        `json:"untimed_observations"`
	Quality      Quality    `json:"quality"`
	Version      string     `json:"version"`
	ResultSHA256 string     `json:"result_sha256"`
	QuerySHA256  string     `json:"query_sha256"`
	Population   string     `json:"population"`
	Selection    Selection  `json:"selection"`
	Metric       Metric     `json:"metric"`
	Denominator  string     `json:"denominator"`
	Total        int        `json:"total"`
	Matched      int        `json:"matched"`
	Next         string     `json:"next"`
	Rows         []QueryRow `json:"rows"`
}
type queryCursor struct {
	Result, Query string
	Offset        int
}

// QuerySet keeps all math on the canonical selection. Search only filters rows;
// it never changes the denominator. Evidence coordinates retain their declared
// source hash and format-specific reference, never a fabricated line number.
func QuerySet(ctx context.Context, set ProfileSet, resultSHA string, q Query) (QueryPage, error) {
	out := QueryPage{Version: QueryVersion, ResultSHA256: resultSHA, Rows: []QueryRow{}, Selection: CopySelection(q.Selection)}
	if len(resultSHA) != 64 || q.Limit < 1 || q.Limit > 200 || len(q.Text) > 1024 {
		return out, fmt.Errorf("PROFILE_QUERY_INVALID")
	}
	identity := q
	identity.Cursor = ""
	out.QuerySHA256 = hash("", []any{QueryVersion, identity})
	offset := 0
	if q.Cursor != "" {
		var cursor queryCursor
		body, e := base64.RawURLEncoding.DecodeString(q.Cursor)
		if e != nil || len(body) > 1024 || json.Unmarshal(body, &cursor) != nil || cursor.Result != resultSHA || cursor.Query != out.QuerySHA256 || cursor.Offset < 0 {
			return out, fmt.Errorf("PROFILE_CURSOR_STALE")
		}
		offset = cursor.Offset
	}
	var from, to *big.Rat
	var err error
	if q.TimeFrom != nil {
		from, err = Number(*q.TimeFrom)
		if err != nil {
			return out, err
		}
	}
	if q.TimeTo != nil {
		to, err = Number(*q.TimeTo)
		if err != nil {
			return out, err
		}
	}
	if from != nil && to != nil && from.Cmp(to) > 0 {
		return out, fmt.Errorf("PROFILE_TIME_WINDOW_INVALID")
	}
	if (from != nil || to != nil) && q.View != "evidence" {
		return out, fmt.Errorf("PROFILE_TIME_WINDOW_REQUIRES_EVIDENCE_VIEW")
	}
	if from != nil || to != nil {
		clock := ""
		for _, p := range set.Profiles {
			selected := false
			for _, id := range q.Selection.ProfileIDs {
				selected = selected || p.ID == id
			}
			if !selected {
				continue
			}
			identity := p.Clock + "/" + p.TimeUnit + "/" + p.TimeScale
			if clock != "" && clock != identity {
				return out, comparisonRefusal("clock_class_mismatch", "time-window query requires matching selected clock kind/unit/scale")
			}
			clock = identity
		}
	}
	tree, e := BuildTree(ctx, set, q.Selection)
	if e != nil {
		return out, e
	}
	out.Availability = tree.Availability
	out.Reason = tree.Reason
	out.Quality = copyQuality(set.Quality)
	out.Population = tree.Population
	out.Denominator = tree.Root.Inclusive
	out.Metric = tree.Metric
	rows := []QueryRow{}
	emit := func(row QueryRow) {
		out.Total++
		if q.Text != "" && !strings.Contains(strings.ToLower(row.Name+" "+row.FrameID+" "+row.ProfileID+" "+row.SourceRef), strings.ToLower(q.Text)) {
			return
		}
		if out.Matched >= offset && len(out.Rows) < q.Limit {
			row.Path = append([]string(nil), row.Path...)
			out.Rows = append(out.Rows, row)
		}
		out.Matched++
	}
	frames := map[string]Frame{}
	for _, f := range set.Frames {
		frames[f.ID] = f
	}
	switch q.View {
	case "tree":

		denominator, _ := Number(tree.Root.Inclusive)
		var visit func(TreeNode, []string, int, *big.Rat)
		visit = func(n TreeNode, path []string, depth int, start *big.Rat) {
			if n.FrameID != "" {
				path = append(path, n.FrameID)
				row := QueryRow{ID: n.ID, FrameID: n.FrameID, Name: queryName(frames[n.FrameID].Display), Inclusive: n.Inclusive, Self: n.Self, Depth: depth, Path: path}
				value, _ := Number(n.Inclusive)
				if !tree.Metric.Signed && denominator.Sign() > 0 && value.Sign() >= 0 {
					x, _ := new(big.Rat).Quo(start, denominator).Float64()
					width, _ := new(big.Rat).Quo(value, denominator).Float64()
					row.LayoutX = &x
					row.LayoutWidth = &width
				}
				emit(row)
			}
			offset := new(big.Rat).Set(start)
			for _, child := range n.Children {
				visit(child, path, depth+1, offset)
				value, _ := Number(child.Inclusive)
				offset.Add(offset, value)
			}
		}
		visit(tree.Root, nil, -1, new(big.Rat))

	case "top", "callers", "callees":
		if q.View != "top" {
			if _, ok := frames[q.FrameID]; !ok {
				return out, fmt.Errorf("PROFILE_FRAME_UNKNOWN")
			}
		}
		totals, self := map[string]*big.Rat{}, map[string]*big.Rat{}
		scale, _ := Number(tree.Metric.Scale)
		e = set.each(ctx, q.Selection, func(_ Profile, _ Sample, path []string, value *big.Rat) error {
			value = new(big.Rat).Mul(value, scale)
			selected := map[string]bool{}
			for i, id := range path {
				if q.View == "top" {
					selected[id] = true
				} else if id == q.FrameID {
					j := i - 1
					if q.View == "callees" {
						j = i + 1
					}
					if j >= 0 && j < len(path) {
						selected[path[j]] = true
					}
				}
			}
			for id := range selected {
				if totals[id] == nil {
					totals[id] = new(big.Rat)
					self[id] = new(big.Rat)
				}
				totals[id].Add(totals[id], value)
				if path[len(path)-1] == id {
					self[id].Add(self[id], value)
				}
			}
			return nil
		})
		if e != nil {
			return out, e
		}
		for id, n := range totals {
			rows = append(rows, QueryRow{ID: id, FrameID: id, Name: queryName(frames[id].Display), Inclusive: Text(n), Self: Text(self[id])})
		}
		sort.Slice(rows, func(i, j int) bool {
			a, _ := Number(rows[i].Inclusive)
			b, _ := Number(rows[j].Inclusive)
			if c := a.Cmp(b); c != 0 {
				return c > 0
			}
			return rows[i].ID < rows[j].ID
		})
	case "evidence":
		scale, _ := Number(tree.Metric.Scale)
		e = set.each(ctx, q.Selection, func(p Profile, s Sample, path []string, n *big.Rat) error {
			if from != nil || to != nil {
				if s.Timestamp == nil {
					out.Untimed++
					return nil
				}
				at, err := Number(*s.Timestamp)
				if err != nil {
					return err
				}
				scale, e := Number(p.TimeScale)
				if e != nil {
					return e
				}
				at.Mul(at, scale)
				if from != nil && at.Cmp(from) < 0 || to != nil && at.Cmp(to) >= 0 {
					return nil
				}
			}
			if q.FrameID != "" {
				found := false
				for _, id := range path {
					found = found || id == q.FrameID
				}
				if !found {
					return nil
				}
			}
			names := []string{}
			for _, id := range path {
				if len(strings.Join(names, ";")) < 4096 {
					names = append(names, queryName(frames[id].Display))
				}
			}
			value := Text(new(big.Rat).Mul(n, scale))
			emit(QueryRow{ID: hash("observation-", []any{p.ID, s}), Name: strings.Join(names, ";"), Inclusive: value, Self: value, ProfileID: p.ID, SourceRef: s.SourceRef, SourceSHA256: append([]string(nil), set.SourceHashes...), Path: path, Timestamp: s.Timestamp, Interval: s.Interval})
			return nil
		})
		if e != nil {
			return out, e
		}
	default:
		return out, fmt.Errorf("PROFILE_VIEW_UNSUPPORTED")
	}
	for _, row := range rows {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		emit(row)
	}
	if offset > out.Matched {
		return out, fmt.Errorf("PROFILE_CURSOR_STALE")
	}
	end := offset + len(out.Rows)
	if end < out.Matched {
		body, _ := json.Marshal(queryCursor{resultSHA, out.QuerySHA256, end})
		out.Next = base64.RawURLEncoding.EncodeToString(body)
	}
	body, _ := json.Marshal(out)
	if len(body) > 1<<20 {
		return QueryPage{}, budget("profile query response exceeds 1 MiB; reduce limit or selection")
	}
	return out, nil
}

func queryName(s string) string {
	if len(s) <= 512 {
		return s
	}
	s = s[:512]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "…"
}
