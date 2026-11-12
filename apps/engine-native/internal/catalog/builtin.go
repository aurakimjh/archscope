// Package catalog explicitly composes built-in parsers and analyzers. No parser
// init hooks or externally supplied function names participate in activation.
package catalog

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	metricanalyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	profileanalyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/serverlog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/serverlog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

//go:embed contracts/*
var Contracts embed.FS

func Schemas() []ingestion.Schema {
	input := ingestion.NewSchema("server-log.utf8", "1.0.0", json.RawMessage(`{"type":"string","contentEncoding":"utf-8","description":"uncompressed server log; structural validation uses the compiled parser"}`), func(b json.RawMessage) error { var text string; return wirejson.Decode(b, &text) })
	options := ingestion.NewSchema("server-log.options", "1.0.0", json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"top_n":{"type":"integer","minimum":0,"maximum":1000},"max_lines":{"type":"integer","minimum":0},"strict":{"type":"boolean"}}}`), func(b json.RawMessage) error {
		var keys map[string]json.RawMessage
		if e := wirejson.Decode(b, &keys); e != nil {
			return e
		}
		for key, value := range keys {
			if string(value) == "null" {
				return fmt.Errorf("null option is not allowed")
			}
			if key != "top_n" && key != "max_lines" && key != "strict" {
				return fmt.Errorf("unknown option")
			}
		}
		var opts parser.ImportOptions
		if e := wirejson.Decode(b, &opts); e != nil {
			return e
		}
		if opts.TopN < 0 || opts.TopN > 1000 || opts.MaxLines < 0 {
			return fmt.Errorf("invalid option range")
		}
		return nil
	})
	record := ingestion.NewSchema("server-log.record", "1.0.0", json.RawMessage(`{"type":"object","required":["Timestamp","Severity","Product","Message","RawLine"]}`), func(b json.RawMessage) error {
		var row parser.Record
		var fields map[string]json.RawMessage
		if e := wirejson.Decode(b, &fields); e != nil {
			return e
		}
		for _, key := range []string{"Timestamp", "Severity", "Product", "Message", "RawLine"} {
			v, ok := fields[key]
			if !ok || string(v) == "null" {
				return fmt.Errorf("missing required record field")
			}
		}
		if e := wirejson.Decode(b, &row); e != nil {
			return e
		}
		if row.Product == "" || row.Severity == "" || row.RawLine == "" {
			return fmt.Errorf("invalid server record")
		}
		return nil
	})
	result := ingestion.NewSchema("server-log.result", "0.1.0", json.RawMessage(`{"type":"object","required":["type","source_files","created_at","summary","series","tables","charts","metadata"]}`), func(b json.RawMessage) error {
		r, e := resultcontract.Decode(b)
		if e != nil {
			return e
		}
		if r.Type != "server_log" || r.Metadata.SchemaVersion != "0.1.0" {
			return fmt.Errorf("result payload mismatch")
		}
		for _, key := range []string{"total_events", "error_count", "warning_count"} {
			if _, ok := r.Summary[key]; !ok {
				return fmt.Errorf("missing legacy server summary")
			}
		}
		return nil
	})
	schemas := []ingestion.Schema{input, options, record, result}
	for i := range schemas {
		schemas[i].Role = []string{"input", "option", "record", "result"}[i]
		schemas[i].Family = "server_log"
	}
	schemas[2].Family = "server-log.record.v1"
	schemas[3].ResultType = "server_log"
	schemas[3].PayloadVersion = "0.1.0"
	schemas[0].Conformance = []ingestion.SchemaCase{{Value: json.RawMessage(`"synthetic"`), Valid: true}, {Value: json.RawMessage(`{}`), Valid: false}}
	schemas[1].Conformance = []ingestion.SchemaCase{{Value: json.RawMessage(`{}`), Valid: true}, {Value: json.RawMessage(`{"strict":1}`), Valid: false}, {Value: json.RawMessage(`{"max_lines":-1}`), Valid: false}}
	schemas[2].Conformance = []ingestion.SchemaCase{{Value: json.RawMessage(`{"Timestamp": "2026-11-11T00:00:00Z","Severity":"INFO","Product":"tomcat","Message":"synthetic","RawLine":"synthetic"}`), Valid: true}, {Value: json.RawMessage(`{}`), Valid: false}}
	sample := models.New("server_log", "fixture")
	sample.Summary = map[string]any{"total_events": 3, "error_count": 1, "warning_count": 1}
	body, _ := json.Marshal(sample)
	schemas[3].Conformance = []ingestion.SchemaCase{{Value: body, Valid: true}, {Value: json.RawMessage(`{}`), Valid: false}}
	return schemas
}
func ResultRegistry() (*resultcontract.Registry, error) {
	scalar := resultcontract.ValueSchema{Nullable: false, Encodings: []string{"json_number"}, Kinds: []string{"record_count"}, Aggregations: []string{"count"}, Units: []string{"count"}}
	descriptors := []resultcontract.Descriptor{}
	for _, entry := range []struct{ kind, from, before string }{{"profile_evidence", "0.1.0", "0.2.0"}, {"profiler_collapsed", "0.1.0", "0.3.0"}, {"profiler_jennifer", "0.2.0", "0.3.0"}, {"profiler_diff", "0.1.0", "0.2.0"}} {
		descriptors = append(descriptors, resultcontract.Descriptor{Type: entry.kind, Payload: resultcontract.Range{Min: entry.from, Before: entry.before}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Operations: []string{"display", "save", "json", "csv", "html", "report-pack"}, LegacyFindingsOptional: true})
	}
	descriptors = append(descriptors, resultcontract.Descriptor{Providers: []resultcontract.Provider{{ID: "server:events", Version: "1.0.0", Kind: "detail", Path: "/tables/events", Schema: "object-rows-v1"}}, ValidatePayload: func(r models.AnalysisResult) error {
		for _, key := range []string{"total_events", "error_count", "warning_count"} {
			value, ok := r.Summary[key]
			if !ok {
				return fmt.Errorf("missing server counter")
			}
			number, e := resultcontract.ExactNumber(value, "json_number")
			if e != nil || !number.IsInt() || number.Sign() < 0 {
				return fmt.Errorf("invalid server counter")
			}
		}
		return nil
	}, Type: "server_log", Payload: resultcontract.Range{Min: "0.1.0", Before: "0.2.0"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Extensions: map[string]resultcontract.Range{"measurement": {Min: "1.0.0", Before: "1.1.0"}}, Scalars: map[string]resultcontract.ValueSchema{"/summary/total_events": scalar, "/summary/error_count": scalar, "/summary/warning_count": scalar}})
	descriptors = append(descriptors, profileanalyzer.ModelDescriptor(), profileanalyzer.LegacySourceModelDescriptor(), profileanalyzer.SourceModelDescriptor(), MappedMetricDescriptor(), metricanalyzer.Descriptor())
	return resultcontract.New(descriptors...)
}
func Analyzer() ingestion.Analyzer {
	return ingestion.Analyzer{BindSelection: func(result *models.AnalysisResult, info ingestion.SourceInfo, selection ingestion.AnalysisSelection) error {
		coverage := "complete_records"
		if selection.SelectionReason != "" {
			coverage = "selected_records:" + selection.SelectionReason
		}
		var comparison *resultcontract.PopulationComparison
		// Partial results require a future explicit baseline coverage policy.
		if !selection.Partial {
			comparison = &resultcontract.PopulationComparison{Version: "1.0.0", SelectionHash: selection.OptionsHash, CoverageKind: coverage}
		}
		result.Metadata.Extra["population_ids"] = []string{"imported_records"}
		result.Metadata.Extra["populations"] = map[string]resultcontract.PopulationIdentity{"imported_records": {
			Comparison: comparison, SourceHashes: []string{info.SHA256}, SelectionHash: selection.OptionsHash,
			SemanticRevision: selection.ImporterID + "@" + selection.AdapterVersion + "/" + result.Type + "@" + result.Metadata.SchemaVersion,
			Coverage:         fmt.Sprintf("records:%d/partial:%t/reason:%s", selection.Records, selection.Partial, selection.PartialReason),
		}}
		return nil
	}, ID: "server-log.legacy", Family: "server-log.record.v1", ResultType: "server_log", Analyze: func(ctx context.Context, source ingestion.RecordSource, info ingestion.SourceInfo, raw json.RawMessage, diags *diagnostics.ParserDiagnostics) (models.AnalysisResult, error) {
		var opts parser.ImportOptions
		if e := wirejson.Decode(raw, &opts); e != nil {
			return models.AnalysisResult{}, e
		}
		format := "auto"
		if diags != nil {
			format = diags.Format
		}
		result, e := analyzer.BuildStream(info.Name, format, diags, analyzer.Options{TopN: opts.TopN, MaxLines: opts.MaxLines, Strict: opts.Strict}, func(yield func(parser.Record) error) error {
			return source.ForEach(ctx, func(r ingestion.Record) error {
				var typed parser.Record
				if e := wirejson.Decode(r.Payload, &typed); e != nil {
					return e
				}
				return yield(typed)
			})
		}, ingestion.SourceMetadata{SourceKind: ingestion.SourceKindServerLog, SourceFormat: format, Product: "application server", File: ingestion.FileIdentity{BaseName: info.Name, Extension: filepath.Ext(info.Name), SizeBytes: info.Size, SanitizedID: info.ID, SanitizedIDVersion: "lease-source-v1"}})
		if e != nil {
			return result, e
		}
		result.Metadata.Extra["envelope_schema_version"] = "1.0.0"
		result.Metadata.Extra["extensions"] = map[string]string{"measurement": "1.0.0"}
		measurements := map[string]resultcontract.Measurement{}
		for _, key := range []string{"total_events", "error_count", "warning_count"} {
			measurements["/summary/"+key] = resultcontract.Measurement{Kind: "record_count", Unit: "count", Scope: "imported_records", Population: "imported_records", Aggregation: "count", Encoding: "json_number", Availability: "observed"}
		}
		result.Metadata.Extra["measurement"] = measurements
		return result, nil
	}}
}
func Builtin() (*ingestion.Catalog, *resultcontract.Registry, error) {
	if e := verifyBundle(); e != nil {
		return nil, nil, ingestion.Problem("IMPORT_REGISTRY_INVALID", "bundled fixture/schema hash mismatch")
	}
	schemas := Schemas()
	manifest, e := Contracts.ReadFile("contracts/manifest.json")
	if e != nil {
		return nil, nil, e
	}
	sum := sha256.Sum256(manifest)
	descriptors := []ingestion.ImporterDescriptor{}
	for _, format := range []string{"tomcat"} {
		aliases := []string{format}
		if format == "apache-error" {
			aliases = append(aliases, "apache")
		}
		if format == "nginx-error" {
			aliases = append(aliases, "nginx")
		}
		d := ingestion.ImporterDescriptor{ID: "server." + format, Aliases: aliases, Family: "server_log", Extensions: []string{".log", ".txt"}, Producer: format, VerifiedVersions: []string{}, AdapterVersion: "1.0.0", InputSchema: schemas[0].Ref, OptionSchema: schemas[1].Ref, RecordSchema: schemas[2].Ref, ResultSchema: schemas[3].Ref, RecordFamily: "server-log.record.v1", NormalizerInput: "server-log.record.v1", NormalizerOutput: "server-log.record.v1", AnalyzerID: "server-log.legacy", MeasurementKinds: []string{"record_count"}, Capabilities: []string{"summary", "table", "json_export"}, SupportStatus: "limited", Limitations: []string{"plain UTF-8 only; no compressed inputs", "legacy timestamp/severity semantics; no producer-version inference"}, FixtureManifest: "diagnostics.ingestion.foundation.v1", FixtureHash: hex.EncodeToString(sum[:]), ConformanceVersion: "1.0.0", ProbeBudget: ingestion.MaxProbeBytes, SupportsPartial: true}
		descriptors = append(descriptors, parser.NewImporter(format, d))
	}
	profileDescriptor, profileSchemas, profileAnalyzer, e := profileRegistration()
	if e != nil {
		return nil, nil, ingestion.Problem("IMPORT_REGISTRY_INVALID", "profile bundle composition failed")
	}
	descriptors = append(descriptors, profileDescriptor)
	schemas = append(schemas, profileSchemas...)
	sourceDescriptor, sourceSchemas, sourceAnalyzer, e := profileSourceRegistration()
	if e != nil {
		return nil, nil, fmt.Errorf("source profile catalog: %w", e)
	}
	descriptors = append(descriptors, sourceDescriptor)
	schemas = append(schemas, sourceSchemas...)
	metricDescriptor, metricSchemas, metricAnalyzer, e := metricHistoryRegistration()
	if e != nil {
		return nil, nil, fmt.Errorf("metric history catalog: %w", e)
	}
	descriptors = append(descriptors, metricDescriptor)
	schemas = append(schemas, metricSchemas...)
	c, e := ingestion.NewCatalog(descriptors, schemas, []ingestion.Analyzer{Analyzer(), profileAnalyzer, sourceAnalyzer, metricAnalyzer})
	if e != nil {
		return nil, nil, e
	}
	r, e := ResultRegistry()
	return c, r, e
}

func verifyBundle() error {
	raw, e := Contracts.ReadFile("contracts/manifest.json")
	if e != nil {
		return e
	}
	var manifest struct {
		Files  map[string]string `json:"files"`
		Schema string            `json:"measurement_schema_sha256"`
	}
	if e = wirejson.Decode(raw, &manifest); e != nil {
		return e
	}
	for name, digest := range manifest.Files {
		if filepath.Base(name) != name {
			return fmt.Errorf("invalid fixture name")
		}
		body, e := Contracts.ReadFile("contracts/" + name)
		if e != nil {
			return e
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != digest {
			return fmt.Errorf("fixture digest mismatch")
		}
	}
	body, e := Contracts.ReadFile("contracts/measurement-descriptor.schema.json")
	if e != nil {
		return e
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != manifest.Schema {
		return fmt.Errorf("schema digest mismatch")
	}
	return nil
}

// LegacyRegistry is a confidence-based compatibility wrapper. Its tie ordering
// and API remain independent of explicit verified/hint selection in new jobs.
func LegacyRegistry(c *ingestion.Catalog) (*ingestion.FormatRegistry, error) {
	formats := []ingestion.SourceFormat{}
	for _, view := range c.List() {
		format := view
		formats = append(formats, ingestion.SourceFormat{ID: format.ID, SourceKind: format.Family, Extensions: format.Extensions, Detector: func(p ingestion.Probe) ingestion.Detection {
			v, e := c.DetectOne(context.Background(), format.ID, p)
			if e != nil {
				return ingestion.Detection{}
			}
			return ingestion.Detection{FormatID: format.ID, Confidence: v.Confidence, Reason: v.Reason, Product: strings.TrimSpace(format.Producer)}
		}})
	}
	return ingestion.NewFormatRegistry(formats...)
}
