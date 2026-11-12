package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strconv"

	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const ResultType = "inference_serving"
const PayloadVersion = "0.1.0"

type Observation struct {
	SignalID       string  `json:"signal_id"`
	PointID        string  `json:"point_id"`
	Time           *string `json:"time_ns"`
	Raw            *string `json:"original_value"`
	Value          *string `json:"normalized_value"`
	Unit           string  `json:"normalized_unit"`
	Delta          *string `json:"interval_delta"`
	Rate           *string `json:"interval_rate"`
	DeltaUnit      string  `json:"delta_unit"`
	RateMeaning    string  `json:"rate_meaning"`
	RateUnit       string  `json:"rate_unit"`
	HistogramCount *string `json:"histogram_count"`
	Status         string  `json:"status"`
	Gap            bool    `json:"gap"`
	Refs           string  `json:"source_refs"`
}
type Request struct {
	EventID      string  `json:"event_id"`
	EntityID     string  `json:"entity_id"`
	ContextID    string  `json:"context_id"`
	RequestID    string  `json:"request_id"`
	Boundary     string  `json:"boundary"`
	Population   string  `json:"population"`
	Outcome      string  `json:"outcome"`
	TTFT         *string `json:"ttft_ns"`
	TTFTStatus   string  `json:"ttft_status"`
	TPOT         *string `json:"tpot_ns"`
	TPOTStatus   string  `json:"tpot_status"`
	OutputTokens *string `json:"output_tokens"`
	SourceRef    string  `json:"source_ref"`
}

func request(v n.Event) Request {
	r := Request{v.ID, v.EntityID, v.ContextID, v.RequestID, v.Boundary, v.Population, v.Outcome, nil, "boundary_unavailable", nil, "boundary_or_token_count_unavailable", v.OutputTokens, v.Ref.SourceID + ":" + v.Ref.Position}
	difference := func(a, b *string) (*big.Int, bool) {
		x, ok := m.Time(a)
		y, yes := m.Time(b)
		if !ok || !yes || y < x {
			return nil, false
		}
		return new(big.Int).SetUint64(y - x), true
	}
	if value, ok := difference(v.RequestStart, v.FirstToken); ok {
		r.TTFT = m.Pointer(value.String())
		r.TTFTStatus = "observed_token_boundaries"
	} else if v.RequestStart != nil && v.FirstToken != nil {
		r.TTFTStatus = "boundary_order_invalid"
	}
	if v.OutputTokens != nil {
		count, _ := m.Uint(*v.OutputTokens)
		if count.Cmp(big.NewInt(2)) < 0 {
			r.TPOTStatus = "at_least_two_output_tokens_required"
		} else if span, ok := difference(v.FirstToken, v.LastToken); ok {
			count.Sub(count, big.NewInt(1))
			r.TPOT = m.Pointer(m.Text(new(big.Rat).SetFrac(span, count)))
			r.TPOTStatus = "observed_token_boundaries"
		} else if v.FirstToken != nil && v.LastToken != nil {
			r.TPOTStatus = "boundary_order_invalid"
		}
	}
	return r
}
func Observations(ctx context.Context, s n.Set) ([]Observation, error) {
	intervals, e := m.Intervals(ctx, s.Metrics)
	if e != nil {
		return nil, e
	}
	series := map[string]m.Series{}
	for _, v := range s.Metrics.Series {
		series[v.ID] = v
	}
	entities := map[string]n.Entity{}
	for _, v := range s.Entities {
		entities[v.ID] = v
	}
	signals := map[string]n.Signal{}
	definitions := map[string]n.SemanticDefinition{}
	for _, v := range s.Signals {
		signals[v.SeriesID] = v
		d, e := n.Definition(v, series[v.SeriesID], entities[v.EntityID])
		if e != nil {
			return nil, e
		}
		definitions[v.ID] = d
	}
	quality := map[string]n.Quality{}
	for _, v := range s.Quality {
		quality[v.PointID] = v
	}
	rows := []Observation{}
	for _, v := range intervals {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		signal, ok := signals[v.SeriesID]
		if !ok {
			continue
		}
		d := definitions[signal.ID]
		refs, _ := json.Marshal(v.Refs)
		row := Observation{SignalID: signal.ID, PointID: v.PointID, Time: v.End, Raw: v.Value, Unit: d.Unit, Status: v.Status, Gap: v.Gap, Refs: string(refs)}
		convert := func(value *string) *string {
			if value == nil {
				return nil
			}
			x, e := m.Number(*value)
			if e != nil {
				return nil
			}
			return m.Pointer(m.Text(new(big.Rat).Mul(x, d.Factor)))
		}
		row.Value = convert(v.Value)
		row.Delta = convert(v.Delta)
		row.Rate = convert(v.Rate)
		row.DeltaUnit = d.Unit
		row.RateMeaning = v.RateMeaning
		if row.Rate != nil {
			row.RateUnit = d.Unit + "/second"
		}
		if v.ObservedHistogram != nil {
			row.HistogramCount = m.Pointer(v.ObservedHistogram.Count)
			row.Delta = v.Delta
			row.Rate = v.Rate
			row.DeltaUnit = "observations"
			if row.Rate != nil {
				row.RateUnit = "observations/second"
			}
		}
		if q, ok := quality[v.PointID]; ok {
			row.Status = q.Status + ": " + q.Raw
		}
		if row.Value != nil {
			x, _ := m.Number(*row.Value)
			if (d.Unit != "celsius" && x.Sign() < 0) || (d.Unit == "ratio" && x.Cmp(big.NewRat(1, 1)) > 0) {
				row.Value = nil
				row.Delta = nil
				row.Rate = nil
				row.RateUnit = ""
				row.Status = "physical_value_out_of_range"
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}
func Build(ctx context.Context, s n.Set, source string) (models.AnalysisResult, error) {
	r := models.New(ResultType, "inference-evidence.v1")
	r.Metadata.SchemaVersion = PayloadVersion
	r.SourceFiles = []string{source}
	if e := s.Validate(ctx); e != nil {
		return r, e
	}
	rows, e := Observations(ctx, s)
	if e != nil {
		return r, e
	}
	requests := []Request{}
	timeline := []map[string]any{}
	for _, v := range s.Events {
		if v.Kind == "request" {
			requests = append(requests, request(v))
		}
		if v.Time != nil {
			timeline = append(timeline, map[string]any{"time": *m.Timestamp(v.Time), "clock": "unix", "event_id": v.ID, "entity_id": v.EntityID, "kind": v.Kind, "source_ref": v.Ref.SourceID + ":" + v.Ref.Position})
		}
	}
	signals := []map[string]any{}
	series := map[string]m.Series{}
	for _, v := range s.Metrics.Series {
		series[v.ID] = v
	}
	for _, v := range s.Signals {
		signals = append(signals, map[string]any{"signal_id": v.ID, "semantic": v.Semantic, "series_id": v.SeriesID, "original_name": series[v.SeriesID].Name, "original_unit": series[v.SeriesID].Unit, "kind": series[v.SeriesID].Kind, "temporality": series[v.SeriesID].Temporality, "entity_id": v.EntityID, "context_id": v.ContextID, "boundary": v.Boundary, "population": v.Population, "statistical_form": v.Form, "conversion": v.Conversion})
	}
	r.Summary = map[string]any{"entity_count": strconv.Itoa(len(s.Entities)), "signal_count": strconv.Itoa(len(s.Signals)), "observed_point_count": strconv.Itoa(len(rows)), "unmapped_series_count": strconv.Itoa(len(s.Metrics.Series) - len(s.Signals)), "event_count": strconv.Itoa(len(s.Events)), "request_count": strconv.Itoa(len(requests))}
	r.Tables = map[string]any{"entities": s.Entities, "links": s.Links, "signals": signals, "observations": rows, "contexts": s.Contexts, "events": s.Events, "requests": requests, "timeline": timeline}
	r.Series = map[string]any{}
	r.Charts = map[string]any{}
	r.Metadata.Extra["envelope_schema_version"] = "1.0.0"
	r.Metadata.Extra["extensions"] = map[string]string{"inference_core": n.Version, "measurement": "1.0.0"}
	r.Metadata.Extra["inference_core"] = s
	r.Metadata.Extra["identity_basis"] = "declared"
	r.Metadata.Extra["clock_ids"] = []string{"unix"}
	r.Metadata.Extra["limitations"] = []string{"File-only declared mappings/producer semantics; no original-publisher or collection authentication.", "Metrics are stored once and referenced by series/point IDs. No device/MIG/pod replication, automatic workload attribution or cross-series sum.", "Utilization is not useful-work efficiency. Device memory, framework allocations and engine KV cache remain different scopes.", "No causal bottleneck verdict, request reconstruction from histograms, phase percentile addition or histogram quantile estimation.", "Client/server/gateway and request outcome/population/context remain separate; warmup exclusions are declarations, not automatic filtering.", "Specific vLLM aliases remain Q03-gated; unbound series remain available in original metric history.", "Event source references are declared coordinates; checksums do not authenticate producer semantics or prove a raw log interpretation."}
	measurements := map[string]resultcontract.Measurement{}
	for key := range r.Summary {
		measurements["/summary/"+key] = resultcontract.Measurement{Kind: "record_count", Unit: "count", Scope: "inference_evidence", Aggregation: "count", Encoding: "exact_integer_string", Availability: "observed"}
	}
	r.Metadata.Extra["measurement"] = measurements
	raw, e := json.Marshal(r)
	if e != nil {
		return r, e
	}
	if len(raw) > 64<<20 {
		return r, fmt.Errorf("INFERENCE_RESULT_BUDGET: %d bytes exceeds 67108864; select fewer original series/events; no silent truncation", len(raw))
	}
	return r, ctx.Err()
}
func ReadCore(r models.AnalysisResult) (n.Set, error) {
	var s n.Set
	if r.Type != ResultType || r.Metadata.SchemaVersion != PayloadVersion {
		return s, fmt.Errorf("INFERENCE_RESULT_VERSION_UNSUPPORTED")
	}
	body, e := json.Marshal(r.Metadata.Extra["inference_core"])
	if e != nil {
		return s, e
	}
	return n.Decode(context.Background(), body)
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
	for _, key := range []string{"identity_basis", "clock_ids", "limitations", "measurement"} {
		pairs = append(pairs, [2]any{r.Metadata.Extra[key], expected.Metadata.Extra[key]})
	}
	for _, pair := range pairs {
		left, _ := json.Marshal(pair[0])
		right, _ := json.Marshal(pair[1])
		var a, b any
		if wirejson.DecodeWithLimits(left, &a, wirejson.ResultLimits(64<<20, 128)) != nil || wirejson.DecodeWithLimits(right, &b, wirejson.ResultLimits(64<<20, 128)) != nil || !reflect.DeepEqual(a, b) {
			return fmt.Errorf("INFERENCE_RESULT_MODEL_MISMATCH")
		}
	}
	return nil
}
func Descriptor() resultcontract.Descriptor {
	scalars := map[string]resultcontract.ValueSchema{}
	for _, key := range []string{"entity_count", "signal_count", "observed_point_count", "unmapped_series_count", "event_count", "request_count"} {
		scalars["/summary/"+key] = resultcontract.ValueSchema{Encodings: []string{"exact_integer_string"}, Kinds: []string{"record_count"}, Units: []string{"count"}, Aggregations: []string{"count"}}
	}
	providers := []resultcontract.Provider{}
	for _, key := range []string{"entities", "signals", "observations", "links", "contexts", "events", "requests"} {
		providers = append(providers, resultcontract.Provider{ID: "inference:" + key, Version: "1.0.0", Kind: "detail", Path: "/tables/" + key, Schema: "object-rows-v1"})
	}
	providers = append(providers, resultcontract.Provider{ID: "inference:timeline", Version: "1.0.0", Kind: "timeline", Path: "/tables/timeline", Schema: "timeline-v1", Time: "time", Clock: "clock", Evidence: "event_id"})
	return resultcontract.Descriptor{Type: ResultType, Payload: resultcontract.Range{Min: PayloadVersion, Before: "0.1.1"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Operations: []string{"display", "save", "compute", "json", "csv", "html", "report-pack"}, Extensions: map[string]resultcontract.Range{"inference_core": {Min: n.Version, Before: "1.0.1"}, "measurement": {Min: "1.0.0", Before: "1.1.0"}}, Required: map[string][]string{"save": {"inference_core", "measurement"}, "compute": {"inference_core", "measurement"}}, Scalars: scalars, Providers: providers, ValidatePayload: Validate}
}

type Inventory struct {
	ResultSHA256   string      `json:"result_sha256,omitempty"`
	SourceSHA256   string      `json:"source_sha256,omitempty"`
	Entities       []n.Entity  `json:"entities"`
	Signals        []n.Signal  `json:"signals"`
	Contexts       []n.Context `json:"contexts"`
	Links          []n.Link    `json:"links"`
	UnmappedSeries int         `json:"unmapped_series"`
	MetricPoints   int         `json:"metric_points"`
	Events         int         `json:"events"`
	Warnings       []string    `json:"warnings"`
}

func Inspect(s n.Set) Inventory {
	return Inventory{Entities: s.Entities, Signals: s.Signals, Contexts: s.Contexts, Links: s.Links, UnmappedSeries: len(s.Metrics.Series) - len(s.Signals), MetricPoints: len(s.Metrics.Points), Events: len(s.Events), Warnings: []string{"Explicit declared scope only; no device/rank attribution by local index or name.", "Open original metric history for exact samples, interval/reset/gap status and original histogram buckets; no inferred percentile.", "Version-specific vLLM aliases and real GPU/serving runtime acceptance remain unqualified."}}
}
