package platform

import (
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/platform"
)

const ResultType = "kubernetes_evidence"

type Options struct {
	Format string
	TopN   int
}

func Analyze(path string, opts Options) (models.AnalysisResult, error) {
	records, diags, err := parser.ParseFile(path, opts.Format, parser.Options{})
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return Build(records, path, diags, opts), nil
}

func Build(records []parser.Record, sourceFile string, diags *diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = 50
	}
	reasonCounts := map[string]int{}
	kindCounts := map[string]int{}
	security := 0
	restarts := 0
	rows := []map[string]any{}
	for _, record := range records {
		reasonCounts[firstNonEmpty(record.Reason, record.Operation, "platform_event")]++
		kindCounts[record.Kind]++
		if record.Kind == "cloud_audit" {
			security++
		}
		restarts += record.RestartCount
		if len(rows) < topN {
			rows = append(rows, row(record))
		}
	}
	result := models.New(ResultType, "platform_evidence")
	result.SourceFiles = []string{sourceFile}
	result.Summary = map[string]any{
		"total_events":           len(records),
		"oom_killed_count":       reasonCounts["OOMKilled"] + reasonCounts["oomkilled"],
		"restart_count":          restarts,
		"eviction_count":         reasonCounts["Evicted"] + reasonCounts["evicted"],
		"scheduling_issue_count": reasonCounts["FailedScheduling"] + reasonCounts["failedscheduling"],
		"image_pull_issue_count": reasonCounts["ImagePullBackOff"] + reasonCounts["imagepullbackoff"],
		"readiness_issue_count":  reasonCounts["readiness"],
		"node_pressure_count":    reasonCounts["nodepressure"],
		"security_event_count":   security,
	}
	result.Series = map[string]any{"reason_distribution": rowsFromCounts(reasonCounts, "reason", topN), "kind_distribution": rowsFromCounts(kindCounts, "kind", topN)}
	result.Tables = map[string]any{"events": rows}
	if diags == nil {
		diags = diagnostics.New(firstNonEmpty(opts.Format, "auto"))
		diags.TotalLines = len(records)
		diags.ParsedRecords = len(records)
	}
	result.Metadata.SchemaVersion = "0.1.0"
	result.Metadata.Diagnostics = diags
	result.Metadata.Extra["format"] = firstNonEmpty(opts.Format, "auto")
	addFindings(&result)
	ingestion.AttachSourceMetadata(&result, ingestion.NewSourceMetadata(sourceFile, ingestion.SourceMetadataOptions{SourceKind: ingestion.SourceKindPlatformEvidence, SourceFormat: firstNonEmpty(opts.Format, "auto"), Product: "Kubernetes/container/cloud"}))
	return result
}

func row(record parser.Record) map[string]any {
	return map[string]any{"kind": record.Kind, "timestamp": record.Timestamp, "severity": record.Severity, "cluster": record.Cluster, "namespace": record.Namespace, "workload": record.Workload, "pod": record.Pod, "container": record.Container, "node": record.Node, "image": record.Image, "restart_count": record.RestartCount, "reason": record.Reason, "message": record.Message, "cloud_provider": record.CloudProvider, "actor": record.Actor, "operation": record.Operation, "resource": record.Resource, "security_category": record.SecurityCategory}
}

func addFindings(result *models.AnalysisResult) {
	checks := []struct{ key, code, msg, sev string }{
		{"oom_killed_count", "K8S_OOMKILLED", "Pod OOMKilled evidence is present.", "critical"},
		{"restart_count", "K8S_RESTARTS_PRESENT", "Pod/container restarts are present.", "warning"},
		{"eviction_count", "K8S_EVICTION", "Pod eviction evidence is present.", "warning"},
		{"scheduling_issue_count", "K8S_SCHEDULING_ISSUE", "Scheduling issue evidence is present.", "warning"},
		{"image_pull_issue_count", "K8S_IMAGE_PULL_ISSUE", "Image pull issue evidence is present.", "warning"},
		{"readiness_issue_count", "K8S_READINESS_ISSUE", "Readiness or probe issue evidence is present.", "warning"},
		{"node_pressure_count", "K8S_NODE_PRESSURE", "Node pressure evidence is present.", "warning"},
		{"security_event_count", "CLOUD_AUDIT_SECURITY_EVENT", "Cloud audit/security event evidence is present.", "warning"},
	}
	for _, check := range checks {
		if asInt(result.Summary[check.key]) > 0 {
			result.AddFinding(check.sev, check.code, check.msg, map[string]any{check.key: result.Summary[check.key]})
		}
	}
}

func rowsFromCounts(counts map[string]int, key string, limit int) []map[string]any {
	type kv struct {
		key   string
		count int
	}
	items := []kv{}
	for k, v := range counts {
		items = append(items, kv{k, v})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].key < items[j].key
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := []map[string]any{}
	for _, item := range items {
		out = append(out, map[string]any{key: item.key, "count": item.count})
	}
	return out
}

func asInt(v any) int {
	if n, ok := v.(int); ok {
		return n
	}
	return 0
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
