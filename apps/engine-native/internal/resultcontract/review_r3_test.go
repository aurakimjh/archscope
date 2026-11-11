package resultcontract_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	c "github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func TestR3ComparisonRefusalCodesAndJSONCauses(t *testing.T) {
	registry, a := fixture(t)
	descriptor := measurement()
	descriptor.Population = "all"
	a.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/value": descriptor}
	a.Metadata.Extra["population_ids"] = []string{"all"}
	policy := c.PopulationComparison{Version: "1.0.0", SelectionHash: strings.Repeat("b", 64), CoverageKind: "complete_records"}
	identity := c.PopulationIdentity{SourceHashes: []string{strings.Repeat("a", 64)}, SelectionHash: strings.Repeat("b", 64), SemanticRevision: "fixture@1.0.0", Coverage: "records:0", Comparison: &policy}
	a.Metadata.Extra["populations"] = map[string]c.PopulationIdentity{"all": identity}
	for _, tc := range []struct {
		code    string
		edit    func(*c.PopulationIdentity)
		partial bool
	}{
		{c.ComparisonSelectionMismatch, func(i *c.PopulationIdentity) { i.Comparison.SelectionHash = strings.Repeat("d", 64) }, false},
		{c.ComparisonCoverageMismatch, func(i *c.PopulationIdentity) { i.Comparison.CoverageKind = "selected_records:max_lines" }, false},
		{c.ComparisonPartialNotComparable, func(i *c.PopulationIdentity) { i.Comparison = nil }, true},
		{c.ComparisonSemanticRevisionMismatch, func(i *c.PopulationIdentity) { i.SemanticRevision = "fixture@2.0.0" }, false},
		{c.ComparisonPolicyMissing, func(i *c.PopulationIdentity) { i.Comparison = nil }, false},
		{c.ComparisonPolicyUnsupported, func(i *c.PopulationIdentity) { i.Comparison.Version = "2.0.0" }, false},
		{c.ComparisonIdentityMissing, func(i *c.PopulationIdentity) { i.SourceHashes = nil }, false},
		{c.ComparisonIdentityInvalid, func(i *c.PopulationIdentity) { i.SourceHashes = []string{"not-a-hash"} }, false},
	} {
		t.Run(tc.code, func(t *testing.T) {
			raw, _ := json.Marshal(a)
			b, e := c.Decode(raw)
			if e != nil {
				t.Fatal(e)
			}
			p := policy
			target := identity
			target.Comparison = &p
			target.SourceHashes = []string{strings.Repeat("c", 64)}
			tc.edit(&target)
			b.Metadata.Extra["populations"] = map[string]c.PopulationIdentity{"all": target}
			if tc.partial {
				b.Metadata.Extra["import_coverage"] = map[string]any{"partial": true}
			}
			delta, e := registry.Compare(a, b, "/summary/value")
			var refusal *c.ComparisonError
			if !errors.As(e, &refusal) || refusal.Code != tc.code || delta.Value != nil {
				t.Fatal(delta, e)
			}
			// The pinned Wails default marshaler uses json.Marshal(&err), so codes
			// survive as CallError.cause.code rather than requiring string parsing.
			cause, err := json.Marshal(&e)
			if err != nil {
				t.Fatal(err)
			}
			var wire struct{ Code, Message string }
			if err = json.Unmarshal(cause, &wire); err != nil || wire.Code != tc.code || wire.Message == "" {
				t.Fatal(string(cause), err)
			}
		})
	}
	delta, e := registry.Compare(a, a, "/summary/value")
	if e != nil || delta.Comparison == nil || delta.Comparison.IdentityBasis != "declared" {
		t.Fatal(delta, e)
	}
}
