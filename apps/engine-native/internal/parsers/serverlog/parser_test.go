package serverlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseServerLogFormats(t *testing.T) {
	cases := []struct {
		format string
		line   string
		event  string
	}{
		{"tomcat", "16-May-2026 10:00:00.123 SEVERE [main] org.apache.catalina.core.StandardContext Deployment failed for /shop", "deployment"},
		{"jetty", "2026-05-16 10:00:00.123:WARN:oejs.Server:main: Server startup completed", "startup"},
		{"jboss", "2026-05-16 10:00:00,123 ERROR [org.jboss.as.server.deployment] (ServerService Thread Pool -- 1) Deployment failed", "deployment"},
		{"weblogic", "<May 16, 2026 10:00:00 AM KST> <Error> <JDBC> <host1> <AdminServer> <ExecuteThread: '1'> <> <> <BEA-000000> <DataSource connection pool exhausted>", "datasource_pool"},
		{"websphere", "[5/16/26 10:00:00:123 KST] 00000001 ThreadMonitor W WSVR0605W: Thread hung thread detected", "hung_thread"},
		{"glassfish", "[2026-05-16T10:00:00.123+0900] [GlassFish 7] [SEVERE] [] [javax.enterprise.system.core] [tid: _ThreadID=1] [[thread pool rejectedExecution]]", "thread_pool_pressure"},
		{"nginx-error", "2026/05/16 10:00:00 [error] 123#0: *1 connect() failed (111: Connection refused) while connecting to upstream, request_id=req-1", "worker_error"},
		{"apache-error", "[Sat May 16 10:00:00.123456 2026] [proxy:error] [pid 123:tid 456] AH01114: HTTP: failed to make connection to backend", "worker_error"},
	}
	for _, tc := range cases {
		rec, perr := ParseLineForFormat(tc.line, tc.format)
		if perr != nil {
			t.Fatalf("%s parse error: %+v", tc.format, perr)
		}
		if rec.EventType != tc.event {
			t.Fatalf("%s event type = %q, want %q: %+v", tc.format, rec.EventType, tc.event, rec)
		}
		if rec.Product == "" || rec.Message == "" {
			t.Fatalf("%s missing normalized fields: %+v", tc.format, rec)
		}
	}
}

func TestForEachRecordAutoDetectsServerLogs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.log")
	input := "16-May-2026 10:00:00.123 SEVERE [main] org.apache.catalina.core.StandardContext Deployment failed trace_id=abc\n" +
		"2026/05/16 10:00:01 [error] 123#0: *1 upstream timed out request_id=req-2\n" +
		"not a supported line\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var records []Record
	diags, err := ForEachRecord(path, "auto", Options{}, func(record Record) error {
		records = append(records, record)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachRecord: %v", err)
	}
	if len(records) != 2 || diags.ParsedRecords != 2 || diags.SkippedLines != 1 {
		t.Fatalf("records=%d diagnostics=%+v", len(records), diags)
	}
}
