package inference_test

import (
	"bytes"
	"context"
	"encoding/json"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/inference"
	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/inference"
	h "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"os"
	"strings"
	"testing"
)

func TestR16DeclaredPrometheusBindingsAndOriginalIdentity(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"synthetic_tokens"},"values":[[1,"100"],[2,"110"]]}]}}`)
	metrics, e := h.ParseBytes(ctx, raw, "range.json", h.Options{Format: "prometheus-range-json", Query: json.RawMessage(`{"query":"synthetic_tokens","step_ns":"1000000000"}`)})
	if e != nil {
		t.Fatal(e)
	}
	s := fixture(t)
	s.Version = n.Version
	s.Metrics = metrics
	s.Quality = nil
	s.Events = nil
	s.Sources = nil
	s.Signals = []n.Signal{{ID: "declared-output", SeriesID: metrics.Series[0].ID, Semantic: "output_tokens", EntityID: "synthetic-worker-0", ContextID: "synthetic-context", Boundary: "server", Population: "warm", Form: "counter", Conversion: "identity", Declaration: &n.Declaration{Unit: "tokens", Kind: "sum", Temporality: "cumulative", Monotonic: true, Start: m.Pointer("0")}}}
	before, _ := json.Marshal(s.Metrics)
	rows, e := a.Observations(ctx, s)
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 2 || rows[1].Rate == nil || *rows[1].Rate != "10" || rows[1].PointID != metrics.Points[1].ID {
		t.Fatal(rows)
	}
	after, _ := json.Marshal(s.Metrics)
	if !bytes.Equal(before, after) {
		t.Fatal("original changed")
	}
	s.Signals[0].Declaration.Start = nil
	rows, e = a.Observations(ctx, s)
	if e != nil || rows[1].Rate != nil {
		t.Fatal("guessed lifetime", rows, e)
	}
	s.Signals[0].Declaration.Unit = "seconds"
	if e = s.Validate(ctx); e == nil || !strings.Contains(e.Error(), "declared-output") || !strings.Contains(e.Error(), "synthetic_tokens") {
		t.Fatal("unactionable error", e)
	}
}
func TestR16EventIDsOnlyAtInputNormalizationAndSelection(t *testing.T) {
	ctx := context.Background()
	s := fixture(t)
	for i := range s.Events {
		s.Events[i].ID = ""
	}
	model, _ := json.Marshal(s.Metrics)
	s.Metrics = m.Set{}
	binding, _ := json.Marshal(s)
	var keys map[string]json.RawMessage
	json.Unmarshal(binding, &keys)
	delete(keys, "metrics")
	binding, _ = json.Marshal(keys)
	normalized, e := p.ParseBytes(ctx, model, "model.json", p.Options{Format: "metric-bindings-json", Bindings: binding})
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range normalized.Events {
		if v.ID != n.EventID(v) {
			t.Fatal(v)
		}
	}
	bad := normalized
	bad.Events = append([]n.Event(nil), normalized.Events...)
	bad.Events[0].ID = ""
	canonical, _ := json.Marshal(bad)
	if _, e = n.Decode(ctx, canonical); e == nil {
		t.Fatal("saved reader repaired bad ID")
	}
	bad.Events[0].ID = "supplied-wrong"
	bad.Metrics = m.Set{}
	binding, _ = json.Marshal(bad)
	json.Unmarshal(binding, &keys)
	delete(keys, "metrics")
	binding, _ = json.Marshal(keys)
	if _, e = p.ParseBytes(ctx, model, "model.json", p.Options{Format: "metric-bindings-json", Bindings: binding}); e == nil {
		t.Fatal("supplied ID overwritten")
	}
	id := normalized.Metrics.Series[0].ID
	selected, e := n.SelectSeries(ctx, normalized, []string{id})
	if e != nil {
		t.Fatal(e)
	}
	want := 0
	for _, v := range normalized.Metrics.Points {
		if v.SeriesID == id {
			want++
		}
	}
	if len(selected.Metrics.Series) != 1 || len(selected.Metrics.Points) != want || len(selected.Events) != len(normalized.Events) {
		t.Fatal("retained context lost")
	}
}
func TestR16EqualTokenBoundaryAndLegacyReader(t *testing.T) {
	s := fixture(t)
	for i := range s.Events {
		if s.Events[i].Kind == "request" && s.Events[i].OutputTokens != nil && *s.Events[i].OutputTokens == "3" {
			s.Events[i].LastToken = s.Events[i].FirstToken
			s.Events[i].ID = n.EventID(s.Events[i])
		}
	}
	r, e := a.Build(context.Background(), s, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	requests := r.Tables["requests"].([]a.Request)
	if requests[0].TPOT != nil || requests[0].TPOTStatus != "token_boundaries_not_distinct" {
		t.Fatal(requests)
	}
	raw, e := os.ReadFile("testdata/legacy-0.1.0.json")
	if e != nil {
		t.Fatal(e)
	}
	old, e := resultcontract.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	if old.Metadata.SchemaVersion != "0.1.0" {
		t.Fatal("not legacy")
	}
	if e = a.Validate(old); e != nil {
		t.Fatal(e)
	}
}
