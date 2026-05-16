package databaselog

import (
	"testing"

	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/databaselog"
)

func TestAnalyzeDatabaseEvidence(t *testing.T) {
	result := Build([]parser.Record{
		{Engine: "postgresql", Database: "shop", Query: "select * from orders where id = 1", Fingerprint: "select * from orders where id = ?", DurationMS: 1200, Rows: 2000},
		{Engine: "mysql", Database: "shop", Query: "select * from users where id = 2", Fingerprint: "select * from users where id = ?", DurationMS: 900, LockWaitMS: 50},
		{Engine: "postgresql", Error: "ERROR: deadlock detected"},
		{Engine: "postgresql", Query: "select * from orders", Plan: true, PlanSummary: "Seq Scan"},
	}, "db.log", nil, Options{})
	if result.Summary["total_queries"] != 4 || result.Summary["error_count"] != 1 || result.Summary["plan_count"] != 1 {
		t.Fatalf("summary=%+v", result.Summary)
	}
	if rows := result.Tables["service_dependencies"].([]map[string]any); len(rows) == 0 {
		t.Fatalf("missing service dependency rows")
	}
	codes := map[string]bool{}
	for _, finding := range result.Metadata.Findings {
		codes[finding["code"].(string)] = true
	}
	for _, code := range []string{"SLOW_QUERY_PRESENT", "LOCK_WAIT_PRESENT", "DB_ERRORS_PRESENT", "HIGH_ROWS_EXAMINED", "EXPLAIN_PLAN_IMPORTED"} {
		if !codes[code] {
			t.Fatalf("missing finding %s in %+v", code, result.Metadata.Findings)
		}
	}
}
