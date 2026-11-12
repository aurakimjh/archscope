package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

const metricHistoryFamily = "metric-history.chunk.v1"

func metricHistoryRegistration() (ingestion.ImporterDescriptor, []ingestion.Schema, ingestion.Analyzer, error) {
	var empty ingestion.ImporterDescriptor
	manifest, err := Contracts.ReadFile("contracts/metric-history-set.manifest.json")
	if err != nil {
		return empty, nil, ingestion.Analyzer{}, err
	}
	var bundle struct {
		Files []struct {
			File   string `json:"file"`
			SHA256 string `json:"sha256"`
		}
		Schema string `json:"schema_sha256"`
	}
	if err = json.Unmarshal(manifest, &bundle); err != nil {
		return empty, nil, ingestion.Analyzer{}, err
	}
	for _, entry := range bundle.Files {
		name, digest := entry.File, entry.SHA256
		if filepath.Base(name) != name {
			return empty, nil, ingestion.Analyzer{}, fmt.Errorf("metric fixture path invalid")
		}
		embedded := name
		if name != "metric-history-set.json" && name != "metric-history-set.schema.json" {
			embedded = "metric-history-" + name
		}
		body, e := Contracts.ReadFile("contracts/" + embedded)
		sum := sha256.Sum256(body)
		if e != nil || hex.EncodeToString(sum[:]) != digest {
			return empty, nil, ingestion.Analyzer{}, fmt.Errorf("metric fixture digest mismatch: %s", name)
		}
	}
	fixture, err := Contracts.ReadFile("contracts/metric-history-set.json")
	if err != nil {
		return empty, nil, ingestion.Analyzer{}, err
	}
	definition, err := Contracts.ReadFile("contracts/metric-history-set.schema.json")
	if err != nil {
		return empty, nil, ingestion.Analyzer{}, err
	}
	sum := sha256.Sum256(definition)
	if hex.EncodeToString(sum[:]) != bundle.Schema {
		return empty, nil, ingestion.Analyzer{}, fmt.Errorf("metric schema digest mismatch")
	}
	inputValidate := func(raw json.RawMessage) error { _, e := m.Decode(raw); return e }
	selectionValidate := func(raw json.RawMessage) error {
		var selection m.Selection
		if e := m.Strict(raw, &selection); e != nil {
			return e
		}
		if len(selection.SeriesIDs) == 0 || len(selection.SeriesIDs) > m.MaxSeries {
			return fmt.Errorf("explicit metric series_ids required")
		}
		return nil
	}
	schemas := []ingestion.Schema{
		ingestion.NewSchema("metric-history.input", m.Version, definition, inputValidate),
		ingestion.NewSchema("metric-history.selection", m.Version, json.RawMessage(`{"type":"object","additionalProperties":false,"required":["series_ids"],"properties":{"series_ids":{"type":"array","minItems":1,"maxItems":10000,"uniqueItems":true,"items":{"type":"string"}},"from_ns":{"type":"string"},"to_ns":{"type":"string"}}}`), selectionValidate),
		ingestion.NewSchema("metric-history.chunk", m.Version, json.RawMessage(`{"type":"object","additionalProperties":false,"required":["offset","data"],"properties":{"offset":{"type":"integer","minimum":0},"data":{"type":"string","minLength":4,"maxLength":349528}}}`), func(raw json.RawMessage) error { _, e := decodeProfileChunk(raw); return e }),
		ingestion.NewSchema("metric-history.result", a.PayloadVersion, json.RawMessage(`{"type":"object","required":["type","summary","series","tables","charts","metadata","source_files","created_at"]}`), func(raw json.RawMessage) error {
			result, e := resultcontract.Decode(raw)
			if e != nil {
				return e
			}
			return a.Validate(result)
		}),
	}
	set, err := m.Decode(fixture)
	if err != nil {
		return empty, nil, ingestion.Analyzer{}, err
	}
	selection := m.Selection{SeriesIDs: []string{set.Series[0].ID}}
	selectionJSON, _ := json.Marshal(selection)
	chunkJSON, _ := json.Marshal(profileChunk{0, fixture})
	sample, err := a.Build(context.Background(), set, selection, "synthetic")
	if err != nil {
		return empty, nil, ingestion.Analyzer{}, err
	}
	resultJSON, _ := json.Marshal(sample)
	for i, value := range []json.RawMessage{fixture, selectionJSON, chunkJSON, resultJSON} {
		schemas[i].Role = []string{"input", "option", "record", "result"}[i]
		schemas[i].Family = "metric_history"
		schemas[i].Conformance = []ingestion.SchemaCase{{Value: value, Valid: true}, {Value: json.RawMessage(`{}`), Valid: false}}
	}
	schemas[2].Family = metricHistoryFamily
	schemas[3].ResultType = a.ResultType
	schemas[3].PayloadVersion = a.PayloadVersion
	manifestSum := sha256.Sum256(manifest)
	d := ingestion.ImporterDescriptor{ID: "metrics.history-json", Aliases: []string{"metric-history"}, Family: "metric_history", Extensions: []string{".json"}, Producer: "internal MetricHistory transport", AdapterVersion: m.Normalizer, InputSchema: schemas[0].Ref, OptionSchema: schemas[1].Ref, RecordSchema: schemas[2].Ref, ResultSchema: schemas[3].Ref, RecordFamily: metricHistoryFamily, NormalizerInput: metricHistoryFamily, NormalizerOutput: metricHistoryFamily, AnalyzerID: "metric-history.exact", MeasurementKinds: []string{"record_count"}, Capabilities: []string{"summary", "intervals", "selection", "timeline"}, SupportStatus: "limited", Limitations: []string{"bounded canonical JSON transport; original OTLP/snapshot/range sources require explicit normalization", "SDK Python 1.38.0 fixture slice only; declared identity and no GenAI alias qualification", "no quantile averaging, cross-series sum or OTLP/Prometheus dedup without provenance"}, FixtureManifest: "diagnostics.metric-history.v1", FixtureHash: hex.EncodeToString(manifestSum[:]), ConformanceVersion: m.Version, ProbeBudget: ingestion.MaxProbeBytes}
	read := func(ctx context.Context, source ingestion.Source) ([]byte, error) {
		if source.Info().Size > m.MaxBytes {
			return nil, ingestion.BudgetProblem("metric_model_bytes", "metric_model_bytes", source.Info().Size, m.MaxBytes)
		}
		f, e := source.Open(ctx)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		body, e := io.ReadAll(io.LimitReader(f, m.MaxBytes+1))
		if e != nil {
			return nil, e
		}
		if len(body) > m.MaxBytes {
			return nil, fmt.Errorf("METRIC_BUDGET_EXCEEDED: model bytes")
		}
		return body, ctx.Err()
	}
	d.Detector = func(ctx context.Context, p ingestion.Probe) (ingestion.Candidate, error) {
		if e := ctx.Err(); e != nil {
			return ingestion.Candidate{}, e
		}
		if inputValidate(p.Head) == nil {
			return ingestion.Candidate{Grade: ingestion.Verified, Confidence: 1, Reason: "validated MetricHistory model"}, nil
		}
		if p.Extension == ".json" {
			return ingestion.Candidate{Grade: ingestion.Hint, Confidence: 0.1, Reason: "JSON filename only; model validation required"}, nil
		}
		return ingestion.Candidate{}, nil
	}
	d.ValidateInput = func(ctx context.Context, source ingestion.Source) error {
		body, e := read(ctx, source)
		if e != nil {
			return e
		}
		if e = inputValidate(body); e != nil {
			return metricModelProblem(e)
		}
		return nil
	}
	d.ParserFactory = func(options json.RawMessage) ingestion.Parser {
		return ingestion.ParserFunc(func(ctx context.Context, source ingestion.Source, sink ingestion.RecordSink) (ingestion.ParseReport, error) {
			body, e := read(ctx, source)
			if e != nil {
				return ingestion.ParseReport{}, e
			}
			set, e := m.Decode(body)
			if e != nil {
				return ingestion.ParseReport{}, e
			}
			var selection m.Selection
			if e = m.Strict(options, &selection); e != nil {
				return ingestion.ParseReport{}, e
			}
			if e = set.ValidateSelection(selection); e != nil {
				return ingestion.ParseReport{}, e
			}
			for offset, sequence := 0, uint64(1); offset < len(body); sequence++ {
				end := min(offset+profileChunkBytes, len(body))
				payload, _ := json.Marshal(profileChunk{int64(offset), body[offset:end]})
				if e = sink.Write(ctx, ingestion.Batch{Family: metricHistoryFamily, Records: []ingestion.Record{{Sequence: sequence, Position: ingestion.Position{Byte: int64(offset), Event: int64(sequence)}, Payload: payload}}}); e != nil {
					return ingestion.ParseReport{}, e
				}
				offset = end
			}
			return ingestion.ParseReport{Complete: true}, nil
		})
	}
	d.Normalize = func(ctx context.Context, r ingestion.Record) (ingestion.Record, error) { return r, ctx.Err() }
	analyzer := ingestion.Analyzer{ID: d.AnalyzerID, Family: metricHistoryFamily, ResultType: a.ResultType, Analyze: func(ctx context.Context, records ingestion.RecordSource, info ingestion.SourceInfo, raw json.RawMessage, _ *diagnostics.ParserDiagnostics) (models.AnalysisResult, error) {
		var body []byte
		err := records.ForEach(ctx, func(record ingestion.Record) error {
			chunk, e := decodeProfileChunk(record.Payload)
			if e != nil {
				return e
			}
			body, e = appendProfileChunk(body, chunk)
			return e
		})
		if err != nil {
			return models.AnalysisResult{}, err
		}
		set, e := m.Decode(body)
		if e != nil {
			return models.AnalysisResult{}, e
		}
		var selection m.Selection
		if e = m.Strict(raw, &selection); e != nil {
			return models.AnalysisResult{}, e
		}
		return a.Build(ctx, set, selection, info.Name)
	}}
	return d, schemas, analyzer, nil
}

func metricModelProblem(err error) error {
	var limit *m.BudgetError
	if errors.As(err, &limit) {
		return ingestion.BudgetProblem("metric_"+limit.Resource, limit.Resource, limit.Actual, limit.Limit)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	message := err.Error()
	if len(message) > 256 {
		message = message[:256]
	}
	message = strings.ToValidUTF8(message, "")
	return ingestion.Problem("IMPORT_FORMAT_MISMATCH", "invalid MetricHistory model: "+message)
}
