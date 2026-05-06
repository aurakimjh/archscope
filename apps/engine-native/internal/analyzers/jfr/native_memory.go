// native_memory.go ports archscope_engine.analyzers.native_memory_analyzer.
//
// The native memory analyzer pairs NativeMemoryAllocation /
// NativeMemoryFree events emitted by async-profiler's `nativemem`
// mode and reports the call sites that hold *unfreed* memory at the
// end of the recording — the canonical fingerprint of a native memory
// leak.
//
// Two output flavors are produced from the same event stream:
//
//   - "leak"  — only allocations without a matching free, optionally
//     ignoring allocations made in the last `tail_ratio` of the
//     recording (defaults to 10% per async-profiler).
//   - "alloc" — all allocations regardless of free, useful for
//     general hot-spot analysis.
//
// The flame tree counts `size` (bytes) when available, falling back to
// `1` per event when not. Engine returns flame `samples` in bytes so
// the existing flamegraph renderer just works — display labels can show
// units via `metadata.unit`.
package jfr

import (
	"sort"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	jfrparser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/jfr"
)

// NativeMemoryResultType mirrors Python `AnalysisResult(type="native_memory", ...)`.
const NativeMemoryResultType = "native_memory"

// NativeMemoryParserName mirrors the Python `metadata.parser` literal.
const NativeMemoryParserName = "jdk_jfr_native_memory"

// NativeMemorySchemaVersion mirrors the Python schema_version literal.
const NativeMemorySchemaVersion = "0.1.0"

// NativeMemoryTopCallSitesLimit mirrors `stacks.most_common(50)`.
const NativeMemoryTopCallSitesLimit = 50

// NativeMemoryDefaultTailRatio mirrors the Python `tail_ratio=0.10`.
const NativeMemoryDefaultTailRatio = 0.10

var (
	allocEventTypes = setOf(
		"jdk.NativeMemoryAllocation",
		"jdk.NativeAllocation",
	)
	freeEventTypes = setOf(
		"jdk.NativeMemoryFree",
		"jdk.NativeFree",
	)
)

// NativeMemoryOptions captures the analyze_native_memory kwargs.
type NativeMemoryOptions struct {
	LeakOnly      bool
	TailRatio     float64
	tailRatioSet  bool // distinguishes a user-set 0 from the zero value
	leakOnlySet   bool
}

// NewNativeMemoryOptions returns Options with the Python defaults
// (leak_only=true, tail_ratio=0.10) and a "user-set" flag so callers
// can distinguish "leave default" from "explicitly false".
func NewNativeMemoryOptions() NativeMemoryOptions {
	return NativeMemoryOptions{
		LeakOnly:     true,
		TailRatio:    NativeMemoryDefaultTailRatio,
		tailRatioSet: true,
		leakOnlySet:  true,
	}
}

// AnalyzeNativeMemory parses `path` and returns the populated
// AnalysisResult. Mirrors `analyze_native_memory`.
func AnalyzeNativeMemory(path string, opts NativeMemoryOptions) (models.AnalysisResult, error) {
	diags := diagnostics.New("jfr")
	events, info, err := jfrparser.ParseRecording(path, diags)
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return BuildNativeMemory(events, path, info, diags, opts), nil
}

// BuildNativeMemory assembles the AnalysisResult from already-parsed
// events. Mirrors the bulk of `analyze_native_memory` after parsing.
func BuildNativeMemory(events []jfrparser.Event, sourceFile string, info jfrparser.SourceInfo, diags *diagnostics.ParserDiagnostics, opts NativeMemoryOptions) models.AnalysisResult {
	if !opts.leakOnlySet && !opts.LeakOnly {
		// Caller built struct literally — apply Python's True default.
		// Detect by checking the explicit-set flag.
		opts.LeakOnly = true
	}
	if !opts.tailRatioSet && opts.TailRatio == 0 {
		opts.TailRatio = NativeMemoryDefaultTailRatio
	}

	allocs := filterFunc(events, func(e jfrparser.Event) bool {
		_, ok := allocEventTypes[e.EventType]
		return ok
	})
	frees := filterFunc(events, func(e jfrparser.Event) bool {
		_, ok := freeEventTypes[e.EventType]
		return ok
	})

	totalAllocBytes := sumSizeBytes(allocs)
	totalFreeBytes := sumSizeBytes(frees)

	freedAddresses := map[int64]struct{}{}
	for _, e := range frees {
		if e.Address != nil {
			freedAddresses[*e.Address] = struct{}{}
		}
	}

	var cutoff *time.Time
	if opts.TailRatio > 0 {
		cutoff = nativeMemTailCutoff(allocs, opts.TailRatio)
	}

	leakEvents := make([]jfrparser.Event, 0, len(allocs))
	for _, e := range allocs {
		if opts.LeakOnly {
			if e.Address != nil {
				if _, freed := freedAddresses[*e.Address]; freed {
					continue
				}
			}
			if cutoff != nil {
				ts := parseEventTime(e.Time)
				if ts != nil && ts.After(*cutoff) {
					continue
				}
			}
		}
		leakEvents = append(leakEvents, e)
	}

	leakBytes := sumSizeBytes(leakEvents)

	// stacks Counter: ";"-joined frame paths → bytes (or 1 per event
	// when size is missing/zero).
	stacks := map[string]int64{}
	for _, e := range leakEvents {
		if len(e.Frames) == 0 {
			continue
		}
		key := strings.Join(e.Frames, ";")
		var weight int64 = 1
		if e.Size != nil && *e.Size > 0 {
			weight = *e.Size
		}
		stacks[key] += weight
	}

	flame := buildFlameTreeFromCollapsed(stacks)
	topSites := topCallSites(stacks, NativeMemoryTopCallSitesLimit)

	summary := map[string]any{
		"alloc_event_count":   len(allocs),
		"free_event_count":    len(frees),
		"alloc_bytes_total":   totalAllocBytes,
		"free_bytes_total":    totalFreeBytes,
		"unfreed_event_count": len(leakEvents),
		"unfreed_bytes_total": leakBytes,
		"tail_ratio":          opts.TailRatio,
		"tail_cutoff":         nil,
		"leak_only":           opts.LeakOnly,
	}
	if cutoff != nil {
		summary["tail_cutoff"] = formatPythonISO(*cutoff)
	}

	tables := map[string]any{
		"top_call_sites": topSites,
	}

	charts := map[string]any{
		"flamegraph": flame,
	}

	result := models.New(NativeMemoryResultType, NativeMemoryParserName)
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary
	result.Series = map[string]any{}
	result.Tables = tables
	result.Charts = charts
	result.Metadata.SchemaVersion = NativeMemorySchemaVersion
	result.Metadata.Diagnostics = diags
	if result.Metadata.Extra == nil {
		result.Metadata.Extra = map[string]any{}
	}
	result.Metadata.Extra["unit"] = "bytes"
	result.Metadata.Extra["source_format"] = info.SourceFormat
	if info.JFRCli != nil {
		result.Metadata.Extra["jfr_cli"] = *info.JFRCli
	} else {
		result.Metadata.Extra["jfr_cli"] = nil
	}
	return result
}

func sumSizeBytes(events []jfrparser.Event) int64 {
	var total int64
	for _, e := range events {
		if e.Size != nil {
			total += *e.Size
		}
	}
	return total
}

// nativeMemTailCutoff mirrors `_tail_cutoff` — start + duration *
// (1 - tail_ratio). Returns nil when there are no parseable
// timestamps or the recording has zero duration.
func nativeMemTailCutoff(allocs []jfrparser.Event, tailRatio float64) *time.Time {
	var first, last *time.Time
	for _, e := range allocs {
		ts := parseEventTime(e.Time)
		if ts == nil {
			continue
		}
		if first == nil || ts.Before(*first) {
			t := *ts
			first = &t
		}
		if last == nil || ts.After(*last) {
			t := *ts
			last = &t
		}
	}
	if first == nil || last == nil {
		return nil
	}
	durationS := last.Sub(*first).Seconds()
	if durationS <= 0 || tailRatio <= 0 {
		return nil
	}
	cutoffS := durationS * (1 - tailRatio)
	cutoff := first.Add(time.Duration(cutoffS * float64(time.Second)))
	return &cutoff
}

// topCallSites mirrors `stacks.most_common(50)` — sort by bytes desc,
// break ties by stack key ascending for determinism.
func topCallSites(stacks map[string]int64, limit int) []map[string]any {
	type kv struct {
		stack string
		bytes int64
	}
	rows := make([]kv, 0, len(stacks))
	for k, v := range stacks {
		rows = append(rows, kv{k, v})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].bytes != rows[j].bytes {
			return rows[i].bytes > rows[j].bytes
		}
		return rows[i].stack < rows[j].stack
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"stack": r.stack,
			"bytes": r.bytes,
		})
	}
	return out
}

// ── flame tree builder (minimal port of flamegraph_builder.build_flame_tree_from_collapsed) ─

// flameNode mirrors the Python FlameNode wire shape — keys match
// `_node_to_dict_shallow`. Children are emitted in samples-desc order
// to match the Python `sorted(..., key=samples, reverse=True)`.
type flameNode struct {
	id       string
	parentID *string
	name     string
	samples  int64
	path     []string
	children map[string]*flameNode
}

// buildFlameTreeFromCollapsed mirrors
// `flamegraph_builder.build_flame_tree_from_collapsed`. Returns the
// `to_dict()` representation directly so the caller can drop it under
// `charts.flamegraph` without further conversion.
func buildFlameTreeFromCollapsed(stacks map[string]int64) map[string]any {
	root := &flameNode{
		id:       "root",
		parentID: nil,
		name:     "All",
		samples:  0,
		path:     []string{},
		children: map[string]*flameNode{},
	}
	for _, samples := range stacks {
		if samples > 0 {
			root.samples += samples
		}
	}
	for stack, samples := range stacks {
		if samples <= 0 {
			continue
		}
		current := root
		path := []string{}
		for _, frame := range strings.Split(stack, ";") {
			if frame == "" {
				continue
			}
			path = append(path, frame)
			child, ok := current.children[frame]
			if !ok {
				idCopy := current.id
				child = &flameNode{
					id:       nodeID(path),
					parentID: &idCopy,
					name:     frame,
					path:     append([]string(nil), path...),
					children: map[string]*flameNode{},
				}
				current.children[frame] = child
			}
			child.samples += samples
			current = child
		}
	}
	totalSamples := root.samples
	if totalSamples < 1 {
		totalSamples = 1
	}
	return freezeNode(root, totalSamples)
}

func freezeNode(node *flameNode, totalSamples int64) map[string]any {
	ratio := 0.0
	if totalSamples > 0 {
		ratio = roundHalfEven(float64(node.samples)/float64(totalSamples)*100, 4)
	}
	out := map[string]any{
		"id":       node.id,
		"parentId": nil,
		"name":     node.name,
		"samples":  node.samples,
		"ratio":    ratio,
		"category": nil,
		"color":    nil,
		"path":     node.path,
	}
	if node.parentID != nil {
		out["parentId"] = *node.parentID
	}

	type childKV struct {
		key  string
		node *flameNode
	}
	children := make([]childKV, 0, len(node.children))
	for k, v := range node.children {
		children = append(children, childKV{k, v})
	}
	sort.SliceStable(children, func(i, j int) bool {
		if children[i].node.samples != children[j].node.samples {
			return children[i].node.samples > children[j].node.samples
		}
		return children[i].key < children[j].key
	})
	frozenChildren := make([]map[string]any, 0, len(children))
	for _, c := range children {
		frozenChildren = append(frozenChildren, freezeNode(c.node, totalSamples))
	}
	out["children"] = frozenChildren
	return out
}

func nodeID(path []string) string {
	parts := make([]string, len(path))
	for i, part := range path {
		parts[i] = slugFlame(part)
	}
	return "frame:" + strings.Join(parts, "/")
}

// slugFlame mirrors Python's `_slug` — replace specific separators with
// underscores then truncate to 160 chars.
func slugFlame(value string) string {
	r := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		";", "_",
		" ", "_",
	)
	out := r.Replace(value)
	if len(out) > 160 {
		out = out[:160]
	}
	return out
}
