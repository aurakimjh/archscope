// ─────────────────────────────────────────────────────────────────────
// [한글] html — HTML 형태로 export 된 flame graph 입력 처리.
//
// 책임/목적
//   async-profiler / perf flamegraph 등이 만든 단일 HTML 파일을 읽어
//   collapsed stack map(map[stackKey]samples) 으로 변환한다. 형식이 두
//   가지 — inline SVG 와 JS data block — 이라 정규식으로 둘 다 시도.
//
// 입력 케이스
//   1) Inline SVG (perf flamegraph 등)
//      <svg>...</svg> 블록을 정규식으로 잘라 svg.go 의 ParseSvgFlamegraph
//      Text 에 위임. 결과 진단은 mergeSvgDiagnostics 로 host 진단에 합산.
//   2) async-profiler JS data block
//      "var root = {...};" 같은 JS 변수 선언을 정규식으로 캡처해 JS 객체
//      문자열을 추출. JS 키는 unquoted 라 htmlJsKeyRE 로 따옴표 보강 후
//      json.Unmarshal. walkAsyncProfilerNode 가 트리를 재귀 순회하며
//      collapsed stack 으로 평탄화 (self-time = parent - sum(children)).
//
// 둘 다 실패하면 UNSUPPORTED_HTML_FORMAT 진단과 빈 결과.
//
// 트리키한 부분
//   • async-profiler 의 키 이름은 버전마다 root/levels/profileTree/flame
//     중 하나라 정규식이 alternation 으로 모두 허용.
//   • children 의 sample value 는 samples/value/v 키 어느 것이든 가능.
//     name 은 name/title/frame 어느 것이든 허용. firstNonNil 로 우선순위.
//   • self-time 계산 (parent - sum(children)) 은 음수가 될 수 있어
//     >0 가드. children 이 없으면 parent 자체를 self-time 으로 본다.
//   • intFromAny 는 float64/string 모두 받아 음수면 0 으로 정규화.
// ─────────────────────────────────────────────────────────────────────

package profiler

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var (
	htmlSvgBlockRE = regexp.MustCompile(`(?is)<svg\b[^>]*>.*?</svg>`)
	// async-profiler 2.x/3.x emits a JS variable that holds the root flame node.
	// We accept any of the common identifiers (root / levels / profileTree / flame).
	htmlAsyncProfilerRootRE = regexp.MustCompile(`(?s)(?:var|let|const)\s+(?:root|levels|profileTree|flame)\s*=\s*(\{.*?\})\s*;`)
	// Used to quote unquoted keys before json.Unmarshal — the async-profiler
	// payload is JS-shape, not strict JSON.
	htmlJsKeyRE = regexp.MustCompile(`([\{,])\s*([A-Za-z_][A-Za-z0-9_]*)\s*:`)
)

type HtmlProfilerParseResult struct {
	Stacks         map[string]int
	Diagnostics    ParserDiagnostics
	DetectedFormat string
}

// [한글] ParseHtmlProfilerText — HTML 텍스트를 stack map 으로.
// 두 형식(inline SVG, async-profiler JS) 을 순차 시도해 첫 성공한 결과 반환.
// 둘 다 실패하면 UNSUPPORTED_HTML_FORMAT 진단 + 빈 stacks.
func ParseHtmlProfilerText(text string, debugLog *DebugLog) HtmlProfilerParseResult {
	diagnostics := ParserDiagnostics{
		Format:          "flamegraph_html",
		SkippedByReason: map[string]int{},
	}
	diagnostics.TotalLines = strings.Count(text, "\n") + 1

	// 1) Inline-SVG flamegraph (perf flamegraph + html shell, etc.)
	if match := htmlSvgBlockRE.FindString(text); match != "" {
		svgResult := ParseSvgFlamegraphText(match)
		if len(svgResult.Stacks) > 0 {
			merged := mergeSvgDiagnostics(diagnostics, svgResult.Diagnostics)
			debugLog.SetTotals(merged.TotalLines, merged.ParsedRecords, merged.SkippedLines)
			return HtmlProfilerParseResult{
				Stacks:         svgResult.Stacks,
				Diagnostics:    merged,
				DetectedFormat: "inline_svg",
			}
		}
	}

	// 2) async-profiler embedded JS tree.
	if match := htmlAsyncProfilerRootRE.FindStringSubmatch(text); match != nil {
		stacks := stacksFromAsyncProfilerJS(match[1], &diagnostics, debugLog)
		if len(stacks) > 0 {
			parsed := 0
			for _, count := range stacks {
				parsed += count
			}
			diagnostics.ParsedRecords = parsed
			debugLog.SetTotals(diagnostics.TotalLines, diagnostics.ParsedRecords, diagnostics.SkippedLines)
			return HtmlProfilerParseResult{
				Stacks:         stacks,
				Diagnostics:    diagnostics,
				DetectedFormat: "async_profiler_js",
			}
		}
	}

	preview := text
	if len(preview) > 200 {
		preview = preview[:200]
	}
	msg := "HTML payload does not contain an inline <svg> flamegraph nor a recognized async-profiler JS data block."
	addDiagnosticError(&diagnostics, 0, "UNSUPPORTED_HTML_FORMAT", msg, preview)
	debugLog.AddParseError(0, "UNSUPPORTED_HTML_FORMAT", msg, "htmlSvgBlockRE|htmlAsyncProfilerRootRE", map[string]string{"target": preview}, nil)
	return HtmlProfilerParseResult{
		Stacks:         map[string]int{},
		Diagnostics:    diagnostics,
		DetectedFormat: "unsupported",
	}
}

func ParseHtmlProfilerFile(path string, debugLog *DebugLog) (HtmlProfilerParseResult, error) {
	bytes, err := readAllUTF8(path)
	if err != nil {
		return HtmlProfilerParseResult{}, err
	}
	result := ParseHtmlProfilerText(string(bytes), debugLog)
	source := path
	result.Diagnostics.SourceFile = &source
	return result, nil
}

func mergeSvgDiagnostics(host ParserDiagnostics, svg ParserDiagnostics) ParserDiagnostics {
	host.ParsedRecords = svg.ParsedRecords
	host.SkippedLines += svg.SkippedLines
	if host.SkippedByReason == nil {
		host.SkippedByReason = map[string]int{}
	}
	for reason, count := range svg.SkippedByReason {
		host.SkippedByReason[reason] += count
	}
	host.Samples = append(host.Samples, svg.Samples...)
	if len(host.Samples) > 20 {
		host.Samples = host.Samples[:20]
	}
	host.Warnings = append(host.Warnings, svg.Warnings...)
	host.WarningCount = len(host.Warnings)
	host.Errors = append(host.Errors, svg.Errors...)
	host.ErrorCount = len(host.Errors)
	return host
}

func stacksFromAsyncProfilerJS(jsText string, diagnostics *ParserDiagnostics, debugLog *DebugLog) map[string]int {
	normalized := htmlJsKeyRE.ReplaceAllString(jsText, `$1"$2":`)
	var root any
	if err := json.Unmarshal([]byte(normalized), &root); err != nil {
		preview := jsText
		if len(preview) > 200 {
			preview = preview[:200]
		}
		addDiagnosticError(diagnostics, 0, "INVALID_ASYNC_PROFILER_JSON", err.Error(), preview)
		debugLog.AddParseError(0, "INVALID_ASYNC_PROFILER_JSON", err.Error(), "json.Unmarshal", map[string]string{"target": preview}, nil)
		return nil
	}
	stacks := map[string]int{}
	walkAsyncProfilerNode(root, nil, stacks)
	return stacks
}

// [한글] walkAsyncProfilerNode — async-profiler JS tree 재귀 평탄화.
// 각 node 는 name/samples/children 을 가진다 (key 별칭 지원).
// 자식이 있으면 self-time = parent.samples - sum(children.samples) 로
// 노드별 exclusive 분량을 collapsed stack 으로 누적.
// 자식이 없으면 parent.samples 자체가 leaf samples.
func walkAsyncProfilerNode(node any, path []string, stacks map[string]int) {
	obj, ok := node.(map[string]any)
	if !ok {
		return
	}
	name := stringFromAny(firstNonNil(obj["name"], obj["title"], obj["frame"]))
	nextPath := path
	if name != "" {
		nextPath = append(append([]string{}, path...), name)
	} else {
		nextPath = append([]string{}, path...)
	}

	rawSamples := firstNonNil(obj["samples"], obj["value"], obj["v"])
	parentSamples := intFromAny(rawSamples)

	rawChildren := firstNonNil(obj["children"], obj["c"])
	children, _ := rawChildren.([]any)

	if len(children) == 0 {
		if parentSamples > 0 && len(nextPath) > 0 {
			stacks[strings.Join(nextPath, ";")] += parentSamples
		}
		return
	}

	childrenTotal := 0
	for _, child := range children {
		childObj, ok := child.(map[string]any)
		if !ok {
			continue
		}
		childRaw := firstNonNil(childObj["samples"], childObj["value"], childObj["v"])
		childrenTotal += intFromAny(childRaw)
	}
	selfSamples := parentSamples - childrenTotal
	if selfSamples > 0 && len(nextPath) > 0 {
		stacks[strings.Join(nextPath, ";")] += selfSamples
	}
	for _, child := range children {
		walkAsyncProfilerNode(child, nextPath, stacks)
	}
}

func firstNonNil(values ...any) any {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case nil:
		return 0
	case float64:
		if v < 0 {
			return 0
		}
		return int(v)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || parsed < 0 {
			return 0
		}
		return parsed
	default:
		return 0
	}
}
