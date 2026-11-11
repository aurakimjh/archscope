package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const MaxOptionBytes = 64 * 1024

type ImportError struct {
	Code    string
	Message string
}

func (e *ImportError) Error() string     { return e.Code + ": " + e.Message }
func Problem(code, message string) error { return &ImportError{code, message} }

type SchemaRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Hash    string `json:"hash"`
}
type Schema struct {
	Ref        SchemaRef
	Definition json.RawMessage
	Validate   func(json.RawMessage) error
}

func NewSchema(id, version string, definition json.RawMessage, validate func(json.RawMessage) error) Schema {
	sum := sha256.Sum256(definition)
	return Schema{SchemaRef{id, version, hex.EncodeToString(sum[:])}, append(json.RawMessage(nil), definition...), validate}
}

type EvidenceGrade string

const (
	Verified EvidenceGrade = "verified"
	Hint     EvidenceGrade = "hint"
)

type Candidate struct {
	ImporterID      string        `json:"importerId"`
	Grade           EvidenceGrade `json:"grade"`
	Confidence      float64       `json:"confidence"`
	Reason          string        `json:"reason"`
	ProducerVersion string        `json:"producerVersion,omitempty"`
}
type DetectionSet struct {
	State      string      `json:"state"`
	Selected   string      `json:"selected,omitempty"`
	Candidates []Candidate `json:"candidates"`
}
type SourceInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type Source interface {
	Info() SourceInfo
	Open(context.Context) (io.ReadCloser, error)
}
type Position struct {
	Byte  int64 `json:"byte"`
	Line  int64 `json:"line"`
	Event int64 `json:"event"`
}
type Record struct {
	Sequence   uint64          `json:"sequence"`
	Position   Position        `json:"position"`
	Payload    json.RawMessage `json:"payload"`
	StackDepth int             `json:"stackDepth,omitempty"`
	Keys       []string        `json:"keys,omitempty"`
}
type Batch struct {
	Family  string
	Records []Record
}
type RecordSink interface {
	Write(context.Context, Batch) error
}
type ParseReport struct {
	Complete      bool
	SafeBoundary  bool
	PartialReason string
	Diagnostics   *diagnostics.ParserDiagnostics
}
type Parser interface {
	Parse(context.Context, Source, RecordSink) (ParseReport, error)
}
type ParserFunc func(context.Context, Source, RecordSink) (ParseReport, error)

func (f ParserFunc) Parse(c context.Context, s Source, k RecordSink) (ParseReport, error) {
	return f(c, s, k)
}

type RecordSource interface {
	ForEach(context.Context, func(Record) error) error
}
type Analyzer struct {
	ID, Family, ResultType string
	Analyze                func(context.Context, RecordSource, SourceInfo, json.RawMessage, *diagnostics.ParserDiagnostics) (models.AnalysisResult, error)
}
type ImporterDescriptor struct {
	ReleaseEligible      bool                                            `json:"releaseEligible"`
	ConverterRequirement string                                          `json:"converterRequirement,omitempty"`
	ID                   string                                          `json:"id"`
	Aliases              []string                                        `json:"aliases"`
	Family               string                                          `json:"family"`
	Extensions           []string                                        `json:"extensions"`
	Producer             string                                          `json:"producer"`
	VerifiedVersions     []string                                        `json:"verifiedVersions"`
	AdapterVersion       string                                          `json:"adapterVersion"`
	InputSchema          SchemaRef                                       `json:"inputSchema"`
	OptionSchema         SchemaRef                                       `json:"optionSchema"`
	RecordSchema         SchemaRef                                       `json:"recordSchema"`
	ResultSchema         SchemaRef                                       `json:"resultSchema"`
	RecordFamily         string                                          `json:"recordFamily"`
	NormalizerInput      string                                          `json:"normalizerInput"`
	NormalizerOutput     string                                          `json:"normalizerOutput"`
	AnalyzerID           string                                          `json:"analyzerId"`
	MeasurementKinds     []string                                        `json:"measurementKinds"`
	Capabilities         []string                                        `json:"capabilities"`
	SupportStatus        string                                          `json:"supportStatus"`
	Limitations          []string                                        `json:"limitations"`
	FixtureManifest      string                                          `json:"fixtureManifest"`
	FixtureHash          string                                          `json:"fixtureHash"`
	ConformanceVersion   string                                          `json:"conformanceVersion"`
	ProbeBudget          int                                             `json:"probeBudget"`
	SupportsPartial      bool                                            `json:"supportsPartial"`
	NeedsSeek            bool                                            `json:"needsSeek"`
	Detector             func(context.Context, Probe) (Candidate, error) `json:"-"`
	ValidateInput        func(context.Context, Source) error             `json:"-"`
	ParserFactory        func(json.RawMessage) Parser                    `json:"-"`
	Normalize            func(context.Context, Record) (Record, error)   `json:"-"`
}
type ResolvedImporter struct {
	Descriptor     ImporterDescriptor
	Options        json.RawMessage
	OptionHash     string
	PolicyRevision string
	Analyzer       Analyzer
}
type Catalog struct {
	descriptors map[string]ImporterDescriptor
	aliases     map[string]string
	schemas     map[SchemaRef]Schema
	analyzers   map[string]Analyzer
	list        []ImporterDescriptor
}

var identifier = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
var semver = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func NewCatalog(importers []ImporterDescriptor, schemas []Schema, analyzers []Analyzer) (*Catalog, error) {
	invalid := func(s string) (*Catalog, error) { return nil, Problem("IMPORT_REGISTRY_INVALID", s) }
	c := &Catalog{descriptors: map[string]ImporterDescriptor{}, aliases: map[string]string{}, schemas: map[SchemaRef]Schema{}, analyzers: map[string]Analyzer{}}
	for _, s := range schemas {
		sum := sha256.Sum256(s.Definition)
		if s.Ref.ID == "" || !semver.MatchString(s.Ref.Version) || s.Ref.Hash != hex.EncodeToString(sum[:]) || s.Validate == nil {
			return invalid("invalid bundled schema")
		}
		if _, ok := c.schemas[s.Ref]; ok {
			return invalid("duplicate schema")
		}
		var doc any
		if wirejson.Decode(s.Definition, &doc) != nil {
			return invalid("invalid schema definition")
		}
		c.schemas[s.Ref] = s
	}
	for _, a := range analyzers {
		if !identifier.MatchString(a.ID) || a.Family == "" || a.ResultType == "" || a.Analyze == nil {
			return invalid("invalid analyzer")
		}
		if _, ok := c.analyzers[a.ID]; ok {
			return invalid("duplicate analyzer")
		}
		c.analyzers[a.ID] = a
	}
	if len(importers) > 256 {
		return invalid("catalog exceeds 256 importers")
	}
	for _, d := range importers {
		if len(d.ID) > 128 || !identifier.MatchString(d.ID) || d.ID == "auto" || !semver.MatchString(d.AdapterVersion) || !semver.MatchString(d.ConformanceVersion) || d.Detector == nil || d.ValidateInput == nil || d.ParserFactory == nil || d.Normalize == nil || d.ProbeBudget < 1 || d.ProbeBudget > MaxProbeBytes {
			return invalid("invalid importer descriptor")
		}
		for _, key := range append([]string{d.ID}, d.Aliases...) {
			if !identifier.MatchString(key) || key == "auto" {
				return invalid("invalid ID/alias")
			}
			if _, ok := c.aliases[key]; ok {
				return invalid("duplicate ID/alias")
			}
			c.aliases[key] = d.ID
		}
		for _, ref := range []SchemaRef{d.InputSchema, d.OptionSchema, d.RecordSchema, d.ResultSchema} {
			if _, ok := c.schemas[ref]; !ok {
				return invalid("unresolved schema ID/version/hash")
			}
		}
		a, ok := c.analyzers[d.AnalyzerID]
		if !ok || d.RecordFamily == "" || d.RecordFamily != d.NormalizerInput || d.NormalizerOutput != a.Family {
			return invalid("record/normalizer/analyzer family mismatch")
		}
		digest, digestErr := hex.DecodeString(d.FixtureHash)
		if (d.FixtureManifest != "" && (digestErr != nil || len(digest) != 32)) || (d.SupportStatus == "verified" && d.FixtureManifest == "") {
			return invalid("release importer requires fixture evidence")
		}
		d.ReleaseEligible = d.FixtureManifest != "" && digestErr == nil && len(digest) == 32 && (d.SupportStatus == "verified" || d.SupportStatus == "limited")
		// Freeze caller-owned slices; executable closures are compiled trusted code.
		b, _ := json.Marshal(d)
		var frozen ImporterDescriptor
		json.Unmarshal(b, &frozen)
		frozen.Detector = d.Detector
		frozen.ValidateInput = d.ValidateInput
		frozen.ParserFactory = d.ParserFactory
		frozen.Normalize = d.Normalize
		c.descriptors[d.ID] = frozen
		c.list = append(c.list, frozen)
	}
	sort.Slice(c.list, func(i, j int) bool { return c.list[i].ID < c.list[j].ID })
	return c, nil
}
func (c *Catalog) List() []ImporterDescriptor {
	b, _ := json.Marshal(c.list)
	out := []ImporterDescriptor{}
	json.Unmarshal(b, &out)
	return out
}
func (c *Catalog) Canonical(id string) (string, bool) {
	v, ok := c.aliases[strings.ToLower(strings.TrimSpace(id))]
	return v, ok
}
func (c *Catalog) DetectCandidates(ctx context.Context, p Probe) (DetectionSet, error) {
	set := DetectionSet{State: "unsupported", Candidates: []Candidate{}}
	for _, d := range c.list {
		if e := ctx.Err(); e != nil {
			return set, e
		}
		probe := p
		probe.Head = append([]byte(nil), p.Head[:min(len(p.Head), d.ProbeBudget)]...)
		detectorCtx, cancel := context.WithTimeout(ctx, time.Second)
		v, e := d.Detector(detectorCtx, probe)
		deadline := detectorCtx.Err()
		cancel()
		if deadline != nil {
			return set, Problem("IMPORT_BUDGET_EXCEEDED", "detector time budget exceeded")
		}
		if e != nil {
			return set, Problem("IMPORT_DETECTOR_INVALID", "detector failed")
		}
		if math.IsNaN(v.Confidence) || math.IsInf(v.Confidence, 0) || v.Confidence < 0 || v.Confidence > 1 {
			return set, Problem("IMPORT_DETECTOR_INVALID", "invalid confidence")
		}
		if v.Confidence == 0 {
			continue
		}
		if v.Grade != Verified && v.Grade != Hint {
			return set, Problem("IMPORT_DETECTOR_INVALID", "missing evidence grade")
		}
		v.ImporterID = d.ID
		set.Candidates = append(set.Candidates, v)
	}
	sort.Slice(set.Candidates, func(i, j int) bool {
		a, b := set.Candidates[i], set.Candidates[j]
		if a.Grade != b.Grade {
			return a.Grade == Verified
		}
		if a.Confidence != b.Confidence {
			return a.Confidence > b.Confidence
		}
		return a.ImporterID < b.ImporterID
	})
	n := 0
	for _, v := range set.Candidates {
		if v.Grade == Verified {
			n++
			set.Selected = v.ImporterID
		}
	}
	if n == 1 {
		set.State = "selected"
	} else if len(set.Candidates) > 0 {
		set.State = "needs_selection"
		set.Selected = ""
	}
	return set, nil
}
func (c *Catalog) Resolve(id string, options json.RawMessage, policy string) (ResolvedImporter, error) {
	if len(options) > MaxOptionBytes {
		return ResolvedImporter{}, Problem("IMPORT_OPTIONS_INVALID", "options exceed 64 KiB")
	}
	id, ok := c.Canonical(id)
	if !ok {
		return ResolvedImporter{}, Problem("IMPORT_FORMAT_UNSUPPORTED", "unknown importer")
	}
	stored := c.descriptors[id]
	if stored.SupportStatus == "unsupported" || stored.SupportStatus == "conversion_required" {
		return ResolvedImporter{}, Problem("IMPORT_FORMAT_UNSUPPORTED", "importer requires a supported adapter or approved conversion")
	}
	b, _ := json.Marshal(stored)
	var d ImporterDescriptor
	json.Unmarshal(b, &d)
	d.Detector = stored.Detector
	d.ValidateInput = stored.ValidateInput
	d.ParserFactory = stored.ParserFactory
	d.Normalize = stored.Normalize
	if len(options) == 0 {
		options = json.RawMessage(`{}`)
	}
	var value map[string]any
	if e := wirejson.Decode(options, &value); e != nil || value == nil {
		return ResolvedImporter{}, Problem("IMPORT_OPTIONS_INVALID", "options must be a bounded JSON object")
	}
	canonical, e := json.Marshal(value)
	if e != nil {
		return ResolvedImporter{}, e
	}
	if e = c.schemas[d.OptionSchema].Validate(canonical); e != nil {
		return ResolvedImporter{}, Problem("IMPORT_OPTIONS_INVALID", "options violate bundled schema")
	}
	sum := sha256.Sum256(append([]byte("options-v1\x00"), canonical...))
	return ResolvedImporter{d, canonical, hex.EncodeToString(sum[:]), policy, c.analyzers[d.AnalyzerID]}, nil
}
func (c *Catalog) Validate(ref SchemaRef, b json.RawMessage) error {
	s, ok := c.schemas[ref]
	if !ok {
		return fmt.Errorf("unresolved schema")
	}
	return s.Validate(b)
}
