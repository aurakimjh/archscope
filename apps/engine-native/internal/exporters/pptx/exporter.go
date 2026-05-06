// Package pptx ports archscope_engine.exporters.pptx_exporter — a
// minimal, hand-rolled OOXML (.pptx) writer that emits a PowerPoint-
// openable presentation using only `archive/zip` + string templates.
//
// We deliberately avoid third-party Office-doc libraries (unioffice,
// etc.) so the engine-native binary stays dependency-clean. PPTX is a
// zip of XML parts; we emit only the absolute minimum parts PowerPoint
// needs to render text-only slides:
//
//   - [Content_Types].xml
//   - _rels/.rels
//   - ppt/presentation.xml + ppt/_rels/presentation.xml.rels
//   - ppt/theme/theme1.xml
//   - ppt/slideMasters/slideMaster1.xml + its .rels
//   - ppt/slideLayouts/slideLayout1.xml + its .rels
//   - ppt/slides/slideN.xml + its .rels (one per slide)
//
// Charts, images, animations, notes are intentionally NOT supported —
// matching the Python source.
//
// Slides emitted (driven by `_slides_for_payload` in the Python):
//  1. Title slide (report title + result type / created_at /
//     source_files lines)
//  2. Summary metrics (up to 10 `summary` map entries)
//  3. Findings (only when metadata.findings is non-empty; up to 6)
package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Slide width / height in EMUs (English Metric Units) — 16:9 wide.
// Mirror the Python module-level constants.
const (
	slideWidth  = 13_333_500
	slideHeight = 7_500_000
)

// Write renders `result` as a PPTX presentation and writes it to
// `path`, creating any missing parent directories. `result` may be a
// `models.AnalysisResult`, a plain `map[string]any`, or anything that
// round-trips through `encoding/json` to a map — the exporter only
// reads the public envelope keys (`type`, `created_at`,
// `source_files`, `summary`, `metadata.findings`).
//
// Mirrors Python `write_pptx_report(input_path, output_path)` except
// the source is an in-memory value rather than a JSON file on disk
// (the Python re-reads JSON because it sits behind a CLI; the Go
// surface is library-first, so callers pass the value directly —
// matching T-340's `Write(path, result)` shape).
func Write(path string, result any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	body, err := Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// Marshal builds the .pptx byte-stream for `result` without touching
// the filesystem. Matches T-340's `Marshal(result) ([]byte, error)`
// surface so callers can stream over HTTP / stdout.
func Marshal(result any) ([]byte, error) {
	payload, err := toMap(result)
	if err != nil {
		return nil, fmt.Errorf("normalize payload: %w", err)
	}
	slides := slidesForPayload(payload, "")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	parts := []struct {
		name string
		body string
	}{
		{"[Content_Types].xml", contentTypesXML(len(slides))},
		{"_rels/.rels", rootRelsXML()},
		{"ppt/presentation.xml", presentationXML(len(slides))},
		{"ppt/_rels/presentation.xml.rels", presentationRelsXML(len(slides))},
		{"ppt/theme/theme1.xml", themeXML()},
		{"ppt/slideMasters/slideMaster1.xml", slideMasterXML()},
		{"ppt/slideMasters/_rels/slideMaster1.xml.rels", slideMasterRelsXML()},
		{"ppt/slideLayouts/slideLayout1.xml", slideLayoutXML()},
		{"ppt/slideLayouts/_rels/slideLayout1.xml.rels", slideLayoutRelsXML()},
	}
	for _, p := range parts {
		if err := writePart(zw, p.name, p.body); err != nil {
			return nil, err
		}
	}
	for i, s := range slides {
		idx := i + 1
		if err := writePart(zw, fmt.Sprintf("ppt/slides/slide%d.xml", idx), slideXML(s)); err != nil {
			return nil, err
		}
		if err := writePart(zw, fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", idx), slideRelsXML()); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}

// writePart adds `name` -> `body` to the zip with DEFLATE compression
// (matches Python's `ZIP_DEFLATED`).
func writePart(zw *zip.Writer, name, body string) error {
	w, err := zw.CreateHeader(&zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	})
	if err != nil {
		return fmt.Errorf("zip create %s: %w", name, err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		return fmt.Errorf("zip write %s: %w", name, err)
	}
	return nil
}

// slide is the in-memory shape `slidesForPayload` builds and
// `slideXML` consumes — one title + a list of body paragraphs.
type slide struct {
	Title string
	Lines []string
}

// slidesForPayload mirrors Python `_slides_for_payload`. `title` is a
// caller-supplied override; pass "" to use the default
// "ArchScope Report - <type>" form.
func slidesForPayload(payload map[string]any, title string) []slide {
	resultType := stringField(payload, "type", "unknown")
	reportTitle := title
	if reportTitle == "" {
		reportTitle = "ArchScope Report - " + resultType
	}

	createdAt := stringField(payload, "created_at", "unknown")
	sources := strings.Join(stringList(payload["source_files"]), ", ")

	summary := dictField(payload, "summary")
	metadata := dictField(payload, "metadata")
	findings, _ := metadata["findings"].([]any)

	// Up to 10 summary metric lines, deterministic-ish: encoding/json
	// sorts map keys, so to mirror Python's insertion-ordered dict we
	// just project k:v pairs in whatever order the input map yields.
	// The Python source iterates `summary.items()` with no sort, so
	// neither does this — callers that care can pre-sort.
	metricLines := make([]string, 0, len(summary))
	for k, v := range summary {
		metricLines = append(metricLines, fmt.Sprintf("%s: %s", k, formatValue(v)))
		if len(metricLines) == 10 {
			break
		}
	}

	findingLines := make([]string, 0, 6)
	for _, item := range findings {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		severity := stringField(row, "severity", "info")
		message, ok := row["message"].(string)
		if !ok || message == "" {
			// Python falls back to `code` when message missing.
			if code, ok := row["code"].(string); ok {
				message = code
			}
		}
		findingLines = append(findingLines, fmt.Sprintf("%s: %s", severity, message))
		if len(findingLines) == 6 {
			break
		}
	}

	titleLines := []string{
		"Result type: " + resultType,
		"Created at: " + createdAt,
		"Source files: " + sources,
	}
	summaryLines := metricLines
	if len(summaryLines) == 0 {
		summaryLines = []string{"No summary metrics available."}
	}

	slides := []slide{
		{Title: reportTitle, Lines: titleLines},
		{Title: "Summary Metrics", Lines: summaryLines},
	}
	if len(findingLines) > 0 {
		slides = append(slides, slide{Title: "Findings", Lines: findingLines})
	}
	return slides
}

// formatValue renders a summary value the same way Python's
// `f"{key}: {value}"` would: ints/floats keep their string form,
// strings pass through, anything else falls back to `%v`.
func formatValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		// json.Unmarshal turns all numbers into float64. Match
		// Python's str(int_value) when the value is integral —
		// otherwise the slide reads `total_requests: 1` not `1.0`.
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// toMap normalises arbitrary `result` values into a `map[string]any`
// by JSON-roundtripping. This lets callers pass typed structs
// (`models.AnalysisResult`) or pre-built maps interchangeably.
func toMap(result any) (map[string]any, error) {
	switch r := result.(type) {
	case map[string]any:
		return r, nil
	case nil:
		return map[string]any{}, nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func dictField(m map[string]any, key string) map[string]any {
	v, ok := m[key].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return v
}

func stringField(m map[string]any, key, fallback string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func stringList(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		// Allow []string directly (callers passing typed struct).
		if ss, ok := v.([]string); ok {
			return ss
		}
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		out = append(out, fmt.Sprintf("%v", item))
	}
	return out
}

// xmlEscape mirrors Python `xml.sax.saxutils.escape` (default chars):
// & -> &amp;, < -> &lt;, > -> &gt;. We escape `&` first so already-
// escaped entities don't get double-escaped.
func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(s)
}

// ---------------------------------------------------------------------------
// XML part templates. Strings are byte-identical to the Python source
// where practical so the parity diff stays tiny — only `f"{...}"`
// interpolations differ, since Python and Go format integers the same
// way for our use case.
// ---------------------------------------------------------------------------

func contentTypesXML(slideCount int) string {
	var sb strings.Builder
	for i := 1; i <= slideCount; i++ {
		fmt.Fprintf(&sb,
			`<Override PartName="/ppt/slides/slide%d.xml" `+
				`ContentType="application/vnd.openxmlformats-officedocument.`+
				`presentationml.slide+xml"/>`, i)
		if i < slideCount {
			sb.WriteByte('\n')
		}
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
  <Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
  <Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
  ` + sb.String() + `
</Types>`
}

func rootRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`
}

func presentationXML(slideCount int) string {
	var ids strings.Builder
	for i := 1; i <= slideCount; i++ {
		fmt.Fprintf(&ids, `<p:sldId id="%d" r:id="rId%d"/>`, 255+i, i)
		if i < slideCount {
			ids.WriteByte('\n')
		}
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId%d"/></p:sldMasterIdLst>
  <p:sldIdLst>%s</p:sldIdLst>
  <p:sldSz cx="%d" cy="%d" type="wide"/>
  <p:notesSz cx="6858000" cy="9144000"/>
</p:presentation>`, slideCount+1, ids.String(), slideWidth, slideHeight)
}

func presentationRelsXML(slideCount int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := 1; i <= slideCount; i++ {
		fmt.Fprintf(&sb,
			`<Relationship Id="rId%d" `+
				`Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" `+
				`Target="slides/slide%d.xml"/>`, i, i)
	}
	fmt.Fprintf(&sb,
		`<Relationship Id="rId%d" `+
			`Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" `+
			`Target="slideMasters/slideMaster1.xml"/>`, slideCount+1)
	sb.WriteString(`</Relationships>`)
	return sb.String()
}

func slideXML(s slide) string {
	title := xmlEscape(s.Title)
	var body strings.Builder
	for i, line := range s.Lines {
		body.WriteString(paragraphXML(line))
		if i < len(s.Lines)-1 {
			body.WriteByte('\n')
		}
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>
      %s
      %s
    </p:spTree>
  </p:cSld>
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sld>`, textBoxXML(2, "Title", title, 457200, 342900, 12192000, 762000, 3200), bodyBoxXML(body.String()))
}

func bodyBoxXML(body string) string {
	return fmt.Sprintf(`
<p:sp>
  <p:nvSpPr><p:cNvPr id="3" name="Body"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>
  <p:spPr><a:xfrm><a:off x="685800" y="1371600"/><a:ext cx="11963400" cy="5486400"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>
  <p:txBody><a:bodyPr wrap="square"/><a:lstStyle/>%s</p:txBody>
</p:sp>`, body)
}

func textBoxXML(shapeID int, name, text string, x, y, cx, cy, fontSize int) string {
	return fmt.Sprintf(`
<p:sp>
  <p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>
  <p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>
  <p:txBody><a:bodyPr wrap="square"/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US" sz="%d" b="1"/><a:t>%s</a:t></a:r></a:p></p:txBody>
</p:sp>`, shapeID, xmlEscape(name), x, y, cx, cy, fontSize, text)
}

func paragraphXML(text string) string {
	return `<a:p><a:r><a:rPr lang="en-US" sz="1800"/>` +
		`<a:t>` + xmlEscape(text) + `</a:t></a:r></a:p>`
}

func slideRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
</Relationships>`
}

func slideMasterXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld>
  <p:sldLayoutIdLst><p:sldLayoutId id="1" r:id="rId1"/></p:sldLayoutIdLst>
  <p:txStyles><p:titleStyle/><p:bodyStyle/><p:otherStyle/></p:txStyles>
</p:sldMaster>`
}

func slideMasterRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`
}

func slideLayoutXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank" preserve="1">
  <p:cSld name="Blank"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld>
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sldLayout>`
}

func slideLayoutRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>
</Relationships>`
}

func themeXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="ArchScope">
  <a:themeElements>
    <a:clrScheme name="ArchScope"><a:dk1><a:srgbClr val="111827"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="1F2937"/></a:dk2><a:lt2><a:srgbClr val="F8FAFC"/></a:lt2><a:accent1><a:srgbClr val="2563EB"/></a:accent1><a:accent2><a:srgbClr val="16A34A"/></a:accent2><a:accent3><a:srgbClr val="F59E0B"/></a:accent3><a:accent4><a:srgbClr val="DC2626"/></a:accent4><a:accent5><a:srgbClr val="7C3AED"/></a:accent5><a:accent6><a:srgbClr val="0891B2"/></a:accent6><a:hlink><a:srgbClr val="2563EB"/></a:hlink><a:folHlink><a:srgbClr val="7C3AED"/></a:folHlink></a:clrScheme>
    <a:fontScheme name="ArchScope"><a:majorFont><a:latin typeface="Aptos Display"/></a:majorFont><a:minorFont><a:latin typeface="Aptos"/></a:minorFont></a:fontScheme>
    <a:fmtScheme name="ArchScope"><a:fillStyleLst/><a:lnStyleLst/><a:effectStyleLst/><a:bgFillStyleLst/></a:fmtScheme>
  </a:themeElements>
</a:theme>`
}
