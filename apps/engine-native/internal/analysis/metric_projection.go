package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

const metricIntervalsProvider = "metrics:intervals"

func (s *Service) metricIntervalPage(ctx context.Context, handle string, offset, limit int) (ProviderPage, error) {
	if offset < 0 || limit < 1 || limit > resultcontract.MaxPageRows {
		return ProviderPage{}, fmt.Errorf("invalid page bounds")
	}
	return withMetric(s, ctx, handle, "", func(ctx context.Context, prepared *metricSemantic, digest string) (ProviderPage, error) {
		exact, e := prepared.index.ProjectOffset(ctx, digest, prepared.core.Selection, offset, limit)
		if e != nil {
			return ProviderPage{}, e
		}
		page := resultcontract.Page{Provider: metricIntervalsProvider, Columns: []string{"name", "unit", "value", "delta", "rate", "start_ns", "end_ns", "status", "gap", "point_id", "refs"}, Rows: [][]resultcontract.Cell{}, Offset: offset, Next: -1, Total: exact.Total}
		for _, row := range exact.Rows {
			optional := func(v *string) any {
				if v == nil {
					return nil
				}
				return *v
			}
			refs, _ := json.Marshal(row.Refs)
			cells := []resultcontract.Cell{}
			for _, v := range []any{row.Name, row.Unit, optional(row.Value), optional(row.Delta), optional(row.Rate), optional(row.Start), optional(row.End), row.Status, row.Gap, row.PointID, string(refs)} {
				c := resultcontract.DisplayCell(v)
				page.Truncated = page.Truncated || c.Truncated
				page.Escaped = page.Escaped || c.Escaped
				cells = append(cells, c)
			}
			for _, i := range []int{2, 3, 4} {
				cells[i].Unit = cells[1].Text
				cells[i].Population = "individual observed metric series; no sum"
				if cells[i].Null {
					cells[i].Availability = "unavailable"
					cells[i].Reason = row.Status
				} else {
					cells[i].Availability = "available"
				}
				if i == 4 {
					cells[i].Unit = cells[1].Text + "/s"
				}
			}
			proposed := page
			proposed.Rows = append(append([][]resultcontract.Cell(nil), page.Rows...), cells)
			raw, _ := json.Marshal(proposed)
			if len(raw) > resultcontract.MaxPageBytes {
				page.Truncated = true
				break
			}
			page.Rows = append(page.Rows, cells)
		}
		if offset+len(page.Rows) < page.Total {
			page.Next = offset + len(page.Rows)
		}
		return ProviderPage{Handle: handle, ResultSHA256: digest, Page: page}, nil
	})
}
