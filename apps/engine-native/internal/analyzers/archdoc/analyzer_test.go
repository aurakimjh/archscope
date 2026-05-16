package archdoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeArchitectureDocs(t *testing.T) {
	dir := t.TempDir()
	api := writeResult(t, dir, "api.json", map[string]any{
		"type": "api_contract_analysis",
		"tables": map[string]any{
			"operations": []map[string]any{{"method": "GET", "path": "/orders/{id}", "operation_id": "getOrder", "observed": true}},
		},
		"metadata": map[string]any{
			"findings": []map[string]any{{"severity": "warning", "code": "SLOW_API_OPERATION", "message": "Slow API"}},
		},
	})
	stitch := writeResult(t, dir, "stitch.json", map[string]any{
		"type": "stitched_evidence",
		"tables": map[string]any{
			"service_dependencies": []map[string]any{{"caller": "order-service", "callee": "database:shop"}},
			"gaps":                 []map[string]any{{"message": "missing trace", "service": "order-service"}},
		},
		"metadata": map[string]any{
			"findings": []map[string]any{{"severity": "warning", "code": "MISSING_TRACE_ID", "message": "Missing trace ID"}},
		},
	})
	platform := writeResult(t, dir, "platform.json", map[string]any{
		"type": "kubernetes_evidence",
		"tables": map[string]any{
			"events": []map[string]any{{"namespace": "prod", "pod": "orders-1", "node": "node-a", "reason": "OOMKilled"}},
		},
	})

	result, err := AnalyzeFiles([]string{api, stitch, platform}, Options{TopN: 20})
	if err != nil {
		t.Fatalf("analyze files: %v", err)
	}
	if result.Type != ResultType {
		t.Fatalf("type = %q", result.Type)
	}
	if result.Summary["arc42_section_count"] != 5 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if result.Summary["risk_count"] != 2 || result.Summary["quality_requirement_count"] != 2 {
		t.Fatalf("risk/quality summary = %#v", result.Summary)
	}
	if rows, ok := result.Tables["adr_drafts"].([]map[string]any); !ok || len(rows) == 0 {
		t.Fatalf("missing ADR drafts: %#v", result.Tables["adr_drafts"])
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
	if _, ok := payload["tables"]; !ok {
		payload["tables"] = map[string]any{}
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
