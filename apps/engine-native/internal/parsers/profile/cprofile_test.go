package profile_test

import (
	"context"
	"encoding/json"
	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	csv "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/csv"
	html "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	models "github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActualCProfileFlatAndCommonResult(t *testing.T) {
	body, e := os.ReadFile("testdata/cprofile/actual.json")
	if e != nil {
		t.Fatal(e)
	}
	s, e := parser.ParseSourceBytes(context.Background(), body, parser.SourceOptions{Format: "cprofile-flat-json"})
	if e != nil {
		t.Fatal(e)
	}
	var raw struct {
		Functions []struct {
			Self string `json:"self_seconds"`
		} `json:"functions"`
	}
	json.Unmarshal(body, &raw)
	want := new(big.Rat)
	for _, f := range raw.Functions {
		n, _ := new(big.Rat).SetString(f.Self)
		want.Add(want, n)
	}
	v := tree(t, s, []string{"profile-0"}, "function-self-seconds")
	got, e := m.Number(v.Root.Inclusive)
	if e != nil || got.Cmp(want) != 0 {
		t.Fatal(v.Root.Inclusive, want)
	}
	if s.ProducerVersion != "3.14.4" || s.Profiles[0].Clock != "aggregate" || s.Profiles[0].Samples[0].Timestamp != nil || len(s.Stacks[0].Frames) != 1 {
		t.Fatal("invented stack/time")
	}
	r, e := analyzer.BuildSet(context.Background(), s, m.Selection{ProfileIDs: []string{"profile-0"}, MetricID: "function-self-seconds"}, "actual cProfile")
	if e != nil {
		t.Fatal(e)
	}
	if e = analyzer.ValidateModelResult(r); e != nil {
		t.Fatal(e)
	}
	for _, exp := range []struct {
		name  string
		write func(string, any) error
	}{{"result.csv", csv.Write}, {"result.html", html.Write}} {
		if e = exp.write(filepath.Join(t.TempDir(), exp.name), r); e != nil {
			t.Fatal(e)
		}
	}
	saved, _ := json.Marshal(r)
	var restored models.AnalysisResult
	if e = json.Unmarshal(saved, &restored); e != nil {
		t.Fatal(e)
	}
	if e = analyzer.ValidateModelResult(restored); e != nil {
		t.Fatal(e)
	}
	var round map[string]any
	if e = json.Unmarshal(saved, &round); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(string(body), "3.14.4", "9.9.9", 1), strings.Replace(string(body), `"self_seconds": "`, `"self_seconds": "-`, 1)} {
		if _, e = parser.ParseSourceBytes(context.Background(), []byte(bad), parser.SourceOptions{Format: "cprofile-flat-json"}); e == nil {
			t.Fatal("accepted invalid input")
		}
	}
}
