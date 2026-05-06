package profiler

import (
	"os"
	"path/filepath"
	"testing"
)

const brendanSVG = `<?xml version="1.0" standalone="no"?>
<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="100" height="48">
  <g class="func_g">
    <title>all (10 samples, 100.00%)</title>
    <rect x="0" y="32" width="100" height="16" fill="#ff8800"/>
  </g>
  <g class="func_g">
    <title>main (6 samples, 60.00%)</title>
    <rect x="0" y="16" width="60" height="16" fill="#ff9900"/>
  </g>
  <g class="func_g">
    <title>worker (4 samples, 40.00%)</title>
    <rect x="60" y="16" width="40" height="16" fill="#ffaa00"/>
  </g>
  <g class="func_g">
    <title>compute (3 samples, 30.00%)</title>
    <rect x="0" y="0" width="30" height="16" fill="#ff7700"/>
  </g>
  <g class="func_g">
    <title>render (3 samples, 30.00%)</title>
    <rect x="30" y="0" width="30" height="16" fill="#ff6600"/>
  </g>
  <g class="func_g">
    <title>poll (4 samples, 40.00%)</title>
    <rect x="60" y="0" width="40" height="16" fill="#ff5500"/>
  </g>
</svg>
`

const icicleSVG = `<?xml version="1.0" standalone="no"?>
<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="100" height="48">
  <g><title>all (10 samples, 100.00%)</title>
     <rect x="0" y="0" width="100" height="16"/></g>
  <g><title>main (6 samples, 60.00%)</title>
     <rect x="0" y="16" width="60" height="16"/></g>
  <g><title>worker (4 samples, 40.00%)</title>
     <rect x="60" y="16" width="40" height="16"/></g>
  <g><title>compute (3 samples, 30.00%)</title>
     <rect x="0" y="32" width="30" height="16"/></g>
  <g><title>render (3 samples, 30.00%)</title>
     <rect x="30" y="32" width="30" height="16"/></g>
  <g><title>poll (4 samples, 40.00%)</title>
     <rect x="60" y="32" width="40" height="16"/></g>
</svg>
`

func TestSvgFlamegraphBrendanLayout(t *testing.T) {
	result := ParseSvgFlamegraphText(brendanSVG)
	expected := map[string]int{
		"all;main;compute": 3,
		"all;main;render":  3,
		"all;worker;poll":  4,
	}
	assertStacksEqual(t, expected, result.Stacks)
	if got, want := result.Diagnostics.TotalLines, 6; got != want {
		t.Fatalf("total_lines = %d, want %d", got, want)
	}
	if got, want := result.Diagnostics.ParsedRecords, 10; got != want {
		t.Fatalf("parsed_records = %d, want %d", got, want)
	}
	if got, want := result.Diagnostics.SkippedLines, 0; got != want {
		t.Fatalf("skipped_lines = %d, want %d", got, want)
	}
}

func TestSvgFlamegraphIcicleLayout(t *testing.T) {
	result := ParseSvgFlamegraphText(icicleSVG)
	expected := map[string]int{
		"all;main;compute": 3,
		"all;main;render":  3,
		"all;worker;poll":  4,
	}
	assertStacksEqual(t, expected, result.Stacks)
}

func TestSvgFlamegraphCommaSampleCount(t *testing.T) {
	svg := `<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg">
  <g><title>all (12,345 samples, 100.00%)</title>
     <rect x="0" y="16" width="100" height="16"/></g>
  <g><title>main (12,345 samples, 100.00%)</title>
     <rect x="0" y="0" width="100" height="16"/></g>
</svg>`
	result := ParseSvgFlamegraphText(svg)
	assertStacksEqual(t, map[string]int{"all;main": 12345}, result.Stacks)
}

func TestSvgFlamegraphSkipsTitlesWithoutSampleMarker(t *testing.T) {
	svg := `<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg">
  <g><title>Search</title><rect x="0" y="0" width="50" height="16"/></g>
  <g><title>all (5 samples, 100%)</title>
     <rect x="0" y="32" width="100" height="16"/></g>
  <g><title>leaf (5 samples, 100%)</title>
     <rect x="0" y="16" width="100" height="16"/></g>
</svg>`
	result := ParseSvgFlamegraphText(svg)
	assertStacksEqual(t, map[string]int{"all;leaf": 5}, result.Stacks)
	if result.Diagnostics.SkippedByReason["UNPARSEABLE_TITLE"] < 1 {
		t.Fatalf("expected at least one UNPARSEABLE_TITLE; got %+v", result.Diagnostics.SkippedByReason)
	}
}

func TestSvgFlamegraphInvalidSvg(t *testing.T) {
	result := ParseSvgFlamegraphText("<svg><not closed")
	if len(result.Stacks) != 0 {
		t.Fatalf("expected empty stacks; got %+v", result.Stacks)
	}
	if _, ok := result.Diagnostics.SkippedByReason["INVALID_SVG"]; !ok {
		t.Fatalf("expected INVALID_SVG diagnostic; got %+v", result.Diagnostics.SkippedByReason)
	}
}

func TestSvgFlamegraphFromDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flame.svg")
	if err := os.WriteFile(path, []byte(brendanSVG), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := ParseSvgFlamegraphFile(path, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	total := 0
	for _, count := range result.Stacks {
		total += count
	}
	if total != 10 {
		t.Fatalf("total samples = %d, want 10", total)
	}
	if _, ok := result.Stacks["all;main;compute"]; !ok {
		t.Fatalf("missing 'all;main;compute' in stacks; got %+v", result.Stacks)
	}
}

func TestAnalyzeFlamegraphSVGFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flame.svg")
	if err := os.WriteFile(path, []byte(brendanSVG), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := AnalyzeFlamegraphSVGFile(path, Options{IntervalMS: 100, TopN: 5, ProfileKind: "wall"})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if got, want := result.Type, "profiler_collapsed"; got != want {
		t.Fatalf("type = %q, want %q", got, want)
	}
	if got, want := result.Metadata.Parser, "flamegraph_svg"; got != want {
		t.Fatalf("parser = %q, want %q", got, want)
	}
	if got, want := result.Summary.TotalSamples, 10; got != want {
		t.Fatalf("total_samples = %d, want %d", got, want)
	}
}

func assertStacksEqual(t *testing.T, expected, got map[string]int) {
	t.Helper()
	if len(expected) != len(got) {
		t.Fatalf("stacks size mismatch: expected %d (%+v), got %d (%+v)", len(expected), expected, len(got), got)
	}
	for key, value := range expected {
		if got[key] != value {
			t.Fatalf("stacks[%q] = %d, want %d (full got=%+v)", key, got[key], value, got)
		}
	}
}
