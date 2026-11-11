package analysis_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
)

func TestR2N01DifferentLogSourcesNonzeroDeltaAndSavedReferences(t *testing.T) {
	s, path, _ := setup(t, nil)
	first := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat"}))
	body, _ := os.ReadFile(path)
	targetPath := filepath.Join(t.TempDir(), "after.log")
	os.WriteFile(targetPath, append(body, []byte("11-Nov-2026 10:00:03.000 ERROR [worker] org.apache.catalina.core.ContainerBase Synthetic later failure\n")...), 0600)
	second := wait(t, s, start(t, s, analysis.Request{Path: targetPath, Importer: "tomcat"}))
	if first.State != "completed" || second.State != "completed" {
		t.Fatal(first, second)
	}
	delta, e := s.Compare(first.ResultHandle, second.ResultHandle, "/summary/total_events")
	if e != nil || delta.Value == nil || *delta.Value != "1" || delta.Percent == nil || *delta.Percent != "100/3" || delta.Comparison == nil {
		t.Fatal(delta, e)
	}
	refs := delta.Comparison
	if refs.Mode != "baseline_target" || refs.Baseline.Handle != first.ResultHandle || refs.Target.Handle != second.ResultHandle || refs.Baseline.SourceHashes[0] == refs.Target.SourceHashes[0] {
		t.Fatal(refs)
	}
	for _, side := range []struct{ handle, hash string }{{first.ResultHandle, refs.Baseline.ResultSHA256}, {second.ResultHandle, refs.Target.ResultSHA256}} {
		raw, _, e := s.Read(side.handle)
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(raw)
		if side.hash != hex.EncodeToString(sum[:]) {
			t.Fatal("reference did not hash exact snapshot")
		}
	}
	raw, e := s.Export(second.ResultHandle, "json")
	if e != nil {
		t.Fatal(e)
	}
	saved := filepath.Join(t.TempDir(), "saved.json")
	os.WriteFile(saved, raw, 0600)
	view, e := s.Load(context.Background(), saved)
	if e != nil {
		t.Fatal(e)
	}
	reread, e := s.Compare(first.ResultHandle, view.Handle, "/summary/error_count")
	if e != nil || reread.Value == nil || *reread.Value != "1" {
		t.Fatal(reread, e)
	}
	changed := wait(t, s, start(t, s, analysis.Request{Path: targetPath, Importer: "tomcat", Options: json.RawMessage(`{"max_lines":1}`)}))
	if _, e = s.Compare(first.ResultHandle, changed.ResultHandle, "/summary/total_events"); e == nil {
		t.Fatal("different selection accepted")
	}
	partial := wait(t, s, start(t, s, analysis.Request{Path: targetPath, Importer: "tomcat", Budgets: ingestion.Budgets{Records: 1}, AllowPartial: true}))
	if partial.State != "partial" {
		t.Fatal(partial)
	}
	if _, e = s.Compare(first.ResultHandle, partial.ResultHandle, "/summary/total_events"); e == nil {
		t.Fatal("partial coverage compared as complete")
	}
}

func TestR2N03MaxLinesExactEOF(t *testing.T) {
	s, path, _ := setup(t, nil)
	body, _ := os.ReadFile(path)
	for _, trailing := range []bool{true, false} {
		for _, lines := range []int{2, 3, 4} {
			t.Run(stringMustJSON(t, []any{trailing, lines}), func(t *testing.T) {
				content := body
				if !trailing {
					content = []byte(strings.TrimSuffix(string(body), "\n"))
				}
				p := filepath.Join(t.TempDir(), "lines.log")
				os.WriteFile(p, content, 0600)
				job := wait(t, s, start(t, s, analysis.Request{Path: p, Importer: "tomcat", Options: json.RawMessage(stringMustJSON(t, map[string]int{"max_lines": lines}))}))
				if job.State != "completed" {
					t.Fatal(job)
				}
				_, r, e := s.Read(job.ResultHandle)
				if e != nil {
					t.Fatal(e)
				}
				_, selected := r.Metadata.Extra["import_coverage"]
				if selected != (lines < 3) {
					t.Fatal("selection does not match excluded suffix", r.Metadata.Extra)
				}
				expected := 3
				if lines < expected {
					expected = lines
				}
				if r.Summary["total_events"] != json.Number(stringMustJSON(t, expected)) {
					t.Fatal(r.Summary)
				}
			})
		}
	}
}

func TestR2N01ProfileCompareAndScalarRoundtripWithOldSnapshots(t *testing.T) {
	s, _, _ := setup(t, nil)
	baseline := importModel(t, s, "duration")
	body, _ := catalog.Contracts.ReadFile("contracts/profile-set.json")
	set, e := profilemodel.Decode(body)
	if e != nil {
		t.Fatal(e)
	}
	set.SourceHashes = []string{strings.Repeat("e", 64)}
	for i := range set.Profiles {
		for j := range set.Profiles[i].Samples {
			sample := &set.Profiles[i].Samples[j]
			if sample.MetricID == "duration" && sample.Weight == "0.25" {
				sample.Weight = "0.30"
			}
		}
	}
	body, _ = json.Marshal(set)
	path := filepath.Join(t.TempDir(), "target-model.json")
	os.WriteFile(path, body, 0600)
	target := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "profile-set", Options: json.RawMessage(`{"profile_ids":["worker-a","worker-b"],"metric_id":"duration"}`)}))
	if target.State != "completed" {
		t.Fatal(target)
	}
	diff, e := s.ProfileCompare(context.Background(), baseline.ResultHandle, target.ResultHandle)
	if e != nil || diff.ComparisonMode != "baseline_target" || diff.Baseline.Handle != baseline.ResultHandle || diff.Target.Handle != target.ResultHandle {
		t.Fatal(diff, e)
	}
	found := false
	for _, row := range diff.Rows {
		if row.ID == "root" {
			found = true
			if row.Delta != "0.05" {
				t.Fatal(row)
			}
		}
	}
	if !found {
		t.Fatal("root missing")
	}
	scalar, e := s.Compare(baseline.ResultHandle, target.ResultHandle, "/summary/total_weight")
	if e != nil || scalar.Value == nil || *scalar.Value != "1/20" {
		t.Fatal(scalar, e)
	}
	// Pre-R2 model snapshots lack optional comparison policy. They stay readable,
	// exportable and usable by the model-derived profile comparator, without
	// acquiring source-independent scalar compatibility from labels.
	raw, _, _ := s.Read(baseline.ResultHandle)
	var document map[string]any
	json.Unmarshal(raw, &document)
	delete(document["metadata"].(map[string]any)["populations"].(map[string]any)["selected"].(map[string]any), "comparison")
	raw, _ = json.Marshal(document)
	saved := filepath.Join(t.TempDir(), "old-result.json")
	os.WriteFile(saved, raw, 0600)
	old, e := s.Load(context.Background(), saved)
	if e != nil || !old.Compatibility.Allowed {
		t.Fatal(old, e)
	}
	if _, e = s.Export(old.Handle, "json"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ProfileCompare(context.Background(), old.Handle, target.ResultHandle); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Compare(old.Handle, baseline.ResultHandle, "/summary/total_weight"); e != nil {
		t.Fatal("old/new same-input compatibility lost", e)
	}
	if _, e = s.Compare(old.Handle, target.ResultHandle, "/summary/total_weight"); e == nil {
		t.Fatal("old scalar policy inferred")
	}
}
