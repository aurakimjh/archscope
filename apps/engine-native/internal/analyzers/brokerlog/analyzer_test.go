package brokerlog

import (
	"testing"

	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/brokerlog"
)

func TestAnalyzeBrokerEvidence(t *testing.T) {
	result := Build([]parser.Record{
		{Broker: "kafka", Severity: "ERROR", EventType: "replication", Topic: "orders", Message: "ISR shrunk"},
		{Broker: "kafka", Severity: "WARN", EventType: "rebalance", Topic: "orders", Message: "consumer rebalance"},
		{Broker: "rabbitmq", Severity: "WARN", EventType: "queue_pressure", Queue: "payments", Message: "queue backlog"},
		{Broker: "activemq", Severity: "WARN", EventType: "dead_letter", Queue: "DLQ", Message: "dead letter"},
	}, "broker.log", nil, Options{})
	if result.Summary["total_events"] != 4 || result.Summary["replication_issue_count"] != 1 {
		t.Fatalf("summary=%+v", result.Summary)
	}
	if rows := result.Tables["service_dependencies"].([]map[string]any); len(rows) == 0 {
		t.Fatalf("missing service dependency rows")
	}
	codes := map[string]bool{}
	for _, finding := range result.Metadata.Findings {
		codes[finding["code"].(string)] = true
	}
	for _, code := range []string{"BROKER_REPLICATION_ISSUE", "BROKER_REBALANCE", "BROKER_QUEUE_PRESSURE", "BROKER_DEAD_LETTER"} {
		if !codes[code] {
			t.Fatalf("missing finding %s in %+v", code, result.Metadata.Findings)
		}
	}
}
