package resultcontract

import (
	"context"
	"encoding/json"
	"fmt"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

// addMetricIntervals enriches a disposable HTML view, never the saved envelope.
// Explicit export budgets refuse the whole projection rather than truncating it.
func addMetricIntervals(r models.AnalysisResult) (models.AnalysisResult, error) {
	if r.Type != "metrics_timeseries" {
		return r, nil
	}
	if r.Metadata.SchemaVersion != "0.1.0" && r.Metadata.SchemaVersion != "0.2.0" {
		return r, fmt.Errorf("METRIC_VERSION_UNSUPPORTED")
	}
	var core struct {
		Version   string      `json:"version"`
		Model     m.Set       `json:"model"`
		Selection m.Selection `json:"selection"`
	}
	raw, e := json.Marshal(r.Metadata.Extra["metrics_core"])
	if e != nil {
		return r, e
	}
	if e = wirejson.DecodeWithLimits(raw, &core, wirejson.ResultLimits(64<<20, 128)); e != nil {
		return r, e
	}
	if core.Version != m.Version {
		return r, fmt.Errorf("METRIC_VERSION_UNSUPPORTED")
	}
	rows, e := m.SelectRows(context.Background(), core.Model, core.Selection)
	if e != nil {
		return r, e
	}
	if len(rows) > MaxProjectionEntries/14 {
		return r, &ProjectionBudgetError{"metric interval entry count; use dedicated interval CSV or narrower selection"}
	}
	projected := make([]map[string]any, 0, len(rows))
	bytes := 0
	for _, row := range rows {
		hist, _ := json.Marshal(row.ObservedHistogram)
		deltaHist, _ := json.Marshal(row.Histogram)
		refs, _ := json.Marshal(row.Refs)
		value := func(v *string) any {
			if v == nil {
				return nil
			}
			return *v
		}
		entry := map[string]any{"name": row.Name, "unit": row.Unit, "value": value(row.Value), "delta": value(row.Delta), "rate": value(row.Rate), "start_ns": value(row.Start), "end_ns": value(row.End), "status": row.Status, "gap": row.Gap, "point_id": row.PointID, "refs": string(refs), "observed_histogram": string(hist), "interval_histogram": string(deltaHist), "special": row.Special}
		b, _ := json.Marshal(entry)
		bytes += len(b) + 1
		if bytes > MaxProjectionBytes {
			return r, &ProjectionBudgetError{"metric interval bytes; use dedicated interval CSV or narrower selection"}
		}
		projected = append(projected, entry)
	}
	r.Tables["metric_intervals"] = projected
	return r, nil
}
