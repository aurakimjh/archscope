package inference_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/inference"
	"os"
	"strings"
	"testing"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	v, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestNvidiaDeclaredQueryClockUnavailableAndBindings(t *testing.T) {
	ctx := context.Background()
	var c p.Capture
	if e := n.Strict(read(t, "nvidia-capture.json"), &c); e != nil {
		t.Fatal(e)
	}
	raw := read(t, "nvidia-query.csv")
	options := p.Options{Format: "nvidia-smi-csv", Capture: &c}
	s, e := p.ParseBytes(ctx, raw, "nvidia-query.csv", options)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Entities) != 2 || len(s.Metrics.Points) != 21 || len(s.Quality) != 3 || s.Metrics.Sources[0].SHA256 != p.Digest(raw) {
		t.Fatal(s)
	}
	var z bytes.Buffer
	gzipWriter := gzip.NewWriter(&z)
	gzipWriter.Write(raw)
	gzipWriter.Close()
	compressed, e := p.ParseBytes(ctx, z.Bytes(), "source.gz", options)
	if e != nil || compressed.Metrics.Sources[0].SHA256 != p.Digest(z.Bytes()) {
		t.Fatal(e)
	}
	c.Times = nil
	s, e = p.ParseBytes(ctx, raw, "nvidia-query.csv", options)
	if e != nil || s.Metrics.Points[0].End != nil {
		t.Fatal("mtime/invented time", e)
	}
	bound, e := p.ParseBytes(ctx, read(t, "metric-model.json"), "metric-model.json", p.Options{Format: "metric-bindings-json", Bindings: read(t, "bindings.json")})
	if e != nil {
		t.Fatal(e)
	}
	if len(bound.Signals) != 24 || len(bound.Events) != 14 {
		t.Fatal(len(bound.Signals), len(bound.Events))
	}
	fixture, e := n.Decode(ctx, read(t, "inference-evidence-set.json"))
	if e != nil {
		t.Fatal(e)
	}
	a, _ := json.Marshal(bound)
	b, _ := json.Marshal(fixture)
	if !bytes.Equal(a, b) {
		t.Fatal("binding changed original model")
	}
}
func TestCsvRefusesGuessesAndSourceMutation(t *testing.T) {
	var capture p.Capture
	n.Strict(read(t, "nvidia-capture.json"), &capture)
	raw := read(t, "nvidia-query.csv")
	for name, edit := range map[string]func(*p.Capture){"fields": func(c *p.Capture) { c.Fields = "name,utilization.gpu" }, "version": func(c *p.Capture) { c.Version = "unknown" }, "clock arity": func(c *p.Capture) { c.Times = c.Times[:1] }, "host": func(c *p.Capture) { c.Host = "unknown" }} {
		t.Run(name, func(t *testing.T) {
			copy := capture
			edit(&copy)
			if _, e := p.ParseBytes(context.Background(), raw, "x", p.Options{Format: "nvidia-smi-csv", Capture: &copy}); e == nil {
				t.Fatal("unqualified capture accepted")
			}
		})
	}
	for _, bad := range [][]byte{[]byte("header\n"), bytes.Replace(raw, []byte("75"), []byte("1/3"), 1), bytes.Replace(raw, []byte("GPU-synthetic-a"), []byte("0"), 1), bytes.Replace(raw, []byte("535.104.05"), []byte("another-version"), 1), []byte{0xff}} {
		if _, e := p.ParseBytes(context.Background(), bad, "x", p.Options{Format: "nvidia-smi-csv", Capture: &capture}); e == nil {
			t.Fatal("invalid CSV accepted")
		}
	}
	if _, e := p.DecodeOptions([]byte(`{"format":"nvidia-smi-csv","exec":"tool"}`)); e == nil {
		t.Fatal("executable option accepted")
	}
	if _, e := p.DecodeOptions([]byte(`{"format":"nvidia-smi-csv","format":"other"}`)); e == nil {
		t.Fatal("duplicate option accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := p.ParseBytes(ctx, raw, "x", p.Options{Format: "nvidia-smi-csv", Capture: &capture}); e == nil {
		t.Fatal("cancel ignored")
	}
	if _, e := p.ParseBytes(context.Background(), read(t, "metric-model.json"), "x", p.Options{Format: "metric-bindings-json", Bindings: []byte(`{"metrics":{}}`)}); e == nil || !strings.Contains(e.Error(), "INFERENCE_BINDING_INVALID") {
		t.Fatal(e)
	}
}
