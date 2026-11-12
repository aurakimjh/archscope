package execution_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/execution"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../catalog/contracts/execution-evidence-" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestPinnedBenchmarkAndActualCrashNormalization(t *testing.T) {
	for _, c := range []struct{ name, file string }{{"k6", "k6-1.3.0.json"}, {"llmperf", "llmperf-summary.json"}, {"jmeter", "jmeter-5.6.3.csv"}, {"crash", "hs-err-21.0.10.txt"}} {
		t.Run(c.name, func(t *testing.T) {
			o, err := p.DecodeOptions(fixture(t, c.name+"-options.json"))
			if err != nil {
				t.Fatal(err)
			}
			set, err := p.ParseBytes(context.Background(), fixture(t, c.file), o)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(set)
			want, err := e.Decode(context.Background(), fixture(t, c.name+"-set.json"))
			if err != nil {
				t.Fatal(err)
			}
			expected, _ := json.Marshal(want)
			if !bytes.Equal(raw, expected) {
				t.Fatal("normalization drift")
			}
			for _, body := range []string{"SYNTHETIC_BODY_NOT_RETAINED", "SYNTHETIC_URL_NOT_RETAINED", "SYNTHETIC_FAILURE_NOT_RETAINED", "SYNTHETIC_ERROR_NOT_RETAINED", "SYNTHETIC_COMMAND_NOT_RETAINED", "SYNTHETIC_ENV_NOT_RETAINED"} {
				if bytes.Contains(raw, []byte(body)) {
					t.Fatal("raw text retained", body)
				}
			}
			if c.name == "crash" {
				if set.Crash.Signal != "SIGSEGV" || set.Source.Version != "21.0.10+7" || set.Crash.Completeness != "partial_or_unknown" || len(set.Crash.Frames) == 0 {
					t.Fatal(set)
				}
			}
			if c.name == "jmeter" {
				groups := map[string]bool{}
				errors := 0
				for _, s := range set.Samples {
					groups[s.GroupSHA] = true
					if !s.Success {
						errors++
					}
				}
				if len(groups) != 2 || errors != 1 {
					t.Fatal(groups, errors)
				}
			}
			if c.name == "llmperf" {
				seen := map[string]bool{}
				for _, m := range set.Metrics {
					seen[m.Name] = true
					if m.Name == "errors" && *m.Value != "1" {
						t.Fatal(m)
					}
					if m.Name == "ttft" && m.Population != "successful_requests" {
						t.Fatal(m)
					}
					if m.Name == "per_request_output_throughput" && m.Unit != "tokens/second/request" {
						t.Fatal(m)
					}
				}
				if !seen["output_throughput"] || !seen["per_request_output_throughput"] || !seen["itl"] {
					t.Fatal(seen)
				}
			}
		})
	}
}
func TestRefusalsAndImmutableGzipSourceIdentity(t *testing.T) {
	ctx := context.Background()
	raw := fixture(t, "k6-1.3.0.json")
	o, _ := p.DecodeOptions(fixture(t, "k6-options.json"))
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	z.Write(raw)
	z.Close()
	s, err := p.ParseBytes(ctx, compressed.Bytes(), o)
	if err != nil || s.Source.SHA != p.Digest(compressed.Bytes()) {
		t.Fatal(s.Source, err)
	}
	o.Context = nil
	if _, err = p.ParseBytes(ctx, raw, o); err == nil {
		t.Fatal("missing context accepted")
	}
	o, _ = p.DecodeOptions(fixture(t, "k6-options.json"))
	o.ProducerVersion = "unqualified"
	if _, err = p.ParseBytes(ctx, raw, o); err == nil {
		t.Fatal("version invented")
	}
	if _, err = e.Decode(ctx, []byte(`{"schema_version":"1.0.0","schema_version":"1.0.0"}`)); err == nil {
		t.Fatal("duplicate key")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	o, _ = p.DecodeOptions(fixture(t, "k6-options.json"))
	if _, err = p.ParseBytes(ctx, raw, o); err == nil {
		t.Fatal("cancel ignored")
	}
	o = p.Options{Format: "hs-err-text"}
	if _, err = p.ParseBytes(context.Background(), []byte("binary\x00core"), o); err == nil {
		t.Fatal("core accepted")
	}
	o, _ = p.DecodeOptions(fixture(t, "jmeter-options.json"))
	bad := strings.Replace(string(fixture(t, "jmeter-5.6.3.csv")), "1700000000000", "2026-01-01", 1)
	if _, err = p.ParseBytes(context.Background(), []byte(bad), o); err == nil {
		t.Fatal("timestamp format guessed")
	}
}
