package metrichistory

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const ResultType = "metrics_timeseries"
const PayloadVersion = "0.2.0"

type Core struct {
	Version   string      `json:"version"`
	Model     m.Set       `json:"model"`
	Selection m.Selection `json:"selection"`
}
type pointProjection struct {
	PointID string `json:"point_id"`
	Status  string `json:"status"`
}
type timeProjection struct {
	Time    string `json:"time"`
	Clock   string `json:"clock"`
	PointID string `json:"point_id"`
}

// Series selection removes unrelated model payload. Keep all points of retained
// series so predecessor/reset/overlap context is not changed by a time window.
func selectedModel(set m.Set, selection m.Selection) (m.Set, error) {
	if e := set.ValidateSelection(selection); e != nil {
		return set, e
	}
	ids := map[string]bool{}
	for _, id := range selection.SeriesIDs {
		ids[id] = true
	}
	out := m.Set{Version: set.Version, Normalizer: set.Normalizer, Sources: []m.Source{}, Series: []m.Series{}, Points: []m.Point{}, Warnings: append([]string(nil), set.Warnings...)}
	sources := map[string]bool{}
	for _, series := range set.Series {
		if ids[series.ID] {
			out.Series = append(out.Series, series)
		}
	}
	for _, point := range set.Points {
		if ids[point.SeriesID] {
			out.Points = append(out.Points, point)
			out.Duplicates += len(point.Refs) - 1
			for _, ref := range point.Refs {
				sources[ref.SourceID] = true
			}
		}
	}
	// Empty selected series retain the bounded source ledger for provenance.
	for _, source := range set.Sources {
		if sources[source.ID] || len(out.Points) == 0 {
			out.Sources = append(out.Sources, source)
		}
	}
	return out, nil
}
func Build(ctx context.Context, set m.Set, selection m.Selection, source string) (models.AnalysisResult, error) {
	result := models.New(ResultType, "metric-history")
	result.Metadata.SchemaVersion = PayloadVersion
	result.SourceFiles = []string{source}
	if e := set.Validate(ctx); e != nil {
		return result, e
	}
	retained, e := selectedModel(set, selection)
	if e != nil {
		return result, e
	}
	rows, e := m.SelectRows(ctx, retained, selection)
	if e != nil {
		return result, e
	}
	points := make([]pointProjection, 0, len(rows))
	timed := []timeProjection{}
	evidence := make([]string, 0, len(rows))
	statuses := map[string]int{}
	derived := 0
	for _, row := range rows {
		points = append(points, pointProjection{row.PointID, row.Status})
		if row.Timestamp != nil {
			timed = append(timed, timeProjection{*row.Timestamp, row.Clock, row.PointID})
		}
		evidence = append(evidence, row.PointID)
		statuses[row.Status]++
		if row.Rate != nil {
			derived++
		}
	}
	result.Summary = map[string]any{"series_count": strconv.Itoa(len(selection.SeriesIDs)), "point_count": strconv.Itoa(len(rows)), "derived_interval_count": strconv.Itoa(derived), "untimed_point_count": strconv.Itoa(len(rows) - len(timed)), "deduplicated_point_count": strconv.Itoa(retained.Duplicates)}
	result.Tables = map[string]any{"points": points, "timed_points": timed}
	result.Series = map[string]any{}
	result.Charts = map[string]any{}
	result.Metadata.Extra["envelope_schema_version"] = "1.0.0"
	result.Metadata.Extra["extensions"] = map[string]string{"measurement": "1.0.0", "metrics_core": m.Version}
	result.Metadata.Extra["metrics_core"] = Core{m.Version, retained, selection}
	result.Metadata.Extra["status_counts"] = statuses
	result.Metadata.Extra["window_policy"] = "end_timestamp_selection; no clipping, interpolation, histogram quantiles or cross-series sum"
	result.Metadata.Extra["identity_basis"] = "declared"
	result.Metadata.Extra["clock_ids"] = []string{"unix"}
	result.Metadata.Extra["evidence_ids"] = evidence
	result.Metadata.Extra["comparison_scope"] = "metric-level comparison policy not qualified; cross-unit/client-server/OTLP-Prometheus aggregation is unavailable"
	result.Metadata.Extra["storage_policy"] = "selected series only; all their points retain interval context; detailed values/buckets are queried from metrics_core; timeline stores time/clock/reference only"
	measurement := map[string]resultcontract.Measurement{}
	for key := range result.Summary {
		measurement["/summary/"+key] = resultcontract.Measurement{Kind: "record_count", Unit: "count", Scope: "selected_metric_points", Aggregation: "count", Encoding: "exact_integer_string", Availability: "observed"}
	}
	result.Metadata.Extra["measurement"] = measurement
	body, e := json.Marshal(result)
	if e != nil {
		return result, e
	}
	if len(body) > 64<<20 {
		return result, fmt.Errorf("METRIC_RESULT_BUDGET: %d bytes exceeds %d bytes; selected points=%d series=%d retained model points=%d. Select fewer series or split the original export; time windows retain predecessor/overlap context", len(body), 64<<20, len(rows), len(selection.SeriesIDs), len(retained.Points))
	}
	return result, nil
}
func ReadCore(result models.AnalysisResult) (Core, error) {
	if result.Metadata.SchemaVersion == LegacyPayloadVersion {
		return readLegacyCore(result)
	}
	var core Core
	if result.Type != ResultType || result.Metadata.SchemaVersion != PayloadVersion {
		return core, fmt.Errorf("METRIC_RESULT_VERSION_UNSUPPORTED")
	}
	raw, e := json.Marshal(result.Metadata.Extra["metrics_core"])
	if e != nil {
		return core, e
	}
	if e = wirejson.DecodeWithLimits(raw, &core, wirejson.ResultLimits(64<<20, 128)); e != nil {
		return core, e
	}
	if core.Version != m.Version {
		return core, fmt.Errorf("METRIC_CORE_VERSION_UNSUPPORTED")
	}
	if e = core.Model.Validate(context.Background()); e != nil {
		return core, e
	}
	return core, core.Model.ValidateSelection(core.Selection)
}
func Validate(result models.AnalysisResult) error {
	if result.Metadata.SchemaVersion == LegacyPayloadVersion {
		return ValidateLegacy(result)
	}
	core, e := ReadCore(result)
	if e != nil {
		return e
	}
	expected, e := Build(context.Background(), core.Model, core.Selection, "")
	if e != nil {
		return e
	}
	for _, pair := range [][2]any{{result.Summary, expected.Summary}, {result.Tables, expected.Tables}, {result.Series, expected.Series}, {result.Charts, expected.Charts}} {
		if !equalJSON(pair[0], pair[1]) {
			return fmt.Errorf("METRIC_RESULT_MODEL_MISMATCH")
		}
	}
	for _, key := range []string{"measurement", "status_counts", "window_policy", "identity_basis", "clock_ids", "evidence_ids", "comparison_scope", "storage_policy"} {
		if !equalJSON(result.Metadata.Extra[key], expected.Metadata.Extra[key]) {
			return fmt.Errorf("METRIC_RESULT_MODEL_MISMATCH")
		}
	}
	subset, e := selectedModel(core.Model, core.Selection)
	if e != nil {
		return e
	}
	if !equalJSON(core.Model, subset) {
		return fmt.Errorf("METRIC_UNSELECTED_MODEL_PAYLOAD")
	}
	return nil
}
func equalJSON(a, b any) bool {
	left, e := json.Marshal(a)
	if e != nil {
		return false
	}
	right, e := json.Marshal(b)
	if e != nil {
		return false
	}
	var x, y any
	if wirejson.DecodeWithLimits(left, &x, wirejson.ResultLimits(64<<20, 128)) != nil || wirejson.DecodeWithLimits(right, &y, wirejson.ResultLimits(64<<20, 128)) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}
func Descriptor() resultcontract.Descriptor {
	d := LegacyDescriptor()
	d.Payload = resultcontract.Range{Min: PayloadVersion, Before: "0.2.1"}
	d.ValidatePayload = Validate
	for i := range d.Providers {
		d.Providers[i].Version = "2.0.0"
	}
	return d
}
