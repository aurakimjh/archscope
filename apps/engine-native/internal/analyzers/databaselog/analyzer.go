package databaselog

import (
	"sort"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/databaselog"
)

const (
	ResultType  = "database_slow_query"
	ParserName  = "database_log"
	DefaultTopN = 30
)

type Options struct {
	Format string
	TopN   int
	Strict bool
}

func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	records, diags, err := parser.ParseFile(path, opts.Format, parser.Options{Strict: opts.Strict})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return Build(records, path, diags, opts), nil
}

func Build(records []parser.Record, sourceFile string, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}
	total := len(records)
	errorCount := 0
	lockWaitCount := 0
	planCount := 0
	rowsExaminedHigh := 0
	var durations []float64
	fp := map[string]*fingerprintStats{}
	edges := map[string]*edgeStats{}
	rows := []map[string]any{}
	for _, record := range records {
		if record.Error != "" {
			errorCount++
		}
		if record.LockWaitMS > 0 {
			lockWaitCount++
		}
		if record.Plan {
			planCount++
		}
		if record.Rows >= 1000 {
			rowsExaminedHigh++
		}
		if record.DurationMS > 0 {
			durations = append(durations, record.DurationMS)
		}
		key := firstNonEmpty(record.Fingerprint, parser.Fingerprint(record.Query), "(empty)")
		stats := fp[key]
		if stats == nil {
			stats = &fingerprintStats{fingerprint: key}
			fp[key] = stats
		}
		stats.add(record)
		db := firstNonEmpty(record.Database, record.Engine, "database")
		edge := edges[db]
		if edge == nil {
			edge = &edgeStats{callee: "database:" + db}
			edges[db] = edge
		}
		edge.add(record)
		if len(rows) < topN {
			rows = append(rows, row(record))
		}
	}
	result := models.New(ResultType, ParserName)
	result.SourceFiles = []string{sourceFile}
	result.Summary = map[string]any{
		"total_queries":            total,
		"slow_query_count":         len(durations),
		"error_count":              errorCount,
		"lock_wait_count":          lockWaitCount,
		"plan_count":               planCount,
		"high_rows_examined_count": rowsExaminedHigh,
		"p95_query_ms":             percentile(durations, 95),
		"p99_query_ms":             percentile(durations, 99),
		"fingerprint_count":        len(fp),
	}
	result.Tables = map[string]any{
		"queries":              rows,
		"query_fingerprints":   fingerprintRows(fp, topN),
		"explain_plans":        filterPlanRows(rows),
		"service_dependencies": dependencyRows(edges),
	}
	result.Series = map[string]any{
		"engine_distribution": engineDistribution(records),
	}
	if diags == nil {
		diags = diagnostics.New(firstNonEmpty(opts.Format, "auto"))
		diags.TotalLines = total
		diags.ParsedRecords = total
	}
	result.Metadata.SchemaVersion = "0.1.0"
	result.Metadata.Diagnostics = diags
	result.Metadata.Extra["format"] = firstNonEmpty(opts.Format, "auto")
	result.Metadata.Extra["analysis_options"] = map[string]any{"top_n": topN}
	addFindings(&result)
	ingestion.AttachSourceMetadata(&result, ingestion.NewSourceMetadata(sourceFile, ingestion.SourceMetadataOptions{
		SourceKind:   ingestion.SourceKindDatabaseLog,
		SourceFormat: firstNonEmpty(opts.Format, "auto"),
		Product:      dominantEngine(records),
	}))
	return result
}

type fingerprintStats struct {
	fingerprint string
	count       int
	totalMS     float64
	maxMS       float64
	errorCount  int
	rows        int
}

func (s *fingerprintStats) add(record parser.Record) {
	s.count++
	s.totalMS += record.DurationMS
	if record.DurationMS > s.maxMS {
		s.maxMS = record.DurationMS
	}
	if record.Error != "" {
		s.errorCount++
	}
	s.rows += record.Rows
}

type edgeStats struct {
	callee string
	count  int
	total  float64
	errors int
}

func (e *edgeStats) add(record parser.Record) {
	e.count++
	e.total += record.DurationMS
	if record.Error != "" {
		e.errors++
	}
}

func row(record parser.Record) map[string]any {
	return map[string]any{
		"timestamp":    timeString(record.Timestamp),
		"engine":       record.Engine,
		"database":     record.Database,
		"schema":       record.Schema,
		"user":         record.User,
		"query":        record.Query,
		"fingerprint":  firstNonEmpty(record.Fingerprint, parser.Fingerprint(record.Query)),
		"duration_ms":  record.DurationMS,
		"lock_wait_ms": record.LockWaitMS,
		"rows":         record.Rows,
		"error":        record.Error,
		"operation":    record.Operation,
		"collection":   record.Collection,
		"plan":         record.Plan,
		"plan_summary": record.PlanSummary,
	}
}

func fingerprintRows(stats map[string]*fingerprintStats, limit int) []map[string]any {
	rows := []map[string]any{}
	for _, stat := range stats {
		avg := 0.0
		if stat.count > 0 {
			avg = stat.totalMS / float64(stat.count)
		}
		rows = append(rows, map[string]any{"fingerprint": stat.fingerprint, "count": stat.count, "avg_duration_ms": avg, "max_duration_ms": stat.maxMS, "error_count": stat.errorCount, "rows": stat.rows})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if asInt(rows[i]["count"]) != asInt(rows[j]["count"]) {
			return asInt(rows[i]["count"]) > asInt(rows[j]["count"])
		}
		return asString(rows[i]["fingerprint"]) < asString(rows[j]["fingerprint"])
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

func dependencyRows(edges map[string]*edgeStats) []map[string]any {
	rows := []map[string]any{}
	for _, edge := range edges {
		avg := 0.0
		errRate := 0.0
		if edge.count > 0 {
			avg = edge.total / float64(edge.count)
			errRate = float64(edge.errors) / float64(edge.count)
		}
		rows = append(rows, map[string]any{"caller": "application", "callee": edge.callee, "call_count": edge.count, "total_duration_ms": edge.total, "avg_duration_ms": avg, "error_count": edge.errors, "error_rate": errRate})
	}
	return rows
}

func filterPlanRows(rows []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, row := range rows {
		if row["plan"] == true {
			out = append(out, row)
		}
	}
	return out
}

func addFindings(result *models.AnalysisResult) {
	if asInt(result.Summary["slow_query_count"]) > 0 {
		result.AddFinding("warning", "SLOW_QUERY_PRESENT", "Database slow-query evidence is present.", map[string]any{"slow_query_count": result.Summary["slow_query_count"]})
	}
	if asInt(result.Summary["lock_wait_count"]) > 0 {
		result.AddFinding("warning", "LOCK_WAIT_PRESENT", "Database lock-wait evidence is present.", map[string]any{"lock_wait_count": result.Summary["lock_wait_count"]})
	}
	if asInt(result.Summary["error_count"]) > 0 {
		result.AddFinding("critical", "DB_ERRORS_PRESENT", "Database error evidence is present.", map[string]any{"error_count": result.Summary["error_count"]})
	}
	if asInt(result.Summary["high_rows_examined_count"]) > 0 {
		result.AddFinding("warning", "HIGH_ROWS_EXAMINED", "Queries examined high row counts.", map[string]any{"high_rows_examined_count": result.Summary["high_rows_examined_count"]})
	}
	if asInt(result.Summary["plan_count"]) > 0 {
		result.AddFinding("info", "EXPLAIN_PLAN_IMPORTED", "EXPLAIN plan evidence is linked to query fingerprints.", map[string]any{"plan_count": result.Summary["plan_count"]})
	}
}

func engineDistribution(records []parser.Record) []map[string]any {
	counts := map[string]int{}
	for _, record := range records {
		counts[firstNonEmpty(record.Engine, "unknown")]++
	}
	rows := []map[string]any{}
	for engine, count := range counts {
		rows = append(rows, map[string]any{"engine": engine, "count": count})
	}
	return rows
}

func dominantEngine(records []parser.Record) string {
	rows := engineDistribution(records)
	if len(rows) == 0 {
		return ""
	}
	return asString(rows[0]["engine"])
}

func percentile(values []float64, pct float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	idx := int((pct / 100) * float64(len(values)-1))
	return values[idx]
}

func timeString(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func asInt(v any) int {
	if n, ok := v.(int); ok {
		return n
	}
	return 0
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
