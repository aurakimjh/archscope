// [한글] html_test — HTML 입력의 두 형식(인라인 SVG / async-profiler JS)
// 정확 인식, 형식 미인식 시 UNSUPPORTED_HTML_FORMAT 진단, JS data block 의
// 키 별칭(samples/value/v, name/title/frame, children/c) 처리 등 검증.

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

// Producer-shaped records from async-profiler 2.x (absolute coordinates) and
// 3.x+ (prefix-compressed pool, delta coordinates, inherited width).
func TestHtmlProfilerCanvas(t *testing.T) {
	for name, data := range map[string]string{
		"legacy": `f(0,0,10,0,'all')
f(1,1,6,1,'main')
f(2,2,2,1,'compute')
f(2,4,3,1,'render')
f(1,7,3,1,'worker')`,
		"compressed": `const cpool = ['all',' main',' compute',' render',' worker'];
unpack(cpool);
f(0,0,0,10)
f(8,1,1,6)
f(16,2,1,2)
n(24,3)
f(32,1,3)`,
	} {
		t.Run(name, func(t *testing.T) {
			result := ParseHtmlProfilerText("<html><body><svg><path/></svg><canvas/><script>\n"+data+"\n</script></body></html>", nil)
			if result.DetectedFormat != "async_profiler_canvas" || result.Diagnostics.ErrorCount != 0 {
				t.Fatalf("%+v", result)
			}
			assertStacksEqual(t, map[string]int{"all": 1, "all;main": 1, "all;main;compute": 2, "all;main;render": 3, "all;worker": 3}, result.Stacks)
		})
	}
}

func TestHtmlProfilerCanvasPoolAndInheritedWidth(t *testing.T) {
	html := `<html><script>
const cpool = [
'all',
' com/acme/Worker.run',
'0stop',
' a\'b,)]\\x\uD83D\uDE00'
];
unpack(cpool);
f(0,0,0,10)
u(8,4)
u(16)
f(24,1,4,0)
</script></html>`
	// '0' encodes a 16-character prefix, preserving com/acme/Worker.
	result := ParseHtmlProfilerText(html, nil)
	if result.Diagnostics.ErrorCount != 0 {
		t.Fatalf("%+v", result.Diagnostics)
	}
	assertStacksEqual(t, map[string]int{"all": 2, "all;com/acme/Worker.run;com/acme/Worker.stop": 4, "all;a'b,)]\\x😀": 4}, result.Stacks)
}

func TestHtmlProfilerCanvasRejectsCorruptData(t *testing.T) {
	for _, records := range []string{
		"f(80,0,0,10)",                // Invalid pool index.
		"f(0,0,0,10)\nu(8,11)",        // Child exceeds parent.
		"f(0,0,0,10)\nu(8,6)\nn(8,6)", // Sibling exceeds parent.
		"f(0,0,0,10)\nf(8,4097,0,1)",
		"f(0,0,0,10)\nu(8,notData())", // Do not evaluate expressions.
		"f(0,0,0,10)\nf(8,1,-1,1)",
	} {
		result := ParseHtmlProfilerText("<html><script>\nconst cpool=['all',' child'];\nunpack(cpool);\n"+records+"\n</script></html>", nil)
		if result.Diagnostics.ErrorCount == 0 || len(result.Stacks) != 0 {
			t.Fatalf("accepted %q: %+v", records, result)
		}
	}
}

func TestAnalyzeArthasCanvasHTMLFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arthas.html")
	if err := os.WriteFile(path, []byte("<html><script>\nf(0,0,12,0,'all')\nf(1,0,12,1,'com/acme/Worker.run')\n</script></html>"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := AnalyzeFlamegraphHTMLFile(path, Options{})
	if err != nil || result.Summary.TotalSamples != 12 || result.Metadata.Diagnostics.ErrorCount != 0 {
		t.Fatalf("%+v %v", result, err)
	}
}
