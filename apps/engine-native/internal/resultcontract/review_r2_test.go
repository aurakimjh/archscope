package resultcontract_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	c "github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func TestR2N01SourceProvenanceSeparateFromCompatibility(t *testing.T) {
	registry, a := fixture(t)
	descriptor := measurement()
	descriptor.Population = "all"
	a.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/value": descriptor}
	a.Metadata.Extra["population_ids"] = []string{"all"}
	policy := c.PopulationComparison{Version: "1.0.0", SelectionHash: strings.Repeat("b", 64), CoverageKind: "complete_records"}
	identity := c.PopulationIdentity{SourceHashes: []string{strings.Repeat("a", 64)}, SelectionHash: strings.Repeat("b", 64), SemanticRevision: "fixture@1.0.0", Coverage: "records:0", Comparison: &policy}
	a.Metadata.Extra["populations"] = map[string]c.PopulationIdentity{"all": identity}
	raw, _ := json.Marshal(a)
	b, e := c.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	b.Summary["value"] = 1
	target := identity
	target.SourceHashes = []string{strings.Repeat("c", 64)}
	target.Coverage = "records:1"
	b.Metadata.Extra["populations"] = map[string]c.PopulationIdentity{"all": target}
	delta, e := registry.Compare(a, b, "/summary/value")
	if e != nil || delta.Value == nil || *delta.Value != "1" || delta.Reason != "zero_baseline" || delta.Comparison == nil {
		t.Fatal(delta, e)
	}
	refs := delta.Comparison
	if refs.Mode != "baseline_target" || refs.Baseline.SourceHashes[0] != identity.SourceHashes[0] || refs.Target.SourceHashes[0] != target.SourceHashes[0] || refs.Baseline.ResultSHA256 == refs.Target.ResultSHA256 {
		t.Fatal(refs)
	}
	for name, edit := range map[string]func(*c.PopulationIdentity){
		"selection":          func(i *c.PopulationIdentity) { i.Comparison.SelectionHash = strings.Repeat("d", 64) },
		"coverage":           func(i *c.PopulationIdentity) { i.Comparison.CoverageKind = "selected_records:max_lines" },
		"unknown policy":     func(i *c.PopulationIdentity) { i.Comparison.Version = "2.0.0" },
		"semantic revision":  func(i *c.PopulationIdentity) { i.SemanticRevision = "fixture@2.0.0" },
		"old snapshot":       func(i *c.PopulationIdentity) { i.Comparison = nil },
		"missing provenance": func(i *c.PopulationIdentity) { i.SourceHashes = nil },
	} {
		t.Run(name, func(t *testing.T) {
			p := policy
			changed := target
			changed.Comparison = &p
			edit(&changed)
			b.Metadata.Extra["populations"] = map[string]c.PopulationIdentity{"all": changed}
			if _, e := registry.Compare(a, b, "/summary/value"); e == nil {
				t.Fatal("incompatible comparison allowed")
			}
		})
	}
}

func TestR2N04ProjectionEntryAndByteBudgets(t *testing.T) {
	_, r := fixture(t)
	d := measurement()
	r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/tables/rows": {Columns: map[string]c.Measurement{"weight": d, "other": d}}}
	// Refuse before cloning/multiplying this bounded input into availability maps.
	r.Tables["rows"] = make([]map[string]any, c.MaxProjectionEntries/2+1)
	for _, html := range []bool{false, true} {
		_, e := c.ExportPresentation(r, html)
		var budget *c.ProjectionBudgetError
		if !errors.As(e, &budget) || budget.Resource != "entry count" {
			t.Fatal(e)
		}
	}
	d.Availability = "unavailable"
	d.Reason = strings.Repeat("x", 1<<15)
	r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/tables/rows": {Columns: map[string]c.Measurement{"weight": d}}}
	rows := make([]map[string]any, 600)
	for i := range rows {
		rows[i] = map[string]any{"weight": nil}
	}
	r.Tables["rows"] = rows
	before, _ := json.Marshal(r)
	for _, html := range []bool{false, true} {
		out, e := c.ExportPresentation(r, html)
		var budget *c.ProjectionBudgetError
		if !errors.As(e, &budget) || budget.Resource != "byte count" || out.Series["measurement_availability"] != nil {
			t.Fatal(e)
		}
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("failed export mutated canonical input")
	}
}
