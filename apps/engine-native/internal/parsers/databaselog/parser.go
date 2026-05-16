package databaselog

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

type Record struct {
	Timestamp   time.Time
	Engine      string
	Database    string
	Schema      string
	User        string
	Query       string
	Fingerprint string
	DurationMS  float64
	LockWaitMS  float64
	Rows        int
	Error       string
	Operation   string
	Collection  string
	Plan        bool
	PlanSummary string
	Raw         string
}

type Options struct{ Strict bool }

func ParseFile(path, format string, opts Options) ([]Record, *diagnostics.ParserDiagnostics, error) {
	format = canonical(format)
	diags := diagnostics.New(format)
	diags.SetSourceFile(path)
	switch format {
	case "postgres-csv":
		return parsePostgresCSV(path, diags, opts)
	case "mongodb-json", "sqlserver-xevent-json", "postgres-explain-json", "mysql-explain-json":
		return parseDatabaseJSON(path, format, diags, opts)
	default:
		return parseText(path, format, diags, opts)
	}
}

func parseText(path, format string, diags *diagnostics.ParserDiagnostics, opts Options) ([]Record, *diagnostics.ParserDiagnostics, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, diags, err
	}
	defer file.Close()
	var records []Record
	var mysql *Record
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		diags.TotalLines++
		if format == "auto" {
			format = detectText(line)
			diags.Format = format
		}
		switch format {
		case "mysql-slow":
			if strings.HasPrefix(line, "# Time:") {
				if mysql != nil && mysql.Query != "" {
					mysql.Fingerprint = Fingerprint(mysql.Query)
					records = append(records, *mysql)
				}
				mysql = &Record{Engine: "mysql", Timestamp: parseAnyTime(strings.TrimSpace(strings.TrimPrefix(line, "# Time:")))}
			} else if strings.HasPrefix(line, "# Query_time:") && mysql != nil {
				fillMySQLTiming(mysql, line)
			} else if mysql != nil && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "SET ") && !strings.HasPrefix(line, "use ") && strings.TrimSpace(line) != "" {
				mysql.Query += " " + strings.TrimSpace(line)
			}
		case "redis-slowlog":
			if rec, ok := parseRedisSlowLine(line); ok {
				records = append(records, rec)
			}
		default:
			if rec, ok := parsePostgresTextLine(line); ok {
				records = append(records, rec)
			}
		}
	}
	if mysql != nil && mysql.Query != "" {
		mysql.Query = strings.TrimSpace(mysql.Query)
		mysql.Fingerprint = Fingerprint(mysql.Query)
		records = append(records, *mysql)
	}
	if err := scanner.Err(); err != nil {
		return records, diags, err
	}
	diags.ParsedRecords = len(records)
	return records, diags, nil
}

func parsePostgresCSV(path string, diags *diagnostics.ParserDiagnostics, _ Options) ([]Record, *diagnostics.ParserDiagnostics, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, diags, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, diags, err
	}
	records := []Record{}
	for _, row := range rows {
		diags.TotalLines++
		if len(row) < 14 {
			diags.AddSkipped(diags.TotalLines, "SHORT_CSV_ROW", "PostgreSQL csvlog row has too few columns.", strings.Join(row, ","))
			continue
		}
		message := row[13]
		rec, ok := postgresMessage(message)
		if !ok {
			continue
		}
		rec.Engine = "postgresql"
		rec.Timestamp = parseAnyTime(row[0])
		rec.User = row[1]
		rec.Database = row[2]
		rec.Raw = strings.Join(row, ",")
		records = append(records, rec)
	}
	diags.ParsedRecords = len(records)
	return records, diags, nil
}

func parseDatabaseJSON(path, format string, diags *diagnostics.ParserDiagnostics, _ Options) ([]Record, *diagnostics.ParserDiagnostics, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, diags, err
	}
	diags.TotalLines = 1
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		diags.AddSkipped(1, "INVALID_JSON", err.Error(), string(data[:min(len(data), diagnostics.RawPreviewLimit)]))
		return nil, diags, nil
	}
	records := recordsFromJSON(payload, format)
	diags.ParsedRecords = len(records)
	return records, diags, nil
}

func recordsFromJSON(payload any, format string) []Record {
	items := array(payload)
	if len(items) == 0 {
		items = []any{payload}
	}
	out := []Record{}
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch format {
		case "mongodb-json":
			query := firstNonEmpty(asString(obj["command"]), asString(obj["query"]), asString(obj["attr"]))
			out = append(out, Record{Engine: "mongodb", Database: asString(obj["db"]), Collection: asString(obj["ns"]), Operation: asString(obj["op"]), Query: query, Fingerprint: Fingerprint(query), DurationMS: num(obj["millis"]), Rows: int(num(obj["nreturned"])), Raw: fmt.Sprint(obj)})
		case "sqlserver-xevent-json":
			query := firstNonEmpty(asString(obj["statement"]), asString(obj["sql_text"]))
			out = append(out, Record{Engine: "sqlserver", Database: asString(obj["database_name"]), Query: query, Fingerprint: Fingerprint(query), DurationMS: num(obj["duration_ms"]), LockWaitMS: num(obj["wait_ms"]), Error: firstNonEmpty(asString(obj["error"]), asString(obj["error_number"])), Raw: fmt.Sprint(obj)})
		default:
			out = append(out, Record{Engine: strings.TrimSuffix(strings.TrimSuffix(format, "-explain-json"), "-json"), Query: asString(obj["Query"]), Fingerprint: Fingerprint(asString(obj["Query"])), Plan: true, PlanSummary: planSummary(obj), Raw: fmt.Sprint(obj)})
		}
	}
	return out
}

func parsePostgresTextLine(line string) (Record, bool) {
	return postgresMessage(line)
}

var durationRE = regexp.MustCompile(`duration:\s*([0-9.]+)\s*ms\s+(?:statement|execute [^:]+):\s*(.*)$`)

func postgresMessage(message string) (Record, bool) {
	if m := durationRE.FindStringSubmatch(message); m != nil {
		query := strings.TrimSpace(m[2])
		return Record{Engine: "postgresql", Query: query, Fingerprint: Fingerprint(query), DurationMS: parseFloat(m[1]), Raw: message}, true
	}
	if strings.Contains(message, "ERROR:") {
		return Record{Engine: "postgresql", Error: strings.TrimSpace(message), Operation: "error", Raw: message}, true
	}
	if strings.Contains(strings.ToLower(message), "lock wait") {
		return Record{Engine: "postgresql", LockWaitMS: 1, Operation: "lock_wait", Raw: message}, true
	}
	return Record{}, false
}

func fillMySQLTiming(record *Record, line string) {
	for _, field := range strings.Fields(line) {
		field = strings.TrimSuffix(field, ":")
	}
	re := regexp.MustCompile(`Query_time:\s*([0-9.]+).*Lock_time:\s*([0-9.]+).*Rows_examined:\s*(\d+)`)
	if m := re.FindStringSubmatch(line); m != nil {
		record.DurationMS = parseFloat(m[1]) * 1000
		record.LockWaitMS = parseFloat(m[2]) * 1000
		record.Rows = int(parseFloat(m[3]))
	}
}

func parseRedisSlowLine(line string) (Record, bool) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return Record{}, false
	}
	micro, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return Record{}, false
	}
	query := strings.Join(fields[3:], " ")
	return Record{Engine: "redis", Query: query, Fingerprint: Fingerprint(query), DurationMS: micro / 1000, Operation: fields[3], Raw: line}, true
}

func Fingerprint(query string) string {
	query = strings.ToLower(strings.TrimSpace(query))
	query = regexp.MustCompile(`'[^']*'|"[^"]*"|\b\d+\b`).ReplaceAllString(query, "?")
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")
	if len(query) > 160 {
		query = query[:160]
	}
	return query
}

func canonical(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		return "auto"
	}
	return format
}

func detectText(line string) string {
	switch {
	case strings.HasPrefix(line, "# Time:"):
		return "mysql-slow"
	case strings.Contains(line, "duration:") || strings.Contains(line, "ERROR:"):
		return "postgres-text"
	default:
		return "postgres-text"
	}
}

func parseAnyTime(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.000 MST", "2006-01-02T15:04:05.000000Z", "060102 15:04:05"} {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts
		}
	}
	return time.Time{}
}

func planSummary(obj map[string]any) string {
	raw, _ := json.Marshal(obj)
	text := string(raw)
	if len(text) > 240 {
		text = text[:240]
	}
	return text
}

func array(v any) []any {
	if out, ok := v.([]any); ok {
		return out
	}
	return nil
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		return parseFloat(x)
	}
	return 0
}

func parseFloat(value string) float64 {
	n, _ := strconv.ParseFloat(value, 64)
	return n
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
