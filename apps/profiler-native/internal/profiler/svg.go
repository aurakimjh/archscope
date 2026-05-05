package profiler

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Go's encoding/xml does not expand external entities by default, so SVG
// flamegraph parsing is XXE-safe out of the box (see net/xml docs and
// https://github.com/golang/go/issues/24157). The Python parser uses
// defusedxml for the same property.

// "name (12,345 samples, 0.78%)" or "name (12345 samples, 0.78%, 1.23 seconds)"
var svgTitleRE = regexp.MustCompile(`^(?P<name>.+?)\s*\(\s*(?P<samples>[\d,]+)\s*samples?\b`)

type svgRect struct {
	X       float64
	Y       float64
	Width   float64
	Height  float64
	Name    string
	Samples int
}

type SvgFlamegraphParseResult struct {
	Stacks      map[string]int
	Diagnostics ParserDiagnostics
}

func ParseSvgFlamegraphFile(path string) (SvgFlamegraphParseResult, error) {
	source := path
	diagnostics := ParserDiagnostics{
		SourceFile:      &source,
		Format:          "flamegraph_svg",
		SkippedByReason: map[string]int{},
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return SvgFlamegraphParseResult{Diagnostics: diagnostics}, err
	}
	return parseSvgFlamegraphBytes(bytes, diagnostics), nil
}

func ParseSvgFlamegraphText(text string) SvgFlamegraphParseResult {
	diagnostics := ParserDiagnostics{
		Format:          "flamegraph_svg",
		SkippedByReason: map[string]int{},
	}
	return parseSvgFlamegraphBytes([]byte(text), diagnostics)
}

func parseSvgFlamegraphBytes(payload []byte, diagnostics ParserDiagnostics) SvgFlamegraphParseResult {
	rects, err := extractSvgRects(payload, &diagnostics)
	if err != nil {
		preview := string(payload)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		addDiagnosticError(&diagnostics, 0, "INVALID_SVG", err.Error(), preview)
		return SvgFlamegraphParseResult{Stacks: map[string]int{}, Diagnostics: diagnostics}
	}
	if len(rects) == 0 {
		return SvgFlamegraphParseResult{Stacks: map[string]int{}, Diagnostics: diagnostics}
	}

	stacks := stacksFromSvgRects(rects)
	diagnostics.TotalLines = len(rects)
	parsed := 0
	for _, count := range stacks {
		parsed += count
	}
	diagnostics.ParsedRecords = parsed
	return SvgFlamegraphParseResult{Stacks: stacks, Diagnostics: diagnostics}
}

func extractSvgRects(payload []byte, diagnostics *ParserDiagnostics) ([]svgRect, error) {
	type pendingGroup struct {
		title    string
		hasTitle bool
		rect     *svgRect
	}
	decoder := xml.NewDecoder(strings.NewReader(string(payload)))
	stack := []*pendingGroup{}
	rects := []svgRect{}

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "g":
				stack = append(stack, &pendingGroup{})
			case "title":
				if len(stack) > 0 {
					var content string
					if err := decoder.DecodeElement(&content, &t); err != nil {
						// Skip malformed title text but keep parsing.
						continue
					}
					stack[len(stack)-1].title = strings.TrimSpace(content)
					stack[len(stack)-1].hasTitle = true
				}
			case "rect":
				if len(stack) > 0 {
					rect, ok := parseSvgRectAttrs(t.Attr, stack[len(stack)-1].title, diagnostics)
					if ok {
						copy := rect
						stack[len(stack)-1].rect = &copy
					}
					if err := decoder.Skip(); err != nil {
						return nil, err
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "g" && len(stack) > 0 {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if !top.hasTitle || top.rect == nil {
					continue
				}
				match := svgTitleRE.FindStringSubmatch(top.title)
				if match == nil {
					preview := top.title
					if len(preview) > 80 {
						preview = preview[:80]
					}
					addDiagnosticError(diagnostics, 0, "UNPARSEABLE_TITLE",
						"Title does not match name(N samples) pattern: "+preview, top.title)
					continue
				}
				name := strings.TrimSpace(match[1])
				samplesText := strings.ReplaceAll(strings.ReplaceAll(match[2], ",", ""), " ", "")
				samples, err := strconv.Atoi(samplesText)
				if err != nil {
					addDiagnosticError(diagnostics, 0, "INVALID_SAMPLE_COUNT",
						fmt.Sprintf("Sample count is not an integer: %q", samplesText), top.title)
					continue
				}
				if samples <= 0 {
					continue
				}
				top.rect.Name = name
				top.rect.Samples = samples
				rects = append(rects, *top.rect)
			}
		}
	}
	return rects, nil
}

func parseSvgRectAttrs(attrs []xml.Attr, title string, diagnostics *ParserDiagnostics) (svgRect, bool) {
	out := svgRect{}
	parsed := map[string]bool{}
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "x":
			value, err := strconv.ParseFloat(attr.Value, 64)
			if err != nil {
				preview := title
				if len(preview) > 200 {
					preview = preview[:200]
				}
				addDiagnosticError(diagnostics, 0, "INVALID_RECT_GEOMETRY", "Could not parse rect x attribute.", preview)
				return out, false
			}
			out.X = value
			parsed["x"] = true
		case "y":
			value, err := strconv.ParseFloat(attr.Value, 64)
			if err != nil {
				addDiagnosticError(diagnostics, 0, "INVALID_RECT_GEOMETRY", "Could not parse rect y attribute.", title)
				return out, false
			}
			out.Y = value
			parsed["y"] = true
		case "width":
			value, err := strconv.ParseFloat(attr.Value, 64)
			if err != nil {
				addDiagnosticError(diagnostics, 0, "INVALID_RECT_GEOMETRY", "Could not parse rect width attribute.", title)
				return out, false
			}
			out.Width = value
			parsed["width"] = true
		case "height":
			value, err := strconv.ParseFloat(attr.Value, 64)
			if err != nil {
				addDiagnosticError(diagnostics, 0, "INVALID_RECT_GEOMETRY", "Could not parse rect height attribute.", title)
				return out, false
			}
			out.Height = value
			parsed["height"] = true
		}
	}
	if out.Width <= 0 || out.Height <= 0 {
		return out, false
	}
	return out, true
}

func stacksFromSvgRects(rects []svgRect) map[string]int {
	if len(rects) == 0 {
		return map[string]int{}
	}
	rowHeight := 16.0
	first := true
	for _, r := range rects {
		if r.Height <= 0 {
			continue
		}
		if first || r.Height < rowHeight {
			rowHeight = r.Height
			first = false
		}
	}
	if rowHeight <= 0 {
		rowHeight = 16.0
	}
	rowOf := func(r svgRect) int {
		return int(roundHalfEven(r.Y / rowHeight))
	}

	byRow := map[int][]svgRect{}
	for _, rect := range rects {
		row := rowOf(rect)
		byRow[row] = append(byRow[row], rect)
	}
	for row := range byRow {
		sort.SliceStable(byRow[row], func(i, j int) bool {
			return byRow[row][i].X < byRow[row][j].X
		})
	}

	widest := rects[0]
	for _, rect := range rects[1:] {
		if rect.Width > widest.Width {
			widest = rect
		}
	}
	rootRow := rowOf(widest)

	rowsSorted := make([]int, 0, len(byRow))
	for row := range byRow {
		rowsSorted = append(rowsSorted, row)
	}
	sort.Ints(rowsSorted)
	rootIndex := 0
	for index, row := range rowsSorted {
		if row == rootRow {
			rootIndex = index
			break
		}
	}
	towardLeavesStep := -1
	if rootIndex == 0 {
		towardLeavesStep = 1
	}
	towardRootStep := -towardLeavesStep

	leaves := []svgRect{}
	for _, rect := range rects {
		next := byRow[rowOf(rect)+towardLeavesStep]
		hasDescendant := false
		for _, candidate := range next {
			if !(candidate.X+candidate.Width <= rect.X || candidate.X >= rect.X+rect.Width) {
				hasDescendant = true
				break
			}
		}
		if !hasDescendant {
			leaves = append(leaves, rect)
		}
	}

	stacks := map[string]int{}
	maxRows := len(rowsSorted)
	for _, leaf := range leaves {
		path := []string{}
		current := leaf
		currentRow := rowOf(current)
		for step := 0; step <= maxRows; step++ {
			path = append(path, current.Name)
			if currentRow == rootRow {
				break
			}
			parentRow := currentRow + towardRootStep
			parentRects := byRow[parentRow]
			center := current.X + current.Width/2
			var parent *svgRect
			for index, candidate := range parentRects {
				if candidate.X <= center && center <= candidate.X+candidate.Width {
					parent = &parentRects[index]
					break
				}
			}
			if parent == nil {
				break
			}
			current = *parent
			currentRow = parentRow
		}
		if len(path) == 0 {
			continue
		}
		// reverse path
		for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
			path[i], path[j] = path[j], path[i]
		}
		stacks[strings.Join(path, ";")] += leaf.Samples
	}
	return stacks
}

func roundHalfEven(value float64) float64 {
	// Mirrors Python's round() banker's rounding for stable row indexing
	// across int(rect.y/row_height) edge cases. For non-pathological input
	// (integer y values that are multiples of row_height) this matches
	// strconv-style float rounding.
	if value < 0 {
		return -roundHalfEven(-value)
	}
	floor := float64(int64(value))
	diff := value - floor
	if diff < 0.5 {
		return floor
	}
	if diff > 0.5 {
		return floor + 1
	}
	if int64(floor)%2 == 0 {
		return floor
	}
	return floor + 1
}
