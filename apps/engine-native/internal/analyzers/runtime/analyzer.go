// Package runtime ports archscope_engine.analyzers.runtime_analyzer.
//
// One package wraps four entrypoints — Node.js / Python / Go panic /
// .NET+IIS — because they all collapse onto the same RuntimeStackRecord
// and share the same finding rules
// (REPEATED_RUNTIME_STACK_SIGNATURE / STACKLESS_RUNTIME_RECORDS). The
// .NET path additionally interleaves IIS access records and emits
// IIS_5XX_PRESENT / DOTNET_EXCEPTIONS_PRESENT.
//
// Layout justification: kept separate from `internal/analyzers/exception/`
// because (1) the Python source is split that way, (2) consumed parser
// output is different (`runtimestack.RuntimeStackRecord` vs
// `parsers/exception.Record`), and (3) the result envelopes diverge
// (this analyzer emits four `type` literals, exception emits one).
package runtime

import (
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/runtimestack"
)

const (
	// DefaultTopN matches Python's `top_n=20` default.
	DefaultTopN = 20
	// SchemaVersion mirrors the Python runtime analyzer's
	// metadata.schema_version literal.
	SchemaVersion = "0.1.0"

	// Result type literals (mirror Python verbatim).
	ResultTypeNodejsStack        = "nodejs_stack"
	ResultTypePythonTraceback    = "python_traceback"
	ResultTypeGoPanic            = "go_panic"
	ResultTypeDotnetExceptionIIS = "dotnet_exception_iis"

	// Parser literals (mirror Python's `metadata.parser`).
	ParserNodejsStack     = "nodejs_stack_trace"
	ParserPythonTraceback = "python_traceback"
	ParserGoPanic         = "go_panic_goroutine"
	ParserDotnetIIS       = "dotnet_exception_iis_w3c"
)

// Options carries analyzer-level knobs. Mirrors Python's keyword args.
type Options struct {
	// TopN caps every "top distribution" / table list. 0 means "use default".
	TopN int
	// MaxLines plumbs through to the parser; 0 = unbounded.
	MaxLines int
	// Strict surfaces parser skips as fatal errors.
	Strict bool
}

// AnalyzeNodejsStack ports `analyze_nodejs_stack`.
func AnalyzeNodejsStack(path string, opts Options) (models.AnalysisResult, error) {
	records, diags, err := runtimestack.ParseNodejsFile(path, runtimestack.Options{
		MaxLines: opts.MaxLines,
		Strict:   opts.Strict,
	})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return BuildStackResult(ResultTypeNodejsStack, ParserNodejsStack, records, path, diags, opts), nil
}

// AnalyzePythonTraceback ports `analyze_python_traceback`.
func AnalyzePythonTraceback(path string, opts Options) (models.AnalysisResult, error) {
	records, diags, err := runtimestack.ParsePythonTracebackFile(path, runtimestack.Options{
		MaxLines: opts.MaxLines,
		Strict:   opts.Strict,
	})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return BuildStackResult(ResultTypePythonTraceback, ParserPythonTraceback, records, path, diags, opts), nil
}

// AnalyzeGoPanic ports `analyze_go_panic`.
func AnalyzeGoPanic(path string, opts Options) (models.AnalysisResult, error) {
	records, diags, err := runtimestack.ParseGoPanicFile(path, runtimestack.Options{
		MaxLines: opts.MaxLines,
		Strict:   opts.Strict,
	})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return BuildStackResult(ResultTypeGoPanic, ParserGoPanic, records, path, diags, opts), nil
}

// AnalyzeDotnetExceptionIIS ports `analyze_dotnet_exception_iis`.
func AnalyzeDotnetExceptionIIS(path string, opts Options) (models.AnalysisResult, error) {
	exceptions, iisRecords, diags, err := runtimestack.ParseDotnetFile(path, runtimestack.Options{
		MaxLines: opts.MaxLines,
		Strict:   opts.Strict,
	})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return BuildDotnetResult(exceptions, iisRecords, path, diags, opts), nil
}

// BuildStackResult assembles the AnalysisResult for the three
// "stack-only" runtimes (Node.js / Python / Go panic). Mirrors
// `_build_stack_result`.
func BuildStackResult(resultType, parser string, records []runtimestack.RuntimeStackRecord, sourceFile string, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}

	payload := stackPayload(records, topN)
	summary := map[string]any{
		"total_records":       len(records),
		"unique_record_types": payload.typeCounts.unique(),
		"unique_signatures":   payload.signatureCounts.unique(),
		"top_record_type":     payload.topRecordType,
	}

	if diags == nil {
		diags = diagnostics.New(parser)
	}

	result := models.New(resultType, parser)
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary
	result.Series = payload.series
	result.Tables = payload.tables
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = diags
	for _, finding := range stackFindings(records, payload.signatureCounts) {
		result.Metadata.Findings = append(result.Metadata.Findings, finding)
	}
	return result
}

// BuildDotnetResult mirrors the .NET branch of `analyze_dotnet_exception_iis`.
// Combines the stack-only payload with IIS access aggregations.
func BuildDotnetResult(exceptions []runtimestack.RuntimeStackRecord, iisRecords []runtimestack.IisAccessRecord, sourceFile string, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}

	stack := stackPayload(exceptions, topN)

	statusCounts := newOrderedCounter()
	for _, record := range iisRecords {
		statusCounts.add(statusBucket(record.Status))
	}

	// Python: pick top_n IIS records by time_taken_ms desc (stable on
	// original order, since Python's sorted() is stable). Then build a
	// Counter with f"{method} {uri}" -> time_taken_ms (raw value, NOT
	// a count). The Counter.most_common ordering then becomes
	// "rank by time_taken value, ties by insertion order". We mirror
	// the same shape — including the Counter's ability to lose
	// duplicates if two requests share method+uri (last write wins on
	// the dict, but that matches Python).
	type indexedIis struct {
		idx    int
		record runtimestack.IisAccessRecord
	}
	indexed := make([]indexedIis, 0, len(iisRecords))
	for i, record := range iisRecords {
		indexed = append(indexed, indexedIis{idx: i, record: record})
	}
	sort.SliceStable(indexed, func(i, j int) bool {
		ai := timeTakenOrZero(indexed[i].record)
		aj := timeTakenOrZero(indexed[j].record)
		return ai > aj
	})
	if len(indexed) > topN {
		indexed = indexed[:topN]
	}
	type slowEntry struct {
		key   string
		value int
		pos   int
	}
	slowOrder := make([]string, 0, len(indexed))
	slowMap := map[string]int{}
	slowPos := map[string]int{}
	for _, item := range indexed {
		key := item.record.Method + " " + item.record.URI
		val := timeTakenOrZero(item.record)
		if _, ok := slowMap[key]; !ok {
			slowOrder = append(slowOrder, key)
			slowPos[key] = len(slowOrder) - 1
		}
		slowMap[key] = val
	}
	slowEntries := make([]slowEntry, 0, len(slowOrder))
	for _, k := range slowOrder {
		slowEntries = append(slowEntries, slowEntry{key: k, value: slowMap[k], pos: slowPos[k]})
	}
	sort.SliceStable(slowEntries, func(i, j int) bool {
		if slowEntries[i].value != slowEntries[j].value {
			return slowEntries[i].value > slowEntries[j].value
		}
		return slowEntries[i].pos < slowEntries[j].pos
	})
	if len(slowEntries) > topN {
		slowEntries = slowEntries[:topN]
	}

	iisErrorRequests := 0
	maxIisTime := 0
	for _, record := range iisRecords {
		if record.Status >= 500 {
			iisErrorRequests++
		}
		if v := timeTakenOrZero(record); v > maxIisTime {
			maxIisTime = v
		}
	}

	summary := map[string]any{
		"total_exceptions":      len(exceptions),
		"unique_signatures":     stack.signatureCounts.unique(),
		"iis_requests":          len(iisRecords),
		"iis_error_requests":    iisErrorRequests,
		"max_iis_time_taken_ms": maxIisTime,
	}

	series := map[string]any{}
	for k, v := range stack.series {
		series[k] = v
	}
	statusRows := make([]map[string]any, 0, statusCounts.unique())
	for _, entry := range statusCounts.mostCommon(0) {
		statusRows = append(statusRows, map[string]any{
			"status": entry.key,
			"count":  entry.count,
		})
	}
	series["iis_status_distribution"] = statusRows
	slowestRows := make([]map[string]any, 0, len(slowEntries))
	for _, e := range slowEntries {
		slowestRows = append(slowestRows, map[string]any{
			"uri":           e.key,
			"time_taken_ms": e.value,
		})
	}
	series["iis_slowest_urls"] = slowestRows

	tables := map[string]any{}
	for k, v := range stack.tables {
		tables[k] = v
	}
	iisTableLimit := topN
	if iisTableLimit > len(iisRecords) {
		iisTableLimit = len(iisRecords)
	}
	iisRows := make([]map[string]any, 0, iisTableLimit)
	for i := 0; i < iisTableLimit; i++ {
		record := iisRecords[i]
		var tt any
		if record.TimeTakenMS != nil {
			tt = *record.TimeTakenMS
		}
		iisRows = append(iisRows, map[string]any{
			"method":        record.Method,
			"uri":           record.URI,
			"status":        record.Status,
			"time_taken_ms": tt,
		})
	}
	tables["iis_requests"] = iisRows

	if diags == nil {
		diags = diagnostics.New(ParserDotnetIIS)
	}

	result := models.New(ResultTypeDotnetExceptionIIS, ParserDotnetIIS)
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary
	result.Series = series
	result.Tables = tables
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = diags
	for _, finding := range dotnetFindings(summary) {
		result.Metadata.Findings = append(result.Metadata.Findings, finding)
	}
	return result
}

// stackPayloadResult mirrors Python's `_stack_payload` return dict.
type stackPayloadResult struct {
	typeCounts      *orderedCounter
	signatureCounts *orderedCounter
	topRecordType   any
	series          map[string]any
	tables          map[string]any
}

func stackPayload(records []runtimestack.RuntimeStackRecord, topN int) stackPayloadResult {
	typeCounts := newOrderedCounter()
	signatureCounts := newOrderedCounter()
	for _, record := range records {
		typeCounts.add(record.RecordType)
		signatureCounts.add(record.Signature)
	}
	var topRecordType any
	if top := typeCounts.mostCommon(1); len(top) > 0 {
		topRecordType = top[0].key
	}

	series := map[string]any{
		"record_type_distribution": projectCount(typeCounts.mostCommon(topN), "record_type"),
		"top_stack_signatures":     projectCount(signatureCounts.mostCommon(topN), "signature"),
	}

	tableLimit := topN
	if tableLimit > len(records) {
		tableLimit = len(records)
	}
	rows := make([]map[string]any, 0, tableLimit)
	for i := 0; i < tableLimit; i++ {
		record := records[i]
		var msg any
		if record.Message != nil {
			msg = *record.Message
		}
		var topFrame any
		if len(record.Stack) > 0 {
			topFrame = record.Stack[0]
		}
		stack := record.Stack
		if stack == nil {
			stack = []string{}
		}
		rows = append(rows, map[string]any{
			"runtime":     record.Runtime,
			"record_type": record.RecordType,
			"headline":    record.Headline,
			"message":     msg,
			"signature":   record.Signature,
			"top_frame":   topFrame,
			"stack":       stack,
		})
	}
	tables := map[string]any{
		"records": rows,
	}
	return stackPayloadResult{
		typeCounts:      typeCounts,
		signatureCounts: signatureCounts,
		topRecordType:   topRecordType,
		series:          series,
		tables:          tables,
	}
}

// stackFindings ports `_stack_findings`.
func stackFindings(records []runtimestack.RuntimeStackRecord, signatureCounts *orderedCounter) []map[string]any {
	findings := []map[string]any{}
	if top := signatureCounts.mostCommon(1); len(top) > 0 && top[0].count > 1 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "REPEATED_RUNTIME_STACK_SIGNATURE",
			"message":  "The same runtime stack signature appears multiple times.",
			"evidence": map[string]any{
				"signature": top[0].key,
				"count":     top[0].count,
			},
		})
	}
	stackless := 0
	for _, record := range records {
		if len(record.Stack) == 0 {
			stackless++
		}
	}
	if stackless > 0 {
		findings = append(findings, map[string]any{
			"severity": "info",
			"code":     "STACKLESS_RUNTIME_RECORDS",
			"message":  "Some records did not include stack frames.",
			"evidence": map[string]any{
				"count": stackless,
			},
		})
	}
	return findings
}

// dotnetFindings ports `_dotnet_findings`.
func dotnetFindings(summary map[string]any) []map[string]any {
	findings := []map[string]any{}
	if asInt(summary["iis_error_requests"]) > 0 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "IIS_5XX_PRESENT",
			"message":  "IIS access records include 5xx responses.",
			"evidence": map[string]any{
				"iis_error_requests": summary["iis_error_requests"],
			},
		})
	}
	if asInt(summary["total_exceptions"]) > 0 {
		findings = append(findings, map[string]any{
			"severity": "info",
			"code":     "DOTNET_EXCEPTIONS_PRESENT",
			"message":  ".NET exception stack traces were detected.",
			"evidence": map[string]any{
				"total_exceptions": summary["total_exceptions"],
			},
		})
	}
	return findings
}

func statusBucket(status int) string {
	bucket := status / 100
	switch bucket {
	case 1:
		return "1xx"
	case 2:
		return "2xx"
	case 3:
		return "3xx"
	case 4:
		return "4xx"
	case 5:
		return "5xx"
	default:
		// Mirror Python's f"{status // 100}xx" exactly — for
		// non-standard codes (e.g. 600 => "6xx", 0 => "0xx").
		return statusBucketGeneric(bucket)
	}
}

func statusBucketGeneric(bucket int) string {
	// Tiny helper so callers can stay branch-light. Python uses an
	// f-string, we just sprintf via integer-to-string.
	const digits = "0123456789"
	if bucket >= 0 && bucket < 10 {
		return string(digits[bucket]) + "xx"
	}
	// Two-or-more-digit bucket (rare, status >= 1000): build manually.
	out := []byte{}
	if bucket < 0 {
		out = append(out, '-')
		bucket = -bucket
	}
	rev := []byte{}
	for bucket > 0 {
		rev = append(rev, digits[bucket%10])
		bucket /= 10
	}
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	return string(out) + "xx"
}

func timeTakenOrZero(record runtimestack.IisAccessRecord) int {
	if record.TimeTakenMS == nil {
		return 0
	}
	return *record.TimeTakenMS
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

// orderedCounter mirrors Python's `collections.Counter`. Most_common
// sorts by count desc, ties by insertion order — matches CPython's
// stable-sort over dict iteration order. Stable order is load-bearing
// for the JSON parity gate.
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
