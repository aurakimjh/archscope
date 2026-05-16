package brokerlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBrokerLogsAndDiagnostics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broker.log")
	input := "2026-05-16 10:00:00 ERROR kafka Under replicated partition topic=orders partition=0 ISR shrunk\n" +
		"2026-05-16 10:00:01 WARN rabbitmq queue orders messages backlog\n" +
		"2026-05-16 10:00:02 WARN nats slow consumer detected\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	records, diags, err := ParseFile(path, "auto", Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 3 || diags.ParsedRecords != 3 {
		t.Fatalf("records=%+v diagnostics=%+v", records, diags)
	}
	if records[0].EventType != "replication" || records[1].EventType != "queue_pressure" || records[2].EventType != "slow_consumer" {
		t.Fatalf("event types=%+v", records)
	}
}

func TestParseRabbitDiagnosticsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rabbit.json")
	if err := os.WriteFile(path, []byte(`{"queues":[{"name":"orders","messages":42}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	records, _, err := ParseFile(path, "rabbitmq-diagnostics-json", Options{})
	if err != nil || len(records) != 1 || records[0].EventType != "queue_pressure" {
		t.Fatalf("records=%+v err=%v", records, err)
	}
}
