package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/traceimport"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

const llmTraceFamily = "llm-trace.chunk.v1"

func llmTraceRegistration() (ingestion.ImporterDescriptor, []ingestion.Schema, ingestion.Analyzer, error) {
	var empty ingestion.ImporterDescriptor
	manifest, err := Contracts.ReadFile("contracts/llm-trace-set.manifest.json")
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
			return empty, nil, ingestion.Analyzer{}, fmt.Errorf("LLM trace fixture path invalid")
		}
		embedded := name
		if name != "llm-trace-set.json" && name != "llm-trace-set.schema.json" {
			embedded = "llm-trace-" + name
		}
		body, e := Contracts.ReadFile("contracts/" + embedded)
		sum := sha256.Sum256(body)
		if e != nil || hex.EncodeToString(sum[:]) != digest {
			return empty, nil, ingestion.Analyzer{}, fmt.Errorf("LLM trace fixture digest mismatch: %s", name)
		}
	}
	fixture, err := Contracts.ReadFile("contracts/llm-trace-set.json")
	if err != nil {
		return empty, nil, ingestion.Analyzer{}, err
	}
	definition, err := Contracts.ReadFile("contracts/llm-trace-set.schema.json")
	if err != nil {
		return empty, nil, ingestion.Analyzer{}, err
	}
	sum := sha256.Sum256(definition)
	if hex.EncodeToString(sum[:]) != bundle.Schema {
		return empty, nil, ingestion.Analyzer{}, fmt.Errorf("LLM trace schema digest mismatch")
	}
	inputValidate := func(raw json.RawMessage) error { _, e := m.Decode(context.Background(), raw); return e }
	selectionValidate := func(raw json.RawMessage) error {
		var options map[string]json.RawMessage
		if e := m.Strict(raw, &options, 1024); e != nil {
			return e
		}
		if options == nil || len(options) != 0 {
			return fmt.Errorf("body-free LLM trace model import accepts only {} options")
		}
		return nil
	}
	schemas := []ingestion.Schema{
		ingestion.NewSchema("llm-trace.input", m.Version, definition, inputValidate),
		ingestion.NewSchema("llm-trace.options", m.Version, json.RawMessage(`{"type":"object","additionalProperties":false}`), selectionValidate),
		ingestion.NewSchema("llm-trace.chunk", m.Version, json.RawMessage(`{"type":"object","additionalProperties":false,"required":["offset","data"],"properties":{"offset":{"type":"integer","minimum":0},"data":{"type":"string","minLength":4,"maxLength":349528}}}`), func(raw json.RawMessage) error { _, e := decodeProfileChunk(raw); return e }),
		ingestion.NewSchema("llm-trace.result", a.LLMPayloadVersion, json.RawMessage(`{"type":"object","required":["type","summary","series","tables","charts","metadata","source_files","created_at"]}`), func(raw json.RawMessage) error {
			result, e := resultcontract.Decode(raw)
			if e != nil {
				return e
			}
			return a.ValidateLLM(result)
		}),
	}
	set, err := m.Decode(context.Background(), fixture)
	if err != nil {
		return empty, nil, ingestion.Analyzer{}, err
	}
	selection := map[string]any{}
	selectionJSON, _ := json.Marshal(selection)
	chunkJSON, _ := json.Marshal(profileChunk{0, fixture})
	sample, err := a.BuildLLM(context.Background(), set, "synthetic")
	if err != nil {
		return empty, nil, ingestion.Analyzer{}, err
	}
	resultJSON, _ := json.Marshal(sample)
	for i, value := range []json.RawMessage{fixture, selectionJSON, chunkJSON, resultJSON} {
		schemas[i].Role = []string{"input", "option", "record", "result"}[i]
		schemas[i].Family = "trace_import"
		schemas[i].Conformance = []ingestion.SchemaCase{{Value: value, Valid: true}, {Value: json.RawMessage(`{}`), Valid: false}}
	}
	schemas[1].Conformance = []ingestion.SchemaCase{{Value: selectionJSON, Valid: true}, {Value: json.RawMessage(`{"unknown":true}`), Valid: false}}
	schemas[2].Family = llmTraceFamily
	schemas[3].ResultType = a.ResultType
	schemas[3].PayloadVersion = a.LLMPayloadVersion
	manifestSum := sha256.Sum256(manifest)
	d := ingestion.ImporterDescriptor{ID: "trace.llm-json", Aliases: []string{"llm-trace"}, Family: "trace_import", Extensions: []string{".json"}, Producer: "internal LLMTrace transport", AdapterVersion: m.Version, InputSchema: schemas[0].Ref, OptionSchema: schemas[1].Ref, RecordSchema: schemas[2].Ref, ResultSchema: schemas[3].Ref, RecordFamily: llmTraceFamily, NormalizerInput: llmTraceFamily, NormalizerOutput: llmTraceFamily, AnalyzerID: "llm-trace.exact", MeasurementKinds: []string{"record_count"}, Capabilities: []string{"summary", "llm_spans", "logical_requests", "attempts", "timeline"}, SupportStatus: "limited", Limitations: []string{"bounded file-only LLMTrace transport; explicit structural OTLP normalization first; raw bodies are never canonical input", "OTLP SDK and explicit semantic subsets only; Q01 Langfuse/LangSmith JSON stays gated", "parents/links, client/server/attempts and lifecycle populations stay separate; no causal critical path or cross-attempt token sum"}, FixtureManifest: "diagnostics.llm-trace.v1", FixtureHash: hex.EncodeToString(manifestSum[:]), ConformanceVersion: m.Version, ProbeBudget: ingestion.MaxProbeBytes}
	read := func(ctx context.Context, source ingestion.Source) ([]byte, error) {
		if source.Info().Size > m.MaxBytes {
			return nil, ingestion.BudgetProblem("llm_trace_model_bytes", "llm_trace_model_bytes", source.Info().Size, m.MaxBytes)
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
			return nil, fmt.Errorf("LLMTRACE_BUDGET_EXCEEDED: model bytes")
		}
		return body, ctx.Err()
	}
	d.Detector = func(ctx context.Context, p ingestion.Probe) (ingestion.Candidate, error) {
		if e := ctx.Err(); e != nil {
			return ingestion.Candidate{}, e
		}
		if inputValidate(p.Head) == nil {
			return ingestion.Candidate{Grade: ingestion.Verified, Confidence: 1, Reason: "validated LLMTrace model"}, nil
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
			return ingestion.Problem("IMPORT_FORMAT_MISMATCH", "invalid bounded LLM trace model: "+e.Error())
		}
		return nil
	}
	d.ParserFactory = func(options json.RawMessage) ingestion.Parser {
		return ingestion.ParserFunc(func(ctx context.Context, source ingestion.Source, sink ingestion.RecordSink) (ingestion.ParseReport, error) {
			body, e := read(ctx, source)
			if e != nil {
				return ingestion.ParseReport{}, e
			}
			if _, e = m.Decode(ctx, body); e != nil {
				return ingestion.ParseReport{}, e
			}
			if e = selectionValidate(options); e != nil {
				return ingestion.ParseReport{}, e
			}
			for offset, sequence := 0, uint64(1); offset < len(body); sequence++ {
				end := min(offset+profileChunkBytes, len(body))
				payload, _ := json.Marshal(profileChunk{int64(offset), body[offset:end]})
				if e = sink.Write(ctx, ingestion.Batch{Family: llmTraceFamily, Records: []ingestion.Record{{Sequence: sequence, Position: ingestion.Position{Byte: int64(offset), Event: int64(sequence)}, Payload: payload}}}); e != nil {
					return ingestion.ParseReport{}, e
				}
				offset = end
			}
			return ingestion.ParseReport{Complete: true}, nil
		})
	}
	d.Normalize = func(ctx context.Context, r ingestion.Record) (ingestion.Record, error) { return r, ctx.Err() }
	analyzer := ingestion.Analyzer{ID: d.AnalyzerID, Family: llmTraceFamily, ResultType: a.ResultType, Analyze: func(ctx context.Context, records ingestion.RecordSource, info ingestion.SourceInfo, raw json.RawMessage, _ *diagnostics.ParserDiagnostics) (models.AnalysisResult, error) {
		var body []byte
		err := records.ForEach(ctx, func(record ingestion.Record) error {
			chunk, e := decodeProfileChunk(record.Payload)
			if e != nil {
				return e
			}
			if len(body)+len(chunk.Data) > m.MaxBytes {
				return fmt.Errorf("LLMTRACE_MODEL_BUDGET")
			}
			body, e = appendProfileChunk(body, chunk)
			return e
		})
		if err != nil {
			return models.AnalysisResult{}, err
		}
		set, e := m.Decode(ctx, body)
		if e != nil {
			return models.AnalysisResult{}, e
		}
		if e = selectionValidate(raw); e != nil {
			return models.AnalysisResult{}, e
		}
		return a.BuildLLM(ctx, set, info.Name)
	}}
	return d, schemas, analyzer, nil
}
