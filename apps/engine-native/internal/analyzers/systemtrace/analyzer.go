package systemtrace

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"reflect"
	"strconv"
)

const ResultType = "system_evidence"
const PayloadVersion = "0.1.0"

func Build(ctx context.Context, s m.Set, source string) (models.AnalysisResult, error) {
	if e := s.Validate(ctx); e != nil {
		return models.AnalysisResult{}, e
	}
	intervals, e := Intervals(ctx, s)
	if e != nil {
		return models.AnalysisResult{}, e
	}
	r := models.New(ResultType, "system-evidence.v1")
	r.Metadata.SchemaVersion = PayloadVersion
	r.SourceFiles = []string{source}
	events := []map[string]any{}
	histograms := []map[string]any{}
	timeline := []map[string]any{}
	known, unknown, samples := 0, 0, 0
	eventDefs := map[string]m.EventDefinition{}
	for _, definition := range s.Capture.Events {
		eventDefs[definition.Name] = definition
	}
	defs := map[string]m.HistogramDefinition{}
	for _, d := range s.Capture.Histograms {
		defs[d.Name] = d
	}
	for _, record := range s.Records {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		ref := fmt.Sprintf("line:%d", record.Line)
		if record.Kind == "histogram" {
			d := defs[record.Histogram.Name]
			for i, b := range record.Histogram.Buckets {
				histograms = append(histograms, map[string]any{"event_id": record.ID, "source_ref": ref, "name": record.Histogram.Name, "key": record.Histogram.Key, "bucket": strconv.Itoa(i), "lower_inclusive": b.Lower, "upper_inclusive": b.Upper, "count": b.Count, "unit": d.Unit, "population": d.Population, "aggregation": d.Aggregation, "reset_id": d.ResetID, "from_ns": d.From, "to_ns": d.To})
			}
			continue
		}
		if record.Kind == "sample" {
			samples++
		}
		fields, _ := json.Marshal(record.Fields)
		events = append(events, map[string]any{"event_id": record.ID, "kind": record.Kind, "name": record.Name, "entity_id": record.Entity, "time_ns": record.Time, "end_ns": record.End, "period": record.Period, "period_kind": eventDefs[record.Name].PeriodKind, "period_unit": eventDefs[record.Name].PeriodUnit, "cpu": record.CPU, "context": record.Context, "state": record.State, "stack_depth": strconv.Itoa(len(record.Stack)), "fields": string(fields), "source_ref": ref})
		timeline = append(timeline, map[string]any{"event_id": record.ID, "time": *record.Time, "clock": s.Capture.Clock, "entity_id": record.Entity, "kind": record.Kind, "source_ref": ref})
	}
	for _, row := range intervals {
		if row.Duration != nil {
			known++
		} else {
			unknown++
		}
	}
	r.Summary = map[string]any{"record_count": strconv.Itoa(len(s.Records)), "sample_records": strconv.Itoa(samples), "interval_count": strconv.Itoa(len(intervals)), "qualified_intervals": strconv.Itoa(known), "unavailable_intervals": strconv.Itoa(unknown), "histogram_bucket_count": strconv.Itoa(len(histograms))}
	r.Tables = map[string]any{"events": events, "intervals": intervals, "histograms": histograms, "timeline": timeline}
	r.Series = map[string]any{}
	r.Charts = map[string]any{}
	r.Metadata.Extra["envelope_schema_version"] = "1.0.0"
	r.Metadata.Extra["extensions"] = map[string]string{"system_core": m.Version, "measurement": "1.0.0"}
	r.Metadata.Extra["system_core"] = s
	r.Metadata.Extra["quality"] = s.Capture
	r.Metadata.Extra["identity_basis"] = "declared"
	r.Metadata.Extra["source_sha256"] = []string{s.SourceSHA256}
	r.Metadata.Extra["clock_ids"] = []string{s.Capture.Clock}
	r.Metadata.Extra["limitations"] = []string{"file-only observations; declared producer/clock/entity, not source authentication", "sample count and event-specific period are separate; no frequency-to-time conversion or multiplex correction", "scheduler spans require declared zero loss, clock and entity lifetime; incomplete/ambiguous spans remain null", "off-CPU is not lock contention; on-CPU is endpoint evidence, not a causal request critical path", "histogram counts/bounds are aggregate-only; no event timeline, interval differencing or quantile estimation"}
	measurement := map[string]resultcontract.Measurement{}
	for key := range r.Summary {
		measurement["/summary/"+key] = resultcontract.Measurement{Kind: "record_count", Unit: "count", Scope: "system_capture", Aggregation: "count", Encoding: "exact_integer_string", Availability: "observed"}
	}
	measurement["/tables/intervals"] = resultcontract.Measurement{Columns: map[string]resultcontract.Measurement{"duration_ns": {Kind: "duration", Unit: "nanoseconds", Scope: "declared_entity_interval", Encoding: "exact_integer_string", Aggregation: "none", AvailabilityColumn: "availability", ReasonColumn: "status"}}}
	measurement["/tables/histograms"] = resultcontract.Measurement{Columns: map[string]resultcontract.Measurement{"count": {Kind: "count", Unit: "count", Scope: "original_aggregate_bucket", Encoding: "exact_integer_string", Aggregation: "none", Availability: "observed"}}}
	r.Metadata.Extra["measurement"] = measurement
	body, e := json.Marshal(r)
	if e != nil {
		return r, e
	}
	if len(body) > 64<<20 {
		return r, fmt.Errorf("SYSTEM_RESULT_BUDGET: %d bytes exceeds 67108864; split original; no result truncation", len(body))
	}
	return r, ctx.Err()
}
func ReadCore(r models.AnalysisResult) (m.Set, error) {
	var s m.Set
	if r.Type != ResultType || r.Metadata.SchemaVersion != PayloadVersion {
		return s, fmt.Errorf("SYSTEM_RESULT_VERSION_UNSUPPORTED")
	}
	body, e := json.Marshal(r.Metadata.Extra["system_core"])
	if e != nil {
		return s, e
	}
	return m.Decode(context.Background(), body)
}
func Validate(r models.AnalysisResult) error {
	s, e := ReadCore(r)
	if e != nil {
		return e
	}
	expected, e := Build(context.Background(), s, "")
	if e != nil {
		return e
	}
	pairs := [][2]any{{r.Summary, expected.Summary}, {r.Tables, expected.Tables}, {r.Series, expected.Series}, {r.Charts, expected.Charts}}
	for _, key := range []string{"quality", "identity_basis", "source_sha256", "clock_ids", "limitations", "measurement"} {
		pairs = append(pairs, [2]any{r.Metadata.Extra[key], expected.Metadata.Extra[key]})
	}
	for _, pair := range pairs {
		left, _ := json.Marshal(pair[0])
		right, _ := json.Marshal(pair[1])
		var x, y any
		if wirejson.DecodeWithLimits(left, &x, wirejson.ResultLimits(64<<20, 128)) != nil || wirejson.DecodeWithLimits(right, &y, wirejson.ResultLimits(64<<20, 128)) != nil || !reflect.DeepEqual(x, y) {
			return fmt.Errorf("SYSTEM_RESULT_MODEL_MISMATCH")
		}
	}
	return nil
}
func Descriptor() resultcontract.Descriptor {
	scalars := map[string]resultcontract.ValueSchema{}
	for _, key := range []string{"record_count", "sample_records", "interval_count", "qualified_intervals", "unavailable_intervals", "histogram_bucket_count"} {
		scalars["/summary/"+key] = resultcontract.ValueSchema{Nullable: false, Encodings: []string{"exact_integer_string"}, Kinds: []string{"record_count"}, Units: []string{"count"}, Aggregations: []string{"count"}}
	}
	return resultcontract.Descriptor{Type: ResultType, Payload: resultcontract.Range{Min: PayloadVersion, Before: "0.1.1"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Operations: []string{"display", "save", "compute", "json", "csv", "html", "report-pack"}, Extensions: map[string]resultcontract.Range{"system_core": {Min: m.Version, Before: "1.0.1"}, "measurement": {Min: "1.0.0", Before: "1.1.0"}}, Required: map[string][]string{"save": {"system_core", "measurement"}, "compute": {"system_core", "measurement"}}, Scalars: scalars, Collections: map[string]map[string]resultcontract.ValueSchema{"/tables/intervals": {"duration_ns": {Nullable: true, Encodings: []string{"exact_integer_string"}, Kinds: []string{"duration"}, Units: []string{"nanoseconds"}, Aggregations: []string{"none"}}, "availability": {}, "status": {}}, "/tables/histograms": {"count": {Nullable: false, Encodings: []string{"exact_integer_string"}, Kinds: []string{"count"}, Units: []string{"count"}, Aggregations: []string{"none"}}}}, Providers: []resultcontract.Provider{{ID: "system:events", Version: "1.0.0", Kind: "detail", Path: "/tables/events", Schema: "object-rows-v1"}, {ID: "system:intervals", Version: "1.0.0", Kind: "detail", Path: "/tables/intervals", Schema: "object-rows-v1"}, {ID: "system:histograms", Version: "1.0.0", Kind: "detail", Path: "/tables/histograms", Schema: "object-rows-v1"}, {ID: "system:timeline", Version: "1.0.0", Kind: "timeline", Path: "/tables/timeline", Schema: "timeline-v1", Time: "time", Clock: "clock", Evidence: "event_id"}}, ValidatePayload: Validate}
}
