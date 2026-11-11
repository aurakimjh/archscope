package analysis_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	pp "github.com/google/pprof/profile"
)

func importModel(t *testing.T, s *analysis.Service, metric string) analysis.JobSnapshot {
	t.Helper()
	body, e := catalog.Contracts.ReadFile("contracts/profile-set.json")
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "model.json")
	os.WriteFile(path, body, 0600)
	options, _ := json.Marshal(profilemodel.Selection{ProfileIDs: []string{"worker-a", "worker-b"}, MetricID: metric})
	job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "profile-set", Options: options}))
	if job.State != "completed" {
		t.Fatal(job)
	}
	return job
}
func TestT629CanonicalImportSaveReadQueryExportAndTamperRefusal(t *testing.T) {
	s, _, root := setup(t, nil)
	job := importModel(t, s, "duration")
	raw, result, e := s.Read(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	if result.Type != "profile_evidence" || result.Metadata.SchemaVersion != "0.2.0" || result.Summary["total_weight"] != "0.35" || result.Summary["total_samples"] != nil {
		t.Fatal(result.Summary)
	}
	tree, e := s.ProfileTree(context.Background(), job.ResultHandle)
	if e != nil || tree.Root.Inclusive != "0.35" || tree.SampleRecords != 3 {
		t.Fatal(tree, e)
	}
	scalar, e := s.Scalar(job.ResultHandle, "/summary/total_weight")
	if e != nil || scalar.Value == nil || *scalar.Value != "0.35" || scalar.Unit != "seconds" {
		t.Fatal(scalar, e)
	}
	unknown, e := s.Scalar(job.ResultHandle, "/summary/total_samples")
	if e != nil || unknown.Value != nil || unknown.Availability != "unavailable" {
		t.Fatal(unknown, e)
	}
	for _, format := range []string{"json", "csv", "html", "report-pack"} {
		body, e := s.Export(job.ResultHandle, format)
		if e != nil || len(body) == 0 {
			t.Fatal(format, e)
		}
		if format == "json" && !bytes.Equal(body, raw) {
			t.Fatal("source snapshot changed")
		}
		if format == "report-pack" {
			archive, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, entry := range archive.File {
				if strings.HasPrefix(entry.Name, "sources/") {
					reader, _ := entry.Open()
					data, _ := io.ReadAll(reader)
					reader.Close()
					if !bytes.Equal(data, raw) {
						t.Fatal("pack lost canonical model")
					}
					found = true
				}
			}
			if !found {
				t.Fatal("missing source appendix")
			}
		}
	}
	output := filepath.Join(t.TempDir(), "saved.json")
	if e = analysis.AtomicExport(output, raw); e != nil {
		t.Fatal(e)
	}
	view, e := s.Load(context.Background(), output)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	delta, e := s.ProfileCompare(context.Background(), job.ResultHandle, view.Handle)
	if e != nil {
		t.Fatal(e)
	}
	for _, row := range delta.Rows {
		if row.Delta != "0" {
			t.Fatal(row)
		}
	}
	pprof, e := s.ProfilePprof(context.Background(), view.Handle, "0.01")
	if e != nil {
		t.Fatal(e)
	}
	parsed, e := pp.ParseData(pprof)
	if e != nil {
		t.Fatal(e)
	}
	sum := int64(0)
	zero := false
	for _, sample := range parsed.Sample {
		sum += sample.Value[0]
		zero = zero || sample.Value[0] == 0
	}
	if sum != 35 || !zero || len(parsed.Sample) != 3 {
		t.Fatal(parsed)
	}
	if _, e = s.ProfilePprof(context.Background(), view.Handle, "1"); e == nil {
		t.Fatal("lossy pprof export accepted")
	}
	stages, e := s.ProfileDrilldown(context.Background(), view.Handle, []profilemodel.PathFilter{{Path: []string{tree.Root.Children[0].FrameID}, Match: "prefix"}})
	if e != nil || len(stages) != 2 || stages[1].Tree.Root.Inclusive != "0.35" {
		t.Fatal(stages, e)
	}
	// Preserve optional unknown metadata byte-for-byte, but do not trust forged values.
	result.Metadata.Extra["future_optional"] = json.RawMessage(`{"large":9007199254740993,"precise":0.12345678901234567890123456789}`)
	body, _ := json.Marshal(result)
	os.WriteFile(output, body, 0600)
	future, e := s.Load(context.Background(), output)
	if e != nil || !future.Compatibility.Allowed {
		t.Fatal(future, e)
	}
	again, e := s.Export(future.Handle, "json")
	if e != nil || !bytes.Equal(again, body) {
		t.Fatal("unknown metadata changed", e)
	}
	result.Summary["total_weight"] = "99"
	forged, _ := json.Marshal(result)
	os.WriteFile(output, forged, 0600)
	bad, e := s.Load(context.Background(), output)
	if e != nil || bad.Compatibility.Allowed || !bad.Compatibility.ReadOnly {
		t.Fatal(bad, e)
	}
	if _, e = s.ProfileTree(context.Background(), bad.Handle); e == nil {
		t.Fatal("forged projection activated")
	}
	retained, e := s.Export(bad.Handle, "raw-json")
	if e != nil || !bytes.Equal(retained, forged) {
		t.Fatal("original unsupported snapshot lost", e)
	}
	clean(t, root)
}
func TestT629LargeIntegerCountsAndUnavailableSelection(t *testing.T) {
	s, _, _ := setup(t, nil)
	large := importModel(t, s, "memory")
	tree, e := s.ProfileTree(context.Background(), large.ResultHandle)
	if e != nil || tree.Root.Inclusive != "9007199254740993" {
		t.Fatal(tree, e)
	}
	value, e := s.Scalar(large.ResultHandle, "/summary/total_weight")
	if e != nil || value.Value == nil || *value.Value != "9007199254740993" {
		t.Fatal(value, e)
	}
	counts := importModel(t, s, "samples")
	count, e := s.Scalar(counts.ResultHandle, "/summary/total_samples")
	if e != nil || count.Value == nil || *count.Value != "5" {
		t.Fatal(count, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = s.ProfileTree(ctx, large.ResultHandle); e == nil {
		t.Fatal("cancelled query completed")
	}
	// Legacy payload numbers alone never become a ProfileSet by relabeling.
	_, r, e := s.Read(counts.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	delete(r.Metadata.Extra, "profile_core")
	body, _ := json.Marshal(r)
	path := filepath.Join(t.TempDir(), "relabel.json")
	os.WriteFile(path, body, 0600)
	view, e := s.Load(context.Background(), path)
	if e != nil || view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
}
