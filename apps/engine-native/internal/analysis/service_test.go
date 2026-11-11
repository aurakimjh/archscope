package analysis_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	legacy "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/serverlog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func setup(t *testing.T, edit func(*ingestion.ImporterDescriptor)) (*analysis.Service, string, string) {
	t.Helper()
	c, r, e := catalog.Builtin()
	if e != nil {
		t.Fatal(e)
	}
	resolved, e := c.Resolve("tomcat", nil, "test")
	if e != nil {
		t.Fatal(e)
	}
	d := resolved.Descriptor
	if edit != nil {
		edit(&d)
		c, e = ingestion.NewCatalog([]ingestion.ImporterDescriptor{d}, catalog.Schemas(), []ingestion.Analyzer{catalog.Analyzer()})
		if e != nil {
			t.Fatal(e)
		}
	}
	root := t.TempDir()
	s, e := analysis.New(root, c, r)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	body, _ := catalog.Contracts.ReadFile("contracts/server.log")
	path := filepath.Join(t.TempDir(), "fixture.log")
	os.WriteFile(path, body, 0600)
	return s, path, root
}
func wait(t *testing.T, s *analysis.Service, j analysis.JobSnapshot) analysis.JobSnapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, e := s.Wait(ctx, j.ID, j.Generation)
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func start(t *testing.T, s *analysis.Service, r analysis.Request) analysis.JobSnapshot {
	t.Helper()
	j, e := s.Start(r)
	if e != nil {
		t.Fatal(e)
	}
	return j
}
func clean(t *testing.T, root string) {
	t.Helper()
	entries, e := os.ReadDir(filepath.Join(root, "staging"))
	if e != nil || len(entries) != 0 {
		t.Fatal("staging not cleaned", entries, e)
	}
}
func TestC4LegacyParityAtomicResultAndReopen(t *testing.T) {
	s, path, root := setup(t, nil)
	job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat"}))
	if job.State != "completed" || job.ResultHandle == "" {
		t.Fatal(job)
	}
	raw, result, e := s.Read(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	old, e := legacy.Analyze(path, "tomcat", legacy.Options{})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := json.Marshal(old.Summary)
	b, _ := json.Marshal(result.Summary)
	if string(a) != string(b) {
		t.Fatal("legacy scalar semantics changed", string(a), string(b))
	}
	for _, format := range []string{"json", "csv", "html", "report-pack"} {
		body, e := s.Export(job.ResultHandle, format)
		if e != nil || len(body) == 0 {
			t.Fatal(format, e)
		}
	}
	destination := filepath.Join(t.TempDir(), "saved.json")
	if e = analysis.AtomicExport(destination, raw); e != nil {
		t.Fatal(e)
	}
	view, e := s.Load(context.Background(), destination)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	again, _, e := s.Read(view.Handle)
	if e != nil || string(raw) != string(again) {
		t.Fatal("load/export changed revision", e)
	}
	out, e := s.Cancel(job.ID, job.Generation)
	if e != nil || out.Outcome != "already_terminal" {
		t.Fatal(out, e)
	}
	clean(t, root)
	// Failure must preserve the already-published revision and clean staging.
	bad := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat", Options: json.RawMessage(`{"strict":1}`)}))
	if bad.State != "failed" {
		t.Fatal(bad)
	}
	again, _, e = s.Read(job.ResultHandle)
	if e != nil || string(again) != string(raw) {
		t.Fatal("failure changed prior revision")
	}
	clean(t, root)
}
func TestC2PartialRequiresSafeRecordsImporterAndCaller(t *testing.T) {
	for _, allow := range []bool{false, true} {
		t.Run(map[bool]string{true: "allowed", false: "refused"}[allow], func(t *testing.T) {
			s, path, root := setup(t, nil)
			job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat", Budgets: ingestion.Budgets{Records: 2}, AllowPartial: allow}))
			if allow {
				if job.State != "partial" {
					t.Fatal(job)
				}
				_, r, e := s.Read(job.ResultHandle)
				if e != nil || r.Metadata.Extra["import_coverage"] == nil {
					t.Fatal("partial coverage missing", e)
				}
			} else if job.State != "failed" || job.ResultHandle != "" {
				t.Fatal("unsafe partial published", job)
			}
			clean(t, root)
		})
	}
	for _, family := range []string{"wrong", "server-log.record.v1"} {
		t.Run(family, func(t *testing.T) {
			s, path, root := setup(t, func(d *ingestion.ImporterDescriptor) {
				d.ParserFactory = func(json.RawMessage) ingestion.Parser {
					return ingestion.ParserFunc(func(ctx context.Context, _ ingestion.Source, sink ingestion.RecordSink) (ingestion.ParseReport, error) {
						e := sink.Write(ctx, ingestion.Batch{Family: family, Records: []ingestion.Record{{Sequence: 1, Payload: json.RawMessage(`{}`)}}})
						return ingestion.ParseReport{SafeBoundary: true, PartialReason: "claimed_safe"}, e
					})
				}
			})
			job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat", AllowPartial: true}))
			if job.State != "failed" || job.ResultHandle != "" {
				t.Fatal(job)
			}
			clean(t, root)
		})
	}
}
func TestC2CancellationDuringParserAndStaleGeneration(t *testing.T) {
	entered := make(chan struct{})
	s, path, root := setup(t, func(d *ingestion.ImporterDescriptor) {
		d.ParserFactory = func(json.RawMessage) ingestion.Parser {
			return ingestion.ParserFunc(func(ctx context.Context, _ ingestion.Source, _ ingestion.RecordSink) (ingestion.ParseReport, error) {
				close(entered)
				<-ctx.Done()
				return ingestion.ParseReport{}, ctx.Err()
			})
		}
	})
	job := start(t, s, analysis.Request{Path: path, Importer: "tomcat"})
	<-entered
	if _, e := s.Get(job.ID, 2); e == nil {
		t.Fatal("stale generation accepted")
	}
	if _, e := s.Cancel(job.ID, 1); e != nil {
		t.Fatal(e)
	}
	job = wait(t, s, job)
	if job.State != "cancelled" || job.Problem != nil || job.ResultHandle != "" {
		t.Fatal(job)
	}
	out, e := s.Cancel(job.ID, 1)
	if e != nil || out.Outcome != "cancelled" {
		t.Fatal(out, e)
	}
	clean(t, root)
}
func TestC2OwnedSnapshotTamperCannotBecomePartial(t *testing.T) {
	var root string
	s, path, gotRoot := setup(t, func(d *ingestion.ImporterDescriptor) {
		factory := d.ParserFactory
		d.ParserFactory = func(opts json.RawMessage) ingestion.Parser {
			return ingestion.ParserFunc(func(ctx context.Context, source ingestion.Source, sink ingestion.RecordSink) (ingestion.ParseReport, error) {
				report, e := factory(opts).Parse(ctx, source, sink)
				paths, _ := filepath.Glob(filepath.Join(root, "staging", "*", "source.snapshot"))
				for _, p := range paths {
					os.WriteFile(p, []byte("tampered"), 0600)
				}
				return report, e
			})
		}
	})
	root = gotRoot
	job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat", AllowPartial: true}))
	if job.State != "failed" || job.Problem.Code != "SOURCE_CHANGED" || job.ResultHandle != "" {
		t.Fatal(job)
	}
	clean(t, root)
}

func TestC1C2SelectionIsExplicitIdempotentAndNotParsing(t *testing.T) {
	cat, results, e := catalog.Builtin()
	if e != nil {
		t.Fatal(e)
	}
	resolved, _ := cat.Resolve("tomcat", nil, "test")
	first := resolved.Descriptor
	second := first
	second.ID = "fixture.second"
	second.Aliases = []string{"second"}
	entered := make(chan struct{}, 2)
	factory := first.ParserFactory
	wrap := func(opts json.RawMessage) ingestion.Parser {
		return ingestion.ParserFunc(func(ctx context.Context, source ingestion.Source, sink ingestion.RecordSink) (ingestion.ParseReport, error) {
			entered <- struct{}{}
			return factory(opts).Parse(ctx, source, sink)
		})
	}
	first.ParserFactory = wrap
	second.ParserFactory = wrap
	cat, e = ingestion.NewCatalog([]ingestion.ImporterDescriptor{first, second}, catalog.Schemas(), []ingestion.Analyzer{catalog.Analyzer()})
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	svc, e := analysis.New(root, cat, results)
	if e != nil {
		t.Fatal(e)
	}
	defer svc.Close()
	body, _ := catalog.Contracts.ReadFile("contracts/server.log")
	path := filepath.Join(t.TempDir(), "source.log")
	os.WriteFile(path, body, 0600)
	job := start(t, svc, analysis.Request{Path: path, Importer: "auto"})
	deadline := time.Now().Add(time.Second)
	for {
		snapshot, _ := svc.Get(job.ID, 1)
		if snapshot.State == "needs_selection" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(snapshot)
		}
		time.Sleep(time.Millisecond)
	}
	if len(entered) != 0 {
		t.Fatal("parsed before explicit selection")
	}
	if _, e = svc.Select(job.ID, 2, "second"); e == nil {
		t.Fatal("stale selection accepted")
	}
	if _, e = svc.Select(job.ID, 1, "second"); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Select(job.ID, 1, "second"); e != nil {
		t.Fatal("same selection was not idempotent", e)
	}
	if _, e = svc.Select(job.ID, 1, "tomcat"); e == nil {
		t.Fatal("changed importer while running")
	}
	job = wait(t, svc, job)
	if job.State != "completed" || len(entered) != 1 {
		t.Fatal(job)
	}
	clean(t, root)
	hint := second
	hint.Detector = func(context.Context, ingestion.Probe) (ingestion.Candidate, error) {
		return ingestion.Candidate{Grade: ingestion.Hint, Confidence: 0.25, Reason: "hint"}, nil
	}
	forcedCat, e := ingestion.NewCatalog([]ingestion.ImporterDescriptor{first, hint}, catalog.Schemas(), []ingestion.Analyzer{catalog.Analyzer()})
	if e != nil {
		t.Fatal(e)
	}
	forced, e := analysis.New(t.TempDir(), forcedCat, results)
	if e != nil {
		t.Fatal(e)
	}
	defer forced.Close()
	bad := wait(t, forced, start(t, forced, analysis.Request{Path: path, Importer: "second"}))
	if bad.State != "failed" || bad.Problem.Code != "IMPORT_FORMAT_MISMATCH" {
		t.Fatal(bad)
	}
}

func TestC2ResultSchemaFailureNeverPublishesEvenWithPartialConsent(t *testing.T) {
	original, results, e := catalog.Builtin()
	if e != nil {
		t.Fatal(e)
	}
	resolved, _ := original.Resolve("tomcat", nil, "test")
	d := resolved.Descriptor
	a := catalog.Analyzer()
	analyze := a.Analyze
	a.Analyze = func(ctx context.Context, records ingestion.RecordSource, info ingestion.SourceInfo, options json.RawMessage, diags *diagnostics.ParserDiagnostics) (models.AnalysisResult, error) {
		result, e := analyze(ctx, records, info, options, diags)
		result.Metadata.SchemaVersion = "9.0.0"
		return result, e
	}
	custom, e := ingestion.NewCatalog([]ingestion.ImporterDescriptor{d}, catalog.Schemas(), []ingestion.Analyzer{a})
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	svc, e := analysis.New(root, custom, results)
	if e != nil {
		t.Fatal(e)
	}
	defer svc.Close()
	data, _ := catalog.Contracts.ReadFile("contracts/server.log")
	path := filepath.Join(t.TempDir(), "fixture.log")
	os.WriteFile(path, data, 0600)
	job := wait(t, svc, start(t, svc, analysis.Request{Path: path, Importer: "tomcat", AllowPartial: true}))
	if job.State != "failed" || job.ResultHandle != "" || job.Problem.Code != "IMPORT_RESULT_INVALID" || job.Problem.Stage != "validating_result" {
		t.Fatal(job)
	}
	entries, e := os.ReadDir(filepath.Join(root, "results"))
	if e != nil || len(entries) != 0 {
		t.Fatal("invalid result published", entries, e)
	}
	clean(t, root)
}
