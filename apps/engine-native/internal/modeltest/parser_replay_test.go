package modeltest_test

import (
	"context"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/traceimport"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	metric "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	system "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/systemtrace"
	llm "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/traceimport"
	"os"
	"strings"
	"testing"
)

func retained(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/adversarial/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRetainedMetricOverflowAndKnownEpochArithmetic(t *testing.T) {
	ctx := context.Background()
	_, err := metric.ParseBytes(ctx, retained(t, "metric-signed-int64-overflow.otlp.json"), "synthetic", metric.Options{})
	if err == nil || !strings.Contains(err.Error(), "signed_int64_integer") {
		t.Fatal("signed integer source accepted or lost rule", err)
	}
	set, err := metric.ParseBytes(ctx, retained(t, "metric-known-epochs.snapshots.json"), "synthetic", metric.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, series := range set.Series {
		ids = append(ids, series.ID)
	}
	rows, err := m.SelectRows(ctx, set, m.Selection{SeriesIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	observed := 0
	for _, row := range rows {
		if row.End == nil {
			continue
		}
		want := map[string]string{"2000000000": "7", "5000000000": "1"}[*row.End]
		if want != "" {
			if row.Delta == nil || *row.Delta != want {
				t.Fatal("independent exact interval delta", row)
			}
			observed++
		}
	}
	if observed != 2 {
		t.Fatal("missing known-epoch rows", observed)
	}
}

func TestRetainedImportedHashAssertion(t *testing.T) {
	options, err := system.DecodeOptions(retained(t, "system-imported-hash-mismatch.options.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = system.ParseBytes(context.Background(), retained(t, "system-perf-source.txt"), options)
	if err == nil || !strings.Contains(err.Error(), "SYSTEM_CAPTURE_IMPORTED_MISMATCH") {
		t.Fatal(err)
	}
}

func TestRetainedContradictoryTokenBoundaries(t *testing.T) {
	encoded, err := os.ReadFile("../catalog/contracts/llm-trace-neutral-options.json")
	if err != nil {
		t.Fatal(err)
	}
	options, err := llm.DecodeLLMOptions(encoded)
	if err != nil {
		t.Fatal(err)
	}
	set, err := llm.ParseLLMBytes(context.Background(), retained(t, "llm-token-before-span.otlp.json"), options)
	if err != nil {
		t.Fatal(err)
	}
	view, err := a.ProjectLLM(context.Background(), set)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, span := range view.Spans {
		if span.TokenStatus == "token_boundary_order_invalid" {
			found = true
			if span.TTFT != nil || span.TPOT != nil {
				t.Fatal("contradictory bounds exposed numeric timing", span)
			}
		}
	}
	if !found {
		t.Fatal("retained contradiction not recognized")
	}
}
