package gputrace

import (
	"context"
	"encoding/json"
	"fmt"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"reflect"
	"strconv"
)

const ResultType = "trace_import"
const PayloadVersion = "0.4.0"

func Build(ctx context.Context, s g.Set, source string) (models.AnalysisResult, error) {
	out := models.New(ResultType, "gpu-execution.v1")
	if e := s.Validate(ctx); e != nil {
		return out, e
	}
	p, e := Project(ctx, s)
	if e != nil {
		return out, e
	}
	out.Metadata.SchemaVersion = PayloadVersion
	out.SourceFiles = []string{source}
	out.Summary = map[string]any{"source_count": strconv.Itoa(len(s.Sources)), "track_count": strconv.Itoa(len(s.Tracks)), "event_count": strconv.Itoa(len(s.Events)), "gpu_device_count": strconv.Itoa(len(p.Busy)), "relation_count": strconv.Itoa(len(p.Relations)), "summary_rows": strconv.Itoa(len(s.Summaries))}
	type index struct {
		ID       string  `json:"event_id"`
		Track    string  `json:"track_id"`
		Duration string  `json:"duration_ns"`
		Self     *string `json:"observed_self_ns"`
		Status   string  `json:"self_status"`
	}
	events := []index{}
	for _, v := range p.Events {
		events = append(events, index{v.ID, v.Track, v.Duration, v.Self, v.SelfStatus})
	}
	out.Tables = map[string]any{"sources": s.Sources, "tracks": s.Tracks, "events": events, "gpu_busy": p.Busy, "relations": p.Relations, "reported_summaries": s.Summaries}
	out.Metadata.Extra["gpu_execution_core"] = s
	out.Metadata.Extra["extensions"] = map[string]string{"gpu_execution_core": g.Version, "measurement": "1.0.0"}
	out.Metadata.Extra["envelope_schema_version"] = "1.0.0"
	out.Metadata.Extra["identity_basis"] = "declared"
	out.Metadata.Extra["limitations"] = s.Warnings
	measurements := map[string]resultcontract.Measurement{}
	for key := range out.Summary {
		measurements["/summary/"+key] = resultcontract.Measurement{Kind: "record_count", Unit: "count", Scope: "gpu_execution", Encoding: "exact_integer_string", Aggregation: "count", Availability: "observed"}
	}
	out.Metadata.Extra["measurement"] = measurements
	raw, e := json.Marshal(out)
	if e != nil {
		return out, e
	}
	if len(raw) > 64<<20 {
		return out, fmt.Errorf("GPU_TRACE_RESULT_BYTES_BUDGET: select a smaller source window")
	}
	return out, ctx.Err()
}
func ReadCore(ctx context.Context, r models.AnalysisResult) (g.Set, error) {
	if r.Type != ResultType || r.Metadata.SchemaVersion != PayloadVersion {
		return g.Set{}, fmt.Errorf("GPU_TRACE_RESULT_VERSION")
	}
	b, e := json.Marshal(r.Metadata.Extra["gpu_execution_core"])
	if e != nil {
		return g.Set{}, e
	}
	return g.Decode(ctx, b)
}
func Validate(r models.AnalysisResult) error {
	s, e := ReadCore(context.Background(), r)
	if e != nil {
		return e
	}
	expected, e := Build(context.Background(), s, "")
	if e != nil {
		return e
	}
	for _, pair := range [][2]any{{r.Summary, expected.Summary}, {r.Tables, expected.Tables}, {r.Series, expected.Series}, {r.Charts, expected.Charts}, {r.Metadata.Extra["measurement"], expected.Metadata.Extra["measurement"]}, {r.Metadata.Extra["limitations"], expected.Metadata.Extra["limitations"]}, {r.Metadata.Extra["identity_basis"], expected.Metadata.Extra["identity_basis"]}} {
		a, _ := json.Marshal(pair[0])
		b, _ := json.Marshal(pair[1])
		var x, y any
		if wirejson.Decode(a, &x) != nil || wirejson.Decode(b, &y) != nil || !reflect.DeepEqual(x, y) {
			return fmt.Errorf("GPU_TRACE_RESULT_CORE_MISMATCH")
		}
	}
	return nil
}
func Descriptor() resultcontract.Descriptor {
	scalars := map[string]resultcontract.ValueSchema{}
	for _, k := range []string{"source_count", "track_count", "event_count", "gpu_device_count", "relation_count", "summary_rows"} {
		scalars["/summary/"+k] = resultcontract.ValueSchema{Kinds: []string{"record_count"}, Units: []string{"count"}, Encodings: []string{"exact_integer_string"}, Aggregations: []string{"count"}}
	}
	providers := []resultcontract.Provider{}
	for _, key := range []string{"sources", "tracks", "events", "gpu_busy", "relations", "reported_summaries"} {
		providers = append(providers, resultcontract.Provider{ID: "gpu:" + key, Version: "1.0.0", Kind: "detail", Path: "/tables/" + key, Schema: "object-rows-v1"})
	}
	return resultcontract.Descriptor{Panel: &resultcontract.Panel{ID: "gpu_execution", Version: "1.0.0", Actions: []string{"events", "busy", "relations", "svg"}}, Type: ResultType, Payload: resultcontract.Range{Min: PayloadVersion, Before: "0.4.1"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Operations: []string{"display", "save", "compute", "json", "csv", "html", "report-pack"}, Extensions: map[string]resultcontract.Range{"gpu_execution_core": {Min: g.Version, Before: "1.0.1"}, "measurement": {Min: "1.0.0", Before: "1.1.0"}}, Required: map[string][]string{"save": {"gpu_execution_core", "measurement"}, "compute": {"gpu_execution_core", "measurement"}}, Scalars: scalars, Providers: providers, ValidatePayload: Validate}
}
