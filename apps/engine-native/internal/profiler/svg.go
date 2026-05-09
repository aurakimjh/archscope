// ─────────────────────────────────────────────────────────────────────
// [한글] svg — Brendan Gregg flamegraph.pl 가 만든 SVG flame graph 파싱.
//
// 책임/목적
//   "name (12,345 samples, 0.78%)" 형태 title 을 가진 SVG flame graph
//   파일을 읽어 collapsed stack map 으로 복원한다. 이 형식은 텍스트
//   collapsed 가 사라진 상황에서도 시각화 산출물로부터 데이터를 복원하기
//   위한 대안 입력 경로.
//
// 알고리즘 흐름 (트리키함)
//   STEP 1. SVG 파싱
//     XML 디코더로 <g><title>...</title><rect .../></g> 묶음을 추출.
//     Go encoding/xml 은 외부 엔티티를 확장하지 않아 XXE-safe (Python 측은
//     defusedxml 로 동일 보호). 한 group 의 title 과 rect 를 묶어 svgRect 생성.
//   STEP 2. Title 정규식 매칭
//     "name (N samples, ...)" 패턴 추출. 매치 실패는 UNPARSEABLE_TITLE
//     진단으로 skip. samples 정수 변환 실패는 INVALID_SAMPLE_COUNT.
//   STEP 3. Row 추정
//     모든 rect 의 height 중 최소값(또는 16) 을 row height 로 가정.
//     rect.y / rowHeight 의 round-half-even (Python round() 호환) 로 row 번호.
//   STEP 4. Root row / leaf 찾기
//     가장 width 가 큰 rect 의 row 를 root row. 한 row 위쪽으로 가면 자식,
//     아래로 가면 부모인지(또는 그 반대) 는 row index 가 0 인지 max 인지로 결정.
//     자식 rect 가 X-overlap 으로 없는 rect 가 leaf.
//   STEP 5. Leaf → Root 경로 복원
//     leaf 의 X 중심(centerX) 이 부모 row 의 어떤 rect 의 [X, X+Width] 에
//     포함되는지로 부모를 찾고, root row 까지 위로 올라가며 path 누적.
//     그 path 를 ";" 로 join 한 stack key 에 leaf.Samples 누적.
//
// 트리키한 부분
//   • flamegraph.pl 의 좌표계는 "위쪽이 leaf, 아래쪽이 root" 가 보통이지만
//     반대 방향(반전 flame) 도 있어 root row 가 0 이면 "leaves 가 +1
//     쪽"으로, 아니면 "leaves 가 -1 쪽" 으로 자동 판별.
//   • roundHalfEven 은 Python round() 의 banker's rounding 을 명시적으로
//     구현 — int(y/h) 사용 시 floating point edge case 에서 row 번호가
//     달라져 stack 복원이 깨질 수 있음. byte-level parity 를 위해 필요.
//   • parent 매칭은 X 중심점이 [parent.X, parent.X+parent.Width] 안에 있는
//     첫 rect. 이 휴리스틱은 flamegraph.pl 의 layout 가정에서 안전.
// ─────────────────────────────────────────────────────────────────────

package profiler

import (
	"bytes"
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

func ParseSvgFlamegraphFile(path string, debugLog *DebugLog) (SvgFlamegraphParseResult, error) {
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
	return parseSvgFlamegraphBytes(bytes, diagnostics, debugLog), nil
}

func ParseSvgFlamegraphText(text string) SvgFlamegraphParseResult {
	diagnostics := ParserDiagnostics{
		Format:          "flamegraph_svg",
		SkippedByReason: map[string]int{},
	}
	return parseSvgFlamegraphBytes([]byte(text), diagnostics, nil)
}

func parseSvgFlamegraphBytes(payload []byte, diagnostics ParserDiagnostics, debugLog *DebugLog) SvgFlamegraphParseResult {
	rects, err := extractSvgRects(payload, &diagnostics, debugLog)
	if err != nil {
		preview := string(payload)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		addDiagnosticError(&diagnostics, 0, "INVALID_SVG", err.Error(), preview)
		debugLog.AddParseError(0, "INVALID_SVG", err.Error(), "", map[string]string{"target": preview}, nil)
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
	debugLog.SetTotals(diagnostics.TotalLines, diagnostics.ParsedRecords, diagnostics.SkippedLines)
	return SvgFlamegraphParseResult{Stacks: stacks, Diagnostics: diagnostics}
}

// [한글] extractSvgRects — SVG XML 을 순회하며 <g><title>+<rect> 묶음을 svgRect 로.
// pendingGroup 스택으로 중첩 <g> 처리, </g> 시점에 title + rect 가 모두 채워졌으면
// title 정규식 파싱하여 sample count 추출. width<=0 / height<=0 / sample<=0 은 skip.
func extractSvgRects(payload []byte, diagnostics *ParserDiagnostics, debugLog *DebugLog) ([]svgRect, error) {
	type pendingGroup struct {
		title    string
		hasTitle bool
		rect     *svgRect
	}
	decoder := xml.NewDecoder(bytes.NewReader(payload))
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
					rect, ok := parseSvgRectAttrs(t.Attr, stack[len(stack)-1].title, diagnostics, debugLog)
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
					msg := "Title does not match name(N samples) pattern: " + preview
					addDiagnosticError(diagnostics, 0, "UNPARSEABLE_TITLE", msg, top.title)
					debugLog.AddParseError(0, "UNPARSEABLE_TITLE", msg, "svgTitleRE", map[string]string{"target": top.title}, nil)
					continue
				}
				name := strings.TrimSpace(match[1])
				samplesText := strings.ReplaceAll(strings.ReplaceAll(match[2], ",", ""), " ", "")
				samples, err := strconv.Atoi(samplesText)
				if err != nil {
					msg := fmt.Sprintf("Sample count is not an integer: %q", samplesText)
					addDiagnosticError(diagnostics, 0, "INVALID_SAMPLE_COUNT", msg, top.title)
					debugLog.AddParseError(0, "INVALID_SAMPLE_COUNT", msg, "svgTitleRE", map[string]string{"target": top.title}, nil)
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

func parseSvgRectAttrs(attrs []xml.Attr, title string, diagnostics *ParserDiagnostics, debugLog *DebugLog) (svgRect, bool) {
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
				msg := "Could not parse rect x attribute."
				addDiagnosticError(diagnostics, 0, "INVALID_RECT_GEOMETRY", msg, preview)
				debugLog.AddParseError(0, "INVALID_RECT_GEOMETRY", msg, "", map[string]string{"target": preview}, nil)
				return out, false
			}
			out.X = value
			parsed["x"] = true
		case "y":
			value, err := strconv.ParseFloat(attr.Value, 64)
			if err != nil {
				msg := "Could not parse rect y attribute."
				addDiagnosticError(diagnostics, 0, "INVALID_RECT_GEOMETRY", msg, title)
				debugLog.AddParseError(0, "INVALID_RECT_GEOMETRY", msg, "", map[string]string{"target": title}, nil)
				return out, false
			}
			out.Y = value
			parsed["y"] = true
		case "width":
			value, err := strconv.ParseFloat(attr.Value, 64)
			if err != nil {
				msg := "Could not parse rect width attribute."
				addDiagnosticError(diagnostics, 0, "INVALID_RECT_GEOMETRY", msg, title)
				debugLog.AddParseError(0, "INVALID_RECT_GEOMETRY", msg, "", map[string]string{"target": title}, nil)
				return out, false
			}
			out.Width = value
			parsed["width"] = true
		case "height":
			value, err := strconv.ParseFloat(attr.Value, 64)
			if err != nil {
				msg := "Could not parse rect height attribute."
				addDiagnosticError(diagnostics, 0, "INVALID_RECT_GEOMETRY", msg, title)
				debugLog.AddParseError(0, "INVALID_RECT_GEOMETRY", msg, "", map[string]string{"target": title}, nil)
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

// [한글] stacksFromSvgRects — rect 리스트를 collapsed stack map 으로.
// 1) row height 추정 (최소 height 또는 16), 2) 각 rect 를 row 별로 그룹핑
// 후 X 정렬, 3) widest rect 의 row 가 root row, 4) leaf 는 자식 rect 가
// 없는 rect, 5) 각 leaf 에서 X 중심점으로 부모 rect 를 찾아 root 까지 path
// 복원, 6) reverse 후 ";" join 하여 stacks[key] += leaf.Samples.
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
