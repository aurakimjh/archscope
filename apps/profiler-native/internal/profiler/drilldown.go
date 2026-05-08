// ─────────────────────────────────────────────────────────────────────
// [한글] drilldown — flame tree 위에서 점진적 필터링 (drilldown stages).
//
// 책임/목적
//   사용자가 flame graph 에서 특정 패턴(키워드 / 정규식)을 적용해
//   "그 부분만 다시 그리거나" "그 부분을 제외하거나" 하면서 트리를
//   탐색할 수 있도록 지원. 각 단계는 DrilldownStage 한 개를 만들고
//   누적된 stages 가 breadcrumb 으로 UI 에 노출된다.
//
// 알고리즘 흐름
//   STEP 1. CreateRootStage — index=0 "All" stage 생성 (필터 없음).
//   STEP 2. ApplyDrilldownFilter — 부모 stage 의 flame tree 에 필터 적용
//           1) compileFilter (regex 면 unsafe pattern 가드 + RE2 컴파일)
//           2) iterFilteredPaths 로 leaf path 별 매치 인덱스 계산
//           3) ViewMode="reroot_at_match" 면 path 를 매치 위치부터 잘라
//              새 root 를 만들고, "preserve_full_path" 면 그대로 보존
//           4) buildFlameTreeFromPaths 로 새 FlameNode 빌드
//           5) buildDrilldownStage 로 metric/breadcrumb 채워 반환
//   STEP 3. BuildDrilldownStages — 필터 시퀀스를 순차 적용.
//
// MatchMode
//   - anywhere : 어느 한 frame 이라도 패턴 매치되면 path 채택
//   - ordered  : 패턴을 ">"/";" 로 split 한 terms 가 순서대로 등장해야 함
//   - subtree  : (Python 호환) anywhere 와 동일 동작
//
// FilterType
//   - include_text / exclude_text : 단순 substring (case toggle 가능)
//   - regex_include / regex_exclude : RE2 정규식 (case-insensitive 는 (?i))
//
// 주요 함수
//   - CreateRootStage / ApplyDrilldownFilter / BuildDrilldownStages
//   - iterFilteredPaths : path 단위 매치 + reroot 처리
//   - matchedIndex / orderedMatchIndex / firstMatchingFrame
//   - frameMatchesTerm : 단일 frame 매치
//   - compileFilter / compileRegex / isUnsafeRegexPattern
//   - buildFlameTreeFromPaths : 매치된 path 들로 새 flame tree
//
// 트리키한 부분
//   • Go regexp 는 RE2 라 catastrophic backtracking 위험은 없으나,
//     overly long pattern 은 컴파일 메모리 폭증 가능 → 500자 cap.
//   • nestedQuantifierGuard 는 Python 측이 잡아내는 패턴과 동일한
//     "위험 패턴" 을 거부해 byte-level 동등성 유지.
//   • ordered 모드의 regex 는 term 별 lazy 컴파일 (frameMatchesTerm 안).
//   • reroot_at_match 는 include 류에서만 의미있음. exclude 에 적용해도
//     아무 의미 없음(매치된 부분이 아예 없으니).
// ─────────────────────────────────────────────────────────────────────

package profiler

import (
	"fmt"
	"regexp"
	"strings"
)

// DrilldownFilter mirrors the Python `DrilldownFilter` dataclass.
//
// [한글] DrilldownFilter — 한 단계 드릴다운 필터 명세.
// Pattern 은 단순 텍스트 또는 정규식, FilterType/MatchMode/ViewMode 가
// 동작을 결정. Label 이 비어있으면 DisplayLabel() 이 "type:pattern" 자동 생성.
type DrilldownFilter struct {
	Pattern       string `json:"pattern"`
	FilterType    string `json:"filter_type"`    // include_text | exclude_text | regex_include | regex_exclude
	MatchMode     string `json:"match_mode"`     // anywhere | ordered | subtree
	ViewMode      string `json:"view_mode"`      // preserve_full_path | reroot_at_match
	CaseSensitive bool   `json:"case_sensitive"`
	Label         string `json:"label,omitempty"`
}

func (f DrilldownFilter) DisplayLabel() string {
	if f.Label != "" {
		return f.Label
	}
	return f.FilterType + ":" + f.Pattern
}

const maxRegexPatternLength = 500

// Go's regexp uses RE2 — no catastrophic backtracking. We still cap pattern
// length to bound compile-time memory; the nested-quantifier safety net the
// Python implementation needs is unnecessary here.
var nestedQuantifierGuard = regexp.MustCompile(`\((?:[^()\\]|\\.)*[+*](?:[^()\\]|\\.)*\)\s*(?:[+*]|\{\d+(?:,\d*)?\})`)

type filterDiagnostic struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// CreateRootStage produces the index-0 "All" stage.
//
// [한글] CreateRootStage — 드릴다운 시작점("All" stage). 필터가 없으므로
// 부모 비율 100%, 전체 root.Samples 가 그대로 totalSamples 가 된다.
func CreateRootStage(root FlameNode, intervalMS float64, elapsedSec *float64, topN int) DrilldownStage {
	return buildDrilldownStage(stageInputs{
		Index:         0,
		Label:         "All",
		Breadcrumb:    []string{"All"},
		Root:          root,
		FilterSpec:    nil,
		ParentSamples: nil,
		TotalSamples:  root.Samples,
		IntervalMS:    intervalMS,
		ElapsedSec:    elapsedSec,
		TopN:          topN,
	})
}

// ApplyDrilldownFilter returns the next stage after applying `filter` to the
// supplied parent stage.
//
// [한글] ApplyDrilldownFilter — 부모 stage 의 flame tree 에 한 단계 필터 적용.
// regex 컴파일 실패 시 빈 트리 + diagnostic(UNSAFE_REGEX/INVALID_REGEX) 부착,
// 정상이면 iterFilteredPaths 로 매치된 path 만 추려 새 트리 생성.
// totalSamples 는 부모의 metrics["total_samples"] 를 그대로 이어받아 전체
// 비율 기준이 stage 간 일관되게 유지된다.
func ApplyDrilldownFilter(parent DrilldownStage, filter DrilldownFilter, intervalMS float64, elapsedSec *float64, topN int) DrilldownStage {
	parentSamples := parent.Flamegraph.Samples
	totalSamples := parent.Flamegraph.Samples
	if rawTotal, ok := parent.Metrics["total_samples"]; ok {
		switch v := rawTotal.(type) {
		case int:
			totalSamples = v
		case int64:
			totalSamples = int(v)
		case float64:
			totalSamples = int(v)
		}
	}

	compiled, diag := compileFilter(filter)
	if diag != nil {
		emptyRoot := buildFlameTreeFromPaths(nil, filter.DisplayLabel())
		return buildDrilldownStage(stageInputs{
			Index:         parent.Index + 1,
			Label:         filter.DisplayLabel(),
			Breadcrumb:    append(append([]string{}, parent.Breadcrumb...), filter.DisplayLabel()),
			Root:          emptyRoot,
			FilterSpec:    &filter,
			ParentSamples: &parentSamples,
			TotalSamples:  totalSamples,
			IntervalMS:    intervalMS,
			ElapsedSec:    elapsedSec,
			TopN:          topN,
			Diagnostic:    diag,
		})
	}

	paths := iterFilteredPaths(parent.Flamegraph, filter, compiled)
	nextRoot := buildFlameTreeFromPaths(paths, filter.DisplayLabel())
	return buildDrilldownStage(stageInputs{
		Index:         parent.Index + 1,
		Label:         filter.DisplayLabel(),
		Breadcrumb:    append(append([]string{}, parent.Breadcrumb...), filter.DisplayLabel()),
		Root:          nextRoot,
		FilterSpec:    &filter,
		ParentSamples: &parentSamples,
		TotalSamples:  totalSamples,
		IntervalMS:    intervalMS,
		ElapsedSec:    elapsedSec,
		TopN:          topN,
	})
}

// BuildDrilldownStages applies a sequence of filters to the supplied root.
//
// [한글] BuildDrilldownStages — root 위에 filters 시퀀스를 차례로 적용.
// 결과는 [Root, Stage1, Stage2, ...] 순으로 누적된 stage 리스트.
// 각 stage 의 breadcrumb 가 누적되며 UI 의 탐색 경로가 된다.
func BuildDrilldownStages(root FlameNode, filters []DrilldownFilter, intervalMS float64, elapsedSec *float64, topN int) []DrilldownStage {
	stages := []DrilldownStage{CreateRootStage(root, intervalMS, elapsedSec, topN)}
	for _, filter := range filters {
		stages = append(stages, ApplyDrilldownFilter(stages[len(stages)-1], filter, intervalMS, elapsedSec, topN))
	}
	return stages
}

type stageInputs struct {
	Index         int
	Label         string
	Breadcrumb    []string
	Root          FlameNode
	FilterSpec    *DrilldownFilter
	ParentSamples *int
	TotalSamples  int
	IntervalMS    float64
	ElapsedSec    *float64
	TopN          int
	Diagnostic    *filterDiagnostic
}

func buildDrilldownStage(in stageInputs) DrilldownStage {
	intervalSeconds := in.IntervalMS / 1000
	estimatedSeconds := round(float64(in.Root.Samples)*intervalSeconds, 3)
	totalRatio := 0.0
	if in.TotalSamples > 0 {
		totalRatio = round(float64(in.Root.Samples)/float64(in.TotalSamples)*100, 4)
	}
	parentRatio := 100.0
	if in.ParentSamples != nil && *in.ParentSamples > 0 {
		parentRatio = round(float64(in.Root.Samples)/float64(*in.ParentSamples)*100, 4)
	}
	var elapsedRatio *float64
	if in.ElapsedSec != nil && *in.ElapsedSec > 0 {
		value := round(estimatedSeconds/(*in.ElapsedSec)*100, 4)
		elapsedRatio = &value
	}
	metrics := map[string]any{
		"total_samples":      in.TotalSamples,
		"matched_samples":    in.Root.Samples,
		"estimated_seconds":  estimatedSeconds,
		"total_ratio":        totalRatio,
		"parent_stage_ratio": parentRatio,
		"elapsed_ratio":      elapsedRatio,
	}
	stage := DrilldownStage{
		Index:          in.Index,
		Label:          in.Label,
		Breadcrumb:     in.Breadcrumb,
		Filter:         nil,
		Metrics:        metrics,
		Flamegraph:     in.Root,
		TopStacks:      topStacksFromTree(in.Root, in.TopN),
		TopChildFrames: topChildFrames(in.Root, in.TopN),
		Diagnostics:    nil,
	}
	if in.FilterSpec != nil {
		stage.Filter = *in.FilterSpec
	}
	if in.Diagnostic != nil {
		stage.Diagnostics = in.Diagnostic
	}
	return stage
}

type filteredPath struct {
	Path    []string
	Samples int
}

// [한글] iterFilteredPaths — leaf path 별로 필터 적용 + reroot 처리.
// matchedIndex 는 매치된 frame 의 인덱스 (-1 이면 매치 없음).
// exclude 류는 매치 여부 반전. ViewMode=reroot_at_match + include 류 인 경우
// path 를 매치 위치부터 잘라 새 root 를 생성(시각적으로 그 부분만 남는다).
func iterFilteredPaths(root FlameNode, filter DrilldownFilter, compiled *regexp.Regexp) []filteredPath {
	out := []filteredPath{}
	for _, leaf := range iterLeafPaths(root) {
		matched := matchedIndex(leaf.Path, filter, compiled)
		include := matched >= 0
		if filter.FilterType == "exclude_text" || filter.FilterType == "regex_exclude" {
			include = !include
		}
		if !include {
			continue
		}
		nextPath := leaf.Path
		if filter.ViewMode == "reroot_at_match" && matched >= 0 &&
			(filter.FilterType == "include_text" || filter.FilterType == "regex_include") {
			nextPath = leaf.Path[matched:]
		}
		out = append(out, filteredPath{Path: append([]string(nil), nextPath...), Samples: leaf.Samples})
	}
	return out
}

func matchedIndex(path []string, filter DrilldownFilter, compiled *regexp.Regexp) int {
	if filter.MatchMode == "ordered" {
		return orderedMatchIndex(path, filter, compiled)
	}
	return firstMatchingFrame(path, filter, compiled)
}

// [한글] orderedMatchIndex — ordered 모드: pattern 을 ">"/";" 로 split 한
// terms 가 path 위에서 순서대로 등장해야 매치. 첫 매치 frame 의 인덱스를 반환.
// 예) pattern "Controller>Service>DAO" 는 path 에 그 셋이 순서 보존되어
// 등장하는 경우만 매치 → MSA 호출 흐름 매칭에 유용.
func orderedMatchIndex(path []string, filter DrilldownFilter, compiled *regexp.Regexp) int {
	terms := splitTerms(filter.Pattern)
	if len(terms) == 0 {
		return -1
	}
	startIndex := -1
	termIndex := 0
	for frameIndex, frame := range path {
		if frameMatchesTerm(frame, terms[termIndex], filter, compiled) {
			if startIndex < 0 {
				startIndex = frameIndex
			}
			termIndex++
			if termIndex == len(terms) {
				return startIndex
			}
		}
	}
	return -1
}

func firstMatchingFrame(path []string, filter DrilldownFilter, compiled *regexp.Regexp) int {
	for index, frame := range path {
		if frameMatchesTerm(frame, filter.Pattern, filter, compiled) {
			return index
		}
	}
	return -1
}

func frameMatchesTerm(frame, pattern string, filter DrilldownFilter, compiled *regexp.Regexp) bool {
	if filter.FilterType == "regex_include" || filter.FilterType == "regex_exclude" {
		patternToUse := compiled
		if filter.MatchMode == "ordered" {
			candidate, diag := compileRegex(pattern, filter)
			if diag != nil {
				return false
			}
			patternToUse = candidate
		}
		if patternToUse == nil {
			return false
		}
		return patternToUse.MatchString(frame)
	}
	if filter.CaseSensitive {
		return strings.Contains(frame, pattern)
	}
	return strings.Contains(strings.ToLower(frame), strings.ToLower(pattern))
}

func splitTerms(pattern string) []string {
	separators := func(r rune) bool { return r == '>' || r == ';' }
	parts := strings.FieldsFunc(pattern, separators)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// [한글] compileFilter — 필터의 regex 를 미리 컴파일(가능하면).
// regex 가 아니면 nil/nil. ordered 모드의 regex 는 term 별 lazy 컴파일을
// 위해 여기서 nil 만 반환하고, frameMatchesTerm 안에서 term 별 compileRegex.
func compileFilter(filter DrilldownFilter) (*regexp.Regexp, *filterDiagnostic) {
	if filter.FilterType != "regex_include" && filter.FilterType != "regex_exclude" {
		return nil, nil
	}
	if filter.MatchMode == "ordered" {
		// Each term is compiled lazily inside frameMatchesTerm.
		return nil, nil
	}
	return compileRegex(filter.Pattern, filter)
}

func compileRegex(pattern string, filter DrilldownFilter) (*regexp.Regexp, *filterDiagnostic) {
	if isUnsafeRegexPattern(pattern) {
		return nil, &filterDiagnostic{
			Reason:  "UNSAFE_REGEX",
			Message: "Regex pattern is too long or contains a nested quantifier pattern that is disabled for parser safety.",
		}
	}
	final := pattern
	if !filter.CaseSensitive {
		final = "(?i)" + pattern
	}
	compiled, err := regexp.Compile(final)
	if err != nil {
		return nil, &filterDiagnostic{
			Reason:  "INVALID_REGEX",
			Message: fmt.Sprintf("Invalid regex pattern: %v", err),
		}
	}
	return compiled, nil
}

// [한글] isUnsafeRegexPattern — 정규식 거부 가드.
// 1) 길이 > 500 → 메모리/시간 폭증 위험으로 거부
// 2) nestedQuantifierGuard 매치 → Python 측의 catastrophic backtracking
//    위험 패턴과 동일 거부 (Go RE2 는 안전하지만 byte-level parity 유지).
func isUnsafeRegexPattern(pattern string) bool {
	if len(pattern) > maxRegexPatternLength {
		return true
	}
	return nestedQuantifierGuard.MatchString(pattern)
}

func buildFlameTreeFromPaths(paths []filteredPath, rootName string) FlameNode {
	stacks := map[string]int{}
	for _, item := range paths {
		if item.Samples <= 0 || len(item.Path) == 0 {
			continue
		}
		stacks[strings.Join(item.Path, ";")] += item.Samples
	}
	root := buildFlameTree(stacks)
	root.Name = rootName
	return root
}
