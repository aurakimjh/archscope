package stitching

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeFilesBuildsMatchesAndGaps(t *testing.T) {
	dir := t.TempDir()
	access := writeResult(t, dir, "access.json", map[string]any{
		"type": "access_log",
		"tables": map[string]any{
			"sample_records": []map[string]any{
				{"timestamp": "2026-05-16T10:00:00Z", "trace_id": "trace-1", "request_id": "req-1", "uri": "/orders"},
				{"timestamp": "2026-05-16T10:00:01Z", "uri": "/missing-trace"},
			},
		},
	})
	trace := writeResult(t, dir, "trace.json", map[string]any{
		"type": "trace_import",
		"tables": map[string]any{
			"spans": []map[string]any{
				{"trace_id": "trace-1", "span_id": "span-1", "service": "order-service", "span_name": "GET /orders"},
				{"trace_id": "trace-2", "span_id": "span-2", "parent_span_id": "missing-parent", "service": "worker"},
			},
		},
	})
	database := writeResult(t, dir, "database.json", map[string]any{
		"type": "database_slow_query",
		"tables": map[string]any{
			"queries": []map[string]any{
				{"trace_id": "trace-1", "database": "shop", "fingerprint": "select * from orders where id = ?"},
			},
		},
	})

	result, err := AnalyzeFiles([]string{access, trace, database}, Options{TopN: 20})
	if err != nil {
		t.Fatalf("analyze files: %v", err)
	}
	if result.Type != ResultType {
		t.Fatalf("type = %q", result.Type)
	}
	if result.Summary["matched_group_count"] != 1 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if result.Summary["missing_trace_id_count"] != 1 || result.Summary["dropped_parent_span_count"] != 1 {
		t.Fatalf("gap summary = %#v", result.Summary)
	}
	if rows, ok := result.Tables["service_dependencies"].([]map[string]any); !ok || len(rows) != 1 {
		t.Fatalf("service dependencies = %#v", result.Tables["service_dependencies"])
	}
}

func writeResult(t *testing.T, dir, name string, payload map[string]any) string {
	t.Helper()
	if _, ok := payload["source_files"]; !ok {
		payload["source_files"] = []string{name}
	}
	if _, ok := payload["summary"]; !ok {
		payload["summary"] = map[string]any{}
	}
	if _, ok := payload["series"]; !ok {
		payload["series"] = map[string]any{}
	}
	if _, ok := payload["charts"]; !ok {
		payload["charts"] = map[string]any{}
	}
	if _, ok := payload["metadata"]; !ok {
		payload["metadata"] = map[string]any{}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
