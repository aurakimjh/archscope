package analysis_test

import (
	"context"
	"encoding/json"
	"errors"
	legacy "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/serverlog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func TestReviewR02TerminalHistoryReclaimedWithoutLosingRevisions(t *testing.T) {
	s, path, _ := setup(t, nil)
	first := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat"}))
	for i := 1; i < 200; i++ {
		job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat", Options: json.RawMessage(`{"unknown":true}`)}))
		if job.State != "failed" {
			t.Fatal(job)
		}
	}
	if _, e := s.Get(first.ID, 1); e == nil || !strings.Contains(e.Error(), "IMPORT_JOB_STALE") {
		t.Fatal("old terminal history not reclaimed", e)
	}
	if _, _, e := s.Read(first.ResultHandle); e != nil {
		t.Fatal("history eviction deleted revision", e)
	}
}

func TestReviewR03CompositionRejectedBeforeServiceStarts(t *testing.T) {
	c, _, e := catalog.Builtin()
	if e != nil {
		t.Fatal(e)
	}
	unrelated, e := resultcontract.New(resultcontract.Descriptor{Type: "other", Payload: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = analysis.New(t.TempDir(), c, unrelated); e == nil {
		t.Fatal("unregistered analyzer result activated")
	}
	incompatible, e := resultcontract.New(resultcontract.Descriptor{Type: "server_log", Payload: resultcontract.Range{Min: "9.0.0", Before: "10.0.0"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = analysis.New(t.TempDir(), c, incompatible); e == nil {
		t.Fatal("incompatible output version activated")
	}

}

func TestReviewR04PilotPopulationAndR11ManifestExpectations(t *testing.T) {
	s, path, _ := setup(t, nil)
	first := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat"}))
	second := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat"}))
	delta, e := s.Compare(first.ResultHandle, second.ResultHandle, "/summary/total_events")
	if e != nil || delta.Value == nil || *delta.Value != "0" {
		t.Fatal(delta, e)
	}
	_, r, e := s.Read(first.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	manifest, _ := catalog.Contracts.ReadFile("contracts/manifest.json")
	var m struct {
		Expected map[string]int `json:"expected"`
	}
	if e = json.Unmarshal(manifest, &m); e != nil {
		t.Fatal(e)
	}
	for expectation, key := range map[string]string{"server_records": "total_events", "server_errors": "error_count", "server_warnings": "warning_count"} {
		want, ok := m.Expected[expectation]
		if !ok {
			t.Fatal("missing executable expectation", expectation)
		}
		if r.Summary[key] != json.Number(stringMustJSON(t, want)) {
			t.Fatal(key, r.Summary[key], want)
		}
	}
	selected := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat", Options: json.RawMessage(`{"max_lines":1}`)}))
	if selected.State != "completed" {
		t.Fatal(selected)
	}
	if _, e = s.Compare(first.ResultHandle, selected.ResultHandle, "/summary/total_events"); e == nil {
		t.Fatal("different selection compared")
	}
	if first.Source.ID == second.Source.ID {
		t.Fatal("identical population merged capture provenance")
	}
}
func stringMustJSON(t *testing.T, v any) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

func TestReviewR07ExplicitImporterSurvivesUnrelatedDetectorFailure(t *testing.T) {
	c, r, e := catalog.Builtin()
	if e != nil {
		t.Fatal(e)
	}
	base, _ := c.Resolve("tomcat", nil, "test")
	broken := base.Descriptor
	broken.ID = "broken.detector"
	broken.Aliases = nil
	broken.Detector = func(context.Context, ingestion.Probe) (ingestion.Candidate, error) {
		return ingestion.Candidate{}, errors.New("private source path")
	}
	c, e = ingestion.NewCatalog([]ingestion.ImporterDescriptor{base.Descriptor, broken}, catalog.Schemas(), []ingestion.Analyzer{catalog.Analyzer()})
	if e != nil {
		t.Fatal(e)
	}
	s, e := analysis.New(t.TempDir(), c, r)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	raw, _ := catalog.Contracts.ReadFile("contracts/server.log")
	path := filepath.Join(t.TempDir(), "source.log")
	os.WriteFile(path, raw, 0600)
	explicit := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat"}))
	if explicit.State != "completed" || len(explicit.Detection.Diagnostics) != 1 {
		t.Fatal(explicit)
	}
	failed := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "broken.detector"}))
	if failed.State != "failed" || failed.Problem.Code != "IMPORT_DETECTOR_INVALID" {
		t.Fatal(failed)
	}
	auto := start(t, s, analysis.Request{Path: path})
	deadline := time.After(2 * time.Second)
	for {
		current, e := s.Get(auto.ID, 1)
		if e != nil {
			t.Fatal(e)
		}
		if current.State == "needs_selection" {
			break
		}
		select {
		case <-deadline:
			t.Fatal(current)
		default:
			time.Sleep(time.Millisecond)
		}
	}
	s.Cancel(auto.ID, 1)
	wait(t, s, auto)
}

func TestReviewR08LoadDedupReleaseAndExclusiveCrashCleanup(t *testing.T) {
	s, path, root := setup(t, nil)
	job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat"}))
	raw, _, e := s.Read(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	saved := filepath.Join(t.TempDir(), "saved.json")
	os.WriteFile(saved, raw, 0600)
	for i := 0; i < 3; i++ {
		view, e := s.Load(context.Background(), saved)
		if e != nil || view.Handle != job.ResultHandle {
			t.Fatal(view, e)
		}
	}
	files, _ := os.ReadDir(filepath.Join(root, "results"))
	if len(files) != 1 {
		t.Fatal("load leaked revisions", files)
	}
	c, r, _ := catalog.Builtin()
	if other, e := analysis.New(root, c, r); e == nil {
		other.Close()
		t.Fatal("concurrent store accepted")
	}
	if e = s.Release(job.ResultHandle); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Read(job.ResultHandle); e == nil || !strings.Contains(e.Error(), "IMPORT_RESULT_STALE") {
		t.Fatal(e)
	}
	s.Close()
	orphan := filepath.Join(root, "staging", "crash-left")
	os.MkdirAll(orphan, 0700)
	os.WriteFile(filepath.Join(orphan, "data"), []byte("synthetic"), 0600)
	reopened, e := analysis.New(root, c, r)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if _, e = os.Stat(orphan); !os.IsNotExist(e) {
		t.Fatal("orphan staging retained", e)
	}
}

func TestReviewR01DeepResultLoadAndAllEvidenceExports(t *testing.T) {
	s, _, _ := setup(t, nil)
	r := models.New("profile_evidence", "synthetic")
	r.Metadata.SchemaVersion = "0.1.0"
	var tree any = map[string]any{"name": "leaf", "samples": 1, "children": []any{}}
	for i := 0; i < 256; i++ {
		tree = map[string]any{"name": "frame", "samples": 1, "children": []any{tree}}
	}
	r.Charts["flamegraph"] = tree
	raw, _ := json.Marshal(r)
	path := filepath.Join(t.TempDir(), "deep.json")
	os.WriteFile(path, raw, 0600)
	view, e := s.Load(context.Background(), path)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	for _, format := range []string{"json", "html", "csv", "report-pack"} {
		if out, e := s.Export(view.Handle, format); e != nil || len(out) == 0 {
			t.Fatal(format, e)
		}
	}
}

func TestReviewR13QueueAndExecutionHaveSeparateDeadlines(t *testing.T) {
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	var calls atomic.Int32
	s, path, _ := setup(t, func(d *ingestion.ImporterDescriptor) {
		factory := d.ParserFactory
		d.ParserFactory = func(raw json.RawMessage) ingestion.Parser {
			return ingestion.ParserFunc(func(ctx context.Context, source ingestion.Source, sink ingestion.RecordSink) (ingestion.ParseReport, error) {
				if calls.Add(1) <= 4 {
					entered <- struct{}{}
					select {
					case <-release:
					case <-ctx.Done():
						return ingestion.ParseReport{}, ctx.Err()
					}
				}
				return factory(raw).Parse(ctx, source, sink)
			})
		}
	})
	active := []analysis.JobSnapshot{}
	for i := 0; i < 4; i++ {
		active = append(active, start(t, s, analysis.Request{Path: path, Importer: "tomcat"}))
	}
	for i := 0; i < 4; i++ {
		<-entered
	}
	queued := start(t, s, analysis.Request{Path: path, Importer: "tomcat", Budgets: ingestion.Budgets{TimeoutMS: 1000, QueueTimeoutMS: 4000}})
	expired := start(t, s, analysis.Request{Path: path, Importer: "tomcat", Budgets: ingestion.Budgets{TimeoutMS: 1000, QueueTimeoutMS: 20}})
	expiry := wait(t, s, expired)
	if expiry.State != "failed" || expiry.Problem.Code != "IMPORT_QUEUE_TIMEOUT" || !expiry.Problem.Retryable {
		t.Fatal(expiry)
	}
	time.Sleep(1100 * time.Millisecond)
	current, e := s.Get(queued.ID, 1)
	if e != nil || current.State != "queued" {
		t.Fatal(current, e)
	}
	close(release)
	done := wait(t, s, queued)
	if done.State != "completed" {
		t.Fatal(done)
	}
	for _, j := range active {
		if done := wait(t, s, j); done.State != "completed" {
			t.Fatal(done)
		}
	}
}

func TestReviewR05R10RequestedSelectionAndLegacyKeyParity(t *testing.T) {
	s, path, _ := setup(t, nil)
	for _, options := range []struct {
		raw    string
		legacy legacy.Options
	}{
		{`{}`, legacy.Options{}}, {`{"max_lines":1}`, legacy.Options{MaxLines: 1}}, {`{"strict":true,"top_n":1}`, legacy.Options{Strict: true, TopN: 1}},
	} {
		job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat", Options: json.RawMessage(options.raw)}))
		if job.State != "completed" {
			t.Fatal(job)
		}
		_, result, e := s.Read(job.ResultHandle)
		if e != nil {
			t.Fatal(e)
		}
		previous, e := legacy.Analyze(path, "tomcat", options.legacy)
		if e != nil {
			t.Fatal(e)
		}
		for name, pair := range map[string][2]any{"summary": {result.Summary, previous.Summary}, "tables": {result.Tables, previous.Tables}, "correlation_keys": {result.Metadata.Extra["correlation_keys"], previous.Metadata.Extra["correlation_keys"]}} {
			if stringMustJSON(t, pair[0]) != stringMustJSON(t, pair[1]) {
				t.Fatal(name, stringMustJSON(t, pair[0]), stringMustJSON(t, pair[1]))
			}
		}
		if options.legacy.MaxLines > 0 && result.Metadata.Extra["import_coverage"] == nil {
			t.Fatal("selection coverage missing")
		}
	}
	// A trailing empty line never fabricates another record under either path.
	raw, _ := os.ReadFile(path)
	os.WriteFile(path, append(raw, '\n'), 0600)
	job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat"}))
	if job.State != "completed" {
		t.Fatal(job)
	}
	_, r, e := s.Read(job.ResultHandle)
	if e != nil || r.Summary["total_events"] != json.Number("3") {
		t.Fatal(e, r.Summary)
	}
}

func TestReviewR02ActiveJobSurvivesHistoryPressure(t *testing.T) {
	entered := make(chan struct{})
	s, path, _ := setup(t, func(d *ingestion.ImporterDescriptor) {
		factory := d.ParserFactory
		d.ParserFactory = func(options json.RawMessage) ingestion.Parser {
			if strings.Contains(string(options), `"strict":true`) {
				return ingestion.ParserFunc(func(ctx context.Context, _ ingestion.Source, _ ingestion.RecordSink) (ingestion.ParseReport, error) {
					close(entered)
					<-ctx.Done()
					return ingestion.ParseReport{}, ctx.Err()
				})
			}
			return factory(options)
		}
	})
	active := start(t, s, analysis.Request{Path: path, Importer: "tomcat", Options: json.RawMessage(`{"strict":true}`)})
	<-entered
	for i := 0; i < 200; i++ {
		j := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat", Options: json.RawMessage(`{"unknown":true}`)}))
		if j.State != "failed" {
			t.Fatal(j)
		}
	}
	current, e := s.Get(active.ID, 1)
	if e != nil || current.State != "parsing" {
		t.Fatal("active job evicted", current, e)
	}
	s.Cancel(active.ID, 1)
	if j := wait(t, s, active); j.State != "cancelled" {
		t.Fatal(j)
	}
}

func TestReviewR09RetryabilityAndSanitizedValidationFailure(t *testing.T) {
	s, path, _ := setup(t, func(d *ingestion.ImporterDescriptor) {
		d.ValidateInput = func(context.Context, ingestion.Source) error { return errors.New("private/source/token") }
	})
	j := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat"}))
	if j.State != "failed" || j.Problem.Code != "IMPORT_IO_FAILED" || !j.Problem.Retryable || strings.Contains(j.Problem.Message, "private/source") {
		t.Fatal(j)
	}
}
