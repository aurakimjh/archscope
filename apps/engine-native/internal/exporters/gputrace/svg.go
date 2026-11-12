// Package gputrace exports selected interval views. Original parsing stays elsewhere.
package gputrace

import (
	"context"
	"encoding/json"
	"fmt"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gputrace"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	"html"
	"math/big"
	"sort"
	"strings"
)

// SVG requires one source clock, refuses over-budget selections, and preserves
// exact ns in titles/metadata; floating positions are presentation only.
func SVG(ctx context.Context, s g.Set, p a.Projection, digest string, q a.Query) ([]byte, error) {
	if q.Source == "" || q.Kind != "events" || q.Cursor != "" {
		return nil, fmt.Errorf("GPU_TRACE_SVG_SOURCE_REQUIRED: choose one source clock and a bounded event window")
	}
	all := []a.EventView{}
	cursor := ""
	for {
		query := q
		query.Cursor = cursor
		query.Limit = 100
		page, e := a.QuerySet(ctx, s, p, digest, query)
		if e != nil {
			return nil, e
		}
		if page.Total > 2000 {
			return nil, fmt.Errorf("GPU_TRACE_SVG_EVENT_BUDGET: events=%d limit=2000; select a smaller window", page.Total)
		}
		for _, raw := range page.Rows {
			var v a.EventView
			if e = g.Strict(raw, &v, 16<<10); e != nil {
				return nil, e
			}
			all = append(all, v)
		}
		if page.Next == "" {
			break
		}
		cursor = page.Next
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("GPU_TRACE_SVG_NO_INTERVALS: summary-only files have no detailed timeline")
	}
	tracks := map[string]g.Track{}
	for _, v := range s.Tracks {
		tracks[v.ID] = v
	}
	byTrack := map[string][]a.EventView{}
	lo, hi := all[0].From, all[0].To
	for _, v := range all {
		byTrack[v.Track] = append(byTrack[v.Track], v)
		if g.Cmp(v.From, lo) < 0 {
			lo = v.From
		}
		if g.Cmp(v.To, hi) > 0 {
			hi = v.To
		}
	}
	keys := []string{}
	for k := range byTrack {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 64 {
		return nil, fmt.Errorf("GPU_TRACE_SVG_TRACK_BUDGET: tracks=%d limit=64", len(keys))
	}
	span := g.Difference(hi, lo)
	if span == "0" {
		span = "1"
	}
	n, _ := new(big.Int).SetString(span, 10)
	position := func(s string) float64 {
		x, _ := new(big.Int).SetString(g.Difference(s, lo), 10)
		r := new(big.Rat).SetFrac(x, n)
		v, _ := r.Float64()
		return 300 + v*1200
	}
	var out strings.Builder
	selectedSource := g.Source{}
	for _, source := range s.Sources {
		if source.ID == q.Source {
			selectedSource = source
			break
		}
	}
	metadata, _ := json.Marshal(map[string]any{"result_sha256": digest, "query": q, "source": selectedSource, "meaning": "Selected whole intervals in one source clock; exact ns, no inferred hardware utilization or cross-rank alignment"})
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="1540" height="%d" viewBox="0 0 1540 %d"><rect width="100%%" height="100%%" fill="white"/><style>text{font:12px sans-serif;fill:#172033}</style><title>Selected execution intervals; one source clock</title><text x="12" y="22">Execution intervals / one source clock / %d events; exact ns in titles</text><text x="12" y="42">Source %s</text><text x="12" y="62">Result SHA-256 %s</text><text x="12" y="82">Extent [%s, %s] ns; whole intervals, no clipping / no hardware utilization or causal claim</text>`, 120+len(keys)*48, 120+len(keys)*48, len(all), html.EscapeString(q.Source), html.EscapeString(digest), lo, hi)
	out.WriteString("<metadata>" + html.EscapeString(string(metadata)) + "</metadata>")
	for i, k := range keys {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		t := tracks[k]
		y := 105 + i*48
		label := t.Kind + " / thread " + t.Thread + " device " + t.Device + " context " + t.Context + " stream " + t.Stream
		fullLabel := label
		if len([]rune(label)) > 36 {
			label = string([]rune(label)[:36]) + "…"
		}
		fmt.Fprintf(&out, `<text x="12" y="%d">%s</text><line x1="300" y1="%d" x2="1500" y2="%d" stroke="#cbd5e1"/>`, y+12, html.EscapeString(label)+"<title>"+html.EscapeString(fullLabel)+"</title>", y+28, y+28)
		for _, v := range byTrack[k] {
			x, end := position(v.From), position(v.To)
			width := end - x
			if width < 1 {
				width = 1
			}
			color := "#2563eb"
			if v.Kind == "gpu_kernel" {
				color = "#059669"
			} else if v.Kind == "memcpy" {
				color = "#d97706"
			} else if v.Kind == "communication" {
				color = "#7c3aed"
			}
			fmt.Fprintf(&out, `<rect x="%.5f" y="%d" width="%.5f" height="16" fill="%s" opacity="0.6"><title>%s / %s / [%s,%s] ns / duration %s ns / %s</title></rect>`, x, y+16, width, color, html.EscapeString(v.Name), html.EscapeString(v.ID), v.From, v.To, v.Duration, html.EscapeString(v.SelfStatus))
		}
	}
	out.WriteString(`</svg>`)
	return []byte(out.String()), nil
}
