package platform

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

type Record struct {
	Kind             string
	Timestamp        string
	Severity         string
	Cluster          string
	Namespace        string
	Workload         string
	Pod              string
	Container        string
	Node             string
	Image            string
	RestartCount     int
	Reason           string
	Message          string
	CloudProvider    string
	Actor            string
	Operation        string
	Resource         string
	SecurityCategory string
	Raw              string
}

type Options struct{}

func ParseFile(path, format string, _ Options) ([]Record, *diagnostics.ParserDiagnostics, error) {
	format = canonical(format)
	diags := diagnostics.New(format)
	diags.SetSourceFile(path)
	if strings.HasSuffix(format, "json") || format == "auto" {
		if records, ok, err := parseJSONFile(path, format, diags); ok || err != nil {
			return records, diags, err
		}
	}
	return parseText(path, format, diags)
}

func parseJSONFile(path, format string, diags *diagnostics.ParserDiagnostics) ([]Record, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, false, nil
	}
	diags.TotalLines = 1
	detected := detectJSON(payload, format)
	if detected == "" {
		return nil, false, nil
	}
	diags.Format = detected
	records := recordsFromJSON(payload, detected)
	diags.ParsedRecords = len(records)
	return records, true, nil
}

func recordsFromJSON(payload map[string]any, format string) []Record {
	switch format {
	case "kubectl-events-json":
		return kubernetesEvents(payload)
	case "describe-pod-json":
		return podRecord(payload)
	case "aws-cloudtrail-json":
		return awsCloudTrail(payload)
	case "gcp-audit-json":
		return gcpAudit(payload)
	case "azure-activity-json":
		return azureActivity(payload)
	default:
		return nil
	}
}

func kubernetesEvents(payload map[string]any) []Record {
	var out []Record
	for _, item := range array(payload["items"]) {
		obj, _ := item.(map[string]any)
		involved, _ := obj["involvedObject"].(map[string]any)
		out = append(out, Record{
			Kind:      "kubernetes_event",
			Timestamp: firstNonEmpty(str(obj["lastTimestamp"]), str(obj["eventTime"])),
			Severity:  severity(str(obj["type"]), str(obj["reason"]), str(obj["message"])),
			Namespace: str(involved["namespace"]),
			Pod:       str(involved["name"]),
			Reason:    str(obj["reason"]),
			Message:   str(obj["message"]),
			Raw:       fmt.Sprint(obj),
		})
	}
	return out
}

func podRecord(payload map[string]any) []Record {
	meta, _ := payload["metadata"].(map[string]any)
	status, _ := payload["status"].(map[string]any)
	spec, _ := payload["spec"].(map[string]any)
	rec := Record{Kind: "pod", Namespace: str(meta["namespace"]), Pod: str(meta["name"]), Node: str(spec["nodeName"]), Reason: str(status["reason"]), Message: str(status["message"]), Severity: "INFO"}
	for _, item := range array(status["containerStatuses"]) {
		cs, _ := item.(map[string]any)
		if n := intNum(cs["restartCount"]); n > rec.RestartCount {
			rec.RestartCount = n
		}
		rec.Container = firstNonEmpty(rec.Container, str(cs["name"]))
		rec.Image = firstNonEmpty(rec.Image, str(cs["image"]))
	}
	if rec.RestartCount > 0 || strings.Contains(rec.Reason, "OOM") {
		rec.Severity = "WARN"
	}
	return []Record{rec}
}

func awsCloudTrail(payload map[string]any) []Record {
	var out []Record
	for _, item := range array(payload["Records"]) {
		obj, _ := item.(map[string]any)
		out = append(out, Record{Kind: "cloud_audit", Timestamp: str(obj["eventTime"]), Severity: "WARN", CloudProvider: "aws", Actor: fmt.Sprint(obj["userIdentity"]), Operation: str(obj["eventName"]), Resource: str(obj["eventSource"]), SecurityCategory: "audit", Message: str(obj["errorMessage"]), Raw: fmt.Sprint(obj)})
	}
	return out
}

func gcpAudit(payload map[string]any) []Record {
	proto, _ := payload["protoPayload"].(map[string]any)
	return []Record{{Kind: "cloud_audit", Timestamp: str(payload["timestamp"]), Severity: "WARN", CloudProvider: "gcp", Actor: str(proto["authenticationInfo"]), Operation: str(proto["methodName"]), Resource: str(proto["resourceName"]), SecurityCategory: "audit", Message: str(proto["status"]), Raw: fmt.Sprint(payload)}}
}

func azureActivity(payload map[string]any) []Record {
	items := array(payload["value"])
	if len(items) == 0 {
		items = []any{payload}
	}
	var out []Record
	for _, item := range items {
		obj, _ := item.(map[string]any)
		out = append(out, Record{Kind: "cloud_audit", Timestamp: str(obj["eventTimestamp"]), Severity: severity(str(obj["level"]), str(obj["resultType"]), ""), CloudProvider: "azure", Actor: str(obj["caller"]), Operation: str(obj["operationName"]), Resource: str(obj["resourceId"]), SecurityCategory: str(obj["category"]), Message: str(obj["resultDescription"]), Raw: fmt.Sprint(obj)})
	}
	return out
}

func parseText(path, format string, diags *diagnostics.ParserDiagnostics) ([]Record, *diagnostics.ParserDiagnostics, error) {
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
		kind := "kubelet_log"
		if format == "container-runtime-log" || strings.Contains(strings.ToLower(line), "containerd") || strings.Contains(strings.ToLower(line), "docker") || strings.Contains(strings.ToLower(line), "cri-o") {
			kind = "container_runtime_log"
		}
		records = append(records, Record{Kind: kind, Severity: severity(line, "", ""), Reason: reason(line), Message: line, Node: extract(line, "node="), Pod: extract(line, "pod="), Container: extract(line, "container="), Raw: line})
	}
	if err := scanner.Err(); err != nil {
		return records, diags, err
	}
	diags.ParsedRecords = len(records)
	return records, diags, nil
}

func detectJSON(payload map[string]any, format string) string {
	if format != "" && format != "auto" {
		return format
	}
	if _, ok := payload["Records"]; ok {
		return "aws-cloudtrail-json"
	}
	if _, ok := payload["protoPayload"]; ok {
		return "gcp-audit-json"
	}
	if _, ok := payload["operationName"]; ok {
		return "azure-activity-json"
	}
	if _, ok := payload["items"]; ok {
		return "kubectl-events-json"
	}
	if payload["kind"] == "Pod" {
		return "describe-pod-json"
	}
	return ""
}

func reason(text string) string {
	lower := strings.ToLower(text)
	for _, item := range []string{"oomkilled", "evicted", "failedscheduling", "imagepullbackoff", "readiness", "nodepressure", "probe"} {
		if strings.Contains(lower, strings.ToLower(item)) {
			return item
		}
	}
	return "platform_event"
}

func severity(values ...string) string {
	text := strings.ToLower(strings.Join(values, " "))
	if strings.Contains(text, "error") || strings.Contains(text, "fail") || strings.Contains(text, "oom") || strings.Contains(text, "denied") {
		return "ERROR"
	}
	if strings.Contains(text, "warn") || strings.Contains(text, "evict") || strings.Contains(text, "backoff") {
		return "WARN"
	}
	return "INFO"
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
func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func intNum(v any) int {
	if n, ok := v.(float64); ok {
		return int(n)
	}
	return 0
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
func extract(line, prefix string) string {
	idx := strings.Index(line, prefix)
	if idx < 0 {
		return ""
	}
	rest := line[idx+len(prefix):]
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
