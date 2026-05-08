package profiler

import (
	"sort"
	"strings"
)

var timelineOrder = []string{
	"STARTUP_FRAMEWORK",
	"INTERNAL_METHOD",
	"SQL_EXECUTION",
	"DB_NETWORK_WAIT",
	"NETWORK_PREP",
	"EXTERNAL_CALL",
	"EXTERNAL_NETWORK_WAIT",
	"CONNECTION_POOL_WAIT",
	"LOCK_SYNCHRONIZATION_WAIT",
	"NETWORK_IO_WAIT",
	"FILE_IO",
	"JVM_GC_RUNTIME",
	"UNKNOWN",
}

var timelineLabels = map[string]string{
	"STARTUP_FRAMEWORK":         "Startup / framework",
	"INTERNAL_METHOD":           "Internal method",
	"SQL_EXECUTION":             "SQL execution",
	"DB_NETWORK_WAIT":           "DB network wait",
	"NETWORK_PREP":              "External call prep (network)",
	"EXTERNAL_CALL":             "External call",
	"EXTERNAL_NETWORK_WAIT":     "External network wait",
	"CONNECTION_POOL_WAIT":      "Connection pool wait",
	"LOCK_SYNCHRONIZATION_WAIT": "Lock / synchronization wait",
	"NETWORK_IO_WAIT":           "Network / I/O wait",
	"FILE_IO":                   "File I/O",
	"JVM_GC_RUNTIME":            "JVM / GC runtime",
	"UNKNOWN":                   "Unclassified",
}

type timelineAccumulator struct {
	Samples int
	Methods map[string]int
	Chains  map[string]int
	Stacks  map[string]int
}

func buildTimeline(root FlameNode, options Options, originalTotal int) ([]TimelineRow, TimelineScope) {
	baseMethod := strings.TrimSpace(options.TimelineBaseMethod)
	stageTotal := root.Samples
	if baseMethod != "" {
		stageTotal = 0
	}
	custom := normalizedCustomCategories(options.TimelineCategories)
	accumulators := map[string]*timelineAccumulator{}
	for _, leaf := range iterLeafPaths(root) {
		path := timelineScopedPath(leaf.Path, baseMethod)
		if path == nil {
			continue
		}
		if baseMethod != "" {
			stageTotal += leaf.Samples
		}
		// User-supplied patterns win over the built-in classifier.
		// We check the deepest frame first because the leaf is what
		// the timeline conventionally attributes time to.
		segment := matchCustomCategory(path, custom)
		if segment == "" {
			segment = timelineSegment(path)
		}
		acc := accumulators[segment]
		if acc == nil {
			acc = &timelineAccumulator{
				Methods: map[string]int{},
				Chains:  map[string]int{},
				Stacks:  map[string]int{},
			}
			accumulators[segment] = acc
		}
		acc.Samples += leaf.Samples
		increment(acc.Methods, methodName(path), leaf.Samples)
		increment(acc.Chains, methodChain(path, segment), leaf.Samples)
		increment(acc.Stacks, joinPath(path), leaf.Samples)
	}

	scope := timelineScope(baseMethod, stageTotal, originalTotal)
	if baseMethod != "" && stageTotal <= 0 {
		return []TimelineRow{}, scope
	}

	rows := []TimelineRow{}
	intervalSeconds := options.IntervalMS / 1000
	emitted := map[string]bool{}
	emit := func(index int, segment string) {
		acc := accumulators[segment]
		if acc == nil || acc.Samples <= 0 {
			return
		}
		emitted[segment] = true
		label := timelineLabels[segment]
		if label == "" {
			label = segment
		}
		estimated := round(float64(acc.Samples)*intervalSeconds, 3)
		rows = append(rows, TimelineRow{
			Index:            index,
			Segment:          segment,
			Label:            label,
			Samples:          acc.Samples,
			EstimatedSeconds: estimated,
			StageRatio:       ratio(acc.Samples, stageTotal, 4),
			TotalRatio:       ratio(acc.Samples, originalTotal, 4),
			ElapsedRatio:     elapsedRatio(estimated, options.ElapsedSec, 4),
			TopMethods:       topCounter(acc.Methods, options.TopN),
			MethodChains:     topChainRows(acc.Chains, options.TopN),
			TopStacks:        topCounter(acc.Stacks, options.TopN),
		})
	}
	for index, segment := range timelineOrder {
		emit(index, segment)
	}
	// User-supplied custom segments that aren't in timelineOrder
	// still need to surface in the report; emit them after the
	// built-ins, sorted by descending samples for stable ordering.
	customSegments := []string{}
	for segment := range accumulators {
		if !emitted[segment] {
			customSegments = append(customSegments, segment)
		}
	}
	sort.Slice(customSegments, func(i, j int) bool {
		return accumulators[customSegments[i]].Samples > accumulators[customSegments[j]].Samples
	})
	for offset, segment := range customSegments {
		emit(len(timelineOrder)+offset, segment)
	}
	return rows, scope
}

func timelineScopedPath(path []string, baseMethod string) []string {
	if baseMethod == "" {
		return path
	}
	needle := strings.ToLower(baseMethod)
	for index, frame := range path {
		if strings.Contains(strings.ToLower(frame), needle) {
			return path[index:]
		}
	}
	return nil
}

func timelineScope(baseMethod string, baseSamples int, totalSamples int) TimelineScope {
	mode := "full_profile"
	viewMode := "preserve_full_path"
	var base *string
	if baseMethod != "" {
		mode = "base_method"
		viewMode = "reroot_at_base_frame"
		base = &baseMethod
	}
	var baseRatio *float64
	if totalSamples > 0 {
		baseRatio = floatPtr(round(float64(baseSamples)/float64(totalSamples)*100, 4))
	}
	warnings := []TimelineWarning{}
	if baseMethod != "" && baseSamples <= 0 {
		warnings = append(warnings, TimelineWarning{
			Code:    "TIMELINE_BASE_METHOD_NOT_FOUND",
			Message: "No profiler stack matched the configured timeline base method.",
		})
	}
	return TimelineScope{
		Mode:             mode,
		BaseMethod:       base,
		MatchMode:        "frame_contains_case_insensitive",
		ViewMode:         viewMode,
		BaseSamples:      baseSamples,
		TotalSamples:     totalSamples,
		BaseRatioOfTotal: baseRatio,
		Warnings:         warnings,
	}
}

func timelineSegment(path []string) string {
	classification := classifyFrames(path)
	primary := classification.PrimaryCategory
	wait := ""
	if classification.WaitReason != nil {
		wait = *classification.WaitReason
	}
	switch {
	case primary == "SQL_DATABASE" && wait == "NETWORK_IO_WAIT":
		return "DB_NETWORK_WAIT"
	case primary == "EXTERNAL_API_HTTP" && wait == "NETWORK_IO_WAIT":
		return "EXTERNAL_NETWORK_WAIT"
	case primary == "SQL_DATABASE":
		return "SQL_EXECUTION"
	case primary == "EXTERNAL_API_HTTP":
		return "EXTERNAL_CALL"
	case primary == "CONNECTION_POOL_WAIT":
		return "CONNECTION_POOL_WAIT"
	case primary == "LOCK_SYNCHRONIZATION_WAIT":
		return "LOCK_SYNCHRONIZATION_WAIT"
	case primary == "NETWORK_IO_WAIT":
		return "NETWORK_IO_WAIT"
	case primary == "FILE_IO":
		return "FILE_IO"
	case primary == "GC_JVM_RUNTIME":
		return "JVM_GC_RUNTIME"
	case looksLikeStartup(path):
		return "STARTUP_FRAMEWORK"
	case primary == "APPLICATION_LOGIC" || primary == "FRAMEWORK_MIDDLEWARE":
		return "INTERNAL_METHOD"
	default:
		return "UNKNOWN"
	}
}

func methodName(path []string) string {
	if len(path) == 0 {
		return "(no-frame)"
	}
	return path[len(path)-1]
}

func methodChain(path []string, segment string) string {
	frames := selectChainFrames(path, segment)
	if len(frames) == 0 {
		return "(no-frame)"
	}
	return strings.Join(frames, " -> ")
}

func selectChainFrames(path []string, segment string) []string {
	if len(path) <= 6 {
		return path
	}
	tokens := segmentTokens(segment)
	if len(tokens) > 0 {
		selected := []string{}
		for _, frame := range path {
			lowered := strings.ToLower(frame)
			for _, token := range tokens {
				if strings.Contains(lowered, token) {
					selected = append(selected, frame)
					break
				}
			}
		}
		if len(selected) > 0 {
			if len(selected) > 6 {
				return selected[:6]
			}
			return selected
		}
	}
	return path[len(path)-6:]
}

func segmentTokens(segment string) []string {
	switch segment {
	case "SQL_EXECUTION", "DB_NETWORK_WAIT":
		return []string{"oracle.jdbc", "java.sql", "t4cpreparedstatement", "t4cmarengine", "executequery", "executeupdate", "resultset", "socketinputstream.socketread", "niosocketimpl"}
	case "EXTERNAL_CALL", "EXTERNAL_NETWORK_WAIT":
		return []string{"resttemplate", "webclient", "httpclient", "okhttp", "urlconnection", "mainclientexec", "bhttpconnection", "socketinputstream.socketread", "niosocketimpl"}
	case "CONNECTION_POOL_WAIT":
		return []string{"hikaripool.getconnection", "concurrentbag", "synchronousqueue"}
	case "LOCK_SYNCHRONIZATION_WAIT":
		return []string{"locksupport.park", "unsafe.park", "object.wait", "future.get"}
	default:
		return nil
	}
}

func topChainRows(counter map[string]int, limit int) []TimelineChainRow {
	top := topCounter(counter, limit)
	rows := make([]TimelineChainRow, 0, len(top))
	for _, item := range top {
		frames := []string{}
		if item.Name != "(no-frame)" {
			frames = strings.Split(item.Name, " -> ")
		}
		rows = append(rows, TimelineChainRow{Chain: item.Name, Samples: item.Samples, Frames: frames})
	}
	return rows
}

// normalizedCustomCategories lower-cases and trims user-supplied
// patterns so the matcher can do straight substring comparisons.
// We keep the canonical segment IDs as-is so downstream code can
// place a custom category alongside the built-ins (e.g. NETWORK_PREP).
func normalizedCustomCategories(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for segment, patterns := range in {
		seg := strings.TrimSpace(segment)
		if seg == "" {
			continue
		}
		var cleaned []string
		for _, p := range patterns {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			cleaned = append(cleaned, strings.ToLower(p))
		}
		if len(cleaned) > 0 {
			out[seg] = cleaned
		}
	}
	return out
}

// matchCustomCategory returns the first segment whose patterns match
// any frame in the path. We test the leaf first (deepest frame) so a
// user-defined "NETWORK_PREP" sendToService rule wins over an enclosing
// JVM frame. Returns "" when no rule matches.
func matchCustomCategory(path []string, rules map[string][]string) string {
	if len(rules) == 0 || len(path) == 0 {
		return ""
	}
	// Check leaf first, then walk towards root. This biases to the
	// most specific user intent.
	for i := len(path) - 1; i >= 0; i-- {
		frameLower := strings.ToLower(path[i])
		for segment, patterns := range rules {
			for _, p := range patterns {
				if strings.Contains(frameLower, p) {
					return segment
				}
			}
		}
	}
	return ""
}

func sortedSegmentsBySamples(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if values[keys[i]] == values[keys[j]] {
			return keys[i] < keys[j]
		}
		return values[keys[i]] > values[keys[j]]
	})
	return keys
}
