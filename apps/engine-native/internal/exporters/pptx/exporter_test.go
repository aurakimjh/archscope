package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// readZip pulls every part out of a .pptx byte slice into a name -> body
// map so individual tests can assert on whichever XML part they care
// about.
func readZip(t *testing.T, body []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	out := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		buf, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		out[f.Name] = string(buf)
	}
	return out
}

func sampleResult() models.AnalysisResult {
	r := models.New("access_log", "nginx_combined_with_response_time")
	r.SourceFiles = []string{"/var/log/nginx/access.log"}
	r.CreatedAt = "2026-05-07T12:00:00Z"
	r.Summary = map[string]any{
		"total_requests":  42,
		"avg_response_ms": 123.4,
	}
	r.AddFinding("warning", "SLOW_ENDPOINT", "GET /api/orders > 1s p95", nil)
	r.AddFinding("info", "HEADROOM_OK", "Response time within budget", nil)
	return r
}

func TestWriteEmitsExpectedOOXMLParts(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "nested", "report.pptx")

	if err := Write(out, sampleResult()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	parts := readZip(t, body)

	// Every minimum part PowerPoint needs to open the file.
	wantParts := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"ppt/presentation.xml",
		"ppt/_rels/presentation.xml.rels",
		"ppt/theme/theme1.xml",
		"ppt/slideMasters/slideMaster1.xml",
		"ppt/slideMasters/_rels/slideMaster1.xml.rels",
		"ppt/slideLayouts/slideLayout1.xml",
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels",
		"ppt/slides/slide1.xml",
		"ppt/slides/_rels/slide1.xml.rels",
		"ppt/slides/slide2.xml",
		"ppt/slides/_rels/slide2.xml.rels",
		"ppt/slides/slide3.xml",
		"ppt/slides/_rels/slide3.xml.rels",
	}
	for _, want := range wantParts {
		if _, ok := parts[want]; !ok {
			t.Errorf("missing part %q", want)
		}
	}
}

func TestWriteCreatesParentDirectories(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "deeply", "nested", "missing", "dir", "report.pptx")
	if err := Write(out, sampleResult()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output not created: %v", err)
	}
}

func TestSlide1HasTitleAndMetadataLines(t *testing.T) {
	body, err := Marshal(sampleResult())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parts := readZip(t, body)
	slide1 := parts["ppt/slides/slide1.xml"]
	if !strings.Contains(slide1, "ArchScope Report - access_log") {
		t.Errorf("slide1 missing report title; got:\n%s", slide1)
	}
	for _, want := range []string{
		"Result type: access_log",
		"Created at: 2026-05-07T12:00:00Z",
		"Source files: /var/log/nginx/access.log",
	} {
		if !strings.Contains(slide1, want) {
			t.Errorf("slide1 missing %q", want)
		}
	}
}

func TestSlide2HasSummaryMetrics(t *testing.T) {
	body, err := Marshal(sampleResult())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parts := readZip(t, body)
	slide2 := parts["ppt/slides/slide2.xml"]
	if !strings.Contains(slide2, "Summary Metrics") {
		t.Errorf("slide2 missing Summary Metrics title")
	}
	// Map iteration is non-deterministic — assert each metric appears
	// somewhere, not in a specific order. Integer 42 must NOT appear
	// as 42.0 (formatValue collapses integral floats).
	if !strings.Contains(slide2, "total_requests: 42") {
		t.Errorf("slide2 missing integer-formatted total_requests; got:\n%s", slide2)
	}
	if !strings.Contains(slide2, "avg_response_ms: 123.4") {
		t.Errorf("slide2 missing avg_response_ms; got:\n%s", slide2)
	}
}

func TestSlide3HasFindings(t *testing.T) {
	body, err := Marshal(sampleResult())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parts := readZip(t, body)
	slide3, ok := parts["ppt/slides/slide3.xml"]
	if !ok {
		t.Fatalf("slide3 missing")
	}
	if !strings.Contains(slide3, "warning: GET /api/orders &gt; 1s p95") {
		t.Errorf("slide3 missing escaped warning finding; got:\n%s", slide3)
	}
	if !strings.Contains(slide3, "info: Response time within budget") {
		t.Errorf("slide3 missing info finding; got:\n%s", slide3)
	}
}

func TestNoFindingsOmitsThirdSlide(t *testing.T) {
	r := models.New("access_log", "nginx")
	r.SourceFiles = []string{"a.log"}
	body, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parts := readZip(t, body)
	if _, ok := parts["ppt/slides/slide3.xml"]; ok {
		t.Errorf("slide3 should be absent when no findings")
	}
	// Empty summary -> Summary Metrics slide carries the placeholder.
	if !strings.Contains(parts["ppt/slides/slide2.xml"], "No summary metrics available.") {
		t.Errorf("slide2 missing empty-state placeholder")
	}
	// presentation.xml should reflect 2 slides + 3rd rel for master.
	pres := parts["ppt/presentation.xml"]
	if !strings.Contains(pres, `r:id="rId3"/></p:sldMasterIdLst>`) {
		t.Errorf("presentation.xml master rel should be rId3 for 2-slide deck; got:\n%s", pres)
	}
	// presentation.xml.rels must contain exactly 2 slide rels +
	// 1 slideMaster rel.
	rels := parts["ppt/_rels/presentation.xml.rels"]
	if strings.Count(rels, "/relationships/slide\"") != 2 {
		t.Errorf("expected 2 slide rels, got:\n%s", rels)
	}
	if strings.Count(rels, "/relationships/slideMaster\"") != 1 {
		t.Errorf("expected 1 slideMaster rel, got:\n%s", rels)
	}
}

func TestContentTypesEnumeratesEachSlide(t *testing.T) {
	body, err := Marshal(sampleResult())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parts := readZip(t, body)
	ct := parts["[Content_Types].xml"]
	for _, want := range []string{
		`PartName="/ppt/slides/slide1.xml"`,
		`PartName="/ppt/slides/slide2.xml"`,
		`PartName="/ppt/slides/slide3.xml"`,
		`PartName="/ppt/presentation.xml"`,
		`PartName="/ppt/slideMasters/slideMaster1.xml"`,
		`PartName="/ppt/theme/theme1.xml"`,
	} {
		if !strings.Contains(ct, want) {
			t.Errorf("[Content_Types].xml missing %q", want)
		}
	}
}

// TestSlideXMLParses verifies the slide shape is well-formed enough
// for a streaming XML parser to walk to the title <a:t> and find the
// expected text — the same tree PowerPoint walks.
func TestSlideXMLParses(t *testing.T) {
	body, err := Marshal(sampleResult())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parts := readZip(t, body)
	dec := xml.NewDecoder(strings.NewReader(parts["ppt/slides/slide1.xml"]))
	var foundTitle bool
	var sawAnyText bool
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("xml decode: %v", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "t" && start.Name.Space == "http://schemas.openxmlformats.org/drawingml/2006/main" {
			var inner string
			if err := dec.DecodeElement(&inner, &start); err != nil {
				t.Fatalf("decode <a:t>: %v", err)
			}
			sawAnyText = true
			if strings.Contains(inner, "ArchScope Report - access_log") {
				foundTitle = true
			}
		}
	}
	if !sawAnyText {
		t.Errorf("no <a:t> elements parsed — slide XML malformed")
	}
	if !foundTitle {
		t.Errorf("title <a:t> not found in slide1")
	}
}

func TestXMLEscapesAngleBracketsAndAmpersands(t *testing.T) {
	r := models.New("custom", "demo")
	r.Summary = map[string]any{"path": "<root>&fail"}
	body, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parts := readZip(t, body)
	slide2 := parts["ppt/slides/slide2.xml"]
	if !strings.Contains(slide2, "&lt;root&gt;&amp;fail") {
		t.Errorf("expected XML-escaped value in slide2; got:\n%s", slide2)
	}
	if strings.Contains(slide2, "<root>&fail") {
		t.Errorf("unescaped value leaked into slide2")
	}
}

func TestMarshalAcceptsPlainMap(t *testing.T) {
	body, err := Marshal(map[string]any{
		"type":         "custom",
		"created_at":   "2026-05-07",
		"source_files": []any{"a", "b"},
		"summary":      map[string]any{"k": "v"},
		"metadata": map[string]any{
			"findings": []any{
				map[string]any{"severity": "info", "message": "ok"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parts := readZip(t, body)
	if !strings.Contains(parts["ppt/slides/slide1.xml"], "Source files: a, b") {
		t.Errorf("slide1 missing joined sources")
	}
	if !strings.Contains(parts["ppt/slides/slide3.xml"], "info: ok") {
		t.Errorf("slide3 missing finding from plain-map input")
	}
}

func TestFindingFallsBackToCodeWhenMessageMissing(t *testing.T) {
	body, err := Marshal(map[string]any{
		"type": "custom",
		"metadata": map[string]any{
			"findings": []any{
				map[string]any{"severity": "warning", "code": "NO_MSG"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parts := readZip(t, body)
	if !strings.Contains(parts["ppt/slides/slide3.xml"], "warning: NO_MSG") {
		t.Errorf("expected code fallback; got:\n%s", parts["ppt/slides/slide3.xml"])
	}
}

func TestFindingsCappedAtSix(t *testing.T) {
	r := models.New("custom", "demo")
	for i := 0; i < 10; i++ {
		r.AddFinding("info", "C", "msg", nil)
	}
	body, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parts := readZip(t, body)
	got := strings.Count(parts["ppt/slides/slide3.xml"], "info: msg")
	if got != 6 {
		t.Errorf("expected 6 finding lines, got %d", got)
	}
}
