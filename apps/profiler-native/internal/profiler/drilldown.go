package profiler

import (
	"fmt"
	"regexp"
	"strings"
)

// DrilldownFilter mirrors the Python `DrilldownFilter` dataclass.
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
