package visuals

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"html"
	"math"
	"math/big"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func clean(s string) string {
	s, _, _ = resultcontract.DisplayText(s)
	return s
}
func xmlText(s string) string { var b bytes.Buffer; xml.EscapeText(&b, []byte(s)); return b.String() }
func clipped(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:max(0, n-1)]) + "…"
	}
	return s
}
func cellText(c resultcontract.Cell) string {
	if c.Null {
		return "— (unavailable)"
	}
	s := c.Text
	if c.Unit != "" {
		s += " " + c.Unit
	}
	if c.Availability != "" && c.Availability != "observed" {
		s += " [" + c.Availability + "]"
	}
	if c.Reason != "" {
		s += " (" + c.Reason + ")"
	}
	if c.Truncated {
		s += " [display limited]"
	}
	return clean(s)
}

var spreadsheetNumber = regexp.MustCompile(`^-?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)
var spreadsheetFraction = regexp.MustCompile(`^[+-]?[0-9]+/[0-9]+(?:/[0-9]+)?$`)
var spreadsheetDate = regexp.MustCompile(`^(?:[0-9]{4}[-/.][0-9]{1,2}[-/.][0-9]{1,2}|[0-9]{1,2}[-/.][0-9]{1,2}(?:[-/.][0-9]{2,4})?)(?:[ T][0-9]{1,2}:[0-9]{2}(?::[0-9]{2})?)?$`)

func transportText(s string) string {
	return strings.NewReplacer("\t", " ", "\r", " ", "\n", " ", "\x00", "").Replace(s)
}
func formulaRisk(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	return strings.ContainsRune("=+@", rune(t[0])) || (t[0] == '-' && !spreadsheetNumber.MatchString(t))
}
func headerText(s string) string {
	s = transportText(s)
	if formulaRisk(s) {
		return "'" + s
	}
	return s
}
func htmlTableText(s string) string { return headerText(s) }
func tableText(s string) string {
	s = transportText(s)
	t := strings.TrimSpace(s)
	if formulaRisk(s) || spreadsheetFraction.MatchString(t) || (spreadsheetDate.MatchString(t) && !spreadsheetNumber.MatchString(t)) {
		return "'" + s
	}
	if spreadsheetNumber.MatchString(t) {
		digits := 0
		for _, r := range t {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		integer := strings.TrimPrefix(t, "-")
		if digits > 15 || strings.ContainsAny(t, "eE") || (len(integer) > 1 && integer[0] == '0' && integer[1] >= '0' && integer[1] <= '9') {
			return "'" + s
		}
	}
	return s
}

type Protection struct {
	Version   string `json:"version"`
	TSVCells  int    `json:"tsv_changed_cells"`
	HTMLCells int    `json:"html_changed_cells"`
	Headers   int    `json:"formula_headers"`
}

// Counts refer to currently exported/clipboard table encodings, never alter view values.
func TableProtection(view View) Protection {
	return protectionForVersion(view, "spreadsheet-text-0.2.1")
}
func protectionForVersion(view View, version string) Protection {
	out := Protection{Version: version}
	for _, name := range view.Columns {
		if headerText(clean(name)) != clean(name) {
			out.Headers++
		}
	}
	for _, row := range view.Rows {
		for _, cell := range row {
			text := cellText(cell)
			if tableText(text) != text || version == "spreadsheet-text-0.2.0" && spreadsheetDate.MatchString(strings.TrimSpace(transportText(text))) {
				out.TSVCells++
			}
			if htmlTableText(text) != text {
				out.HTMLCells++
			}
		}
	}
	return out
}

func TSV(view View) []byte {
	var b bytes.Buffer
	writer := csv.NewWriter(&b)
	writer.Comma = '\t'
	writer.UseCRLF = true
	headers := []string{}
	for _, name := range view.Columns {
		headers = append(headers, headerText(clean(name)))
	}
	writer.Write(headers)
	for _, row := range view.Rows {
		values := []string{}
		for _, cell := range row {
			values = append(values, tableText(cellText(cell)))
		}
		writer.Write(values)
	}
	writer.Flush()
	return b.Bytes()
}
func StandaloneHTML(view View, spec Spec) string {
	return `<!doctype html><html lang="en"><meta charset="utf-8"><title>Frozen visual table</title><body>` + TableHTML(view, spec) + `</body></html>`
}
func TableHTML(view View, spec Spec) string {
	var b strings.Builder
	b.WriteString(`<table style="border-collapse:collapse;font-family:'Noto Sans KR','Malgun Gothic',Arial,sans-serif;font-size:11pt"><caption>` + html.EscapeString(clean(spec.Presentation.Title)) + ` · rows ` + fmt.Sprintf("%d–%d / %d", view.Offset+min(1, len(view.Rows)), view.Offset+len(view.Rows), view.Total) + ` · ` + html.EscapeString(view.Unit) + ` · denominator ` + html.EscapeString(view.Denominator) + `</caption><thead><tr>`)
	for _, name := range view.Columns {
		b.WriteString(`<th style="border:1px solid #777;padding:5px;background:#edf2f7">` + html.EscapeString(headerText(clean(name))) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, row := range view.Rows {
		b.WriteString(`<tr>`)
		for _, cell := range row {
			b.WriteString(`<td style="border:1px solid #aaa;padding:5px;mso-number-format:'\@'">` + html.EscapeString(htmlTableText(cellText(cell))) + `</td>`)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table><p>` + html.EscapeString("Source "+view.Sources[0].SHA256+" · view "+spec.ViewSHA256+" · "+view.Quality+" · "+strings.Join(view.Warnings, "; ")) + `</p>`)
	return b.String()
}

// ClipboardTableHTML keeps every element inside one table. PowerPoint imports
// following paragraphs as independent shapes overlapping the table caption.
// Relative width lets Word fit the table to its document text area.
// Saved artifact HTML remains unchanged; this is an Office clipboard transport.
func ClipboardTableHTML(view View, spec Spec) string {
	var b strings.Builder
	b.WriteString(`<table width="100%" style="border-collapse:collapse;table-layout:fixed;width:100%;font-family:'Noto Sans KR','Malgun Gothic',Arial,sans-serif;font-size:11pt"><tbody>`)
	metadataRow := func(lines []string) {
		fmt.Fprintf(&b, `<tr><td colspan="%d" style="border:1px solid #aaa;padding:5px;white-space:normal;word-break:break-all;mso-number-format:'\@'"><span style="font-size:8pt">`, max(1, len(view.Columns)))
		for i, line := range lines {
			if i > 0 {
				b.WriteString(`<br/>`)
			}
			b.WriteString(html.EscapeString(htmlTableText(line)))
		}
		b.WriteString(`</span></td></tr>`)
	}
	metadataRow([]string{clean(spec.Presentation.Title) + " · rows " + fmt.Sprintf("%d–%d / %d", view.Offset+min(1, len(view.Rows)), view.Offset+len(view.Rows), view.Total) + " · " + view.Unit + " · denominator " + view.Denominator})
	b.WriteString(`<tr>`)
	for _, name := range view.Columns {
		b.WriteString(`<th style="border:1px solid #777;padding:5px;background:#edf2f7;font-size:11pt">` + html.EscapeString(headerText(clean(name))) + `</th>`)
	}
	b.WriteString(`</tr>`)
	for _, row := range view.Rows {
		b.WriteString(`<tr>`)
		for _, cell := range row {
			b.WriteString(`<td style="border:1px solid #aaa;padding:5px;font-size:11pt;mso-number-format:'\@'">` + html.EscapeString(htmlTableText(cellText(cell))) + `</td>`)
		}
		b.WriteString(`</tr>`)
	}
	metadataRow([]string{"Source " + view.Sources[0].SHA256, "View " + spec.ViewSHA256, view.Quality, strings.Join(view.Warnings, "; ")})
	b.WriteString(`</tbody></table>`)
	return b.String()
}
func SVG(view View, spec Spec, page int, font []byte) ([]byte, error) {
	if e := spec.Validate(view, false); e != nil {
		return nil, e
	}
	if Digest(font) != FontSHA256 {
		return nil, fmt.Errorf("VISUAL_FONT_UNAVAILABLE")
	}
	pages := Pages(view, spec.Presentation)
	if pages > 5 || page < 0 || page >= pages {
		return nil, fmt.Errorf("VISUAL_PAGE_BUDGET: choose fewer rows or more rows per page; maximum five images")
	}
	p := spec.Presentation
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img"><title>%s</title><defs><style>@font-face{font-family:ReportPinned;src:url(data:font/ttf;base64,%s) format('truetype')}text{font-family:ReportPinned,'Noto Sans KR',sans-serif;fill:#172033}</style></defs>`, p.Width, p.Height, p.Width, p.Height, xmlText(clean(p.Title)), base64.StdEncoding.EncodeToString(font))
	b.WriteString(`<metadata>` + xmlText(string(FontLicense)) + `</metadata>`)
	if !p.Transparent {
		fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#ffffff"/>`, p.Width, p.Height)
	}
	text := func(x, y, size int, value string) {
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="%d">%s</text>`, x, y, size, xmlText(value))
	}
	text(64, 72, p.FontSize+12, clipped(clean(p.Title), p.Width/(p.FontSize/2+6)))
	start, end := page*p.RowsPerPage, min((page+1)*p.RowsPerPage, len(view.Rows))
	if view.Kind == "profile_flamegraph" {
		start, end = 0, len(view.Rows)
	}
	text(64, 120, p.FontSize, clipped(fmt.Sprintf("%s · %s · denominator %s · rows %d–%d / %d · image %d/%d", view.Kind, view.Unit, view.Denominator, view.Offset+min(1, end-start)+start, view.Offset+end, view.Total, page+1, pages), p.Width/(p.FontSize/2+2)))
	text(64, 158, p.FontSize-4, clipped("Time: "+view.TimeScope+" · exact strings in table.tsv/view.json; areas/label clipping are presentation only.", p.Width/(p.FontSize/2+2)))
	rowHeight := p.FontSize*3/2 + 8
	if view.Kind == "profile_flamegraph" {
		maxDepth, minDepth := 0, 4096
		for _, n := range view.Nodes {
			maxDepth = max(maxDepth, n.Depth)
			minDepth = min(minDepth, n.Depth)
		}
		if len(view.Nodes) == 0 {
			minDepth = 0
		}
		available := p.Height - 510
		visibleDepths := max(1, available/(p.FontSize+12))
		for _, node := range view.Nodes {
			if node.Depth-minDepth >= visibleDepths || node.Width <= 0 {
				continue
			}
			x := 64 + int(node.X*float64(p.Width-128))
			width := node.Width * float64(p.Width-128)
			y := 205 + (node.Depth-minDepth)*(p.FontSize+12)
			fmt.Fprintf(&b, `<g><title>%s</title><rect x="%d" y="%d" width="%.6f" height="%d" fill="hsl(%d,55%%,76%%)" stroke="#fff"/>`, xmlText(node.Name+" · inclusive "+node.Value+" · self "+node.Self+" "+view.Unit+" · "+node.Evidence), x, y, width, p.FontSize+10, (node.Depth*41)%360)
			if width > float64(p.FontSize*4) {
				text(x+4, y+p.FontSize, p.FontSize-4, clipped(node.Name+" "+node.Value, int(width)/p.FontSize))
			}
			b.WriteString(`</g>`)
		}
		text(64, p.Height-270, p.FontSize-4, clipped(fmt.Sprintf("Depths %d–%d of %d; %d selected nodes. Hidden nodes retain their space; vertical origin follows this query page.", minDepth+1, min(minDepth+visibleDepths, maxDepth+1), maxDepth+1, len(view.Nodes)), p.Width/(p.FontSize/2+2)))
	} else {
		width := (p.Width - 128) / len(view.Columns)
		for j, name := range view.Columns {
			text(70+j*width, 211, p.FontSize, clipped(clean(name), max(1, (width-14)/p.FontSize)))
		}
		maxDelta := new(big.Rat)
		if view.Kind == "profile_diff" {
			for _, row := range view.Rows {
				n, e := profilemodel.Number(row[3].Text)
				if e == nil {
					n.Abs(n)
					if n.Cmp(maxDelta) > 0 {
						maxDelta.Set(n)
					}
				}
			}
		}
		for i, row := range view.Rows[start:end] {
			y := 230 + i*rowHeight
			if i%2 == 0 {
				fmt.Fprintf(&b, `<rect x="64" y="%d" width="%d" height="%d" fill="#f2f5f9"/>`, y, p.Width-128, rowHeight)
			}
			for j, cell := range row {
				value := cellText(cell)
				text(70+j*width, y+p.FontSize, p.FontSize, clipped(value, max(1, (width-14)/p.FontSize)))
			}
			if view.Kind == "profile_diff" && maxDelta.Sign() > 0 {
				n, e := profilemodel.Number(row[3].Text)
				if e == nil {
					ratio, _ := new(big.Rat).Quo(n, maxDelta).Float64()
					ratio = math.Max(-1, math.Min(1, ratio))
					mid := 64 + 3*width + width/2
					size := math.Abs(ratio) * float64(width/2-10)
					x := float64(mid)
					fill := "#b45309"
					if ratio < 0 {
						x -= size
						fill = "#047857"
					}
					fmt.Fprintf(&b, `<rect x="%.3f" y="%d" width="%.3f" height="4" fill="%s"/><line x1="%d" x2="%d" y1="%d" y2="%d" stroke="#475569"/>`, x, y+rowHeight-6, size, fill, mid, mid, y, y+rowHeight)
				}
			}
		}
	}
	footer := p.Height - 230
	label := view.QualityLabel
	if label == "" {
		label = view.Quality
	}
	sourceLabel := view.Sources[0].SHA256[:16]
	if len(view.Sources) == 2 {
		sourceLabel += " → " + view.Sources[1].SHA256[:16]
	}
	lines := []string{"Source " + sourceLabel + " · view " + spec.ViewSHA256[:16] + " · identity declared", "Scope: " + clean(view.ScopeLabel), "Quality: " + clean(label), fmt.Sprintf("Limitations: %d recorded flags; labels/areas may be limited; full context in view.json", len(view.Warnings)), "Query " + Hash(view.Query)[:16] + " · no inferred cross-source alignment", "Annotation: " + clean(p.Annotation)}
	for i, line := range lines {
		text(64, footer+i*34, p.FontSize-6, clipped(line, p.Width/(p.FontSize/2+1)))
	}

	b.WriteString(`</svg>`)
	body := []byte(b.String())
	if !utf8.Valid(body) {
		return nil, fmt.Errorf("VISUAL_TEXT_INVALID")
	}
	return body, nil
}
