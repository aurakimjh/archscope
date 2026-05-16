package archdoc

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const ResultType = "architecture_docs"

type Options struct {
	TopN int
}

type rawResult struct {
	Type        string         `json:"type"`
	SourceFiles []string       `json:"source_files"`
	Summary     map[string]any `json:"summary"`
	Series      map[string]any `json:"series"`
	Tables      map[string]any `json:"tables"`
	Metadata    map[string]any `json:"metadata"`
	filePath    string
}

func AnalyzeFiles(paths []string, opts Options) (models.AnalysisResult, error) {
	if len(paths) == 0 {
		return models.AnalysisResult{}, fmt.Errorf("at least one input result path is required")
	}
	results := make([]rawResult, 0, len(paths))
	for _, path := range paths {
		result, err := readResult(path)
		if err != nil {
			return models.AnalysisResult{}, err
		}
		result.filePath = path
		results = append(results, result)
	}
	return Build(results, opts), nil
}

func Build(results []rawResult, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = 100
	}
	services := serviceRows(results, topN)
	interfaces := interfaceRows(results, topN)
	runtime := runtimeRows(results, topN)
	deployment := deploymentRows(results, topN)
	quality := qualityRequirementRows(results, topN)
	risks := riskRows(results, topN)
	adrs := adrRows(results, risks, quality, topN)
	sections := arc42Sections(services, interfaces, runtime, deployment, quality, risks, adrs)

	result := models.New(ResultType, "architecture_docs")
	result.SourceFiles = sourceFiles(results)
	result.Summary = map[string]any{
		"source_count":              len(results),
		"service_count":             len(services),
		"interface_count":           len(interfaces),
		"runtime_view_count":        len(runtime),
		"deployment_view_count":     len(deployment),
		"quality_requirement_count": len(quality),
		"risk_count":                len(risks),
		"adr_draft_count":           len(adrs),
		"arc42_section_count":       len(sections),
	}
	result.Series = map[string]any{
		"risks_by_severity":      rowsFromCounts(riskSeverityCounts(risks), "severity", topN),
		"quality_by_category":    rowsFromCounts(categoryCounts(quality), "category", topN),
		"sources_by_result_type": rowsFromCounts(resultTypeCounts(results), "result_type", topN),
	}
	result.Tables = map[string]any{
		"arc42_sections":       sections,
		"context_services":     services,
		"interfaces":           interfaces,
		"runtime_views":        runtime,
		"deployment_views":     deployment,
		"quality_requirements": quality,
		"risks":                risks,
		"adr_drafts":           adrs,
	}
	result.Metadata.SchemaVersion = "0.1.0"
	result.Metadata.Extra["input_result_types"] = uniqueStrings(resultTypes(results))
	result.Metadata.Extra["arc42_sections"] = []string{"Context", "Runtime View", "Deployment View", "Quality Requirements", "Risks"}
	addFindings(&result)
	return result
}

func readResult(path string) (rawResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return rawResult{}, err
	}
	var result rawResult
	if err := json.Unmarshal(data, &result); err != nil {
		return rawResult{}, fmt.Errorf("%s: %w", path, err)
	}
	if result.Type == "" {
		return rawResult{}, fmt.Errorf("%s: missing result type", path)
	}
	return result, nil
}

func serviceRows(results []rawResult, limit int) []map[string]any {
	counts := map[string]int{}
	refs := map[string][]string{}
	for _, result := range results {
		for _, table := range []string{"service_dependencies", "msa_edges", "matches"} {
			for index, item := range array(result.Tables[table]) {
				row, ok := item.(map[string]any)
				if !ok {
					continue
				}
				for _, name := range []string{firstString(row, "caller", "caller_application", "service"), firstString(row, "callee", "callee_application", "target")} {
					name = normalizeName(name)
					if name == "" {
						continue
					}
					counts[name]++
					refs[name] = append(refs[name], evidenceRef(result, table, index))
				}
			}
		}
	}
	rows := rowsFromCounts(counts, "service", limit)
	for _, row := range rows {
		service := fmt.Sprint(row["service"])
		row["responsibility_draft"] = responsibilityDraft(service)
		row["evidence_refs"] = uniqueStrings(refs[service])
	}
	return rows
}

func interfaceRows(results []rawResult, limit int) []map[string]any {
	var rows []map[string]any
	for _, result := range results {
		if result.Type == "api_contract_analysis" {
			for index, item := range array(result.Tables["operations"]) {
				row, ok := item.(map[string]any)
				if !ok {
					continue
				}
				rows = append(rows, map[string]any{"interface_type": "http_api", "method": firstString(row, "method"), "path": firstString(row, "path"), "operation_id": firstString(row, "operation_id"), "observed": row["observed"], "evidence_ref": evidenceRef(result, "operations", index)})
			}
			for index, item := range array(result.Tables["event_channels"]) {
				row, ok := item.(map[string]any)
				if !ok {
					continue
				}
				rows = append(rows, map[string]any{"interface_type": "event_channel", "channel": firstString(row, "channel"), "direction": firstString(row, "direction"), "operation_id": firstString(row, "operation_id"), "observed": row["observed"], "evidence_ref": evidenceRef(result, "event_channels", index)})
			}
		}
	}
	return trimRows(rows, limit)
}

func runtimeRows(results []rawResult, limit int) []map[string]any {
	var rows []map[string]any
	for _, result := range results {
		for _, table := range []string{"service_dependencies", "matches", "critical_paths", "gaps"} {
			for index, item := range array(result.Tables[table]) {
				row, ok := item.(map[string]any)
				if !ok {
					continue
				}
				caller := normalizeName(firstString(row, "caller", "service"))
				callee := normalizeName(firstString(row, "callee", "target"))
				if caller == "" && callee == "" && table != "gaps" && table != "matches" {
					continue
				}
				rows = append(rows, map[string]any{
					"view":         runtimeViewName(result.Type, table),
					"source_type":  result.Type,
					"caller":       caller,
					"callee":       callee,
					"description":  runtimeDescription(result.Type, table, row),
					"evidence_ref": evidenceRef(result, table, index),
				})
			}
		}
	}
	return trimRows(rows, limit)
}

func deploymentRows(results []rawResult, limit int) []map[string]any {
	var rows []map[string]any
	for _, result := range results {
		if result.Type != "kubernetes_evidence" {
			continue
		}
		for index, item := range array(result.Tables["events"]) {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			rows = append(rows, map[string]any{
				"cluster":      firstString(row, "cluster"),
				"namespace":    firstString(row, "namespace"),
				"workload":     firstString(row, "workload", "pod"),
				"container":    firstString(row, "container"),
				"node":         firstString(row, "node"),
				"image":        firstString(row, "image"),
				"risk_hint":    firstString(row, "reason", "operation"),
				"evidence_ref": evidenceRef(result, "events", index),
			})
		}
	}
	return trimRows(rows, limit)
}

func qualityRequirementRows(results []rawResult, limit int) []map[string]any {
	var rows []map[string]any
	for _, result := range results {
		findings := array(result.Metadata["findings"])
		for index, item := range findings {
			finding, ok := item.(map[string]any)
			if !ok {
				continue
			}
			code := firstString(finding, "code")
			rows = append(rows, map[string]any{
				"category":     qualityCategory(code),
				"scenario":     qualityScenario(code, firstString(finding, "message")),
				"measure":      qualityMeasure(code),
				"source_type":  result.Type,
				"severity":     firstString(finding, "severity"),
				"evidence_ref": fmt.Sprintf("%s:metadata.findings[%d]", result.Type, index),
				"finding_code": code,
			})
		}
	}
	return trimRows(rows, limit)
}

func riskRows(results []rawResult, limit int) []map[string]any {
	var rows []map[string]any
	for _, result := range results {
		for index, item := range array(result.Metadata["findings"]) {
			finding, ok := item.(map[string]any)
			if !ok {
				continue
			}
			code := firstString(finding, "code")
			severity := firstString(finding, "severity")
			if severity == "" {
				severity = "info"
			}
			rows = append(rows, map[string]any{
				"risk_id":      fmt.Sprintf("R-%03d", len(rows)+1),
				"severity":     severity,
				"source_type":  result.Type,
				"finding_code": code,
				"description":  firstString(finding, "message"),
				"impact":       riskImpact(code),
				"mitigation":   riskMitigation(code),
				"evidence_ref": fmt.Sprintf("%s:metadata.findings[%d]", result.Type, index),
			})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return severityRank(fmt.Sprint(rows[i]["severity"])) > severityRank(fmt.Sprint(rows[j]["severity"]))
	})
	return trimRows(rows, limit)
}

func adrRows(results []rawResult, risks []map[string]any, quality []map[string]any, limit int) []map[string]any {
	var rows []map[string]any
	if len(risks) > 0 {
		top := risks[0]
		rows = append(rows, map[string]any{
			"title":         "Address highest-evidence architecture risk",
			"status":        "draft",
			"context":       fmt.Sprintf("Evidence finding %s indicates %s.", top["finding_code"], top["description"]),
			"decision":      "Define an explicit mitigation owner and measurable acceptance criteria.",
			"alternatives":  []string{"Accept risk temporarily", "Mitigate in current release", "Redesign affected boundary"},
			"tradeoffs":     "Immediate mitigation reduces incident risk but may compete with feature work.",
			"consequences":  "Architecture review receives a traceable risk decision linked to source evidence.",
			"evidence_refs": []string{fmt.Sprint(top["evidence_ref"])},
		})
	}
	if len(quality) > 0 {
		top := quality[0]
		rows = append(rows, map[string]any{
			"title":         "Make evidence-backed quality requirement explicit",
			"status":        "draft",
			"context":       fmt.Sprintf("%s quality scenario was derived from %s.", top["category"], top["finding_code"]),
			"decision":      "Capture the scenario as an architecture quality requirement with a measurable threshold.",
			"alternatives":  []string{"Keep implicit operational target", "Document as SLO only", "Promote to architecture requirement"},
			"tradeoffs":     "A documented requirement improves reviewability but needs ongoing evidence collection.",
			"consequences":  "Future incident reviews can compare evidence against the stated requirement.",
			"evidence_refs": []string{fmt.Sprint(top["evidence_ref"])},
		})
	}
	return trimRows(rows, limit)
}

func arc42Sections(services, interfaces, runtime, deployment, quality, risks, adrs []map[string]any) []map[string]any {
	return []map[string]any{
		{"section_id": "03", "title": "Context and Scope", "draft_markdown": fmt.Sprintf("System context currently references %d services and %d interfaces from imported evidence.", len(services), len(interfaces)), "evidence_tables": []string{"context_services", "interfaces"}},
		{"section_id": "06", "title": "Runtime View", "draft_markdown": fmt.Sprintf("Runtime view candidates contain %d evidence-backed interactions and gaps.", len(runtime)), "evidence_tables": []string{"runtime_views"}},
		{"section_id": "07", "title": "Deployment View", "draft_markdown": fmt.Sprintf("Deployment view candidates contain %d Kubernetes/container/cloud rows.", len(deployment)), "evidence_tables": []string{"deployment_views"}},
		{"section_id": "10", "title": "Quality Requirements", "draft_markdown": fmt.Sprintf("%d quality scenarios were derived from deterministic findings.", len(quality)), "evidence_tables": []string{"quality_requirements"}},
		{"section_id": "11", "title": "Risks and Technical Debt", "draft_markdown": fmt.Sprintf("%d risks and %d ADR drafts are ready for architecture review.", len(risks), len(adrs)), "evidence_tables": []string{"risks", "adr_drafts"}},
	}
}

func addFindings(result *models.AnalysisResult) {
	if asInt(result.Summary["risk_count"]) > 0 {
		result.AddFinding("warning", "ARCH_DOC_RISKS_PRESENT", "Architecture documentation package contains evidence-backed risks.", map[string]any{"risk_count": result.Summary["risk_count"]})
	}
	if asInt(result.Summary["adr_draft_count"]) > 0 {
		result.AddFinding("info", "ARCH_DOC_ADR_DRAFTS_READY", "ADR draft rows are available for architecture review.", map[string]any{"adr_draft_count": result.Summary["adr_draft_count"]})
	}
}

func evidenceRef(result rawResult, table string, index int) string {
	return fmt.Sprintf("%s:tables.%s[%d]", result.Type, table, index)
}

func sourceFiles(results []rawResult) []string {
	out := make([]string, 0, len(results))
	for _, result := range results {
		out = append(out, result.filePath)
	}
	return out
}

func resultTypes(results []rawResult) []string {
	out := make([]string, 0, len(results))
	for _, result := range results {
		out = append(out, result.Type)
	}
	return out
}

func resultTypeCounts(results []rawResult) map[string]int {
	counts := map[string]int{}
	for _, result := range results {
		counts[result.Type]++
	}
	return counts
}

func riskSeverityCounts(rows []map[string]any) map[string]int {
	counts := map[string]int{}
	for _, row := range rows {
		counts[fmt.Sprint(row["severity"])]++
	}
	return counts
}

func categoryCounts(rows []map[string]any) map[string]int {
	counts := map[string]int{}
	for _, row := range rows {
		counts[fmt.Sprint(row["category"])]++
	}
	return counts
}

func rowsFromCounts(counts map[string]int, key string, limit int) []map[string]any {
	type kv struct {
		key   string
		count int
	}
	items := make([]kv, 0, len(counts))
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
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, map[string]any{key: item.key, "count": item.count})
	}
	return rows
}

func runtimeViewName(resultType, table string) string {
	if resultType == "stitched_evidence" {
		return "Stitched evidence correlation"
	}
	if table == "service_dependencies" {
		return "Service interaction"
	}
	return strings.ReplaceAll(resultType+" "+table, "_", " ")
}

func runtimeDescription(resultType, table string, row map[string]any) string {
	if msg := firstString(row, "message", "description"); msg != "" {
		return msg
	}
	if caller := firstString(row, "caller"); caller != "" {
		return caller + " -> " + firstString(row, "callee", "target")
	}
	return runtimeViewName(resultType, table)
}

func responsibilityDraft(service string) string {
	switch {
	case strings.Contains(service, "database"), strings.HasPrefix(service, "db"):
		return "Persists and retrieves business data."
	case strings.Contains(service, "broker"), strings.Contains(service, "queue"):
		return "Mediates asynchronous events between services."
	case strings.Contains(service, "gateway"), strings.Contains(service, "edge"):
		return "Receives external traffic and routes requests to internal services."
	default:
		return "Application or platform component inferred from evidence."
	}
}

func qualityCategory(code string) string {
	code = strings.ToUpper(code)
	switch {
	case strings.Contains(code, "SLOW"), strings.Contains(code, "LATENCY"), strings.Contains(code, "PAUSE"):
		return "performance"
	case strings.Contains(code, "ERROR"), strings.Contains(code, "UNMATCHED"), strings.Contains(code, "MISSING"):
		return "reliability"
	case strings.Contains(code, "SECURITY"), strings.Contains(code, "AUTH"):
		return "security"
	case strings.Contains(code, "PRESSURE"), strings.Contains(code, "OOM"):
		return "operability"
	default:
		return "maintainability"
	}
}

func qualityScenario(code, message string) string {
	if message == "" {
		message = code
	}
	return "When " + strings.ToLower(strings.ReplaceAll(code, "_", " ")) + " occurs, the system must expose actionable evidence: " + message
}

func qualityMeasure(code string) string {
	switch qualityCategory(code) {
	case "performance":
		return "p95/p99 latency or sampled duration remains under the documented threshold."
	case "reliability":
		return "error, unmatched, or missing-correlation counts stay within the agreed budget."
	case "security":
		return "security-sensitive events are traceable to actor, operation, resource, and source evidence."
	default:
		return "Evidence is present, bounded, and linked to the affected component."
	}
}

func riskImpact(code string) string {
	switch qualityCategory(code) {
	case "performance":
		return "User-facing latency or resource saturation can violate service objectives."
	case "reliability":
		return "Incident reconstruction can miss causal links or affected dependencies."
	case "security":
		return "Security investigation may lack sufficient actor/resource traceability."
	default:
		return "Architecture review may miss a documented decision or operational constraint."
	}
}

func riskMitigation(code string) string {
	switch qualityCategory(code) {
	case "performance":
		return "Define a threshold, owner, and before/after evidence check for the affected path."
	case "reliability":
		return "Improve correlation coverage and add regression evidence for the affected flow."
	case "security":
		return "Verify audit-source coverage and preserve sanitized evidence references."
	default:
		return "Document the decision and keep evidence capture in the review checklist."
	}
}

func severityRank(severity string) int {
	switch strings.ToLower(severity) {
	case "critical":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func array(v any) []any {
	if out, ok := v.([]any); ok {
		return out
	}
	return nil
}

func firstString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := str(row[key]); value != "" {
			return value
		}
	}
	return ""
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func normalizeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "broker:")
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.Join(strings.Fields(value), "-")
	return strings.ToLower(value)
}

func trimRows(rows []map[string]any, limit int) []map[string]any {
	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}
