package profilemodel_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	m "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
)

func TestR4QualityMeasurementsDoNotBlockComparison(t *testing.T) {
	set, selection := fixture(t)
	text := func(s string) *string { return &s }
	set.Quality.SymbolResolution = text("0.50")
	formatOnly := copySet(t, set)
	formatOnly.Quality.Lost = text("0.00")
	formatOnly.Quality.SymbolResolution = text("0.500")
	unchanged, e := m.Compare(context.Background(), set, selection, formatOnly, selection)
	if e != nil || len(unchanged.QualityDifferences) != 0 || len(unchanged.Warnings) != 0 {
		t.Fatal("formatting changed measured quality", unchanged, e)
	}
	other := copySet(t, set)
	other.SourceHashes = []string{strings.Repeat("e", 64)}
	other.Quality.Lost = text("3")
	other.Quality.Unknown = text("2")
	other.Quality.Truncated = text("0")
	other.Quality.SymbolResolution = text("0.90")
	diff, e := m.Compare(context.Background(), set, selection, other, selection)
	if e != nil || len(diff.QualityDifferences) != 4 || len(diff.Warnings) != 1 || diff.Warnings[0] != "quality_measurements_differ" {
		t.Fatal(diff, e)
	}
	if diff.Baseline.Quality.Lost == nil || *diff.Baseline.Quality.Lost != "0" || diff.Target.Quality.Lost == nil || *diff.Target.Quality.Lost != "3" || diff.Baseline.Quality.SymbolResolution == nil || *diff.Baseline.Quality.SymbolResolution != "0.50" {
		t.Fatal(diff)
	}
	*diff.Target.Quality.Lost = "999"
	if *other.Quality.Lost != "3" {
		t.Fatal("comparison response aliases model")
	}
	if set.ComparisonIdentity(selection) != other.ComparisonIdentity(selection) || set.SelectionIdentity(selection) == other.SelectionIdentity(selection) {
		t.Fatal("conditions/provenance not separated")
	}
	for _, tc := range []struct {
		code string
		edit func(*m.ProfileSet, *m.Selection)
	}{
		{"entity_mapping_required", func(s *m.ProfileSet, _ *m.Selection) { s.Profiles[0].Process = "pid-200" }},
		{"quality_class_mismatch", func(s *m.ProfileSet, _ *m.Selection) { s.Quality.ClockConfidence = "untrusted" }},
		{"quality_class_mismatch", func(s *m.ProfileSet, _ *m.Selection) { s.Quality.SymbolResolution = text("0") }},
		{"capture_options_mismatch", func(s *m.ProfileSet, _ *m.Selection) { s.CaptureOptions = json.RawMessage(`{"other":true}`) }},
		{"selection_mismatch", func(_ *m.ProfileSet, s *m.Selection) { s.Path = []string{set.Frames[0].ID}; s.Match = "contains" }},
		{"semantic_revision_mismatch", func(s *m.ProfileSet, _ *m.Selection) { s.ProducerVersion = "other" }},
	} {
		t.Run(tc.code, func(t *testing.T) {
			target := copySet(t, other)
			selected := m.CopySelection(selection)
			tc.edit(&target, &selected)
			_, e := m.Compare(context.Background(), set, selection, target, selected)
			var refusal *m.ComparisonError
			if !errors.As(e, &refusal) || refusal.Code != tc.code {
				t.Fatal(e)
			}
		})
	}
	tree, e := m.BuildTree(context.Background(), set, selection)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(tree.Root)
	if e != nil || len(raw) > m.MaxTreeBytes {
		t.Fatal(e, len(raw))
	}
}

func TestR4BudgetRefusalsIdentifyResourceAndCeiling(t *testing.T) {
	for _, tc := range []struct {
		resource string
		ceiling  int64
		edit     func(*m.ProfileSet)
	}{
		{"metrics", 64, func(s *m.ProfileSet) { s.Metrics = make([]m.Metric, 65) }},
		{"stack_depth", m.MaxDepth, func(s *m.ProfileSet) { s.Stacks[0].Frames = make([]string, m.MaxDepth+1) }},
		{"samples", m.MaxSamples, func(s *m.ProfileSet) { s.Profiles[0].Samples = make([]m.Sample, m.MaxSamples+1) }},
	} {
		t.Run(tc.resource, func(t *testing.T) {
			set, _ := fixture(t)
			tc.edit(&set)
			e := set.Validate()
			var refusal *m.BudgetError
			if !errors.As(e, &refusal) || refusal.Resource != tc.resource || refusal.Actual <= tc.ceiling || refusal.Limit != tc.ceiling {
				t.Fatal(e)
			}
		})
	}
}
