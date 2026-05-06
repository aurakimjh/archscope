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
