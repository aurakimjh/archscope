package brokerlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

type Record struct {
	Timestamp time.Time
	Broker    string
	Severity  string
	EventType string
	Topic     string
	Queue     string
	Partition string
	Node      string
	Message   string
	Raw       string
}

type Options struct{ Strict bool }

func ParseFile(path, format string, _ Options) ([]Record, *diagnostics.ParserDiagnostics, error) {
	format = canonical(format)
	diags := diagnostics.New(format)
	diags.SetSourceFile(path)
	if format == "rabbitmq-diagnostics-json" {
		return parseRabbitDiagnostics(path, diags)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, diags, err
	}
	defer file.Close()
	var records []Record
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		diags.TotalLines++
		if strings.TrimSpace(line) == "" {
			continue
		}
		broker := format
		if broker == "auto" {
			broker = detect(line)
		}
		if broker == "" {
			diags.AddSkipped(diags.TotalLines, "NO_BROKER_FORMAT", "line did not match supported broker logs", line)
			continue
		}
		records = append(records, parseLine(line, broker))
	}
	if err := scanner.Err(); err != nil {
		return records, diags, err
	}
	diags.ParsedRecords = len(records)
	return records, diags, nil
}

func parseRabbitDiagnostics(path string, diags *diagnostics.ParserDiagnostics) ([]Record, *diagnostics.ParserDiagnostics, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, diags, err
	}
	diags.TotalLines = 1
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		diags.AddSkipped(1, "INVALID_JSON", err.Error(), string(data[:min(len(data), diagnostics.RawPreviewLimit)]))
		return nil, diags, nil
	}
	var records []Record
	for _, item := range array(payload["queues"]) {
		q, _ := item.(map[string]any)
		name := fmt.Sprint(q["name"])
		messages := int(num(q["messages"]))
		event := "broker_health"
		if messages > 0 {
			event = "queue_pressure"
		}
		records = append(records, Record{Broker: "rabbitmq", EventType: event, Queue: name, Severity: "WARN", Message: fmt.Sprintf("queue %s messages=%d", name, messages)})
	}
	diags.ParsedRecords = len(records)
	return records, diags, nil
}

func parseLine(line, broker string) Record {
	return Record{
		Timestamp: parseTime(line),
		Broker:    broker,
		Severity:  severity(line),
		EventType: classify(line),
		Topic:     extract(line, `topic[ =:]([A-Za-z0-9_.-]+)`),
		Queue:     extract(line, `queue[ =:]([A-Za-z0-9_.-]+)`),
		Partition: extract(line, `partition[ =:]([A-Za-z0-9_.-]+)`),
		Node:      extract(line, `node[ =:]([A-Za-z0-9_.-]+)`),
		Message:   strings.TrimSpace(line),
		Raw:       line,
	}
}

func detect(line string) string {
	text := strings.ToLower(line)
	switch {
	case strings.Contains(text, "kafka") || strings.Contains(text, "isr") || strings.Contains(text, "kraft"):
		return "kafka"
	case strings.Contains(text, "rabbit"):
		return "rabbitmq"
	case strings.Contains(text, "pulsar") || strings.Contains(text, "bookie") || strings.Contains(text, "ledger"):
		return "pulsar"
	case strings.Contains(text, "nats") || strings.Contains(text, "jetstream"):
		return "nats"
	case strings.Contains(text, "activemq"):
		return "activemq"
	default:
		return ""
	}
}

func classify(line string) string {
	text := strings.ToLower(line)
	switch {
	case strings.Contains(text, "rebalance") || strings.Contains(text, "group coordinator"):
		return "rebalance"
	case strings.Contains(text, "under replicated") || strings.Contains(text, "isr") || strings.Contains(text, "replication"):
		return "replication"
	case strings.Contains(text, "kraft") || strings.Contains(text, "quorum"):
		return "kraft_quorum"
	case strings.Contains(text, "dead letter") || strings.Contains(text, "dead-letter") || strings.Contains(text, "dlq"):
		return "dead_letter"
	case strings.Contains(text, "queue") && (strings.Contains(text, "full") || strings.Contains(text, "messages") || strings.Contains(text, "backlog")):
		return "queue_pressure"
	case strings.Contains(text, "slow consumer"):
		return "slow_consumer"
	case strings.Contains(text, "connection") || strings.Contains(text, "disconnected") || strings.Contains(text, "churn"):
		return "connection_churn"
	case strings.Contains(text, "partition"):
		return "partition"
	case strings.Contains(text, "authorization") || strings.Contains(text, "auth failed"):
		return "authorization_failure"
	case strings.Contains(text, "compaction"):
		return "log_compaction"
	case strings.Contains(text, "store usage"):
		return "store_usage"
	case strings.Contains(text, "unhealthy") || strings.Contains(text, "down"):
		return "broker_health"
	default:
		return "broker_event"
	}
}

func severity(line string) string {
	text := strings.ToLower(line)
	switch {
	case strings.Contains(text, "error") || strings.Contains(text, "failed"):
		return "ERROR"
	case strings.Contains(text, "warn"):
		return "WARN"
	default:
		return "INFO"
	}
}

func parseTime(line string) time.Time {
	re := regexp.MustCompile(`\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?`)
	value := re.FindString(line)
	value = strings.Replace(value, ",", ".", 1)
	for _, layout := range []string{"2006-01-02 15:04:05.000", "2006-01-02 15:04:05", "2006-01-02T15:04:05.000", "2006-01-02T15:04:05"} {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts
		}
	}
	return time.Time{}
}

func extract(line, pattern string) string {
	re := regexp.MustCompile(`(?i)` + pattern)
	if m := re.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

func canonical(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		return "auto"
	}
	return format
}

func array(v any) []any {
	if out, ok := v.([]any); ok {
		return out
	}
	return nil
}

func num(v any) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
