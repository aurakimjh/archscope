package apicontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeApiContractFindsCoverageAndGaps(t *testing.T) {
	dir := t.TempDir()
	openapi := writeJSON(t, dir, "openapi.json", map[string]any{
		"openapi": "3.0.3",
		"info":    map[string]any{"title": "Orders API", "version": "1.0.0"},
		"paths": map[string]any{
			"/orders/{id}": map[string]any{"get": map[string]any{"operationId": "getOrder"}},
			"/payments":    map[string]any{"post": map[string]any{"operationId": "createPayment"}},
		},
	})
	access := writeJSON(t, dir, "access.json", map[string]any{
		"type": "access_log",
		"tables": map[string]any{
			"url_stats": []map[string]any{
				{"method": "GET", "uri": "/orders/1001", "count": 10, "p95_response_ms": 1500, "error_rate": 0.1},
				{"method": "GET", "uri": "/internal/health", "count": 3, "p95_response_ms": 10},
			},
		},
	})
	asyncapi := writeJSON(t, dir, "asyncapi.json", map[string]any{
		"asyncapi": "2.6.0",
		"info":     map[string]any{"title": "Orders Events", "version": "1.0.0"},
		"channels": map[string]any{
			"orders.created": map[string]any{"publish": map[string]any{"operationId": "publishOrderCreated"}},
		},
	})
	broker := writeJSON(t, dir, "broker.json", map[string]any{
		"type": "broker_log",
		"tables": map[string]any{
			"events": []map[string]any{
				{"topic": "orders.created", "event_type": "publish"},
				{"topic": "payments.failed", "event_type": "publish"},
			},
		},
	})

	result, err := Analyze(Options{OpenAPIPath: openapi, AccessResultPath: access, AsyncAPIPath: asyncapi, BrokerResultPath: broker, SlowThresholdMS: 1000, ErrorRateThreshold: 5})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.Type != ResultType {
		t.Fatalf("type = %q", result.Type)
	}
	if result.Summary["undocumented_route_count"] != 1 || result.Summary["unused_operation_count"] != 1 {
		t.Fatalf("http summary = %#v", result.Summary)
	}
	if result.Summary["slow_operation_count"] != 1 || result.Summary["high_error_operation_count"] != 1 {
		t.Fatalf("quality summary = %#v", result.Summary)
	}
	if result.Summary["undocumented_event_channel_count"] != 1 {
		t.Fatalf("event summary = %#v", result.Summary)
	}
	if len(result.Metadata.Findings) == 0 {
		t.Fatalf("expected findings")
	}
}

func writeJSON(t *testing.T, dir, name string, payload map[string]any) string {
	t.Helper()
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
