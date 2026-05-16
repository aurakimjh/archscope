package stitching

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const ResultType = "stitched_evidence"

type Options struct {
	TopN int
}

type rawResult struct {
	Type        string         `json:"type"`
	SourceFiles []string       `json:"source_files"`
	CreatedAt   string         `json:"created_at"`
	Summary     map[string]any `json:"summary"`
	Series      map[string]any `json:"series"`
	Tables      map[string]any `json:"tables"`
	Charts      map[string]any `json:"charts"`
	Metadata    map[string]any `json:"metadata"`
	filePath    string
}

type key struct {
	Kind  string
	Value string
}

type evidenceNode struct {
	ID          string
	ResultType  string
	ResultIndex int
	Table       string
	RowIndex    int
	Timestamp   string
	Service     string
	Target      string
	Message     string
	EvidenceRef string
	Keys        []key
	Row         map[string]any
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
	nodes := collectNodes(results, topN*20)
	matches, matchedIDs := buildMatches(nodes, topN)
	gaps := buildGaps(nodes, matchedIDs, topN)
	edges := serviceDependencies(matches, nodes, topN)

	result := models.New(ResultType, "evidence_stitching")
	result.SourceFiles = sourceFiles(results)
	result.Summary = map[string]any{
		"source_count":                  len(results),
		"evidence_node_count":           len(nodes),
		"correlation_key_count":         countCorrelationKeys(nodes),
		"matched_group_count":           len(matches),
		"gap_count":                     len(gaps),
		"missing_trace_id_count":        countGaps(gaps, "MISSING_TRACE_ID"),
		"dropped_parent_span_count":     countGaps(gaps, "DROPPED_PARENT_SPAN"),
		"unmatched_request_log_count":   countGaps(gaps, "UNMATCHED_REQUEST_LOG"),
		"unmatched_database_call_count": countGaps(gaps, "UNMATCHED_DATABASE_CALL"),
		"unmatched_broker_event_count":  countGaps(gaps, "UNMATCHED_BROKER_EVENT"),
		"service_dependency_count":      len(edges),
	}
	result.Series = map[string]any{
		"matches_by_key_kind":          rowsFromCounts(matchKeyKindCounts(matches), "key_kind", topN),
		"gaps_by_code":                 rowsFromCounts(gapCodeCounts(gaps), "code", topN),
		"source_type_distribution":     rowsFromCounts(sourceTypeCounts(nodes), "source_type", topN),
		"correlation_key_distribution": rowsFromCounts(correlationKindCounts(nodes), "key_kind", topN),
	}
	result.Tables = map[string]any{
		"matches":              matches,
		"gaps":                 gaps,
		"evidence_nodes":       nodeRows(nodes, topN),
		"service_dependencies": edges,
	}
	result.Metadata.SchemaVersion = "0.1.0"
	result.Metadata.Extra["input_result_types"] = resultTypes(results)
	result.Metadata.Extra["correlation_key_model"] = []string{"trace_id", "span_id", "parent_span_id", "request_id", "correlation_id", "txid", "tenant_id", "customer_id", "pod", "container", "host", "pid"}
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

func collectNodes(results []rawResult, limit int) []evidenceNode {
	var nodes []evidenceNode
	for resultIndex, result := range results {
		for table, value := range result.Tables {
			for rowIndex, item := range array(value) {
				row, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if !isRelevantTable(result.Type, table, row) {
					continue
				}
				node := evidenceNode{
					ID:          fmt.Sprintf("%s:%d:%s:%d", result.Type, resultIndex, table, rowIndex),
					ResultType:  result.Type,
					ResultIndex: resultIndex,
					Table:       table,
					RowIndex:    rowIndex,
					Timestamp:   firstValue(row, "timestamp", "time", "start_time", "startTime"),
					Service:     normalizeName(firstValue(row, "service", "service_name", "service.name", "caller", "application", "app")),
					Target:      normalizeName(firstValue(row, "callee", "target", "database", "broker", "upstream_service", "resource")),
					Message:     firstValue(row, "message", "uri", "path", "fingerprint", "query", "statement", "event_type", "span_name", "name"),
					EvidenceRef: fmt.Sprintf("tables.%s[%d]", table, rowIndex),
					Keys:        collectKeys(row),
					Row:         row,
				}
				nodes = append(nodes, node)
				if limit > 0 && len(nodes) >= limit {
					return nodes
				}
			}
		}
	}
	return nodes
}

func isRelevantTable(resultType, table string, row map[string]any) bool {
	if len(collectKeys(row)) > 0 {
		return true
	}
	switch resultType {
	case "access_log":
		return table == "sample_records" || table == "slow_requests" || table == "url_stats"
	case "database_slow_query":
		return table == "queries" || table == "events" || table == "service_dependencies"
	case "broker_log":
		return table == "events" || table == "service_dependencies"
	case "trace_import":
		return table == "spans" || table == "service_dependencies" || table == "critical_paths"
	default:
		return false
	}
}

func buildMatches(nodes []evidenceNode, limit int) ([]map[string]any, map[string]bool) {
	buckets := map[string][]evidenceNode{}
	for _, node := range nodes {
		for _, key := range node.Keys {
			buckets[key.Kind+"\x00"+key.Value] = append(buckets[key.Kind+"\x00"+key.Value], node)
		}
	}
	type item struct {
		key   key
		nodes []evidenceNode
	}
	var items []item
	for bucketKey, bucket := range buckets {
		sourceTypes := map[string]bool{}
		for _, node := range bucket {
			sourceTypes[node.ResultType] = true
		}
		if len(sourceTypes) < 2 {
			continue
		}
		parts := strings.SplitN(bucketKey, "\x00", 2)
		items = append(items, item{key: key{Kind: parts[0], Value: parts[1]}, nodes: bucket})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if len(items[i].nodes) != len(items[j].nodes) {
			return len(items[i].nodes) > len(items[j].nodes)
		}
		if items[i].key.Kind != items[j].key.Kind {
			return items[i].key.Kind < items[j].key.Kind
		}
		return items[i].key.Value < items[j].key.Value
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	matchedIDs := map[string]bool{}
	rows := make([]map[string]any, 0, len(items))
	for index, item := range items {
		for _, node := range item.nodes {
			matchedIDs[node.ID] = true
		}
		rows = append(rows, map[string]any{
			"id":            fmt.Sprintf("match-%d", index+1),
			"key_kind":      item.key.Kind,
			"key_value":     item.key.Value,
			"event_count":   len(item.nodes),
			"source_types":  uniqueNodeTypes(item.nodes),
			"evidence_refs": evidenceRefs(item.nodes),
			"first_seen":    minTimestamp(item.nodes),
			"last_seen":     maxTimestamp(item.nodes),
			"services":      uniqueNodeServices(item.nodes),
		})
	}
	return rows, matchedIDs
}

func buildGaps(nodes []evidenceNode, matchedIDs map[string]bool, limit int) []map[string]any {
	spanIDs := map[string]bool{}
	for _, node := range nodes {
		for _, key := range node.Keys {
			if key.Kind == "span_id" {
				spanIDs[key.Value] = true
			}
		}
	}
	var gaps []map[string]any
	addGap := func(code, severity, message string, node evidenceNode, extra map[string]any) {
		row := map[string]any{
			"code":           code,
			"severity":       severity,
			"message":        message,
			"source_type":    node.ResultType,
			"evidence_ref":   node.EvidenceRef,
			"timestamp":      node.Timestamp,
			"service":        node.Service,
			"correlation":    keysAsRows(node.Keys),
			"source_node_id": node.ID,
		}
		for k, v := range extra {
			row[k] = v
		}
		gaps = append(gaps, row)
	}
	for _, node := range nodes {
		if node.ResultType == "trace_import" {
			for _, key := range node.Keys {
				if key.Kind == "parent_span_id" && key.Value != "" && !spanIDs[key.Value] {
					addGap("DROPPED_PARENT_SPAN", "warning", "Span references a parent span that is not present in the imported evidence.", node, map[string]any{"parent_span_id": key.Value})
				}
			}
		}
		if node.ResultType == "access_log" && !hasKey(node.Keys, "trace_id") {
			addGap("MISSING_TRACE_ID", "warning", "Request log row has no trace ID for cross-source stitching.", node, nil)
			continue
		}
		if node.ResultType == "access_log" && len(node.Keys) > 0 && !matchedIDs[node.ID] {
			addGap("UNMATCHED_REQUEST_LOG", "warning", "Request log correlation key did not match trace, database, broker, or runtime evidence.", node, nil)
		}
		if node.ResultType == "database_slow_query" && !matchedIDs[node.ID] {
			addGap("UNMATCHED_DATABASE_CALL", "warning", "Database evidence did not match a request or trace correlation key.", node, nil)
		}
		if node.ResultType == "broker_log" && !matchedIDs[node.ID] {
			addGap("UNMATCHED_BROKER_EVENT", "warning", "Broker evidence did not match a request or trace correlation key.", node, nil)
		}
		if limit > 0 && len(gaps) >= limit {
			return gaps
		}
	}
	return gaps
}

func serviceDependencies(matches []map[string]any, nodes []evidenceNode, limit int) []map[string]any {
	nodesByRef := map[string]evidenceNode{}
	for _, node := range nodes {
		nodesByRef[node.EvidenceRef] = node
	}
	type edge struct {
		caller, callee string
		callCount      int
		refs           []string
		keyKinds       []string
	}
	edges := map[string]*edge{}
	for _, match := range matches {
		refs := stringSlice(match["evidence_refs"])
		var caller evidenceNode
		var targets []evidenceNode
		for _, ref := range refs {
			node := nodesByRef[ref]
			if node.ResultType == "database_slow_query" || node.ResultType == "broker_log" {
				targets = append(targets, node)
				continue
			}
			if caller.ID == "" {
				caller = node
			}
		}
		if caller.ID == "" {
			caller = evidenceNode{Service: "application"}
		}
		for _, target := range targets {
			callee := target.Target
			if callee == "" {
				callee = map[bool]string{true: "database", false: "broker"}[target.ResultType == "database_slow_query"]
			}
			key := normalizeName(firstNonEmpty(caller.Service, "application")) + "\x00" + normalizeName(callee)
			item := edges[key]
			if item == nil {
				item = &edge{caller: normalizeName(firstNonEmpty(caller.Service, "application")), callee: normalizeName(callee)}
				edges[key] = item
			}
			item.callCount++
			item.refs = append(item.refs, refs...)
			item.keyKinds = append(item.keyKinds, str(match["key_kind"]))
		}
	}
	rows := make([]map[string]any, 0, len(edges))
	for _, item := range edges {
		rows = append(rows, map[string]any{
			"caller":        item.caller,
			"callee":        item.callee,
			"call_count":    item.callCount,
			"error_count":   0,
			"error_rate":    0,
			"match_status":  "stitched",
			"evidence_refs": uniqueStrings(item.refs),
			"key_kinds":     uniqueStrings(item.keyKinds),
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return asInt(rows[i]["call_count"]) > asInt(rows[j]["call_count"])
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

func collectKeys(row map[string]any) []key {
	aliases := map[string][]string{
		"trace_id":       {"trace_id", "traceId", "trace.id", "traceID"},
		"span_id":        {"span_id", "spanId", "span.id", "spanID"},
		"parent_span_id": {"parent_span_id", "parentSpanId", "parent.span_id"},
		"request_id":     {"request_id", "requestId", "x_request_id", "req_id"},
		"correlation_id": {"correlation_id", "correlationId", "corr_id"},
		"txid":           {"txid", "transaction_id", "guid"},
		"tenant_id":      {"tenant_id", "tenantId"},
		"customer_id":    {"customer_id", "customerId"},
		"pod":            {"pod", "pod_name", "podName"},
		"container":      {"container", "container_id", "containerId"},
		"host":           {"host", "hostname", "node"},
		"pid":            {"pid", "process_id"},
	}
	var keys []key
	seen := map[string]bool{}
	for kind, names := range aliases {
		for _, name := range names {
			value := firstValue(row, name)
			if value == "" {
				continue
			}
			id := kind + "\x00" + value
			if !seen[id] {
				keys = append(keys, key{Kind: kind, Value: value})
				seen[id] = true
			}
		}
	}
	return keys
}

func addFindings(result *models.AnalysisResult) {
	checks := []struct {
		key, code, msg string
	}{
		{"missing_trace_id_count", "MISSING_TRACE_ID", "Request log rows are missing trace IDs."},
		{"dropped_parent_span_count", "DROPPED_PARENT_SPAN", "Trace spans reference missing parents."},
		{"unmatched_request_log_count", "UNMATCHED_REQUEST_LOG", "Request logs did not match other evidence."},
		{"unmatched_database_call_count", "UNMATCHED_DATABASE_CALL", "Database evidence did not match request or trace evidence."},
		{"unmatched_broker_event_count", "UNMATCHED_BROKER_EVENT", "Broker evidence did not match request or trace evidence."},
	}
	for _, check := range checks {
		if asInt(result.Summary[check.key]) > 0 {
			result.AddFinding("warning", check.code, check.msg, map[string]any{check.key: result.Summary[check.key]})
		}
	}
}

func nodeRows(nodes []evidenceNode, limit int) []map[string]any {
	if limit > 0 && len(nodes) > limit {
		nodes = nodes[:limit]
	}
	rows := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		rows = append(rows, map[string]any{
			"id":           node.ID,
			"source_type":  node.ResultType,
			"table":        node.Table,
			"timestamp":    node.Timestamp,
			"service":      node.Service,
			"target":       node.Target,
			"message":      node.Message,
			"evidence_ref": node.EvidenceRef,
			"correlation":  keysAsRows(node.Keys),
		})
	}
	return rows
}

func sourceFiles(results []rawResult) []string {
	var out []string
	for _, result := range results {
		out = append(out, result.filePath)
	}
	return out
}

func resultTypes(results []rawResult) []string {
	var values []string
	for _, result := range results {
		values = append(values, result.Type)
	}
	return uniqueStrings(values)
}

func countCorrelationKeys(nodes []evidenceNode) int {
	seen := map[string]bool{}
	for _, node := range nodes {
		for _, key := range node.Keys {
			seen[key.Kind+"\x00"+key.Value] = true
		}
	}
	return len(seen)
}

func matchKeyKindCounts(rows []map[string]any) map[string]int {
	counts := map[string]int{}
	for _, row := range rows {
		counts[str(row["key_kind"])]++
	}
	return counts
}

func gapCodeCounts(rows []map[string]any) map[string]int {
	counts := map[string]int{}
	for _, row := range rows {
		counts[str(row["code"])]++
	}
	return counts
}

func sourceTypeCounts(nodes []evidenceNode) map[string]int {
	counts := map[string]int{}
	for _, node := range nodes {
		counts[node.ResultType]++
	}
	return counts
}

func correlationKindCounts(nodes []evidenceNode) map[string]int {
	counts := map[string]int{}
	for _, node := range nodes {
		for _, key := range node.Keys {
			counts[key.Kind]++
		}
	}
	return counts
}

func countGaps(rows []map[string]any, code string) int {
	count := 0
	for _, row := range rows {
		if row["code"] == code {
			count++
		}
	}
	return count
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

func uniqueNodeTypes(nodes []evidenceNode) []string {
	values := make([]string, 0, len(nodes))
	for _, node := range nodes {
		values = append(values, node.ResultType)
	}
	return uniqueStrings(values)
}

func uniqueNodeServices(nodes []evidenceNode) []string {
	values := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if node.Service != "" {
			values = append(values, node.Service)
		}
	}
	return uniqueStrings(values)
}

func evidenceRefs(nodes []evidenceNode) []string {
	values := make([]string, 0, len(nodes))
	for _, node := range nodes {
		values = append(values, node.EvidenceRef)
	}
	return values
}

func minTimestamp(nodes []evidenceNode) string {
	best := ""
	for _, node := range nodes {
		if node.Timestamp != "" && (best == "" || node.Timestamp < best) {
			best = node.Timestamp
		}
	}
	return best
}

func maxTimestamp(nodes []evidenceNode) string {
	best := ""
	for _, node := range nodes {
		if node.Timestamp != "" && node.Timestamp > best {
			best = node.Timestamp
		}
	}
	return best
}

func keysAsRows(keys []key) []map[string]any {
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, map[string]any{"kind": key.Kind, "value": key.Value})
	}
	return rows
}

func hasKey(keys []key, kind string) bool {
	for _, key := range keys {
		if key.Kind == kind {
			return true
		}
	}
	return false
}

func firstValue(row map[string]any, names ...string) string {
	for _, name := range names {
		if value := str(row[name]); value != "" {
			return value
		}
	}
	return ""
}

func array(v any) []any {
	if out, ok := v.([]any); ok {
		return out
	}
	return nil
}

func stringSlice(v any) []string {
	switch values := v.(type) {
	case []string:
		return values
	case []any:
		out := make([]string, 0, len(values))
		for _, item := range values {
			if value := str(item); value != "" {
				out = append(out, value)
			}
		}
		return out
	default:
		return nil
	}
}

func str(v any) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return value.String()
	case float64:
		if value == float64(int64(value)) {
			return fmt.Sprintf("%d", int64(value))
		}
		return fmt.Sprintf("%v", value)
	default:
		return ""
	}
}

func asInt(v any) int {
	switch value := v.(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
}

func normalizeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.Join(strings.Fields(value), "-")
	return strings.ToLower(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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
