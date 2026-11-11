package ingestion_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
)

func descriptor(t *testing.T) ingestion.ImporterDescriptor {
	t.Helper()
	c, _, e := catalog.Builtin()
	if e != nil {
		t.Fatal(e)
	}
	d, e := c.Resolve("tomcat", nil, "test")
	if e != nil {
		t.Fatal(e)
	}
	return d.Descriptor
}
func registry(t *testing.T, ds ...ingestion.ImporterDescriptor) *ingestion.Catalog {
	t.Helper()
	c, e := ingestion.NewCatalog(ds, catalog.Schemas(), []ingestion.Analyzer{catalog.Analyzer()})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestC1CatalogRejectsInvalidRegistrationsAndFreezes(t *testing.T) {
	original := descriptor(t)
	for _, edit := range []func(*ingestion.ImporterDescriptor){func(d *ingestion.ImporterDescriptor) { d.ID = "auto" }, func(d *ingestion.ImporterDescriptor) { d.Aliases = []string{d.ID} }, func(d *ingestion.ImporterDescriptor) { d.Aliases = []string{"AUTO"} }, func(d *ingestion.ImporterDescriptor) { d.InputSchema.Hash = "unknown" }, func(d *ingestion.ImporterDescriptor) { d.NormalizerInput = "wrong" }, func(d *ingestion.ImporterDescriptor) { d.NormalizerOutput = "wrong" }, func(d *ingestion.ImporterDescriptor) { d.AdapterVersion = "1" }, func(d *ingestion.ImporterDescriptor) { d.SupportStatus = "verified"; d.FixtureManifest = "" }, func(d *ingestion.ImporterDescriptor) { d.FixtureHash = strings.Repeat("z", 64) }} {
		d := original
		edit(&d)
		if _, e := ingestion.NewCatalog([]ingestion.ImporterDescriptor{d}, catalog.Schemas(), []ingestion.Analyzer{catalog.Analyzer()}); e == nil {
			t.Fatal("invalid descriptor activated", d.ID)
		}
	}
	if _, e := ingestion.NewCatalog([]ingestion.ImporterDescriptor{original, original}, catalog.Schemas(), []ingestion.Analyzer{catalog.Analyzer()}); e == nil {
		t.Fatal("duplicate canonical accepted")
	}
	c := registry(t, original)
	d, e := c.Resolve(" TOMCAT ", nil, "revision")
	if e != nil || d.Descriptor.ID != original.ID {
		t.Fatal(d, e)
	}
	d.Descriptor.Aliases[0] = "mutated"
	again, _ := c.Resolve("tomcat", nil, "revision")
	if again.Descriptor.Aliases[0] != "tomcat" {
		t.Fatal("resolved descriptor mutated catalog")
	}
	view := c.List()
	if view[0].Detector != nil || view[0].ParserFactory != nil {
		t.Fatal("executable handles exposed")
	}
}
func TestC1VerifiedHintCollisionScoresAndLegacyConfidence(t *testing.T) {
	first := descriptor(t)
	second := first
	second.ID = "server.second"
	second.Aliases = []string{"second"}
	probe := ingestion.Probe{Head: []byte("11-Nov-2026 10:00:00 INFO [main] org.apache.catalina.startup.Catalina Synthetic"), Extension: ".log"}
	c := registry(t, first)
	set, e := c.DetectCandidates(context.Background(), probe)
	if e != nil || set.State != "selected" {
		t.Fatal(set, e)
	}
	c = registry(t, first, second)
	set, e = c.DetectCandidates(context.Background(), probe)
	if e != nil || set.State != "needs_selection" || len(set.Candidates) != 2 {
		t.Fatal(set, e)
	}
	probe.Head = []byte("not a server log")
	set, e = c.DetectCandidates(context.Background(), probe)
	if e != nil || set.State != "needs_selection" || set.Candidates[0].Grade != ingestion.Hint {
		t.Fatal(set, e)
	}
	probe.Extension = ".unknown"
	set, e = c.DetectCandidates(context.Background(), probe)
	if e != nil || set.State != "unsupported" {
		t.Fatal(set, e)
	}
	for _, score := range []float64{math.NaN(), math.Inf(1), -1, 1.01} {
		d := first
		d.Detector = func(context.Context, ingestion.Probe) (ingestion.Candidate, error) {
			return ingestion.Candidate{Grade: ingestion.Verified, Confidence: score}, nil
		}
		if set, err := registry(t, d).DetectCandidates(context.Background(), probe); err != nil || set.State != "detection_failed" || len(set.Diagnostics) != 1 {
			t.Fatal("invalid confidence not diagnosed", set, err)
		}
	}
	legacy, e := catalog.LegacyRegistry(registry(t, first))
	if e != nil {
		t.Fatal(e)
	}
	legacyProbe := ingestion.Probe{Extension: ".log"}
	v, e := legacy.Detect(legacyProbe)
	if e != nil || v.Confidence != 0.25 {
		t.Fatal(v, e)
	}
}
func TestC1OptionCanonicalizationAndCooperativeDeadline(t *testing.T) {
	d := descriptor(t)
	c := registry(t, d)
	a, e := c.Resolve("tomcat", []byte(`{"top_n":3,"strict":false}`), "p")
	if e != nil {
		t.Fatal(e)
	}
	b, e := c.Resolve("tomcat", []byte(`{"strict":false,"top_n":3}`), "p")
	if e != nil || a.OptionHash != b.OptionHash {
		t.Fatal(e)
	}
	for _, raw := range []string{`{"strict":1}`, `{"secret":"path"}`, `{"top_n":1.5}`, `{"top_n":1,"top_n":2}`, `null`} {
		if _, e = c.Resolve("tomcat", []byte(raw), "p"); e == nil {
			t.Fatal("invalid options accepted", raw)
		}
	}
	d.Detector = func(ctx context.Context, _ ingestion.Probe) (ingestion.Candidate, error) {
		<-ctx.Done()
		return ingestion.Candidate{}, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, e = registry(t, d).DetectCandidates(ctx, ingestion.Probe{}); e == nil {
		t.Fatal("detector deadline ignored")
	}
}

type editingContext struct {
	context.Context
	once bool
	path string
}

func (c *editingContext) Err() error {
	if !c.once {
		c.once = true
		os.WriteFile(c.path, []byte("changed-length"), 0600)
	}
	return c.Context.Err()
}
func TestC2LeaseDetectsConcurrentEditAndOwnsSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.log")
	os.WriteFile(path, []byte("original"), 0600)
	stage := t.TempDir()
	b := ingestion.DefaultBudgets()
	if _, e := ingestion.Acquire(&editingContext{Context: context.Background(), path: path}, path, stage, "s", b); e == nil {
		t.Fatal("concurrent source edit accepted")
	}
	os.WriteFile(path, []byte("immutable capture"), 0600)
	lease, e := ingestion.Acquire(context.Background(), path, t.TempDir(), "s", b)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(path, []byte("later source revision"), 0600)
	if e = lease.Verify(context.Background()); e != nil {
		t.Fatal("source alias changed owned revision", e)
	}
	if _, e = (ingestion.Budgets{InputBytes: b.InputBytes + 1}).Within(b); e == nil {
		t.Fatal("policy budget raised")
	}
}

func TestReviewR03SchemaRolesFamiliesOutputAndValidatorConformance(t *testing.T) {
	original := descriptor(t)
	for _, edit := range []func(*ingestion.ImporterDescriptor){
		func(d *ingestion.ImporterDescriptor) { d.OptionSchema = d.RecordSchema },
		func(d *ingestion.ImporterDescriptor) { d.Family = "gpu_metrics" },
		func(d *ingestion.ImporterDescriptor) { d.ResultSchema = d.InputSchema },
	} {
		d := original
		edit(&d)
		if _, e := ingestion.NewCatalog([]ingestion.ImporterDescriptor{d}, catalog.Schemas(), []ingestion.Analyzer{catalog.Analyzer()}); e == nil {
			t.Fatal("invalid composition accepted")
		}
	}
	a := catalog.Analyzer()
	a.ResultType = "unregistered_type"
	if _, e := ingestion.NewCatalog([]ingestion.ImporterDescriptor{original}, catalog.Schemas(), []ingestion.Analyzer{a}); e == nil {
		t.Fatal("analyzer/schema output mismatch accepted")
	}
	schemas := catalog.Schemas()
	schemas[1].Validate = func(json.RawMessage) error { return nil }
	if _, e := ingestion.NewCatalog([]ingestion.ImporterDescriptor{original}, schemas, []ingestion.Analyzer{catalog.Analyzer()}); e == nil {
		t.Fatal("validator drift accepted")
	}
	schemas = catalog.Schemas()
	schemas[1].Definition = json.RawMessage(`{"type":"array"}`)
	schemas[1] = func() ingestion.Schema {
		next := ingestion.NewSchema(schemas[1].Ref.ID, schemas[1].Ref.Version, schemas[1].Definition, schemas[1].Validate)
		next.Role = schemas[1].Role
		next.Family = schemas[1].Family
		next.Conformance = schemas[1].Conformance
		return next
	}()
	changed := original
	changed.OptionSchema = schemas[1].Ref
	if _, e := ingestion.NewCatalog([]ingestion.ImporterDescriptor{changed}, schemas, []ingestion.Analyzer{catalog.Analyzer()}); e == nil {
		t.Fatal("definition drift accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := registry(t, original).DetectCandidates(ctx, ingestion.Probe{}); !errors.Is(e, context.Canceled) {
		t.Fatal("parent cancellation misclassified", e)
	}
}

func TestReviewR03SchemaConformanceArithmeticAndExternalRefsAreBounded(t *testing.T) {
	validate, e := ingestion.CompileSchema(json.RawMessage(`{"type":"integer","minimum":0}`))
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{`1e1000000000`, `1e-1000000000`} {
		if e = validate(json.RawMessage(raw)); e == nil {
			t.Fatal("unbounded schema arithmetic accepted")
		}
	}
	if _, e = ingestion.CompileSchema(json.RawMessage(`{"minimum":1e1000000000}`)); e == nil {
		t.Fatal("unbounded schema literal accepted")
	}
	if _, e = ingestion.CompileSchema(json.RawMessage(`{"$ref":"https://example.invalid/not-loaded"}`)); e == nil {
		t.Fatal("external schema reference accepted")
	}
}
