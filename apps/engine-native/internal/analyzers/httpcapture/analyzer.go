package httpcapture

import (
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/httpcapture"
)

const ResultType = "http_capture"

type Options struct {
	Format           string
	TopN, MaxEntries int
}

func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	parsed, err := parser.ParseFile(path, parser.Options{Format: opts.Format, MaxEntries: opts.MaxEntries})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	result := Build(parsed.Entries, path, parsed.Format, parsed.Dialect, opts)
	result.Metadata.Diagnostics = parsed.Diagnostics
	return result, nil
}
func Build(entries []parser.Entry, sourceFile, format, dialect string, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = 50
	}
	endpoints := map[string]*stat{}
	hosts := map[string]*stat{}
	minutes := map[string]*stat{}
	for _, e := range entries {
		key := first(e.Method, "GET") + " " + first(e.Path, "/")
		add(endpoints, key, e)
		add(hosts, first(e.Host, "(unknown)"), e)
		if !e.StartedAt.IsZero() {
			add(minutes, e.StartedAt.UTC().Format("2006-01-02T15:04:00Z"), e)
		}
	}
	result := models.New(ResultType, "http_capture")
	result.SourceFiles = []string{sourceFile}
	result.Summary = map[string]any{"total_transactions": len(entries), "error_transactions": countErrors(entries), "unique_hosts": len(hosts), "unique_endpoints": len(endpoints), "source_format": format, "dialect": dialect}
	result.Series = map[string]any{"timeline": rows(minutes, "minute", topN)}
	result.Tables = map[string]any{"transactions": transactionRows(entries, topN), "endpoints": rows(endpoints, "endpoint", topN), "hosts": rows(hosts, "host", topN)}
	result.Metadata.SchemaVersion = "0.1.0"
	result.Metadata.Extra["http_capture"] = map[string]any{"dialect": dialect, "fidelity": "har_import", "redaction": "profile_redaction_0.1.0", "detail_storage": "inline_phase1"}
	ingestion.AttachSourceMetadata(&result, ingestion.NewSourceMetadata(sourceFile, ingestion.SourceMetadataOptions{SourceKind: ingestion.SourceKindHTTPCapture, SourceFormat: format, Product: "HAR import"}))
	if countErrors(entries) > 0 {
		result.AddFinding("warning", "HTTP_CAPTURE_ERRORS", "HTTP capture contains failed responses", map[string]any{"error_transactions": countErrors(entries)})
	}
	return result
}
func add(m map[string]*stat, k string, e parser.Entry) {
	s := m[k]
	if s == nil {
		s = &stat{}
		m[k] = s
	}
	s.Count++
	if e.Error {
		s.Errors++
	}
	s.TotalMS += e.DurationMS
	s.RequestBytes += e.RequestBytes
	s.ResponseBytes += e.ResponseBytes
}

type stat struct {
	Count, Errors               int
	TotalMS                     float64
	RequestBytes, ResponseBytes int64
}

func rows(m map[string]*stat, label string, limit int) []map[string]any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]].TotalMS > m[keys[j]].TotalMS })
	if len(keys) > limit {
		keys = keys[:limit]
	}
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		s := m[k]
		out = append(out, map[string]any{label: k, "count": s.Count, "errors": s.Errors, "error_rate": rate(s.Errors, s.Count), "total_duration_ms": s.TotalMS, "avg_duration_ms": s.TotalMS / float64(s.Count), "request_bytes": s.RequestBytes, "response_bytes": s.ResponseBytes})
	}
	return out
}
func transactionRows(entries []parser.Entry, limit int) []map[string]any {
	if len(entries) > limit {
		entries = entries[:limit]
	}
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		out = append(out, map[string]any{"started_at": e.StartedAt, "method": e.Method, "url": e.URL, "host": e.Host, "path": e.Path, "status": e.Status, "duration_ms": e.DurationMS, "wait_ms": e.WaitMS, "request_bytes": e.RequestBytes, "response_bytes": e.ResponseBytes, "fidelity": e.Fidelity})
	}
	return out
}
func countErrors(es []parser.Entry) int {
	n := 0
	for _, e := range es {
		if e.Error {
			n++
		}
	}
	return n
}
func rate(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
func first(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
