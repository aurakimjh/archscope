// Package accesslog ports archscope_engine.analyzers.access_log_analyzer.
//
// The analyzer consumes records produced by
// internal/parsers/accesslog and emits the AnalysisResult envelope
// the engine ↔ UI contract expects: a 22-metric summary, per-minute
// time-series, per-URL aggregates, status code distribution, and a
// findings list (HIGH_ERROR_RATE / SERVER_ERRORS_PRESENT /
// NON_STANDARD_HTTP_STATUS / SLOW_URL_P95 / ERROR_BURST_DETECTED).
package accesslog

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/accesslog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/statistics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/timeutil"
)

const (
	// PercentileSampleLimit matches the Python default
	// (10_000) used by the access-log analyzer's BoundedPercentile
	// reservoirs.
	PercentileSampleLimit = 10_000
	// TopURLLimit caps the per-URL aggregate tables ({by_count,
	// by_avg, by_p95, by_bytes, by_errors}).
	TopURLLimit = 20
	// SampleRecordLimit caps the verbatim record table the renderer
	// uses for "first 25 rows in this log".
	SampleRecordLimit = 25
	// SchemaVersion mirrors the Python access-log analyzer's bump.
	SchemaVersion = "0.2.0"
	// ParserName mirrors the Python `metadata.parser` literal.
	ParserName = "nginx_combined_with_response_time"
	// DefaultPercentileSeed mirrors Python `BoundedPercentile.seed=12345`.
	// Even though Python uses Mersenne Twister and Go uses PCG, this
	// only matters once the reservoir overflows; below 10k samples the
	// two implementations agree exactly.
	DefaultPercentileSeed = 12345
)

var (
	staticExtRE = regexp.MustCompile(
		`(?i)\.(?:js|mjs|css|map|png|jpe?g|gif|ico|svg|webp|avif|bmp|tiff?|` +
			`woff2?|ttf|otf|eot|` +
			`mp4|webm|mp3|ogg|wav|flac|m4a|` +
			`pdf|zip|gz|tar|7z|rar|` +
			`json|xml|txt|md|wasm)(?:\?.*)?$`,
	)
	staticPathHintRE = regexp.MustCompile(
		`(?i)/(?:static|assets|public|dist|build|images?|img|css|js|fonts?|media|files|downloads|cdn)/`,
	)
)

// classifyRequest returns "static" or "api" — best-effort heuristic
// used to split summary metrics into "page weight" vs "service
// traffic". Same regex pair Python uses.
func classifyRequest(uri string) string {
	if staticExtRE.MatchString(uri) || staticPathHintRE.MatchString(uri) {
		return "static"
	}
	return "api"
}

// responseTimeStats keeps a running total + reservoir for percentile
// estimates. Mirrors Python's `ResponseTimeStats` dataclass.
type responseTimeStats struct {
	totalMS float64
	count   int
	pct     *statistics.BoundedPercentile
}

func newResponseTimeStats() *responseTimeStats {
	pct, err := statistics.NewBoundedPercentile(PercentileSampleLimit, DefaultPercentileSeed)
	if err != nil {
		// Constants are validated at compile time; only triggered if
		// someone bumps PercentileSampleLimit to <= 0.
		panic(err)
	}
	return &responseTimeStats{pct: pct}
}

func (r *responseTimeStats) add(valueMS float64) {
	r.totalMS += valueMS
	r.count++
	r.pct.Add(valueMS)
}

func (r *responseTimeStats) average() float64 {
	if r.count == 0 {
		return 0
	}
	return r.totalMS / float64(r.count)
}

func (r *responseTimeStats) percentile(percent float64) float64 {
	return r.pct.Percentile(percent)
}

// urlStats is the per-URI aggregate. Mirrors Python's `_UrlStats`.
type urlStats struct {
	count           int
	totalResponseMS float64
	totalBytes      int
	errorCount      int
	classification  string
	statusCounts    map[string]int
	rt              *responseTimeStats
	sampleMethod    string
}

func newURLStats() *urlStats {
	return &urlStats{
		statusCounts: map[string]int{},
		rt:           newResponseTimeStats(),
	}
}

func (u *urlStats) add(record accesslog.Record) {
	u.count++
	u.totalResponseMS += record.ResponseTimeMS
	u.totalBytes += record.BytesSent
	if record.Status >= 400 {
		u.errorCount++
	}
	u.statusCounts[statusFamily(record.Status)]++
	u.rt.add(record.ResponseTimeMS)
	if u.sampleMethod == "" {
		u.sampleMethod = record.Method
	}
}

// Options matches the Python `AccessLogAnalyzerOptions`. Time pointers
// double as "filter unset" sentinels.
type Options struct {
	MaxLines  int
	StartTime *time.Time
	EndTime   *time.Time
}

// Analyze parses `path` and returns the populated AnalysisResult.
// Format defaults to "nginx".
func Analyze(path, format string, opts Options) (models.AnalysisResult, error) {
	if format == "" {
		format = "nginx"
	}
	records, diags, err := accesslog.ParseFile(path, format, accesslog.Options{
		MaxLines:  opts.MaxLines,
		StartTime: opts.StartTime,
		EndTime:   opts.EndTime,
	})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return Build(records, path, format, diags, opts), nil
}

// Build assembles the AnalysisResult from already-parsed records.
// Splitting Analyze and Build matches Python's two-tier API and lets
// callers (CLI, web server, tests) feed records from elsewhere.
func Build(records []accesslog.Record, sourceFile, format string, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	totalRT := newResponseTimeStats()
	staticRT := newResponseTimeStats()
	apiRT := newResponseTimeStats()
	var (
		errorCount     int
		staticCount    int
		apiCount       int
		staticBytes    int
		apiBytes       int
		totalBytes     int
		total          int
		earliest       *time.Time
		latest         *time.Time
		requestsByMin  = map[string]int{}
		bytesByMin     = map[string]int{}
		rtByMin        = map[string]*responseTimeStats{}
		statusClassMin = map[string]map[string]int{}
		statusDistrib  = map[string]int{}
		statusCodes    = map[int]int{}
		nonStandard    = map[int]int{}
		methodDistrib  = map[string]int{}
		classDistrib   = map[string]int{}
		urlBuckets     = map[string]*urlStats{}
		sampleRecords  = make([]map[string]any, 0, SampleRecordLimit)
	)

	for _, record := range records {
		total++
		totalRT.add(record.ResponseTimeMS)
		if record.Status >= 400 {
			errorCount++
		}
		totalBytes += record.BytesSent
		method := record.Method
		if method == "" {
			method = "?"
		}
		methodDistrib[method]++

		classification := classifyRequest(record.URI)
		classDistrib[classification]++
		if classification == "static" {
			staticCount++
			staticBytes += record.BytesSent
			staticRT.add(record.ResponseTimeMS)
		} else {
			apiCount++
			apiBytes += record.BytesSent
			apiRT.add(record.ResponseTimeMS)
		}

		ts := record.Timestamp
		if earliest == nil || ts.Before(*earliest) {
			t := ts
			earliest = &t
		}
		if latest == nil || ts.After(*latest) {
			t := ts
			latest = &t
		}

		minute := timeutil.MinuteBucket(ts)
		requestsByMin[minute]++
		bytesByMin[minute] += record.BytesSent
		stats, ok := rtByMin[minute]
		if !ok {
			stats = newResponseTimeStats()
			rtByMin[minute] = stats
		}
		stats.add(record.ResponseTimeMS)
		family := statusFamily(record.Status)
		minBucket, ok := statusClassMin[minute]
		if !ok {
			minBucket = map[string]int{}
			statusClassMin[minute] = minBucket
		}
		minBucket[family]++
		statusDistrib[family]++
		statusCodes[record.Status]++
		if record.Status < 100 || record.Status > 599 {
			nonStandard[record.Status]++
		}
		bucket, ok := urlBuckets[record.URI]
		if !ok {
			bucket = newURLStats()
			bucket.classification = classification
			urlBuckets[record.URI] = bucket
		}
		bucket.add(record)

		if len(sampleRecords) < SampleRecordLimit {
			sampleRecords = append(sampleRecords, map[string]any{
				"timestamp":        record.Timestamp.Format(time.RFC3339Nano),
				"method":           record.Method,
				"uri":              record.URI,
				"status":           record.Status,
				"response_time_ms": roundHalfEven(record.ResponseTimeMS, 2),
			})
		}
	}

	// ── Time-series ──────────────────────────────────────────────────────────
	minuteKeys := sortedKeys(requestsByMin)

	requestsPerMinute := make([]map[string]any, 0, len(minuteKeys))
	avgRTPerMinute := make([]map[string]any, 0, len(minuteKeys))
	p50PerMinute := make([]map[string]any, 0, len(minuteKeys))
	p90PerMinute := make([]map[string]any, 0, len(minuteKeys))
	p95PerMinute := make([]map[string]any, 0, len(minuteKeys))
	p99PerMinute := make([]map[string]any, 0, len(minuteKeys))
	bytesPerMinute := make([]map[string]any, 0, len(minuteKeys))
	throughputPerMinute := make([]map[string]any, 0, len(minuteKeys))
	errorRatePerMinute := make([]map[string]any, 0, len(minuteKeys))

	for _, minute := range minuteKeys {
		count := requestsByMin[minute]
		stats := rtByMin[minute]
		requestsPerMinute = append(requestsPerMinute, map[string]any{"time": minute, "value": count})
		avgRTPerMinute = append(avgRTPerMinute, map[string]any{"time": minute, "value": roundHalfEven(stats.average(), 2)})
		p50PerMinute = append(p50PerMinute, map[string]any{"time": minute, "value": roundHalfEven(stats.percentile(50), 2)})
		p90PerMinute = append(p90PerMinute, map[string]any{"time": minute, "value": roundHalfEven(stats.percentile(90), 2)})
		p95PerMinute = append(p95PerMinute, map[string]any{"time": minute, "value": roundHalfEven(stats.percentile(95), 2)})
		p99PerMinute = append(p99PerMinute, map[string]any{"time": minute, "value": roundHalfEven(stats.percentile(99), 2)})
		bytesPerMinute = append(bytesPerMinute, map[string]any{"time": minute, "value": bytesByMin[minute]})
		throughputPerMinute = append(throughputPerMinute, map[string]any{
			"time":             minute,
			"requests_per_sec": roundHalfEven(float64(count)/60.0, 3),
			"bytes_per_sec":    roundHalfEven(float64(bytesByMin[minute])/60.0, 2),
		})
		errors := statusClassMin[minute]["4xx"] + statusClassMin[minute]["5xx"]
		var rate float64
		if count > 0 {
			rate = float64(errors) / float64(count) * 100
		}
		errorRatePerMinute = append(errorRatePerMinute, map[string]any{
			"time":   minute,
			"value":  roundHalfEven(rate, 2),
			"errors": errors,
			"total":  count,
		})
	}

	statusClassMinuteKeys := sortedKeys(statusClassMin)
	statusClassPerMinute := make([]map[string]any, 0, len(statusClassMinuteKeys))
	for _, minute := range statusClassMinuteKeys {
		row := statusClassMin[minute]
		statusClassPerMinute = append(statusClassPerMinute, map[string]any{
			"time":  minute,
			"2xx":   row["2xx"],
			"3xx":   row["3xx"],
			"4xx":   row["4xx"],
			"5xx":   row["5xx"],
			"other": row["other"],
		})
	}

	// ── Per-URL aggregates ───────────────────────────────────────────────────
	urlRows := make([]map[string]any, 0, len(urlBuckets))
	for uri, bucket := range urlBuckets {
		var avg float64
		if bucket.count > 0 {
			avg = bucket.totalResponseMS / float64(bucket.count)
		}
		urlRows = append(urlRows, map[string]any{
			"uri":              uri,
			"method":           bucket.sampleMethod,
			"classification":   bucket.classification,
			"count":            bucket.count,
			"avg_response_ms":  roundHalfEven(avg, 2),
			"p50_response_ms":  roundHalfEven(bucket.rt.percentile(50), 2),
			"p90_response_ms":  roundHalfEven(bucket.rt.percentile(90), 2),
			"p95_response_ms":  roundHalfEven(bucket.rt.percentile(95), 2),
			"p99_response_ms":  roundHalfEven(bucket.rt.percentile(99), 2),
			"total_bytes":      bucket.totalBytes,
			"error_count":      bucket.errorCount,
			"status_2xx":       bucket.statusCounts["2xx"],
			"status_3xx":       bucket.statusCounts["3xx"],
			"status_4xx":       bucket.statusCounts["4xx"],
			"status_5xx":       bucket.statusCounts["5xx"],
		})
	}

	topByCount := topRows(urlRows, "count", false, TopURLLimit)
	topByAvg := topRows(urlRows, "avg_response_ms", false, TopURLLimit)
	topByP95 := topRows(urlRows, "p95_response_ms", false, TopURLLimit)
	topByBytes := topRows(urlRows, "total_bytes", false, TopURLLimit)
	errorRows := make([]map[string]any, 0, len(urlRows))
	for _, row := range urlRows {
		if asInt(row["error_count"]) > 0 {
			errorRows = append(errorRows, row)
		}
	}
	topByErrors := topRows(errorRows, "error_count", false, TopURLLimit)

	topStatusCodes := mostCommonInts(statusCodes, 20)

	// ── Wall time + throughput ───────────────────────────────────────────────
	var wallTimeSec float64
	if earliest != nil && latest != nil {
		wallTimeSec = math.Max(0, latest.Sub(*earliest).Seconds())
	}
	avgRPS := 0.0
	avgBPS := 0.0
	if wallTimeSec > 0 {
		avgRPS = roundHalfEven(float64(total)/wallTimeSec, 3)
		avgBPS = roundHalfEven(float64(totalBytes)/wallTimeSec, 2)
	}

	var earliestStr, latestStr any
	if earliest != nil {
		earliestStr = earliest.Format(time.RFC3339Nano)
	} else {
		earliestStr = nil
	}
	if latest != nil {
		latestStr = latest.Format(time.RFC3339Nano)
	} else {
		latestStr = nil
	}

	errorRate := 0.0
	if total > 0 {
		errorRate = float64(errorCount) / float64(total) * 100
	}

	summary := map[string]any{
		"total_requests":         total,
		"avg_response_ms":        roundHalfEven(totalRT.average(), 2),
		"p50_response_ms":        roundHalfEven(totalRT.percentile(50), 2),
		"p90_response_ms":        roundHalfEven(totalRT.percentile(90), 2),
		"p95_response_ms":        roundHalfEven(totalRT.percentile(95), 2),
		"p99_response_ms":        roundHalfEven(totalRT.percentile(99), 2),
		"error_rate":             roundHalfEven(errorRate, 2),
		"error_count":            errorCount,
		"total_bytes":            totalBytes,
		"wall_time_sec":          roundHalfEven(wallTimeSec, 3),
		"avg_requests_per_sec":   avgRPS,
		"avg_bytes_per_sec":      avgBPS,
		"static_count":           staticCount,
		"api_count":              apiCount,
		"static_bytes":           staticBytes,
		"api_bytes":              apiBytes,
		"static_avg_response_ms": roundHalfEven(staticRT.average(), 2),
		"api_avg_response_ms":    roundHalfEven(apiRT.average(), 2),
		"api_p95_response_ms":    roundHalfEven(apiRT.percentile(95), 2),
		"earliest_timestamp":     earliestStr,
		"latest_timestamp":       latestStr,
		"unique_uris":            len(urlBuckets),
	}

	statusDistribOrdered := make([]map[string]any, 0, len(statusDistrib))
	for _, key := range sortedKeys(statusDistrib) {
		statusDistribOrdered = append(statusDistribOrdered, map[string]any{
			"status": key,
			"count":  statusDistrib[key],
		})
	}

	series := map[string]any{
		"requests_per_minute":          requestsPerMinute,
		"avg_response_time_per_minute": avgRTPerMinute,
		"p50_response_time_per_minute": p50PerMinute,
		"p90_response_time_per_minute": p90PerMinute,
		"p95_response_time_per_minute": p95PerMinute,
		"p99_response_time_per_minute": p99PerMinute,
		"status_class_per_minute":      statusClassPerMinute,
		"error_rate_per_minute":        errorRatePerMinute,
		"bytes_per_minute":             bytesPerMinute,
		"throughput_per_minute":        throughputPerMinute,
		"status_code_distribution":     statusDistribOrdered,
		"method_distribution":          mostCommonStrings(methodDistrib, "method", -1),
		"request_classification":       mostCommonStrings(classDistrib, "classification", -1),
		"top_urls_by_count":            projectURICount(topByCount),
		"top_urls_by_avg_response_time": projectURIAvg(topByAvg),
	}
	tables := map[string]any{
		"sample_records":                 sampleRecords,
		"url_stats":                      urlRows,
		"top_urls_by_count":              topByCount,
		"top_urls_by_avg_response_time":  topByAvg,
		"top_urls_by_p95_response_time":  topByP95,
		"top_urls_by_bytes":              topByBytes,
		"top_urls_by_errors":             topByErrors,
		"top_status_codes":               topStatusCodes,
	}

	findings := buildFindings(summary, statusDistrib, urlRows, errorRatePerMinute, nonStandard)

	if diags == nil {
		diags = defaultDiagnostics(total)
	}

	result := models.New("access_log", ParserName)
	result.SourceFiles = []string{sourceFile}
	result.Summary = summary
	result.Series = series
	result.Tables = tables
	result.Metadata.SchemaVersion = SchemaVersion
	result.Metadata.Diagnostics = diags
	for _, finding := range findings {
		result.Metadata.Findings = append(result.Metadata.Findings, finding)
	}
	if result.Metadata.Extra == nil {
		result.Metadata.Extra = map[string]any{}
	}
	result.Metadata.Extra["format"] = format
	result.Metadata.Extra["analysis_options"] = optionsToDict(opts)
	return result
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func statusFamily(status int) string {
	switch {
	case status >= 200 && status <= 299:
		return "2xx"
	case status >= 300 && status <= 399:
		return "3xx"
	case status >= 400 && status <= 499:
		return "4xx"
	case status >= 500:
		return "5xx"
	default:
		return "other"
	}
}

func roundHalfEven(value float64, decimals int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	pow := math.Pow(10, float64(decimals))
	return math.RoundToEven(value*pow) / pow
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// topRows sorts a copy of `rows` by `field` (numeric) and returns the
// top `limit` entries. Lex-by-uri secondary sort keeps ties stable.
func topRows(rows []map[string]any, field string, ascending bool, limit int) []map[string]any {
	out := append([]map[string]any(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool {
		a := asFloat(out[i][field])
		b := asFloat(out[j][field])
		if a == b {
			return asString(out[i]["uri"]) < asString(out[j]["uri"])
		}
		if ascending {
			return a < b
		}
		return a > b
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
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

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// mostCommonStrings mirrors Python's `Counter.most_common`. Ties broken
// lexicographically (Python uses insertion order — different but
// deterministic). `keyName` is the column header for each row (e.g.
// "method" or "classification").
func mostCommonStrings(counts map[string]int, keyName string, limit int) []map[string]any {
	type kv struct {
		key   string
		count int
	}
	rows := make([]kv, 0, len(counts))
	for k, v := range counts {
		rows = append(rows, kv{k, v})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].key < rows[j].key
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

func mostCommonInts(counts map[int]int, limit int) []map[string]any {
	type kv struct {
		key, count int
	}
	rows := make([]kv, 0, len(counts))
	for k, v := range counts {
		rows = append(rows, kv{k, v})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].key < rows[j].key
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"status": row.key, "count": row.count})
	}
	return out
}

func projectURICount(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"uri": row["uri"], "count": row["count"]})
	}
	return out
}

func projectURIAvg(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"uri":             row["uri"],
			"avg_response_ms": row["avg_response_ms"],
			"count":           row["count"],
		})
	}
	return out
}

func defaultDiagnostics(total int) *diagnostics.ParserDiagnostics {
	d := diagnostics.New("nginx")
	d.TotalLines = total
	d.ParsedRecords = total
	return d
}

func optionsToDict(opts Options) map[string]any {
	out := map[string]any{
		"max_lines":  nil,
		"start_time": nil,
		"end_time":   nil,
	}
	if opts.MaxLines > 0 {
		out["max_lines"] = opts.MaxLines
	}
	if opts.StartTime != nil {
		out["start_time"] = opts.StartTime.Format(time.RFC3339Nano)
	}
	if opts.EndTime != nil {
		out["end_time"] = opts.EndTime.Format(time.RFC3339Nano)
	}
	return out
}

// buildFindings ports `_build_access_log_findings`. Code names match
// Python verbatim (HIGH_ERROR_RATE / ELEVATED_ERROR_RATE /
// SERVER_ERRORS_PRESENT / NON_STANDARD_HTTP_STATUS / SLOW_URL_P95 /
// ERROR_BURST_DETECTED).
func buildFindings(
	summary map[string]any,
	statusDistrib map[string]int,
	urlRows []map[string]any,
	errorRatePerMinute []map[string]any,
	nonStandard map[int]int,
) []map[string]any {
	findings := []map[string]any{}
	errorRate := asFloat(summary["error_rate"])
	switch {
	case errorRate >= 10:
		findings = append(findings, map[string]any{
			"severity": "critical",
			"code":     "HIGH_ERROR_RATE",
			"message":  "Error rate is at or above 10%.",
			"evidence": map[string]any{"error_rate": errorRate},
		})
	case errorRate >= 5:
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "ELEVATED_ERROR_RATE",
			"message":  "Error rate is at or above 5%.",
			"evidence": map[string]any{"error_rate": errorRate},
		})
	}

	if serverErrors := statusDistrib["5xx"]; serverErrors > 0 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "SERVER_ERRORS_PRESENT",
			"message":  "One or more server-side errors were observed.",
			"evidence": map[string]any{"status": "5xx", "count": serverErrors},
		})
	}

	if len(nonStandard) > 0 {
		statuses := make([]int, 0, len(nonStandard))
		total := 0
		for code, count := range nonStandard {
			statuses = append(statuses, code)
			total += count
		}
		sort.Ints(statuses)
		joined := joinInts(statuses, ", ")
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "NON_STANDARD_HTTP_STATUS",
			"message":  "One or more non-standard HTTP status codes were observed.",
			"evidence": map[string]any{
				"statuses": joined,
				"count":    total,
			},
		})
	}

	// Slow URL — pick the slowest by p95 across endpoints with ≥5 hits.
	var slowest map[string]any
	var slowestP95 float64
	for _, row := range urlRows {
		if asInt(row["count"]) < 5 {
			continue
		}
		p95 := asFloat(row["p95_response_ms"])
		if slowest == nil || p95 > slowestP95 {
			slowest = row
			slowestP95 = p95
		}
	}
	if slowest != nil && slowestP95 >= 1000 {
		findings = append(findings, map[string]any{
			"severity": "warning",
			"code":     "SLOW_URL_P95",
			"message":  "An endpoint's p95 response time is at or above 1 second.",
			"evidence": map[string]any{
				"uri":             slowest["uri"],
				"p95_response_ms": slowest["p95_response_ms"],
				"count":           slowest["count"],
				"classification":  slowest["classification"],
			},
		})
	}

	// Error spike — any single minute with > 50% error rate AND ≥5 requests.
	var spike map[string]any
	var spikeRate float64
	for _, row := range errorRatePerMinute {
		if asInt(row["total"]) < 5 {
			continue
		}
		rate := asFloat(row["value"])
		if spike == nil || rate > spikeRate {
			spike = row
			spikeRate = rate
		}
	}
	if spike != nil && spikeRate >= 50 {
		findings = append(findings, map[string]any{
			"severity": "critical",
			"code":     "ERROR_BURST_DETECTED",
			"message":  "A single minute crossed 50% error rate.",
			"evidence": map[string]any{
				"minute":     spike["time"],
				"error_rate": spike["value"],
				"errors":     spike["errors"],
				"requests":   spike["total"],
			},
		})
	}

	return findings
}

func joinInts(values []int, sep string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, sep)
}
