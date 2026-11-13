package inference_test

import (
	"context"
	"encoding/json"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/inference"
	csv "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/csv"
	html "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/inference"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

func TestActualOllamaFinalResponseSemantics(t *testing.T) {
	raw, e := os.ReadFile("testdata/ollama/actual.json")
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	s, e := p.ParseBytes(ctx, raw, "actual", p.Options{Format: "ollama-generate-final-json"})
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Metrics.Series) != 7 || len(s.Metrics.Points) != 14 || len(s.Events) != 2 {
		t.Fatal(len(s.Metrics.Series), len(s.Metrics.Points))
	}
	var f struct {
		Observations []struct {
			Elapsed string `json:"client_elapsed_ns"`
		}
	}
	json.Unmarshal(raw, &f)
	rows, e := m.Intervals(ctx, s.Metrics)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		if r.Name == "ollama.client_elapsed_ns" {
			n, _ := new(big.Rat).SetString(f.Observations[0].Elapsed)
			n.Quo(n, big.NewRat(1000000000, 1))
			want, _ := m.Number(*r.Value)
			if want.Cmp(n) != 0 {
				t.Fatal(r)
			}
			f.Observations = f.Observations[1:]
		}
		if r.Rate != nil {
			t.Fatal("inferred cumulative rate")
		}
	}
	r, e := a.Build(ctx, s, "actual")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Validate(r); e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(r.Tables["requests"])
	var req []a.Request
	json.Unmarshal(b, &req)
	for _, v := range req {
		if v.TTFT != nil || v.TPOT != nil || v.Boundary != "client" {
			t.Fatal("fabricated token timing")
		}
	}
	if e = csv.Write(filepath.Join(t.TempDir(), "r.csv"), r); e != nil {
		t.Fatal(e)
	}
	if e = html.Write(filepath.Join(t.TempDir(), "r.html"), r); e != nil {
		t.Fatal(e)
	}
	var missing map[string]any
	json.Unmarshal(raw, &missing)
	obs := missing["observations"].([]any)
	metrics := obs[0].(map[string]any)["metrics"].(map[string]any)
	delete(metrics, "eval_count")
	missingRaw, _ := json.Marshal(missing)
	without, e := p.ParseBytes(ctx, missingRaw, "missing", p.Options{Format: "ollama-generate-final-json"})
	if e != nil || without.Events[0].OutputTokens != nil {
		t.Fatal("missing became zero", e)
	}
	metrics["eval_count"] = "0"
	zeroRaw, _ := json.Marshal(missing)
	zero, e := p.ParseBytes(ctx, zeroRaw, "zero", p.Options{Format: "ollama-generate-final-json"})
	if e != nil || zero.Events[0].OutputTokens == nil || *zero.Events[0].OutputTokens != "0" {
		t.Fatal("zero lost", e)
	}
	var root map[string]any
	json.Unmarshal(raw, &root)
	root["producer_version"] = "unknown"
	bad, _ := json.Marshal(root)
	if _, e = p.ParseBytes(ctx, bad, "bad", p.Options{Format: "ollama-generate-final-json"}); e == nil {
		t.Fatal("unqualified version accepted")
	}
}
