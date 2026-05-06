// Package exception ports archscope_engine.analyzers.exception_analyzer.
//
// The analyzer consumes records produced by
// internal/parsers/exception and emits the AnalysisResult envelope
// the engine ↔ UI contract expects: a 4-metric summary, three "top-N"
// distribution series (exception_type, root_cause, signature), an
// `exceptions` table (first top_n records with stack frames), and a
// findings list (REPEATED_EXCEPTION_SIGNATURE / ROOT_CAUSE_PRESENT).
//
// Layout: kept separate from `internal/analyzers/runtime/` because:
//   - The Python source lives in two distinct files
//     (`exception_analyzer.py` vs `runtime_analyzer.py`) with no
//     shared helpers.
//   - The two analyzers consume different parser outputs
//     (`parsers/exception.Record` vs `parsers/runtimestack.RuntimeStackRecord`)
//     and emit different result types (`exception_stack` vs four
//     runtime-specific types).
//   - Splitting matches the existing parser layout (one package per
//     domain) and keeps the analyzer surface small enough to read.
package exception

import (
	"sort"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/exception"
)

const (
	// DefaultTopN matches Python's `top_n=20` default.
	DefaultTopN = 20
	// SchemaVersion mirrors the Python exception analyzer's emitted
	// metadata.schema_version.
	SchemaVersion = "0.1.0"
	// ParserName mirrors the Python `metadata.parser` literal.
	ParserName = "java_exception_stack"
	// ResultType mirrors the Python `AnalysisResult.type` literal.
	ResultType = "exception_stack"
)

// Options carries analyzer-level knobs. Mirrors Python's keyword args.
type Options struct {
	// TopN caps every "top distribution" list (series rows + tables.exceptions).
	// 0 means "use default" (matches Python's default kwarg).
	TopN int
	// MaxLines is plumbed through to the parser. 0 means unbounded.
	MaxLines int
	// Strict surfaces parser skips as fatal errors.
	Strict bool
}

// Analyze parses `path` then assembles the AnalysisResult.
func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	records, diags, err := exception.ParseFile(path, exception.FormatJavaExceptionStack, exception.Options{
		MaxLines: opts.MaxLines,
		Strict:   opts.Strict,
	})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return Build(records, path, diags, opts), nil
}

// Build assembles the AnalysisResult from already-parsed records.
// Splitting Analyze and Build matches Python's two-tier API and lets
// callers (CLI, web server, tests) feed records from elsewhere.
func Build(records []exception.Record, sourceFile string, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}

	typeCounts := newOrderedCounter()
	rootCounts := newOrderedCounter()
	signatureCounts := newOrderedCounter()
	rootCauseRecordCount := 0
	for _, record := range records {
		typeCounts.add(record.ExceptionType)
		root := record.ExceptionType
		if record.RootCause != nil {
			root = *record.RootCause
			rootCauseRecordCount++
		}
		rootCounts.add(root)
		signatureCounts.add(record.Signature)
	}

	var topExceptionType any
	if top := typeCounts.mostCommon(1); len(top) > 0 {
		topExceptionType = top[0].key
	}

	summary := map[string]any{
		"total_exceptions":       len(records),
		"unique_exception_types": typeCounts.unique(),
		"unique_signatures":      signatureCounts.unique(),
		"top_exception_type":     topExceptionType,
	}

	series := map[string]any{
		"exception_type_distribution": projectCount(typeCounts.mostCommon(topN), "exception_type"),
		"root_cause_distribution":     projectCount(rootCounts.mostCommon(topN), "root_cause"),
		"top_stack_signatures":        projectCount(signatureCounts.mostCommon(topN), "signature"),
	}

	tableLimit := topN
	if tableLimit > len(records) {
		tableLimit = len(records)
	}
	exceptionRows := make([]map[string]any, 0, tableLimit)
	for i := 0; i < tableLimit; i++ {
		record := records[i]
		var ts any
		if record.Timestamp != nil {
			ts = record.Timestamp.Format(time.RFC3339Nano)
		}
		var msg any
		if record.Message != nil {
			msg = *record.Message
		}
		var rootCause any
		if record.RootCause != nil {
			rootCause = *record.RootCause
		}
		var topFrame any
		if len(record.Stack) > 0 {
			topFrame = record.Stack[0]
		}
		stack := record.Stack
		if stack == nil {
			stack = []string{}
		}
		exceptionRows = append(exceptionRows, map[string]any{
			"timestamp":      ts,
			"language":       record.Language,
			"exception_type": record.ExceptionType,
			"message":        msg,
			"root_cause":     rootCause,
			"signature":      record.Signature,
			"top_frame":      topFrame,
			"stack":          stack,
		})
	}
	tables := map[string]any{
		"exceptions": exceptionRows,
	}

	if diags == nil {
		diags = diagnostics.New(ParserName)
	}

	result := models.New(ResultType, ParserName)
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary
	result.Series = series
	result.Tables = tables
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = diags
	for _, finding := range buildFindings(records, signatureCounts, rootCauseRecordCount) {
		result.Metadata.Findings = append(result.Metadata.Findings, finding)
	}
	return result
}

// buildFindings ports `_build_findings` — REPEATED_EXCEPTION_SIGNATURE
// fires when the most-common signature appears > 1 times;
// ROOT_CAUSE_PRESENT fires when any record has a non-empty root_cause.
func buildFindings(records []exception.Record, signatureCounts *orderedCounter, rootCauseCount int) []map[string]any {
	findings := []map[string]any{}
	if top := signatureCounts.mostCommon(1); len(top) > 0 && top[0].count > 1 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "REPEATED_EXCEPTION_SIGNATURE",
			"message":  "The same exception stack signature appears multiple times.",
			"evidence": map[string]any{
				"signature": top[0].key,
				"count":     top[0].count,
			},
		})
	}
	if rootCauseCount > 0 {
		findings = append(findings, map[string]any{
			"severity": "info",
			"code":     "ROOT_CAUSE_PRESENT",
			"message":  "Nested root-cause exceptions were detected.",
			"evidence": map[string]any{
				"root_cause_count": rootCauseCount,
			},
		})
	}
	return findings
}

// orderedCounter mirrors Python's `collections.Counter` for our needs:
// counts get incremented per `add`, and `most_common(n)` returns the
// top-n entries sorted by count desc with ties broken by insertion
// order — exactly matching CPython's stable-sort on a dict iteration
// order. Stable order is load-bearing for the JSON parity gate.
type orderedCounter struct {
	order  []string
	counts map[string]int
}

func newOrderedCounter() *orderedCounter {
	return &orderedCounter{counts: map[string]int{}}
}

func (c *orderedCounter) add(key string) {
	if _, ok := c.counts[key]; !ok {
		c.order = append(c.order, key)
	}
	c.counts[key]++
}

func (c *orderedCounter) unique() int {
	return len(c.order)
}

type counterEntry struct {
	key   string
	count int
	pos   int
}

func (c *orderedCounter) mostCommon(limit int) []counterEntry {
	entries := make([]counterEntry, 0, len(c.order))
	for i, k := range c.order {
		entries = append(entries, counterEntry{key: k, count: c.counts[k], pos: i})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].pos < entries[j].pos
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

func projectCount(entries []counterEntry, keyName string) []map[string]any {
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		out = append(out, map[string]any{
			keyName: entry.key,
			"count": entry.count,
		})
	}
	return out
}
