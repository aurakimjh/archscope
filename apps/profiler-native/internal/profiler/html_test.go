package profiler

import (
	"os"
	"path/filepath"
	"testing"
)

const inlineSvgHTML = `<!doctype html>
<html><body>
<h1>Profile</h1>
<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="100" height="48">
  <g class="func_g"><title>all (10 samples, 100.00%)</title>
     <rect x="0" y="32" width="100" height="16"/></g>
  <g class="func_g"><title>main (6 samples, 60.00%)</title>
     <rect x="0" y="16" width="60" height="16"/></g>
  <g class="func_g"><title>worker (4 samples, 40.00%)</title>
     <rect x="60" y="16" width="40" height="16"/></g>
  <g class="func_g"><title>compute (3 samples, 30.00%)</title>
     <rect x="0" y="0" width="30" height="16"/></g>
  <g class="func_g"><title>render (3 samples, 30.00%)</title>
     <rect x="30" y="0" width="30" height="16"/></g>
  <g class="func_g"><title>poll (4 samples, 40.00%)</title>
     <rect x="60" y="0" width="40" height="16"/></g>
</svg>
<script>console.log("after");</script>
</body></html>
`

const asyncProfilerHTML = `<!doctype html>
<html><body>
<canvas id="cv"></canvas>
<script>
var root = {name:"all", samples:10, children:[
  {name:"main", samples:6, children:[
    {name:"compute", samples:3, children:[]},
    {name:"render", samples:3, children:[]}
  ]},
  {name:"worker", samples:4, children:[
    {name:"poll", samples:4, children:[]}
  ]}
]};
</script>
</body></html>
`

func TestHtmlProfilerInlineSvg(t *testing.T) {
	result := ParseHtmlProfilerText(inlineSvgHTML, nil)
	if result.DetectedFormat != "inline_svg" {
		t.Fatalf("detected_format = %q, want inline_svg", result.DetectedFormat)
	}
	expected := map[string]int{
		"all;main;compute": 3,
		"all;main;render":  3,
		"all;worker;poll":  4,
	}
	assertStacksEqual(t, expected, result.Stacks)
}

func TestHtmlProfilerAsyncProfilerJS(t *testing.T) {
	result := ParseHtmlProfilerText(asyncProfilerHTML, nil)
	if result.DetectedFormat != "async_profiler_js" {
		t.Fatalf("detected_format = %q, want async_profiler_js", result.DetectedFormat)
	}
	expected := map[string]int{
		"all;main;compute": 3,
		"all;main;render":  3,
		"all;worker;poll":  4,
	}
	assertStacksEqual(t, expected, result.Stacks)
}

func TestHtmlProfilerAsyncProfilerJSWithSelfTime(t *testing.T) {
	// Parent samples > sum of children — extra goes to the parent stack as
	// self-time (e.g. inlined frames that aren't broken out as children).
	html := `<html><body><script>
var root = {name:"all", samples:10, children:[
  {name:"main", samples:6, children:[]},
  {name:"worker", samples:3, children:[]}
]};
</script></body></html>`
	result := ParseHtmlProfilerText(html, nil)
	if result.DetectedFormat != "async_profiler_js" {
		t.Fatalf("detected_format = %q, want async_profiler_js", result.DetectedFormat)
	}
	expected := map[string]int{
		"all":        1, // 10 - (6 + 3) = 1 self
		"all;main":   6,
		"all;worker": 3,
	}
	assertStacksEqual(t, expected, result.Stacks)
}

func TestHtmlProfilerUnsupportedFormat(t *testing.T) {
	html := `<html><body><h1>random page</h1></body></html>`
	result := ParseHtmlProfilerText(html, nil)
	if result.DetectedFormat != "unsupported" {
		t.Fatalf("detected_format = %q, want unsupported", result.DetectedFormat)
	}
	if _, ok := result.Diagnostics.SkippedByReason["UNSUPPORTED_HTML_FORMAT"]; !ok {
		t.Fatalf("expected UNSUPPORTED_HTML_FORMAT diagnostic; got %+v", result.Diagnostics.SkippedByReason)
	}
}

func TestHtmlProfilerFromDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flame.html")
	if err := os.WriteFile(path, []byte(asyncProfilerHTML), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := ParseHtmlProfilerFile(path, nil)
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
}

func TestAnalyzeFlamegraphHTMLFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flame.html")
	if err := os.WriteFile(path, []byte(asyncProfilerHTML), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := AnalyzeFlamegraphHTMLFile(path, Options{IntervalMS: 100, TopN: 5, ProfileKind: "wall"})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.Type != "profiler_collapsed" {
		t.Fatalf("type = %q, want profiler_collapsed", result.Type)
	}
	if result.Metadata.Parser != "flamegraph_html" {
		t.Fatalf("parser = %q, want flamegraph_html", result.Metadata.Parser)
	}
	if result.Summary.TotalSamples != 10 {
		t.Fatalf("total_samples = %d, want 10", result.Summary.TotalSamples)
	}
}
