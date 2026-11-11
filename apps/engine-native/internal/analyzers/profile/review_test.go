package profile_test

import (
	"context"
	"strings"
	"testing"

	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	c "github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func TestR4OldComparisonPolicyValidatesWithoutRewriting(t *testing.T) {
	body, _ := catalog.Contracts.ReadFile("contracts/profile-set.json")
	set, e := m.Decode(body)
	if e != nil {
		t.Fatal(e)
	}
	selection := m.Selection{ProfileIDs: []string{"worker-a", "worker-b"}, MetricID: "duration"}
	result, e := analyzer.BuildSet(context.Background(), set, selection, "")
	if e != nil {
		t.Fatal(e)
	}
	pops := result.Metadata.Extra["populations"].(map[string]c.PopulationIdentity)
	item := pops["selected"]
	oldHash := strings.TrimPrefix(set.LegacyComparisonIdentity(selection), "population-")
	item.Comparison.SelectionHash = oldHash
	pops["selected"] = item
	if e = analyzer.ValidateModelResult(result); e != nil {
		t.Fatal("old snapshot rejected", e)
	}
	if pops["selected"].Comparison.SelectionHash != oldHash {
		t.Fatal("saved declaration rewritten")
	}
	item.Comparison.SelectionHash = strings.Repeat("f", 64)
	if e = analyzer.ValidateModelResult(result); e == nil {
		t.Fatal("forged comparison policy accepted")
	}
}
