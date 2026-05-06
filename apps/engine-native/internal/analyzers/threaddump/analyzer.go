// Package threaddump ports archscope_engine.analyzers.thread_dump_analyzer.
//
// JVM-only single-dump analyzer. Consumes one ThreadDumpBundle (the
// shape produced by internal/threaddump/plugins/javajstack) and emits
// an AnalysisResult with:
//
//   - state distribution (per-thread state) and category distribution
//     (RUNNABLE / BLOCKED / WAITING / NEW / TERMINATED / UNKNOWN)
//   - stack-signature aggregation, top-N most common
//   - per-thread table with top frame, lock info, and frame list
//   - findings: BLOCKED_THREADS_PRESENT, WAITING_THREADS_DOMINATE
//
// The Python code keys distributions off the parsed ThreadDumpRecord
// (state + category strings). The Go side gets ThreadSnapshot, where
// State is the canonical enum and Category is a *string. Empty/nil
// state or category fall back to "UNKNOWN" — same fallback Python
// does when record.state / record.category is None.
package threaddump

import (
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	javajstack "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/javajstack"
)

const (
	// DefaultTopN matches Python's `top_n=20` analyzer default.
	DefaultTopN = 20
	// SchemaVersion mirrors Python's `metadata.schema_version`.
	SchemaVersion = "0.1.0"
	// ParserName mirrors Python's `metadata.parser` literal.
	ParserName = "java_thread_dump"
	// ResultType mirrors Python's `AnalysisResult.type`.
	ResultType = "thread_dump"
	// SignatureDepth mirrors Python's `record.stack[:3]` window in
	// `_stack_signature`. The shared StackSignature method on
	// ThreadSnapshot defaults to 5 — we override here to keep
	// signature aggregation parity with the Python analyzer.
	SignatureDepth = 3
)

// Options carries the analyzer knobs. `TopN <= 0` falls back to
// DefaultTopN — same coercion Python does when callers omit `top_n`.
type Options struct {
	TopN int
}

// Analyze parses `path` through the Java jstack plugin and builds the
// AnalysisResult. Mirrors Python's `analyze_thread_dump(path, top_n=...)`
// entry point.
//
// The Java jstack plugin is the only ThreadDumpBundle producer this
// analyzer accepts — Python equivalents for Go / Python / .NET dumps
// have their own analyzers. We construct it directly (rather than
// going through threaddump.DefaultRegistry) so callers can run this
// analyzer without registering any plugin globally.
func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	plugin := javajstack.New()
	bundle, err := plugin.Parse(path)
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return Build(bundle, path, defaultDiagnostics(len(bundle.Snapshots)), opts), nil
}

// Build assembles the AnalysisResult from an already-parsed bundle.
// Splitting Analyze and Build matches the Python two-tier API
// (`analyze_thread_dump` + `build_thread_dump_result`) so callers can
// feed bundles produced elsewhere (e.g. an HTTP boundary that already
// parsed the file once).
func Build(
	bundle models.ThreadDumpBundle,
	sourceFile string,
	diags *diagnostics.ParserDiagnostics,
	opts Options,
) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}

	snapshots := bundle.Snapshots

	stateCounts := map[string]int{}
	categoryCounts := map[string]int{}
	signatureCounts := map[string]int{}
	threadsWithLocks := 0

	// Insertion-order keys for deterministic Counter.most_common
	// tie-breaks. Python's Counter preserves insertion order on ties;
	// we replicate that by remembering the first-seen index per key.
	stateOrder := map[string]int{}
	categoryOrder := map[string]int{}
	signatureOrder := map[string]int{}

	for _, snap := range snapshots {
		state := stateLabel(snap)
		if _, ok := stateOrder[state]; !ok {
			stateOrder[state] = len(stateOrder)
		}
		stateCounts[state]++

		category := categoryLabel(snap)
		if _, ok := categoryOrder[category]; !ok {
			categoryOrder[category] = len(categoryOrder)
		}
		categoryCounts[category]++

		signature := stackSignature(snap)
		if _, ok := signatureOrder[signature]; !ok {
			signatureOrder[signature] = len(signatureOrder)
		}
		signatureCounts[signature]++

		if snap.LockInfo != nil && *snap.LockInfo != "" {
			threadsWithLocks++
		}
	}

	blocked := categoryCounts["BLOCKED"]
	waiting := categoryCounts["WAITING"]
	runnable := categoryCounts["RUNNABLE"]

	summary := map[string]any{
		"total_threads":      len(snapshots),
		"runnable_threads":   runnable,
		"blocked_threads":    blocked,
		"waiting_threads":    waiting,
		"threads_with_locks": threadsWithLocks,
	}

	stateDistribution := mostCommon(stateCounts, stateOrder, "state", -1)
	categoryDistribution := mostCommon(categoryCounts, categoryOrder, "category", -1)
	topSignatures := mostCommon(signatureCounts, signatureOrder, "signature", topN)

	series := map[string]any{
		"state_distribution":    stateDistribution,
		"category_distribution": categoryDistribution,
		"top_stack_signatures":  topSignatures,
	}

	threadsTable := buildThreadsTable(snapshots, topN)
	tables := map[string]any{
		"threads": threadsTable,
	}

	findings := buildFindings(summary)

	if diags == nil {
		diags = defaultDiagnostics(len(snapshots))
	}

	result := models.New(ResultType, ParserName)
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary
	result.Series = series
	result.Tables = tables
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = diags
	for _, f := range findings {
		result.Metadata.Findings = append(result.Metadata.Findings, f)
	}
	return result
}

// stateLabel returns the canonical state string. Empty / unknown maps
// to "UNKNOWN" — matches Python's `record.state or "UNKNOWN"` fallback.
func stateLabel(snap models.ThreadSnapshot) string {
	if snap.State == "" {
		return string(models.ThreadStateUnknown)
	}
	return string(snap.State)
}

// categoryLabel returns the snapshot's Category, defaulting to
// "UNKNOWN" when nil/empty (Python `record.category or "UNKNOWN"`).
func categoryLabel(snap models.ThreadSnapshot) string {
	if snap.Category == nil || *snap.Category == "" {
		return "UNKNOWN"
	}
	return *snap.Category
}

// stackSignature collapses a thread's top frames into the same
// "f1 | f2 | f3" form Python's `_stack_signature` produces. Frames
// are rendered through StackFrame.Render() so module + file + line
// land in the signature exactly once. Empty stacks render as the
// Python sentinel "(no-stack)".
func stackSignature(snap models.ThreadSnapshot) string {
	if len(snap.StackFrames) == 0 {
		return "(no-stack)"
	}
	limit := SignatureDepth
	if limit > len(snap.StackFrames) {
		limit = len(snap.StackFrames)
	}
	parts := make([]string, 0, limit)
	for _, frame := range snap.StackFrames[:limit] {
		parts = append(parts, frame.Render())
	}
	return strings.Join(parts, " | ")
}

// buildThreadsTable mirrors Python's tables["threads"] projection,
// limited to the first `topN` snapshots. `top_frame` is the rendered
// top frame (or nil), `stack` is the rendered frame list.
func buildThreadsTable(snapshots []models.ThreadSnapshot, topN int) []map[string]any {
	limit := topN
	if limit > len(snapshots) {
		limit = len(snapshots)
	}
	rows := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		snap := snapshots[i]
		row := map[string]any{
			"thread_name": snap.ThreadName,
			"thread_id":   snap.ThreadID,
			"state":       string(snap.State),
			"category":    snap.Category,
			"top_frame":   topFrame(snap),
			"lock_info":   snap.LockInfo,
			"stack":       renderStack(snap.StackFrames),
		}
		rows = append(rows, row)
	}
	return rows
}

// topFrame returns the rendered top frame, or nil for empty stacks.
// Matches Python's `record.stack[0] if record.stack else None`.
func topFrame(snap models.ThreadSnapshot) any {
	if len(snap.StackFrames) == 0 {
		return nil
	}
	return snap.StackFrames[0].Render()
}

// renderStack returns the rendered string list for `tables.threads.*.stack`.
// Mirrors Python's `record.stack` (already strings).
func renderStack(frames []models.StackFrame) []string {
	out := make([]string, 0, len(frames))
	for _, frame := range frames {
		out = append(out, frame.Render())
	}
	return out
}

// mostCommon mirrors Python's `Counter.most_common`. Ties broken by
// insertion order to match Python's dict-iteration semantics on equal
// counts. `keyName` is the per-row column header ("state", "category",
// or "signature"). `limit <= 0` returns every entry.
func mostCommon(counts, order map[string]int, keyName string, limit int) []map[string]any {
	type kv struct {
		key   string
		count int
		order int
	}
	rows := make([]kv, 0, len(counts))
	for k, v := range counts {
		rows = append(rows, kv{k, v, order[k]})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].order < rows[j].order
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{keyName: row.key, "count": row.count})
	}
	return out
}

// buildFindings mirrors Python's `_build_findings`. Two heuristics:
//
//  1. BLOCKED_THREADS_PRESENT — any blocked thread is flagged as a
//     warning. Common indicator of monitor contention or deadlock.
//  2. WAITING_THREADS_DOMINATE — info-level when waiting threads
//     outnumber runnable threads. Often a signal of pool exhaustion
//     or a downstream slowness.
func buildFindings(summary map[string]any) []map[string]any {
	findings := []map[string]any{}
	blocked := asInt(summary["blocked_threads"])
	waiting := asInt(summary["waiting_threads"])
	runnable := asInt(summary["runnable_threads"])

	if blocked > 0 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "BLOCKED_THREADS_PRESENT",
			"message":  "Blocked Java threads were detected.",
			"evidence": map[string]any{"blocked_threads": blocked},
		})
	}
	if waiting > runnable {
		findings = append(findings, map[string]any{
			"severity": "info",
			"code":     "WAITING_THREADS_DOMINATE",
			"message":  "Waiting threads exceed runnable threads.",
			"evidence": map[string]any{
				"waiting_threads":  waiting,
				"runnable_threads": runnable,
			},
		})
	}
	return findings
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}

// defaultDiagnostics builds a minimal ParserDiagnostics when the
// caller didn't supply one (matching Python's default
// `ParserDiagnostics().to_dict()` shape).
func defaultDiagnostics(parsed int) *diagnostics.ParserDiagnostics {
	d := diagnostics.New(ParserName)
	d.TotalLines = parsed
	d.ParsedRecords = parsed
	return d
}

