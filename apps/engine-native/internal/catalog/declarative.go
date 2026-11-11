package catalog

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const metricType = "metrics_snapshot"
const metricPayload = "0.1.1"

type mappedOptions struct {
	TopN int `json:"top_n"`
}

func metricSchemas() []ingestion.Schema {
	definitions := []string{`{"type":"string"}`, `{"type":"object","additionalProperties":false,"properties":{"top_n":{"type":"integer","minimum":0,"maximum":1000}}}`, `{"type":"object","additionalProperties":false,"required":["metric","value","unit","measurement_kind"],"properties":{"metric":{"type":"string"},"value":{"type":"string"},"unit":{"type":"string"},"measurement_kind":{"type":"string"}}}`, `{"type":"object","required":["type","summary","series","tables","charts","metadata","source_files","created_at"]}`}
	roles := []string{"input", "option", "record", "result"}
	versions := []string{"1.0.0", "1.0.0", "1.0.0", metricPayload}
	schemas := []ingestion.Schema{}
	for i, def := range definitions {
		i := i
		validator, e := ingestion.CompileSchema([]byte(def))
		if e != nil {
			panic(e)
		}
		s := ingestion.NewSchema("data-metrics."+roles[i], versions[i], json.RawMessage(def), func(b json.RawMessage) error {
			if e := validator(b); e != nil {
				return e
			}
			switch i {
			case 1:
				var o mappedOptions
				if e := wirejson.Decode(b, &o); e != nil || o.TopN < 0 || o.TopN > 1000 {
					return fmt.Errorf("invalid top_n")
				}
			case 3:
				r, e := resultcontract.Decode(b)
				if e != nil || r.Type != metricType || r.Metadata.SchemaVersion != metricPayload {
					return fmt.Errorf("metric payload mismatch")
				}
			}
			return nil
		})
		s.Role = roles[i]
		s.Family = metricType
		if i == 2 {
			s.Family = "declarative.metric-record.v1"
		}
		if i == 3 {
			s.ResultType = metricType
			s.PayloadVersion = metricPayload
		}
		schemas = append(schemas, s)
	}
	sample := models.New(metricType, "fixture")
	sample.Metadata.SchemaVersion = metricPayload
	b, _ := json.Marshal(sample)
	valid := []json.RawMessage{json.RawMessage(`"fixture"`), json.RawMessage(`{}`), json.RawMessage(`{"metric":"test","value":"1","unit":"count","measurement_kind":"gauge"}`), b}
	for i := range schemas {
		schemas[i].Conformance = []ingestion.SchemaCase{{Value: valid[i], Valid: true}, {Value: json.RawMessage(`null`), Valid: false}}
	}
	return schemas
}
func WithPacks(base *ingestion.Catalog, snapshot packs.Snapshot) (*ingestion.Catalog, error) {
	if len(snapshot.Mappings) == 0 {
		return base, nil
	}
	schemas := metricSchemas()
	ds := []ingestion.ImporterDescriptor{}
	for _, source := range snapshot.Mappings {
		mapping := source.Copy()
		if e := mapping.Validate(); e != nil {
			return nil, e
		}
		pin := &ingestion.PackPin{ID: mapping.Pack.ID, Version: mapping.Pack.Version, SHA256: mapping.Pack.SHA256, Origin: mapping.Pack.Origin, Scope: mapping.Pack.Scope}
		d := ingestion.ImporterDescriptor{Pack: pin, ID: mapping.ID, Aliases: mapping.Aliases, Family: metricType, Extensions: []string{".jsonl", ".ndjson"}, Producer: mapping.Producer, VerifiedVersions: mapping.ProducerVersions, AdapterVersion: packs.Interpreter, InputSchema: schemas[0].Ref, OptionSchema: schemas[1].Ref, RecordSchema: schemas[2].Ref, ResultSchema: schemas[3].Ref, RecordFamily: "declarative.metric-record.v1", NormalizerInput: "declarative.metric-record.v1", NormalizerOutput: "declarative.metric-record.v1", AnalyzerID: "data-metrics.v1", MeasurementKinds: []string{mapping.Kind}, Capabilities: []string{"summary", "table", "json_export"}, SupportStatus: "limited", Limitations: []string{"declarative JSONL snapshots only; no time-series/temporality inference", "literal producer conditions and fixture conformance are not real-producer qualification"}, FixtureManifest: mapping.Pack.ID + "@" + mapping.Pack.Version, FixtureHash: mapping.Pack.SHA256, ConformanceVersion: packs.Interpreter, ProbeBudget: ingestion.MaxProbeBytes}
		d.Detector = func(ctx context.Context, p ingestion.Probe) (ingestion.Candidate, error) {
			if e := ctx.Err(); e != nil {
				return ingestion.Candidate{}, e
			}
			line := bytes.SplitN(p.Head, []byte{'\n'}, 2)[0]
			if _, e := mapping.Evaluate(line); e == nil {
				return ingestion.Candidate{Grade: ingestion.Verified, Confidence: 1, Reason: "bounded literal producer/version and mapped JSON record match", ProducerVersion: mapping.Match[mapping.VersionPath]}, nil
			}
			return ingestion.Candidate{}, nil
		}
		d.ValidateInput = func(ctx context.Context, s ingestion.Source) error { return ctx.Err() }
		d.Normalize = func(ctx context.Context, r ingestion.Record) (ingestion.Record, error) { return r, ctx.Err() }
		d.ParserFactory = func(json.RawMessage) ingestion.Parser {
			return ingestion.ParserFunc(func(ctx context.Context, s ingestion.Source, sink ingestion.RecordSink) (ingestion.ParseReport, error) {
				report := ingestion.ParseReport{Diagnostics: diagnostics.New("declarative.jsonl")}
				f, e := s.Open(ctx)
				if e != nil {
					return report, e
				}
				defer f.Close()
				scanner := bufio.NewScanner(f)
				scanner.Buffer(make([]byte, 4096), 64<<10+1)
				var sequence uint64
				var position int64
				for scanner.Scan() {
					if e = ctx.Err(); e != nil {
						return report, e
					}
					line := append([]byte(nil), scanner.Bytes()...)
					report.Diagnostics.TotalLines++
					r, e := mapping.Evaluate(line)
					if e != nil {
						return report, e
					}
					body, _ := json.Marshal(r)
					sequence++
					if e = sink.Write(ctx, ingestion.Batch{Family: "declarative.metric-record.v1", Records: []ingestion.Record{{Sequence: sequence, Position: ingestion.Position{Line: int64(sequence), Byte: position}, Payload: body}}}); e != nil {
						return report, e
					}
					position += int64(len(line)) + 1
					report.Diagnostics.ParsedRecords++
				}
				if e = scanner.Err(); e != nil {
					return report, ingestion.Problem("IMPORT_INPUT_INVALID", "bounded JSONL read failed")
				}
				if sequence == 0 {
					return report, ingestion.Problem("IMPORT_INPUT_INVALID", "at least one mapped record required")
				}
				report.Complete = true
				return report, nil
			})
		}
		ds = append(ds, d)
	}
	return base.Extend(ds, schemas, []ingestion.Analyzer{mappedAnalyzer()}, snapshot.Choices)
}
func mappedAnalyzer() ingestion.Analyzer {
	a := ingestion.Analyzer{ID: "data-metrics.v1", Family: "declarative.metric-record.v1", ResultType: metricType}
	a.Analyze = func(ctx context.Context, source ingestion.RecordSource, info ingestion.SourceInfo, raw json.RawMessage, diags *diagnostics.ParserDiagnostics) (models.AnalysisResult, error) {
		var opts mappedOptions
		if e := wirejson.Decode(raw, &opts); e != nil {
			return models.AnalysisResult{}, e
		}
		limit := opts.TopN
		if limit == 0 {
			limit = 50
		}
		r := models.New(metricType, "declarative.jsonl")
		r.Metadata.SchemaVersion = metricPayload
		r.SourceFiles = []string{info.Name}
		rows := []map[string]any{}
		names := map[string]bool{}
		total := 0
		unit, kind := "", ""
		e := source.ForEach(ctx, func(record ingestion.Record) error {
			var sample packs.Record
			if e := wirejson.Decode(record.Payload, &sample); e != nil {
				return e
			}
			if total > 0 && (sample.Unit != unit || sample.Kind != kind) {
				return ingestion.Problem("IMPORT_RESULT_INVALID", "mapping emits inconsistent measurement units/kinds")
			}
			unit, kind = sample.Unit, sample.Kind
			total++
			names[sample.Metric] = true
			if len(names) > 10000 {
				return ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "metric name cardinality")
			}
			if len(rows) < limit {
				rows = append(rows, map[string]any{"metric": sample.Metric, "value": sample.Value, "unit": sample.Unit, "measurement_kind": sample.Kind, "source_line": strconv.FormatInt(record.Position.Line, 10)})
			}
			return nil
		})
		if e != nil {
			return r, e
		}
		r.Summary = map[string]any{"total_samples": total, "metric_count": len(names)}
		r.Tables["samples"] = rows
		r.Metadata.Diagnostics = diags
		r.Metadata.Extra["envelope_schema_version"] = "1.0.0"
		r.Metadata.Extra["extensions"] = map[string]string{"measurement": "1.0.0"}
		r.Metadata.Extra["measurement"] = map[string]resultcontract.Measurement{"/summary/total_samples": {Kind: "record_count", Unit: "count", Scope: "imported_samples", Population: "imported_samples", Aggregation: "count", Encoding: "json_number", Availability: "observed"}, "/tables/samples": {Columns: map[string]resultcontract.Measurement{"value": {Kind: kind, Unit: unit, Scope: "snapshot_values", Population: "imported_samples", Aggregation: "snapshot", Encoding: "decimal_string", Availability: "observed"}}}}
		return r, nil
	}
	a.BindSelection = func(r *models.AnalysisResult, info ingestion.SourceInfo, selection ingestion.AnalysisSelection) error {
		r.Metadata.Extra["population_ids"] = []string{"imported_samples"}
		r.Metadata.Extra["populations"] = map[string]resultcontract.PopulationIdentity{"imported_samples": {SourceHashes: []string{info.SHA256}, SelectionHash: selection.OptionsHash, SemanticRevision: selection.ImporterID + "@" + selection.AdapterVersion + "/" + metricType + "@" + metricPayload, Coverage: "complete_records", Comparison: &resultcontract.PopulationComparison{Version: "1.0.0", SelectionHash: selection.OptionsHash, CoverageKind: "complete_records"}}}
		return nil
	}
	return a
}
func MappedMetricDescriptor() resultcontract.Descriptor {
	count := resultcontract.ValueSchema{Encodings: []string{"json_number"}, Kinds: []string{"record_count"}, Units: []string{"count"}, Aggregations: []string{"count"}, RequirePopulation: true}
	value := resultcontract.ValueSchema{Encodings: []string{"decimal_string"}, Kinds: []string{"gauge", "counter"}, Units: []string{"seconds", "milliseconds", "microseconds", "nanoseconds", "bytes", "count", "ratio", "percent"}, Aggregations: []string{"snapshot"}, RequirePopulation: true}
	return resultcontract.Descriptor{Type: metricType, Payload: resultcontract.Range{Min: metricPayload, Before: "0.1.2"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Extensions: map[string]resultcontract.Range{"measurement": {Min: "1.0.0", Before: "1.1.0"}}, Required: map[string][]string{"save": {"measurement"}, "compare": {"measurement"}}, Scalars: map[string]resultcontract.ValueSchema{"/summary/total_samples": count}, Collections: map[string]map[string]resultcontract.ValueSchema{"/tables/samples": {"value": value}}, Providers: []resultcontract.Provider{{ID: "data:metric-samples", Version: "1.0.0", Kind: "detail", Path: "/tables/samples", Schema: "object-rows-v1"}}, ValidatePayload: func(r models.AnalysisResult) error {
		for _, key := range []string{"total_samples", "metric_count"} {
			n, e := resultcontract.ExactNumber(r.Summary[key], "json_number")
			if e != nil || !n.IsInt() || n.Sign() < 0 {
				return fmt.Errorf("invalid %s", key)
			}
		}
		raw, e := json.Marshal(r.Tables["samples"])
		if e != nil {
			return e
		}
		var rows []struct {
			Metric     string `json:"metric"`
			Value      string `json:"value"`
			Unit       string `json:"unit"`
			Kind       string `json:"measurement_kind"`
			SourceLine string `json:"source_line"`
		}
		if e = packs.Strict(raw, &rows); e != nil || rows == nil || len(rows) > 1000 {
			return fmt.Errorf("invalid metric sample table")
		}
		units := map[string]bool{"seconds": true, "milliseconds": true, "microseconds": true, "nanoseconds": true, "bytes": true, "count": true, "ratio": true, "percent": true}
		unit, kind := "", ""
		for i, row := range rows {
			n, e := resultcontract.ExactNumber(row.Value, "decimal_string")
			line, lineErr := strconv.ParseUint(row.SourceLine, 10, 64)
			if e != nil || lineErr != nil || line == 0 || strconv.FormatUint(line, 10) != row.SourceLine || row.Metric == "" || len(row.Metric) > 256 || !units[row.Unit] || (row.Kind != "gauge" && row.Kind != "counter") || row.Kind == "counter" && n.Sign() < 0 {
				return fmt.Errorf("invalid metric sample")
			}
			if i > 0 && (unit != row.Unit || kind != row.Kind) {
				return fmt.Errorf("inconsistent metric unit/kind")
			}
			unit, kind = row.Unit, row.Kind
		}
		total, _ := resultcontract.ExactNumber(r.Summary["total_samples"], "json_number")
		if total.Cmp(new(big.Rat).SetInt64(int64(len(rows)))) < 0 {
			return fmt.Errorf("sample count below displayed rows")
		}
		return nil
	}}
}
