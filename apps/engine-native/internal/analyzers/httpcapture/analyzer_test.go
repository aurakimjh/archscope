package httpcapture

import (
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
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

func TestBuildLiveDerivesWeakestFidelityAndArchScopeProvenance(t *testing.T) {
	entries := []models.CaptureTransaction{
		{
			ID: "decoded", State: models.TxComplete, CaptureMode: "proxy_mitm",
			ObservationPoint: "proxy", Fidelity: "decoded_wire", Coverage: "confirmed",
			Request: unknownMessage(), Response: unknownMessage(),
		},
		{
			ID: "opaque", State: models.TxComplete, CaptureMode: "proxy_passthrough",
			ObservationPoint: "proxy", Fidelity: "unsupported", Coverage: "confirmed",
			Request: unknownMessage(), Response: unknownMessage(),
		},
	}
	redactionSummary := redact.Summary{
		Applied: true, Version: redact.PolicyVersion,
		Rules: []string{"query_value"}, Counts: map[string]int{"query_value": 1},
	}
	result := BuildLive(entries, "capture://cap-test", "canonical-v1", redactionSummary, Options{})
	metadata := result.Metadata.Extra["http_capture"].(map[string]any)
	if metadata["capture_mode"] != "mixed" ||
		metadata["observation_point"] != "proxy" ||
		metadata["fidelity"] != "unsupported" {
		t.Fatalf("live metadata=%+v", metadata)
	}
	if metadata["capture_mode"] == "har_import" ||
		metadata["observation_point"] == "foreign_tool" ||
		metadata["fidelity"] == "semantic" {
		t.Fatalf("live analysis claimed HAR semantic provenance: %+v", metadata)
	}
	if result.Summary["redaction_applied"] != true {
		t.Fatalf("redaction summary=%+v", result.Summary)
	}
	foundRedaction := false
	for _, finding := range result.Metadata.Findings {
		if finding["code"] == "CAPTURE_REDACTED" {
			foundRedaction = true
			break
		}
	}
	if !foundRedaction {
		t.Fatalf("findings=%+v", result.Metadata.Findings)
	}
}

func unknownMessage() models.HTTPMessage {
	return models.HTTPMessage{HeaderSize: -1, BodySize: -1, BodyDecoded: -1, TransferSize: -1, BodyStorage: "omitted"}
}
