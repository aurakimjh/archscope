package execution

import (
	"context"
	"encoding/json"
	"fmt"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"reflect"
	"strconv"
)

const ResultType = "execution_evidence"
const PayloadVersion = "0.1.0"
const DistributionPayloadVersion = "0.2.0"

func Build(ctx context.Context, s e.Set, source string) (models.AnalysisResult, error) {
	return build(ctx, s, source, s.Version == e.DistributionVersion)
}

// BuildCurrent publishes the latest projection for newly imported models.
func BuildCurrent(ctx context.Context, s e.Set, source string) (models.AnalysisResult, error) {
	return build(ctx, s, source, true)
}
func build(ctx context.Context, s e.Set, source string, current bool) (models.AnalysisResult, error) {
	out := models.New(ResultType, "execution-evidence.v1")
	if err := s.Validate(ctx); err != nil {
		return out, err
	}
	out.Metadata.SchemaVersion = PayloadVersion
	if current {
		out.Metadata.SchemaVersion = DistributionPayloadVersion
	}
	out.SourceFiles = []string{source}
	frames := []e.Frame{}
	crashes := []e.Crash{}
	contexts := []e.Context{}
	if s.Crash != nil {
		frames = s.Crash.Frames
		crash := *s.Crash
		crash.Frames = []e.Frame{}
		crashes = append(crashes, crash)
	}
	if s.Context != nil {
		contexts = append(contexts, *s.Context)
	}
	out.Summary = map[string]any{"source_count": "1", "metric_count": strconv.Itoa(len(s.Metrics)), "sample_records": strconv.Itoa(len(s.Samples)), "crash_frames": strconv.Itoa(len(frames)), "ignored_metrics": strconv.Itoa(s.IgnoredMetrics)}
	out.Tables = map[string]any{"sources": []e.Source{s.Source}, "contexts": contexts, "metrics": s.Metrics, "samples": s.Samples, "crash": crashes, "crash_frames": frames}
	timeline := []map[string]any{}
	for _, sample := range s.Samples {
		if t := m.Timestamp(sample.Start); t != nil {
			timeline = append(timeline, map[string]any{"time": *t, "clock": "unix", "sample_id": sample.ID, "group_sha256": sample.GroupSHA, "latency_seconds": sample.Latency, "success": sample.Success})
		}
	}
	out.Tables["timeline"] = timeline
	if current {
		out.Tables["distributions"] = e.DistributionRows(s)
		total := 0
		for _, d := range s.Distributions {
			total += d.Count
		}
		out.Summary["aggregated_samples"] = strconv.Itoa(total)
	}
	out.Metadata.Extra["execution_core"] = s
	out.Metadata.Extra["extensions"] = map[string]string{"execution_core": s.Version, "measurement": "1.0.0"}
	out.Metadata.Extra["envelope_schema_version"] = "1.0.0"
	out.Metadata.Extra["identity_basis"] = "declared"
	out.Metadata.Extra["clock_ids"] = []string{"unix"}
	out.Metadata.Extra["limitations"] = s.Limitations
	measurements := map[string]resultcontract.Measurement{}
	for key := range out.Summary {
		measurements["/summary/"+key] = resultcontract.Measurement{Kind: "record_count", Unit: "count", Scope: "execution_evidence", Encoding: "exact_integer_string", Aggregation: "count", Availability: "observed"}
	}
	out.Metadata.Extra["measurement"] = measurements
	raw, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	if len(raw) > 64<<20 {
		return out, fmt.Errorf("EXECUTION_RESULT_BYTES_BUDGET: split original export")
	}
	return out, ctx.Err()
}
func ReadCore(ctx context.Context, result models.AnalysisResult) (e.Set, error) {
	if result.Type != ResultType || !e.In(result.Metadata.SchemaVersion, PayloadVersion, DistributionPayloadVersion) {
		return e.Set{}, fmt.Errorf("EXECUTION_RESULT_VERSION_UNSUPPORTED")
	}
	raw, err := json.Marshal(result.Metadata.Extra["execution_core"])
	if err != nil {
		return e.Set{}, err
	}
	set, err := e.Decode(ctx, raw)
	if err == nil && result.Metadata.SchemaVersion == PayloadVersion && set.Version == e.DistributionVersion {
		err = fmt.Errorf("EXECUTION_RESULT_CORE_VERSION_MISMATCH")
	}
	if err == nil {
		extRaw, _ := json.Marshal(result.Metadata.Extra["extensions"])
		var extensions map[string]string
		if json.Unmarshal(extRaw, &extensions) != nil || extensions["execution_core"] != set.Version {
			err = fmt.Errorf("EXECUTION_RESULT_CORE_VERSION_MISMATCH")
		}
	}
	return set, err
}
func Validate(result models.AnalysisResult) error {
	set, err := ReadCore(context.Background(), result)
	if err != nil {
		return err
	}
	expected, err := build(context.Background(), set, "", result.Metadata.SchemaVersion == DistributionPayloadVersion)
	if err != nil {
		return err
	}
	for _, pair := range [][2]any{{result.Summary, expected.Summary}, {result.Tables, expected.Tables}, {result.Series, expected.Series}, {result.Charts, expected.Charts}, {result.Metadata.Extra["limitations"], expected.Metadata.Extra["limitations"]}, {result.Metadata.Extra["measurement"], expected.Metadata.Extra["measurement"]}, {result.Metadata.Extra["identity_basis"], expected.Metadata.Extra["identity_basis"]}, {result.Metadata.Extra["clock_ids"], expected.Metadata.Extra["clock_ids"]}} {
		a, _ := json.Marshal(pair[0])
		b, _ := json.Marshal(pair[1])
		var x, y any
		if wirejson.Decode(a, &x) != nil || wirejson.Decode(b, &y) != nil || !reflect.DeepEqual(x, y) {
			return fmt.Errorf("EXECUTION_RESULT_CORE_MISMATCH")
		}
	}
	return nil
}
func Descriptor() resultcontract.Descriptor {
	scalars := map[string]resultcontract.ValueSchema{}
	for _, key := range []string{"source_count", "metric_count", "sample_records", "crash_frames", "ignored_metrics"} {
		scalars["/summary/"+key] = resultcontract.ValueSchema{Kinds: []string{"record_count"}, Units: []string{"count"}, Encodings: []string{"exact_integer_string"}, Aggregations: []string{"count"}}
	}
	providers := []resultcontract.Provider{}
	for _, key := range []string{"sources", "contexts", "metrics", "samples", "crash", "crash_frames"} {
		providers = append(providers, resultcontract.Provider{ID: "execution:" + key, Version: "1.0.0", Kind: "detail", Path: "/tables/" + key, Schema: "object-rows-v1"})
	}
	providers = append(providers, resultcontract.Provider{ID: "execution:timeline", Version: "1.0.0", Kind: "timeline", Path: "/tables/timeline", Schema: "timeline-v1", Time: "time", Clock: "clock", Evidence: "sample_id"})
	return resultcontract.Descriptor{Panel: &resultcontract.Panel{ID: "execution", Version: "1.0.0", Actions: []string{"details", "compare"}}, Type: ResultType, Payload: resultcontract.Range{Min: PayloadVersion, Before: "0.1.1"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Operations: []string{"display", "save", "compute", "json", "csv", "html", "report-pack"}, Extensions: map[string]resultcontract.Range{"execution_core": {Min: e.Version, Before: "1.0.1"}, "measurement": {Min: "1.0.0", Before: "1.1.0"}}, Required: map[string][]string{"save": {"execution_core", "measurement"}, "compute": {"execution_core", "measurement"}}, Scalars: scalars, Providers: providers, ValidatePayload: Validate}
}

// DistributionDescriptor is separate so saved 0.1.0 projections stay exact.
func DistributionDescriptor() resultcontract.Descriptor {
	d := Descriptor()
	d.Payload = resultcontract.Range{Min: DistributionPayloadVersion, Before: "0.2.1"}
	d.Extensions["execution_core"] = resultcontract.Range{Min: e.Version, Before: "1.2.0"}
	d.Scalars["/summary/aggregated_samples"] = resultcontract.ValueSchema{Kinds: []string{"record_count"}, Units: []string{"count"}, Encodings: []string{"exact_integer_string"}, Aggregations: []string{"count"}}
	d.Providers = append(d.Providers, resultcontract.Provider{ID: "execution:distributions", Version: "1.0.0", Kind: "detail", Path: "/tables/distributions", Schema: "object-rows-v1"})
	return d
}
