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
			if c.name == "crash" {
				for i := range want.Crash.Frames {
					want.Crash.Frames[i].Section = "problematic"
					want.Crash.Frames[i].Symbol = "Unsafe_PutLong(JNIEnv_*, _jobject*, _jobject*, long, long)"
				}
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

func TestR1902InlineElapsedFullSymbolsAndSections(t *testing.T) {
	raw := append(fixture(t, "hs-err-21.0.10.txt"), []byte("\nTime: synthetic elapsed time: 0.627140 seconds (0d 0h 0m 0s)\nNative frames:\nV  [libjvm.dylib+0xa2cb34]  Unsafe_PutLong(JNIEnv_*, _jobject*, _jobject*, long, long)+0x124\nj  C.level3()V+0\n\nJava frames:\nj  C.level3()V+0\n\n")...)
	set, err := p.ParseBytes(context.Background(), raw, p.Options{Format: "hs-err-text"})
	if err != nil {
		t.Fatal(err)
	}
	if set.Crash.Elapsed == nil || *set.Crash.Elapsed != "0.62714" || len(set.Crash.Frames) != 4 {
		t.Fatal(set.Crash)
	}
	if set.Crash.Frames[0].Section != "problematic" || set.Crash.Frames[1].Section != "native" || set.Crash.Frames[3].Section != "java" || set.Crash.Frames[1].Symbol != "Unsafe_PutLong(JNIEnv_*, _jobject*, _jobject*, long, long)" {
		t.Fatal(set.Crash.Frames)
	}
}

func TestR1902ActualHotSpotSectionExcerpt(t *testing.T) {
	raw := fixture(t, "hs-err-21.0.10-full-sections.txt")
	set, err := p.ParseBytes(context.Background(), raw, p.Options{Format: "hs-err-text"})
	if err != nil {
		t.Fatal(err)
	}
	sections := map[string]bool{}
	full := false
	for _, f := range set.Crash.Frames {
		sections[f.Section] = true
		full = full || strings.Contains(f.Symbol, "_jobject*, long, long")
	}
	if set.Source.Version != "21.0.10+7" || set.Crash.Elapsed == nil || !sections["problematic"] || !sections["native"] || !sections["java"] || !full {
		t.Fatal(set.Crash)
	}
}

func TestR1903JTLWarnsWithoutGuessingUnitsAndNullsBadDurations(t *testing.T) {
	o, err := p.DecodeOptions(fixture(t, "jmeter-options.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("timeStamp,elapsed,label,success,Latency,Connect\n1700000000,100,synthetic,true,500,600\n1700000000000,100,synthetic,true,40,50\n")
	set, err := p.ParseBytes(context.Background(), raw, o)
	if err != nil {
		t.Fatal(err)
	}
	if *set.Samples[0].Start != "1700000000000000" || set.Samples[0].FirstByte != nil || set.Samples[0].Connect != nil || len(set.Samples[0].TimingWarnings) != 4 {
		t.Fatal(set.Samples)
	}
	if set.Samples[1].FirstByte == nil || *set.Samples[1].FirstByte != "1/25" || set.Samples[1].Connect != nil {
		t.Fatal(set.Samples[1])
	}
	if !strings.Contains(strings.Join(set.Limitations, " "), "affected_rows=") {
		t.Fatal("aggregate warnings absent")
	}
}
