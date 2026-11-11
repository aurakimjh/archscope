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

	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

const profileFamily = "profile-set.document.v1"

func profileRegistration() (ingestion.ImporterDescriptor, []ingestion.Schema, ingestion.Analyzer, error) {
	fixture, e := Contracts.ReadFile("contracts/profile-set.json")
	if e != nil {
		return ingestion.ImporterDescriptor{}, nil, ingestion.Analyzer{}, e
	}
	definition, e := Contracts.ReadFile("contracts/profile-set.schema.json")
	if e != nil {
		return ingestion.ImporterDescriptor{}, nil, ingestion.Analyzer{}, e
	}
	manifest, e := Contracts.ReadFile("contracts/profile-set.manifest.json")
	if e != nil {
		return ingestion.ImporterDescriptor{}, nil, ingestion.Analyzer{}, e
	}
	var bundle struct {
		Files  map[string]string `json:"files"`
		Schema string            `json:"schema_sha256"`
	}
	if e = json.Unmarshal(manifest, &bundle); e != nil {
		return ingestion.ImporterDescriptor{}, nil, ingestion.Analyzer{}, e
	}
	for name, want := range bundle.Files {
		if filepath.Base(name) != name {
			return ingestion.ImporterDescriptor{}, nil, ingestion.Analyzer{}, fmt.Errorf("invalid profile fixture name")
		}
		body, e := Contracts.ReadFile("contracts/" + name)
		if e != nil {
			return ingestion.ImporterDescriptor{}, nil, ingestion.Analyzer{}, e
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != want {
			return ingestion.ImporterDescriptor{}, nil, ingestion.Analyzer{}, fmt.Errorf("profile fixture digest mismatch")
		}
	}
	sum := sha256.Sum256(definition)
	if hex.EncodeToString(sum[:]) != bundle.Schema {
		return ingestion.ImporterDescriptor{}, nil, ingestion.Analyzer{}, fmt.Errorf("profile schema digest mismatch")
	}
	inputValidate := func(raw json.RawMessage) error { _, e := profilemodel.Decode(raw); return e }
	selectionValidate := func(raw json.RawMessage) error {
		selection, e := profilemodel.DecodeSelection(raw)
		if e != nil {
			return e
		}
		if len(selection.ProfileIDs) == 0 || selection.MetricID == "" {
			return fmt.Errorf("explicit profile_ids and metric_id required")
		}
		return nil
	}
	resultValidate := func(raw json.RawMessage) error {
		result, e := resultcontract.Decode(raw)
		if e != nil {
			return e
		}
		return analyzer.ValidateModelResult(result)
	}
	input := ingestion.NewSchema("profile-set.input", profilemodel.Version, definition, inputValidate)
	options := ingestion.NewSchema("profile-set.selection", profilemodel.Version, json.RawMessage(`{"type":"object","required":["profile_ids","metric_id"],"additionalProperties":false,"properties":{"profile_ids":{"type":"array","minItems":1,"items":{"type":"string"}},"metric_id":{"type":"string","minLength":1},"path":{"type":"array","items":{"type":"string"}},"match":{"enum":["prefix","contains"]},"filters":{"type":"array","items":{"type":"object","required":["path","match"],"additionalProperties":false,"properties":{"path":{"type":"array","items":{"type":"string"}},"match":{"enum":["prefix","contains"]}}}}}}`), selectionValidate)
	record := ingestion.NewSchema("profile-set.record", profilemodel.Version, definition, inputValidate)
	result := ingestion.NewSchema("profile-set.result", analyzer.ModelPayloadVersion, json.RawMessage(`{"type":"object","required":["type","summary","series","tables","charts","metadata","source_files","created_at"]}`), resultValidate)
	schemas := []ingestion.Schema{input, options, record, result}
	for i := range schemas {
		schemas[i].Role = []string{"input", "option", "record", "result"}[i]
		schemas[i].Family = "profile_evidence"
		schemas[i].Conformance = []ingestion.SchemaCase{{Value: fixture, Valid: true}, {Value: json.RawMessage(`{}`), Valid: false}}
	}
	selection := profilemodel.Selection{ProfileIDs: []string{"worker-a", "worker-b"}, MetricID: "duration"}
	selectionJSON, _ := json.Marshal(selection)
	schemas[1].Conformance = []ingestion.SchemaCase{{Value: selectionJSON, Valid: true}, {Value: json.RawMessage(`{}`), Valid: false}}
	schemas[2].Family = profileFamily
	schemas[3].ResultType = analyzer.ResultType
	schemas[3].PayloadVersion = analyzer.ModelPayloadVersion
	set, e := profilemodel.Decode(fixture)
	if e != nil {
		return ingestion.ImporterDescriptor{}, nil, ingestion.Analyzer{}, e
	}
	sampleResult, e := analyzer.BuildSet(context.Background(), set, selection, "synthetic")
	if e != nil {
		return ingestion.ImporterDescriptor{}, nil, ingestion.Analyzer{}, e
	}
	resultJSON, _ := json.Marshal(sampleResult)
	schemas[3].Conformance = []ingestion.SchemaCase{{Value: resultJSON, Valid: true}, {Value: json.RawMessage(`{}`), Valid: false}}
	manifestHash := sha256.Sum256(manifest)
	d := ingestion.ImporterDescriptor{ID: "profile.set-json", Aliases: []string{"profile-set"}, Family: "profile_evidence", Extensions: []string{".json"}, Producer: "internal ProfileSet transport", AdapterVersion: profilemodel.Version, InputSchema: schemas[0].Ref, OptionSchema: schemas[1].Ref, RecordSchema: schemas[2].Ref, ResultSchema: schemas[3].Ref, RecordFamily: profileFamily, NormalizerInput: profileFamily, NormalizerOutput: profileFamily, AnalyzerID: "profile-set.exact", MeasurementKinds: []string{"sample_count", "cpu_time", "wall_time", "duration", "bytes", "signed_difference"}, Capabilities: []string{"summary", "exact_tree", "selection", "exact_compare", "pprof_export"}, SupportStatus: "limited", Limitations: []string{"internal versioned JSON transport; not producer conformance", "one normalized document within the configured record budget", "explicit profile and metric selection; original producer corrections remain separate"}, FixtureManifest: "diagnostics.profile-core.v1", FixtureHash: hex.EncodeToString(manifestHash[:]), ConformanceVersion: profilemodel.Version, ProbeBudget: ingestion.MaxProbeBytes}
	read := func(ctx context.Context, source ingestion.Source) ([]byte, error) {
		f, e := source.Open(ctx)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		body, e := io.ReadAll(io.LimitReader(f, 64<<20+1))
		if e != nil {
			return nil, e
		}
		if len(body) > 64<<20 {
			return nil, ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "ProfileSet byte budget exceeded")
		}
		return body, ctx.Err()
	}
	d.Detector = func(ctx context.Context, probe ingestion.Probe) (ingestion.Candidate, error) {
		if e := ctx.Err(); e != nil {
			return ingestion.Candidate{}, e
		}
		if _, e := profilemodel.Decode(probe.Head); e == nil {
			return ingestion.Candidate{Grade: ingestion.Verified, Confidence: 1, Reason: "validated internal ProfileSet structure"}, nil
		}
		if probe.Extension == ".json" {
			return ingestion.Candidate{Grade: ingestion.Hint, Confidence: 0.1, Reason: "JSON filename only; explicit model validation required"}, nil
		}
		return ingestion.Candidate{}, nil
	}
	d.ValidateInput = func(ctx context.Context, source ingestion.Source) error {
		body, e := read(ctx, source)
		if e != nil {
			return e
		}
		if e = inputValidate(body); e != nil {
			return ingestion.Problem("IMPORT_FORMAT_MISMATCH", "invalid ProfileSet model")
		}
		return nil
	}
	d.ParserFactory = func(options json.RawMessage) ingestion.Parser {
		return ingestion.ParserFunc(func(ctx context.Context, source ingestion.Source, sink ingestion.RecordSink) (ingestion.ParseReport, error) {
			body, e := read(ctx, source)
			if e != nil {
				return ingestion.ParseReport{}, e
			}
			set, decodeErr := profilemodel.Decode(body)
			if decodeErr != nil {
				return ingestion.ParseReport{}, decodeErr
			}
			selection, selectionErr := profilemodel.DecodeSelection(options)
			if selectionErr != nil {
				return ingestion.ParseReport{}, ingestion.Problem("IMPORT_OPTIONS_INVALID", "invalid model selection")
			}
			if selectionErr = set.ValidateSelection(selection); selectionErr != nil {
				return ingestion.ParseReport{}, ingestion.Problem("IMPORT_OPTIONS_INVALID", "selection is not valid for this model")
			}
			e = sink.Write(ctx, ingestion.Batch{Family: profileFamily, Records: []ingestion.Record{{Sequence: 1, Position: ingestion.Position{Event: 1}, Payload: body}}})
			return ingestion.ParseReport{Complete: e == nil}, e
		})
	}
	d.Normalize = func(ctx context.Context, r ingestion.Record) (ingestion.Record, error) { return r, ctx.Err() }
	a := ingestion.Analyzer{ID: d.AnalyzerID, Family: profileFamily, ResultType: analyzer.ResultType, Analyze: func(ctx context.Context, records ingestion.RecordSource, info ingestion.SourceInfo, raw json.RawMessage, _ *diagnostics.ParserDiagnostics) (models.AnalysisResult, error) {
		selection, e := profilemodel.DecodeSelection(raw)
		if e != nil {
			return models.AnalysisResult{}, e
		}
		var set profilemodel.ProfileSet
		count := 0
		e = records.ForEach(ctx, func(r ingestion.Record) error {
			count++
			if count != 1 {
				return fmt.Errorf("ProfileSet requires one document")
			}
			var decodeErr error
			set, decodeErr = profilemodel.Decode(r.Payload)
			return decodeErr
		})
		if e != nil {
			return models.AnalysisResult{}, e
		}
		if count != 1 {
			return models.AnalysisResult{}, fmt.Errorf("missing ProfileSet document")
		}
		result, e := analyzer.BuildSet(ctx, set, selection, info.Name)
		var limit *profilemodel.BudgetError
		if errors.As(e, &limit) {
			return result, ingestion.Problem("IMPORT_BUDGET_EXCEEDED", "profile model resource budget exceeded")
		}
		if e == nil {
			ingestion.AttachSourceMetadata(&result, ingestion.SourceMetadata{SourceKind: ingestion.SourceKindRuntimeProfile, SourceFormat: "profile-set-json", Product: "internal ProfileSet transport", File: ingestion.FileIdentity{BaseName: info.Name, Extension: filepath.Ext(info.Name), SizeBytes: info.Size, SanitizedID: info.ID, SanitizedIDVersion: "lease-source-v1"}})
		}
		return result, e
	}}
	return d, schemas, a, nil
}
