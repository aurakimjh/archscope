package observability

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

type Record struct {
	Kind       string
	Timestamp  string
	Severity   string
	Service    string
	Message    string
	TraceID    string
	SpanID     string
	Dashboard  string
	PanelID    string
	PanelTitle string
	Attributes map[string]string
}

type Options struct{}

func ParseFile(path, format string, _ Options) ([]Record, *diagnostics.ParserDiagnostics, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	diags := diagnostics.New(firstNonEmpty(format, "auto"))
	diags.SetSourceFile(path)
	diags.TotalLines = 1
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		diags.AddSkipped(1, "INVALID_JSON", err.Error(), string(data[:min(len(data), diagnostics.RawPreviewLimit)]))
		return nil, diags, nil
	}
	records := []Record{}
	switch detect(payload, format) {
	case "loki-json":
		records = append(records, parseLoki(payload)...)
		diags.Format = "loki-json"
	case "tempo-json":
		records = append(records, parseTempo(payload)...)
		diags.Format = "tempo-json"
	case "grafana-dashboard-json":
		records = append(records, parseGrafana(payload)...)
		diags.Format = "grafana-dashboard-json"
	default:
		diags.AddSkipped(1, "UNKNOWN_OBSERVABILITY_JSON", "JSON did not match Loki, Tempo, or Grafana dashboard export.", "")
	}
	diags.ParsedRecords = len(records)
	return records, diags, nil
}

func detect(payload map[string]any, format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "" && format != "auto" {
		return format
	}
	if data, ok := payload["data"].(map[string]any); ok {
		if _, ok := data["result"]; ok {
			return "loki-json"
		}
	}
	if _, ok := payload["panels"]; ok {
		return "grafana-dashboard-json"
	}
	if _, ok := payload["trace"]; ok {
		return "tempo-json"
	}
	if _, ok := payload["batches"]; ok {
		return "tempo-json"
	}
	return ""
}

func parseLoki(payload map[string]any) []Record {
	var out []Record
	data, _ := payload["data"].(map[string]any)
	for _, item := range array(data["result"]) {
		obj, _ := item.(map[string]any)
		stream := strMap(obj["stream"])
		for _, pair := range array(obj["values"]) {
			values := array(pair)
			if len(values) < 2 {
				continue
			}
			msg := fmt.Sprint(values[1])
			out = append(out, Record{
				Kind:       "loki_log",
				Timestamp:  unixNanoString(fmt.Sprint(values[0])),
				Severity:   inferSeverity(msg, stream["level"]),
				Service:    firstNonEmpty(stream["service"], stream["app"], stream["job"]),
				Message:    msg,
				TraceID:    firstNonEmpty(stream["trace_id"], stream["traceID"]),
				Attributes: stream,
			})
		}
	}
	return out
}

func parseTempo(payload map[string]any) []Record {
	var out []Record
	if trace, ok := payload["trace"].(map[string]any); ok {
		for _, item := range array(trace["spans"]) {
			span, _ := item.(map[string]any)
			out = append(out, Record{
				Kind:     "tempo_span",
				TraceID:  firstNonEmpty(fmt.Sprint(trace["traceID"]), fmt.Sprint(trace["traceId"])),
				SpanID:   firstNonEmpty(fmt.Sprint(span["spanID"]), fmt.Sprint(span["spanId"])),
				Service:  fmt.Sprint(span["serviceName"]),
				Message:  firstNonEmpty(fmt.Sprint(span["name"]), fmt.Sprint(span["operationName"])),
				Severity: inferSeverity(fmt.Sprint(span["statusMessage"]), fmt.Sprint(span["statusCode"])),
			})
		}
	}
	return out
}

func parseGrafana(payload map[string]any) []Record {
	title := fmt.Sprint(payload["title"])
	uid := fmt.Sprint(payload["uid"])
	var out []Record
	for _, item := range array(payload["panels"]) {
		panel, _ := item.(map[string]any)
		out = append(out, Record{
			Kind:       "grafana_panel",
			Dashboard:  firstNonEmpty(uid, title),
			PanelID:    fmt.Sprint(panel["id"]),
			PanelTitle: fmt.Sprint(panel["title"]),
			Message:    fmt.Sprintf("%s / %s", title, panel["title"]),
			Attributes: map[string]string{"dashboard_title": title, "panel_type": fmt.Sprint(panel["type"])},
		})
	}
	return out
}

func array(v any) []any {
	if out, ok := v.([]any); ok {
		return out
	}
	return nil
}

func strMap(v any) map[string]string {
	out := map[string]string{}
	if m, ok := v.(map[string]any); ok {
		for k, value := range m {
			out[k] = fmt.Sprint(value)
		}
	}
	return out
}

func unixNanoString(raw string) string {
	ns, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return ""
	}
	return time.Unix(0, ns).UTC().Format(time.RFC3339Nano)
}

func inferSeverity(values ...string) string {
	text := strings.ToLower(strings.Join(values, " "))
	switch {
	case strings.Contains(text, "error") || strings.Contains(text, "fail") || strings.Contains(text, "status_code_error"):
		return "ERROR"
	case strings.Contains(text, "warn"):
		return "WARN"
	default:
		return "INFO"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
