package inference_test

import (
	"bytes"
	"context"
	"encoding/json"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/inference"
	history "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	csv "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/csv"
	html "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/inference"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const genaiBindings = `{"semantic_profile":"otel-genai-ollama.v1.37","producer":"opentelemetry-python","producer_version":"1.38.0","engine_version":"0.40.16","host_id":"declared-host","boot_id":"declared-lifetime"}`

func TestActualGenAIMetricsScopedAliasAndHistograms(t *testing.T) {
	raw, e := os.ReadFile("testdata/genai/actual.otlp.json")
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	o := p.Options{Format: "otlp-genai-ollama-json", Bindings: json.RawMessage(genaiBindings)}
	s, e := p.ParseBytes(ctx, raw, "actual", o)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Signals) != 3 || len(s.Events) != 0 || len(s.Metrics.Series) != 3 {
		t.Fatal(s)
	}
	for _, point := range s.Metrics.Points {
		if point.Histogram == nil || point.Histogram.Count != "2" || point.Value != nil {
			t.Fatal("lost raw histogram")
		}
	}
	var actual struct {
		Observations []struct {
			Metrics map[string]string `json:"metrics"`
		} `json:"observations"`
	}
	evidence, err := os.ReadFile("testdata/genai/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(evidence, &actual)
	expected := map[string]*big.Int{"input": new(big.Int), "output": new(big.Int)}
	for _, v := range actual.Observations {
		for kind, key := range map[string]string{"input": "prompt_eval_count", "output": "eval_count"} {
			n, e := m.Uint(v.Metrics[key])
			if e != nil {
				t.Fatal(e)
			}
			expected[kind].Add(expected[kind], n)
		}
	}
	for _, series := range s.Metrics.Series {
		if series.Name != "gen_ai.client.token.usage" {
			continue
		}
		var attrs map[string]map[string]string
		json.Unmarshal(series.Attributes, &attrs)
		kind := attrs["gen_ai.token.type"]["stringValue"]
		for _, point := range s.Metrics.Points {
			if point.SeriesID == series.ID {
				sum, e := m.Number(*point.Histogram.Sum)
				if e != nil || sum.Cmp(new(big.Rat).SetInt(expected[kind])) != 0 {
					t.Fatal("token count/sum mismatch")
				}
			}
		}
	}
	ids := []string{}
	for _, series := range s.Metrics.Series {
		ids = append(ids, series.ID)
	}
	generic, e := history.Build(ctx, s.Metrics, m.Selection{SeriesIDs: ids}, "raw metric")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(generic.Metadata.Extra["comparison_scope"].(string), "not qualified") {
		t.Fatal("enabled unsupported comparison")
	}
	rows, e := m.Intervals(ctx, s.Metrics)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		if r.Rate != nil || r.Delta != nil {
			t.Fatal("invented first cumulative predecessor")
		}
	}
	r, e := a.Build(ctx, s, "actual")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Validate(r); e != nil {
		t.Fatal(e)
	}
	for _, exp := range []struct {
		name  string
		write func(string, any) error
	}{{"r.csv", csv.Write}, {"r.html", html.Write}} {
		if e = exp.write(filepath.Join(t.TempDir(), exp.name), r); e != nil {
			t.Fatal(e)
		}
	}
	for _, bad := range []string{strings.Replace(string(raw), "https://opentelemetry.io/schemas/1.37.0", "https://opentelemetry.io/schemas/1.36.0", 1), strings.Replace(string(raw), "manual.ollama.genai", "other.instrumentation", 1), strings.ReplaceAll(string(raw), `"aggregationTemporality": 2`, `"aggregationTemporality": 1`), strings.ReplaceAll(string(raw), "gen_ai.provider.name", "gen_ai.system")} {
		if _, e = p.ParseBytes(ctx, []byte(bad), "bad", o); e == nil {
			t.Fatal("aliased incompatible scope/schema/temporality")
		}
	}
	// A missing token series does not become a zero series. An explicit zero
	// histogram sum remains present; neither case creates request events.
	var f map[string]any
	json.Unmarshal(raw, &f)
	resources := f["resourceMetrics"].([]any)
	groups := resources[0].(map[string]any)["scopeMetrics"].([]any)
	metrics := groups[0].(map[string]any)["metrics"].([]any)
	groups[0].(map[string]any)["metrics"] = metrics[:1]
	missing, _ := json.Marshal(f)
	only, e := p.ParseBytes(ctx, missing, "missing", o)
	if e != nil || len(only.Signals) != 1 {
		t.Fatal(e)
	}
	json.Unmarshal(raw, &f)
	resources = f["resourceMetrics"].([]any)
	groups = resources[0].(map[string]any)["scopeMetrics"].([]any)
	metrics = groups[0].(map[string]any)["metrics"].([]any)
	points := metrics[1].(map[string]any)["histogram"].(map[string]any)["dataPoints"].([]any)
	buckets := points[0].(map[string]any)["bucketCounts"].([]any)
	for i := range buckets {
		buckets[i] = "0"
	}
	buckets[0] = "2"
	points[0].(map[string]any)["sum"] = float64(0)
	points[0].(map[string]any)["min"] = float64(0)
	points[0].(map[string]any)["max"] = float64(0)
	zeroRaw, _ := json.Marshal(f)
	zero, e := p.ParseBytes(ctx, zeroRaw, "zero", o)
	if e != nil {
		t.Fatal(e)
	}
	encoded, _ := json.Marshal(zero)
	if !bytes.Contains(encoded, []byte(`"sum":"0"`)) {
		t.Fatal("zero sum lost")
	}
}
