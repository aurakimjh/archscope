//go:build ignore

package main

import (
	"context"
	"encoding/json"
	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/inference"
	h "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"os"
	"path/filepath"
)

func main() {
	base := os.Args[1]
	read := func(name string) []byte {
		raw, e := os.ReadFile(filepath.Join(base, name))
		if e != nil {
			panic(e)
		}
		return raw
	}
	write := func(name string, v any) {
		raw, e := json.MarshalIndent(v, "", "  ")
		if e != nil {
			panic(e)
		}
		if e = os.WriteFile(filepath.Join(base, name), append(raw, '\n'), 0600); e != nil {
			panic(e)
		}
	}
	ctx := context.Background()
	raw := read("r16-prometheus-range.json")
	q := read("r16-prometheus-query.json")
	metrics, e := h.ParseBytes(ctx, raw, "synthetic-range.json", h.Options{Format: "prometheus-range-json", Query: q})
	if e != nil {
		panic(e)
	}
	s, e := n.Decode(ctx, read("inference-evidence-set.json"))
	if e != nil {
		panic(e)
	}
	s.Version = n.Version
	s.Metrics = metrics
	s.Quality = nil
	s.Signals = nil
	for _, series := range metrics.Series {
		v := n.Signal{ID: "signal-" + series.Name, SeriesID: series.ID, EntityID: "synthetic-worker-0", ContextID: "synthetic-context", Boundary: "server", Population: "warm", Conversion: "identity"}
		if series.Name == "synthetic_output_tokens" {
			v.Semantic = "output_tokens"
			v.Form = "counter"
			v.Declaration = &n.Declaration{Unit: "tokens", Kind: "sum", Temporality: "cumulative", Monotonic: true, Start: m.Pointer("0")}
		} else {
			v.Semantic = "queue_requests"
			v.Form = "gauge"
			v.Declaration = &n.Declaration{Unit: "requests", Kind: "gauge", Temporality: "unspecified"}
		}
		s.Signals = append(s.Signals, v)
	}
	write("r16-metric-model.json", metrics)
	var b map[string]json.RawMessage
	body, _ := json.Marshal(s)
	json.Unmarshal(body, &b)
	delete(b, "metrics")
	var events []n.Event
	json.Unmarshal(b["events"], &events)
	for i := range events {
		events[i].ID = ""
	}
	b["events"], _ = json.Marshal(events)
	write("r16-bindings-user-input.json", b)
	binding, _ := json.Marshal(b)
	normalized, e := p.ParseBytes(ctx, mustJSON(metrics), "synthetic-range.json", p.Options{Format: "metric-bindings-json", Bindings: binding})
	if e != nil {
		panic(e)
	}
	write("r16-declared-set.json", normalized)
}
func mustJSON(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
