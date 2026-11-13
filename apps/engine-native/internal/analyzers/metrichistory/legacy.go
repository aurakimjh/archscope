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

const LegacyPayloadVersion = "0.1.0"

func BuildLegacy(ctx context.Context, set m.Set, selection m.Selection, source string) (models.AnalysisResult, error) {
	result := models.New(ResultType, "metric-history")
	result.Metadata.SchemaVersion = LegacyPayloadVersion
	result.SourceFiles = []string{source}
	rows, e := m.SelectRows(ctx, set, selection)
	if e != nil {
		return result, e
	}
	timed := []m.Interval{}
	statuses := map[string]int{}
	derived := 0
	for _, row := range rows {
		statuses[row.Status]++
		if row.Rate != nil {
			derived++
		}
		if row.Timestamp != nil {
			timed = append(timed, row)
		}
	}
	result.Summary = map[string]any{"series_count": strconv.Itoa(len(selection.SeriesIDs)), "point_count": strconv.Itoa(len(rows)), "derived_interval_count": strconv.Itoa(derived), "untimed_point_count": strconv.Itoa(len(rows) - len(timed)), "deduplicated_point_count": strconv.Itoa(set.Duplicates)}
	result.Tables = map[string]any{"points": rows, "timed_points": timed}
	result.Series = map[string]any{}
	result.Charts = map[string]any{}
	result.Metadata.Extra["envelope_schema_version"] = "1.0.0"
	result.Metadata.Extra["extensions"] = map[string]string{"measurement": "1.0.0", "metrics_core": m.Version}
	result.Metadata.Extra["metrics_core"] = Core{m.Version, set, selection}
	result.Metadata.Extra["status_counts"] = statuses
	result.Metadata.Extra["window_policy"] = "end_timestamp_selection; no clipping, interpolation, histogram quantiles or cross-series sum"
	result.Metadata.Extra["identity_basis"] = "declared"
	result.Metadata.Extra["clock_ids"] = []string{"unix"}
	evidenceIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		evidenceIDs = append(evidenceIDs, row.PointID)
	}
	result.Metadata.Extra["evidence_ids"] = evidenceIDs
	result.Metadata.Extra["comparison_scope"] = "metric-level comparison policy not qualified; cross-unit/client-server/OTLP-Prometheus aggregation is unavailable"
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
		return result, fmt.Errorf("METRIC_RESULT_BUDGET: result exceeds 64 MiB")
	}
	return result, nil
}
func readLegacyCore(result models.AnalysisResult) (Core, error) {
	var core Core
	if result.Type != ResultType || result.Metadata.SchemaVersion != LegacyPayloadVersion {
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
func ValidateLegacy(result models.AnalysisResult) error {
	core, e := readLegacyCore(result)
	if e != nil {
		return e
	}
	expected, e := BuildLegacy(context.Background(), core.Model, core.Selection, "")
	if e != nil {
		return e
	}
	for _, pair := range [][2]any{{result.Summary, expected.Summary}, {result.Tables, expected.Tables}, {result.Series, expected.Series}, {result.Charts, expected.Charts}, {result.Metadata.Extra["measurement"], expected.Metadata.Extra["measurement"]}, {result.Metadata.Extra["status_counts"], expected.Metadata.Extra["status_counts"]}, {result.Metadata.Extra["window_policy"], expected.Metadata.Extra["window_policy"]}, {result.Metadata.Extra["identity_basis"], expected.Metadata.Extra["identity_basis"]}, {result.Metadata.Extra["clock_ids"], expected.Metadata.Extra["clock_ids"]}, {result.Metadata.Extra["evidence_ids"], expected.Metadata.Extra["evidence_ids"]}, {result.Metadata.Extra["comparison_scope"], expected.Metadata.Extra["comparison_scope"]}} {
		a, _ := json.Marshal(pair[0])
		b, _ := json.Marshal(pair[1])
		var left, right any
		wirejson.DecodeWithLimits(a, &left, wirejson.ResultLimits(64<<20, 128))
		wirejson.DecodeWithLimits(b, &right, wirejson.ResultLimits(64<<20, 128))
		if !reflect.DeepEqual(left, right) {
			return fmt.Errorf("METRIC_RESULT_MODEL_MISMATCH")
		}
	}
	return nil
}
func LegacyDescriptor() resultcontract.Descriptor {
	scalar := resultcontract.ValueSchema{Nullable: false, Encodings: []string{"exact_integer_string"}, Kinds: []string{"record_count"}, Units: []string{"count"}, Aggregations: []string{"count"}}
	scalars := map[string]resultcontract.ValueSchema{}
	for _, key := range []string{"series_count", "point_count", "derived_interval_count", "untimed_point_count", "deduplicated_point_count"} {
		scalars["/summary/"+key] = scalar
	}
	return resultcontract.Descriptor{Panel: &resultcontract.Panel{ID: "metric_history", Version: "1.0.0", Actions: []string{"intervals", "export-csv"}}, Type: ResultType, Payload: resultcontract.Range{Min: LegacyPayloadVersion, Before: "0.1.1"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Operations: []string{"display", "save", "compute", "json", "csv", "html", "report-pack"}, Extensions: map[string]resultcontract.Range{"metrics_core": {Min: m.Version, Before: "1.0.1"}, "measurement": {Min: "1.0.0", Before: "1.1.0"}}, Required: map[string][]string{"save": {"metrics_core", "measurement"}, "compute": {"metrics_core", "measurement"}}, Scalars: scalars, Providers: []resultcontract.Provider{{ID: "metrics:points", Version: "1.0.0", Kind: "detail", Path: "/tables/points", Schema: "object-rows-v1"}, {ID: "metrics:timeline", Version: "1.0.0", Kind: "timeline", Path: "/tables/timed_points", Schema: "timeline-v1", Time: "time", Clock: "clock", Evidence: "point_id"}}, ValidatePayload: ValidateLegacy}
}
