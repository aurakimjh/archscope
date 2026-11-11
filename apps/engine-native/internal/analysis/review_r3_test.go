package analysis_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func TestR3WhitespaceSuffixKeepsCompleteCoverage(t *testing.T) {
	s, path, _ := setup(t, nil)
	source, _ := os.ReadFile(path)
	options := json.RawMessage(`{"max_lines":3}`)
	first := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat", Options: options}))
	for _, tc := range []struct {
		name, suffix string
		selected     bool
	}{
		{"exact EOF", "", false}, {"final newline", "\n", false}, {"blank line", "\n\n", false}, {"CRLF blank", "\n\r\n", false},
		{"Unicode whitespace", "\n\t \r\n\u2003\n", false},
		// Cross the parser's 1 MiB buffer boundary inside Unicode whitespace.
		{"large whitespace tail", "\n" + strings.Repeat("\u2003", (1<<20)/3+10), false},
		{"nonblank suffix", "\n \t\nnot parsed after max_lines", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "tail.log")
			os.WriteFile(file, []byte(strings.TrimSuffix(string(source), "\n")+tc.suffix), 0600)
			job := wait(t, s, start(t, s, analysis.Request{Path: file, Importer: "tomcat", Options: options}))
			if job.State != "completed" {
				t.Fatal(job)
			}
			_, r, e := s.Read(job.ResultHandle)
			if e != nil {
				t.Fatal(e)
			}
			_, selected := r.Metadata.Extra["import_coverage"]
			if selected != tc.selected {
				t.Fatal(r.Metadata.Extra)
			}
			if r.Summary["total_events"] != json.Number("3") {
				t.Fatal(r.Summary)
			}
			delta, e := s.Compare(first.ResultHandle, job.ResultHandle, "/summary/total_events")
			if tc.selected {
				var refusal *resultcontract.ComparisonError
				if !errors.As(e, &refusal) || refusal.Code != resultcontract.ComparisonCoverageMismatch {
					t.Fatal(e)
				}
			} else if e != nil || delta.Value == nil || *delta.Value != "0" {
				t.Fatal(delta, e)
			}
		})
	}
}

func TestR3EditedSavedIdentityRemainsDeclared(t *testing.T) {
	s, path, _ := setup(t, nil)
	baseline := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat"}))
	selected := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "tomcat", Options: json.RawMessage(`{"max_lines":2}`)}))
	raw, base, _ := s.Read(baseline.ResultHandle)
	_ = raw
	_, target, e := s.Read(selected.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	// Deliberately forge plausible identity/policy declarations in a saved local
	// file. A result digest identifies these bytes, not their authenticity.
	populations := base.Metadata.Extra["populations"].(map[string]any)
	population := populations["imported_records"].(map[string]any)
	population["source_sha256"] = []string{strings.Repeat("f", 64)}
	target.Metadata.Extra["populations"] = populations
	target.Metadata.Extra["identity_basis"] = "verified_by_reimport"
	edited, e := json.Marshal(target)
	if e != nil {
		t.Fatal(e)
	}
	saved := filepath.Join(t.TempDir(), "edited.json")
	os.WriteFile(saved, edited, 0600)
	view, e := s.Load(context.Background(), saved)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	delta, e := s.Compare(baseline.ResultHandle, view.Handle, "/summary/total_events")
	if e != nil || delta.Value == nil || *delta.Value != "-1" || delta.Comparison == nil || delta.Comparison.IdentityBasis != "declared" {
		t.Fatal(delta, e)
	}
	sum := sha256.Sum256(edited)
	if delta.Comparison.Target.ResultSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("saved digest changed")
	}
	output, e := s.Export(view.Handle, "json")
	if e != nil || string(output) != string(edited) {
		t.Fatal("loaded source bytes changed", e)
	}
}
