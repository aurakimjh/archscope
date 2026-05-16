package serverlog

import (
	"testing"
	"time"

	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/serverlog"
)

func TestAnalyzeServerLogFindingsAndTables(t *testing.T) {
	base := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	records := []parser.Record{
		{
			Timestamp: base, Product: "tomcat", Severity: "ERROR", Component: "deploy",
			EventType: "deployment", Message: "Deployment failed", TraceID: "trace-1",
		},
		{
			Timestamp: base.Add(time.Second), Product: "weblogic", Severity: "WARN", Component: "JDBC",
			EventType: "datasource_pool", Message: "DataSource connection pool exhausted", RequestID: "req-1",
		},
		{
			Timestamp: base.Add(2 * time.Second), Product: "nginx", Severity: "ERROR", Component: "nginx.worker",
			EventType: "worker_error", Message: "connect() failed while connecting to upstream",
		},
		{
			Timestamp: base.Add(3 * time.Second), Product: "websphere", Severity: "WARN", Component: "ThreadMonitor",
			EventType: "hung_thread", Message: "Hung thread detected",
		},
	}
	result := Build(records, "server.log", "auto", nil, Options{})
	if got := result.Summary["total_events"]; got != 4 {
		t.Fatalf("total_events=%v", got)
	}
	if got := result.Summary["correlated_event_count"]; got != 2 {
		t.Fatalf("correlated_event_count=%v", got)
	}
	if rows := result.Tables["events"].([]map[string]any); len(rows) != 4 {
		t.Fatalf("events rows=%d", len(rows))
	}
	codes := map[string]bool{}
	for _, finding := range result.Metadata.Findings {
		codes[finding["code"].(string)] = true
	}
	for _, code := range []string{"SERVER_SEVERE_ERRORS", "DEPLOYMENT_FAILURE", "DATASOURCE_POOL_WARNING", "HUNG_THREAD_DETECTED", "WORKER_ERROR_PRESENT"} {
		if !codes[code] {
			t.Fatalf("missing finding %s in %+v", code, result.Metadata.Findings)
		}
	}
	if result.Metadata.Extra["source_metadata"] == nil || result.Metadata.Extra["correlation_keys"] == nil {
		t.Fatalf("missing source/correlation metadata: %+v", result.Metadata.Extra)
	}
}
