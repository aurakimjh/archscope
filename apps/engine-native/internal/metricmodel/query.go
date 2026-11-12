package metricmodel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
)

type Selection struct {
	SeriesIDs []string `json:"series_ids"`
	From      *string  `json:"from_ns,omitempty"`
	To        *string  `json:"to_ns,omitempty"`
}
type Query struct {
	Selection Selection `json:"selection"`
	Cursor    string    `json:"cursor,omitempty"`
	Limit     int       `json:"limit"`
}
type Page struct {
	Version      string     `json:"version"`
	ResultSHA256 string     `json:"result_sha256"`
	QuerySHA256  string     `json:"query_sha256"`
	Selection    Selection  `json:"selection"`
	WindowPolicy string     `json:"window_policy"`
	Total        int        `json:"total"`
	Next         string     `json:"next"`
	Rows         []Interval `json:"rows"`
}

func (s Set) ValidateSelection(selection Selection) error {
	if len(selection.SeriesIDs) == 0 || len(selection.SeriesIDs) > MaxSeries {
		return fmt.Errorf("METRIC_SELECTION_REQUIRED")
	}
	seen := map[string]bool{}
	known := map[string]bool{}
	for _, series := range s.Series {
		known[series.ID] = true
	}
	for _, id := range selection.SeriesIDs {
		if !known[id] || seen[id] {
			return fmt.Errorf("METRIC_SELECTION_INVALID")
		}
		seen[id] = true
	}
	from, fk := Time(selection.From)
	to, tk := Time(selection.To)
	if selection.From != nil && !fk || selection.To != nil && !tk || fk && tk && from >= to {
		return fmt.Errorf("METRIC_WINDOW_INVALID")
	}
	return nil
}
func SelectRows(ctx context.Context, set Set, selection Selection) ([]Interval, error) {
	if e := set.ValidateSelection(selection); e != nil {
		return nil, e
	}
	all, e := Intervals(ctx, set)
	if e != nil {
		return nil, e
	}
	ids := map[string]bool{}
	for _, id := range selection.SeriesIDs {
		ids[id] = true
	}
	rows := []Interval{}
	from, fk := Time(selection.From)
	to, tk := Time(selection.To)
	for _, row := range all {
		if !ids[row.SeriesID] {
			continue
		}
		end, known := Time(row.End)
		if (fk || tk) && !known {
			continue
		}
		if known && (fk && end < from || tk && end >= to) {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}
func QuerySet(ctx context.Context, set Set, digest string, q Query) (Page, error) {
	out := Page{Version: Version, ResultSHA256: digest, Selection: q.Selection, WindowPolicy: "end_timestamp_selection; intervals are not clipped or redistributed", Rows: []Interval{}}
	if len(digest) != 64 || q.Limit < 1 || q.Limit > 200 || len(q.Cursor) > 2048 {
		return out, fmt.Errorf("METRIC_QUERY_INVALID")
	}
	identity := q
	identity.Cursor = ""
	identity.Selection.SeriesIDs = append([]string(nil), q.Selection.SeriesIDs...)
	sort.Strings(identity.Selection.SeriesIDs)
	out.QuerySHA256 = Hash(identity)
	offset := 0
	if q.Cursor != "" {
		var c struct {
			Result, Query string
			Offset        int
		}
		raw, e := base64.RawURLEncoding.DecodeString(q.Cursor)
		if e != nil || json.Unmarshal(raw, &c) != nil || c.Result != digest || c.Query != out.QuerySHA256 || c.Offset < 0 {
			return out, fmt.Errorf("METRIC_CURSOR_STALE")
		}
		offset = c.Offset
	}
	rows, e := SelectRows(ctx, set, q.Selection)
	if e != nil {
		return out, e
	}
	out.Total = len(rows)
	if offset > len(rows) {
		return out, fmt.Errorf("METRIC_CURSOR_STALE")
	}
	end := min(offset+q.Limit, len(rows))
	out.Rows = append(out.Rows, rows[offset:end]...)
	if end < len(rows) {
		raw, _ := json.Marshal(struct {
			Result, Query string
			Offset        int
		}{digest, out.QuerySHA256, end})
		out.Next = base64.RawURLEncoding.EncodeToString(raw)
	}
	body, _ := json.Marshal(out)
	if len(body) > 1<<20 {
		return Page{}, fmt.Errorf("METRIC_QUERY_BUDGET: response exceeds 1 MiB; reduce page size")
	}
	return out, nil
}

// Merge preserves source namespaces and only deduplicates exact qualified export
// positions or byte-identical input copies. Inputs are immutable.
func Merge(ctx context.Context, sets []Set) (Set, error) {
	out := Set{Version: Version, Normalizer: Normalizer, Sources: []Source{}, Series: []Series{}, Points: []Point{}, Warnings: []string{}}
	series := map[string]bool{}
	warnings := map[string]bool{}
	for _, set := range sets {
		if e := set.Validate(ctx); e != nil {
			return out, e
		}
		mapping := map[string]string{}
		if len(out.Sources)+len(set.Sources) > 128 {
			return out, fmt.Errorf("METRIC_BUDGET_EXCEEDED: merged sources")
		}
		for _, source := range set.Sources {
			next := source
			next.ID = fmt.Sprintf("source-%d", len(out.Sources))
			mapping[source.ID] = next.ID
			out.Sources = append(out.Sources, next)
		}
		for _, s := range set.Series {
			if !series[s.ID] {
				if len(out.Series) >= MaxSeries {
					return out, fmt.Errorf("METRIC_BUDGET_EXCEEDED: merged series")
				}
				out.Series = append(out.Series, s)
				series[s.ID] = true
			}
		}
		for _, point := range set.Points {
			for _, ref := range point.Refs {
				next := point
				next.Refs = []Ref{{SourceID: mapping[ref.SourceID], Position: ref.Position}}
				next.ID = PointID(next)
				if len(out.Points) >= MaxPoints {
					return out, fmt.Errorf("METRIC_BUDGET_EXCEEDED: merged references")
				}
				out.Points = append(out.Points, next)
			}
		}
		for _, warning := range set.Warnings {
			if !warnings[warning] {
				out.Warnings = append(out.Warnings, warning)
				warnings[warning] = true
			}
		}
	}
	return Normalize(ctx, out)
}
