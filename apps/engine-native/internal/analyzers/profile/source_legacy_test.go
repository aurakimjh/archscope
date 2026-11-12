package profile_test

import (
	"encoding/json"
	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"os"
	"strings"
	"testing"
)

func TestR1101LegacySourceSnapshotKeepsHashesAndRequiresReimportForCompare(t *testing.T) {
	raw, e := os.ReadFile("testdata/source-0.3.0.saved.json")
	if e != nil {
		t.Fatal(e)
	}
	result, e := resultcontract.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	core, e := analyzer.ReadCore(result)
	if e != nil {
		t.Fatal(e)
	}
	if e = analyzer.ValidateModelResult(result); e != nil {
		t.Fatal(e)
	}
	var populations map[string]resultcontract.PopulationIdentity
	body, _ := json.Marshal(result.Metadata.Extra["populations"])
	json.Unmarshal(body, &populations)
	if strings.TrimPrefix(core.Model.StrictComparisonIdentity(core.Selection), "population-") != populations["selected"].Comparison.SelectionHash {
		t.Fatal("rewrote saved v2 comparison hash")
	}
	registry, e := catalog.ResultRegistry()
	if e != nil {
		t.Fatal(e)
	}
	if !registry.Check(result, "compute").Allowed || !registry.Check(result, "save").Allowed || registry.Check(result, "compare").Allowed {
		t.Fatal("legacy snapshot read/save/compare gate")
	}
	if core.Model.Metrics[0].SourceType != "" {
		t.Fatal("relabeled old metric")
	}
}
