package traceimport_test

import (
	"bytes"
	"context"
	"encoding/json"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/traceimport"
	csv "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/csv"
	html "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/traceimport"
	"os"
	"path/filepath"
	"testing"
)

func TestActualLangfuseObservationAPIAndMissingZero(t *testing.T) {
	raw, e := os.ReadFile("testdata/langfuse/actual.json")
	if e != nil {
		t.Fatal(e)
	}
	o := p.LLMOptions{Profile: p.LangfuseProfile, Namespace: "synthetic-lab", Producer: "langfuse", ProducerVersion: "4.79.0", Clock: "unix"}
	ctx := context.Background()
	s, e := p.ParseLLMBytes(ctx, raw, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Spans) != 3 || s.Spans[0].Role != "llm" || *s.Spans[0].InputTokens != "3" || *s.Spans[0].OutputTokens != "2" || s.Spans[1].InputTokens != nil || s.Spans[2].InputTokens != nil {
		t.Fatal(s.Spans)
	}
	if *s.Spans[0].End != "1794532368764000000" || *s.Spans[0].Start != "1794532367864000000" {
		t.Fatal(s.Spans[0])
	}
	for _, v := range s.Spans {
		if v.FirstToken != nil || v.LastToken != nil || v.Accounting != "none" || v.Boundary != "unknown" {
			t.Fatal("inferred token boundary/authority")
		}
	}
	result, e := a.BuildLLM(ctx, s, "actual")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.ValidateLLM(result); e != nil {
		t.Fatal(e)
	}
	for _, exp := range []struct {
		name  string
		write func(string, any) error
	}{{"r.csv", csv.Write}, {"r.html", html.Write}} {
		if e = exp.write(filepath.Join(t.TempDir(), exp.name), result); e != nil {
			t.Fatal(e)
		}
	}
	var f map[string]any
	json.Unmarshal(raw, &f)
	data := f["data"].([]any)
	data[0].(map[string]any)["input"] = "BODY_SENTINEL"
	data[0].(map[string]any)["name"] = "PRIVATE_NAME_SENTINEL"
	data[0].(map[string]any)["usageDetails"].(map[string]any)["input"] = float64(0)
	changed, _ := json.Marshal(f)
	zero, e := p.ParseLLMBytes(ctx, changed, o)
	if e != nil || zero.Spans[0].InputTokens == nil || *zero.Spans[0].InputTokens != "0" {
		t.Fatal(e)
	}
	normalized, _ := json.Marshal(zero)
	if bytes.Contains(normalized, []byte("SENTINEL")) {
		t.Fatal("body/name escaped privacy allowlist")
	}
	o.ProducerVersion = "unknown"
	if _, e = p.ParseLLMBytes(ctx, raw, o); e == nil {
		t.Fatal("unqualified version")
	}
}
