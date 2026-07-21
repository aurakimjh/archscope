package httpcapture

import (
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestBuildHTTPHARAnalysisIsDeterministicAndBounded(t *testing.T) {
	entries := []models.CaptureTransaction{
		{ID: "one", StartedAt: "2026-07-20T10:00:00Z", Method: "GET", Path: "/orders", Host: "api", StatusCode: 200, TotalMS: 25, State: models.TxComplete, Request: unknownMessage(), Response: unknownMessage(), Process: &models.ProcessInstance{Key: models.ProcessKey{PID: 42}, Name: "client", ExecPath: ".../client", CommandLine: "client --password=[REDACTED]", User: "should-not-export"}},
		{ID: "two", StartedAt: "2026-07-20T10:00:05Z", Method: "GET", Path: "/orders", Host: "api", StatusCode: 503, TotalMS: 50, State: models.TxComplete, Request: unknownMessage(), Response: unknownMessage()},
	}
	result := Build(entries, "sample.har", "har", "chrome", Options{TopN: 1})
	if result.Type != ResultType || result.Summary["total_transactions"] != 2 || result.Summary["error_transactions"] != 1 {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
	if rows, ok := result.Tables["endpoints"].([]map[string]any); !ok || len(rows) != 1 || rows[0]["avg_duration_ms"] != 37.5 {
		t.Fatalf("unexpected endpoints: %#v", result.Tables["endpoints"])
	}
	if result.Metadata.Extra["http_capture"].(map[string]any)["fidelity"] != "semantic" {
		t.Fatal("missing fidelity metadata")
	}
	if _, ok := result.Metadata.Extra["capture_aggregate_snapshot"]; !ok {
		t.Fatal("HAR must use capture aggregate snapshot")
	}
	transactions, ok := result.Tables["transactions"].([]map[string]any)
	if !ok || len(transactions) != 1 {
		t.Fatalf("transaction details were not bounded: %#v", result.Tables["transactions"])
	}
	process, ok := transactions[0]["process"].(map[string]any)
	if !ok || process["pid"] != int32(42) {
		t.Fatalf("safe process metadata missing: %#v", transactions[0]["process"])
	}
	if _, exists := process["command_line"]; exists {
		t.Fatal("command line must not be exported by default")
	}
	if _, exists := process["user"]; exists {
		t.Fatal("process user must not be exported by default")
	}
}

func unknownMessage() models.HTTPMessage {
	return models.HTTPMessage{HeaderSize: -1, BodySize: -1, BodyDecoded: -1, TransferSize: -1, BodyStorage: "omitted"}
}
