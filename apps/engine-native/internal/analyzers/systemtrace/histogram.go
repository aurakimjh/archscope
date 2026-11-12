package systemtrace

import (
	"context"
	"fmt"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"math/big"
)

type HistogramBar struct {
	Lower *string `json:"lower"`
	Upper *string `json:"upper"`
	Count string  `json:"count"`
	Width float64 `json:"width"`
}
type HistogramPage struct {
	ResultSHA256  string                `json:"result_sha256"`
	EventID       string                `json:"event_id"`
	SourceSHA256  string                `json:"source_sha256"`
	SourceRef     string                `json:"source_ref"`
	Definition    m.HistogramDefinition `json:"definition"`
	Key           string                `json:"key"`
	Offset        int                   `json:"offset"`
	Next          int                   `json:"next"`
	Total         int                   `json:"total"`
	RetainedCount string                `json:"retained_count"`
	Rows          []HistogramBar        `json:"rows"`
	Warnings      []string              `json:"warnings"`
}

func Histogram(ctx context.Context, s m.Set, digest, id string, offset, limit int) (HistogramPage, error) {
	var out HistogramPage
	if offset < 0 || limit < 1 || limit > 100 {
		return out, fmt.Errorf("SYSTEM_HISTOGRAM_QUERY_INVALID")
	}
	var record *m.Record
	for i := range s.Records {
		if s.Records[i].ID == id {
			record = &s.Records[i]
			break
		}
	}
	if record == nil || record.Histogram == nil {
		return out, fmt.Errorf("SYSTEM_HISTOGRAM_REFERENCE_REQUIRED")
	}
	h := record.Histogram
	if offset > len(h.Buckets) {
		return out, fmt.Errorf("SYSTEM_HISTOGRAM_OFFSET_INVALID")
	}
	out = HistogramPage{ResultSHA256: digest, EventID: id, SourceSHA256: s.SourceSHA256, SourceRef: fmt.Sprintf("line:%d", record.Line), Key: h.Key, Offset: offset, Next: -1, Total: len(h.Buckets), Rows: []HistogramBar{}, Warnings: []string{"Original aggregate buckets: integer bounds are inclusive; absent bounds are open tails.", "Widths use the full retained histogram maximum. Exact counts remain strings.", "Retained bucket count is not a complete request population; omitted map keys/buckets cannot be recovered.", "No event chronology, cumulative differencing, quantile estimation or causal latency attribution."}}
	for _, d := range s.Capture.Histograms {
		if d.Name == h.Name {
			out.Definition = d
		}
	}
	maximum, total := new(big.Int), new(big.Int)
	for _, b := range h.Buckets {
		n, _ := new(big.Int).SetString(b.Count, 10)
		total.Add(total, n)
		if n.Cmp(maximum) > 0 {
			maximum.Set(n)
		}
	}
	out.RetainedCount = total.String()
	end := min(len(h.Buckets), offset+limit)
	for _, b := range h.Buckets[offset:end] {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		width := 0.0
		if maximum.Sign() > 0 {
			n, _ := new(big.Int).SetString(b.Count, 10)
			width, _ = new(big.Rat).SetFrac(n, maximum).Float64()
		}
		out.Rows = append(out.Rows, HistogramBar{b.Lower, b.Upper, b.Count, width})
	}
	if end < len(h.Buckets) {
		out.Next = end
	}
	return out, ctx.Err()
}
