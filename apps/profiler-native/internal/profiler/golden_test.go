package profiler

import (
	"path/filepath"
	"strings"
	"testing"
)

// Golden parity tests pin the Go analyzer output against known facts about
// the sample fixtures shipped under examples/profiler/. The Python engine's
// own regression tests (engines/python/tests/test_*.py) verify the same
// fixtures land at the same totals, so any drift between the two implementations
// will surface here as well.
//
// When the fixtures change, regenerate the golden numbers from Python:
//   python3 -m archscope_engine.cli profiler analyze \
//     --collapsed examples/profiler/sample-wall.collapsed \
//     --interval-ms 100 --elapsed-sec 1336.559

const (
	goldenCollapsedTotal      = 32629
	goldenJenniferTotal       = 115
	goldenJenniferTopStack    = "com.example.checkout.CheckoutController.checkout;com.example.checkout.CheckoutService.placeOrder;oracle.jdbc.driver.OraclePreparedStatement.executeQuery"
	goldenJenniferTopSamples  = 45
	goldenCollapsedHottestSub = "InternalHttpClient.doExecute"
)

func TestGoldenCollapsedSampleWall(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "examples", "profiler", "sample-wall.collapsed")
	elapsed := 1336.559
	result, err := AnalyzeCollapsedFile(path, Options{
		IntervalMS:  100,
		ElapsedSec:  &elapsed,
		TopN:        20,
		ProfileKind: "wall",
	})
	if err != nil {
		t.Fatalf("AnalyzeCollapsedFile: %v", err)
	}
	if result.Summary.TotalSamples != goldenCollapsedTotal {
		t.Fatalf("total_samples = %d, want %d", result.Summary.TotalSamples, goldenCollapsedTotal)
	}
	if len(result.Series.TopStacks) == 0 {
		t.Fatalf("expected non-empty top_stacks")
	}
	if !strings.Contains(result.Series.TopStacks[0].Stack, goldenCollapsedHottestSub) {
		t.Fatalf("top stack should mention %q; got %q", goldenCollapsedHottestSub, result.Series.TopStacks[0].Stack)
	}
	if result.Metadata.Parser != "async_profiler_collapsed" {
		t.Fatalf("parser = %q, want async_profiler_collapsed", result.Metadata.Parser)
	}
	if result.Summary.EstimatedSeconds <= 0 {
		t.Fatalf("estimated_seconds should be positive; got %v", result.Summary.EstimatedSeconds)
	}
	if result.Summary.ElapsedSeconds == nil || *result.Summary.ElapsedSeconds != elapsed {
		t.Fatalf("elapsed_seconds carry-through; got %+v", result.Summary.ElapsedSeconds)
	}
}

func TestGoldenJenniferSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "examples", "profiler", "sample-jennifer-flame.csv")
	result, err := AnalyzeJenniferFile(path, Options{
		IntervalMS:  100,
		TopN:        20,
		ProfileKind: "wall",
	})
	if err != nil {
		t.Fatalf("AnalyzeJenniferFile: %v", err)
	}
	if result.Summary.TotalSamples != goldenJenniferTotal {
		t.Fatalf("total_samples = %d, want %d", result.Summary.TotalSamples, goldenJenniferTotal)
	}
	if len(result.Series.TopStacks) == 0 {
		t.Fatalf("expected non-empty top_stacks")
	}
	if result.Series.TopStacks[0].Stack != goldenJenniferTopStack {
		t.Fatalf("top stack = %q, want %q", result.Series.TopStacks[0].Stack, goldenJenniferTopStack)
	}
	if result.Series.TopStacks[0].Samples != goldenJenniferTopSamples {
		t.Fatalf("top stack samples = %d, want %d", result.Series.TopStacks[0].Samples, goldenJenniferTopSamples)
	}
}

func TestGoldenSvgRoundTripMatchesCollapsed(t *testing.T) {
	// Brendan-default SVG used in svg_test.go expands to the same three
	// collapsed stacks. Keeping it in the golden suite ensures any SVG
	// parser regression also fails parity.
	result := ParseSvgFlamegraphText(brendanSVG)
	got := 0
	for _, count := range result.Stacks {
		got += count
	}
	if got != 10 {
		t.Fatalf("brendan SVG total = %d, want 10", got)
	}
	for _, key := range []string{"all;main;compute", "all;main;render", "all;worker;poll"} {
		if _, ok := result.Stacks[key]; !ok {
			t.Fatalf("missing stack %q in golden SVG output; got %+v", key, result.Stacks)
		}
	}
}

func TestGoldenHtmlAsyncProfilerJSMatchesSvg(t *testing.T) {
	// Same logical tree, two different inputs (async-profiler embedded JS
	// vs Brendan SVG): Go must produce the same collapsed stacks.
	html := ParseHtmlProfilerText(asyncProfilerHTML, nil)
	svg := ParseSvgFlamegraphText(brendanSVG)
	for key, expected := range svg.Stacks {
		if html.Stacks[key] != expected {
			t.Fatalf("html vs svg parity mismatch on %q: html=%d svg=%d", key, html.Stacks[key], expected)
		}
	}
}
