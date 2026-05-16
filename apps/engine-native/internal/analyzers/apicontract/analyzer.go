package apicontract

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/apicontract"
)

const ResultType = "api_contract_analysis"

type Options struct {
	OpenAPIPath        string
	AccessResultPath   string
	AsyncAPIPath       string
	BrokerResultPath   string
	TopN               int
	SlowThresholdMS    float64
	ErrorRateThreshold float64
}

type rawResult struct {
	Type        string         `json:"type"`
	SourceFiles []string       `json:"source_files"`
	Summary     map[string]any `json:"summary"`
	Series      map[string]any `json:"series"`
	Tables      map[string]any `json:"tables"`
	Metadata    map[string]any `json:"metadata"`
}

type observedRoute struct {
	Method       string
	Path         string
	Template     string
	Count        int
	AvgMS        float64
	P95MS        float64
	P99MS        float64
	ErrorCount   int
	ErrorRatePct float64
	EvidenceRef  string
}

type observedChannel struct {
	Name        string
	Kind        string
	Count       int
	ErrorCount  int
	EvidenceRef string
}

func Analyze(opts Options) (models.AnalysisResult, error) {
	topN := opts.TopN
	if topN <= 0 {
		topN = 100
	}
	slowThreshold := opts.SlowThresholdMS
	if slowThreshold <= 0 {
		slowThreshold = 1000
	}
	errorThreshold := opts.ErrorRateThreshold
	if errorThreshold <= 0 {
		errorThreshold = 5
	}

	var openapi parser.OpenAPIContract
	var asyncapi parser.AsyncAPIContract
	var diags []*diagnostics.ParserDiagnostics
	var sources []string
	var err error
	if strings.TrimSpace(opts.OpenAPIPath) != "" {
		openapi, err = parseOpenAPI(opts.OpenAPIPath)
		if err != nil {
			return models.AnalysisResult{}, err
		}
		if _, d, parseErr := parser.ParseOpenAPIFile(opts.OpenAPIPath, parser.Options{}); parseErr == nil {
			diags = append(diags, d)
		}
		sources = append(sources, opts.OpenAPIPath)
	}
	if strings.TrimSpace(opts.AsyncAPIPath) != "" {
		asyncapi, err = parseAsyncAPI(opts.AsyncAPIPath)
		if err != nil {
			return models.AnalysisResult{}, err
		}
		if _, d, parseErr := parser.ParseAsyncAPIFile(opts.AsyncAPIPath, parser.Options{}); parseErr == nil {
			diags = append(diags, d)
		}
		sources = append(sources, opts.AsyncAPIPath)
	}
	accessRoutes := []observedRoute{}
	if strings.TrimSpace(opts.AccessResultPath) != "" {
		result, readErr := readResult(opts.AccessResultPath)
		if readErr != nil {
			return models.AnalysisResult{}, readErr
		}
		accessRoutes = extractAccessRoutes(result, topN*20)
		sources = append(sources, opts.AccessResultPath)
	}
	brokerChannels := []observedChannel{}
	if strings.TrimSpace(opts.BrokerResultPath) != "" {
		result, readErr := readResult(opts.BrokerResultPath)
		if readErr != nil {
			return models.AnalysisResult{}, readErr
		}
		brokerChannels = extractBrokerChannels(result, topN*20)
		sources = append(sources, opts.BrokerResultPath)
	}
	return Build(openapi, accessRoutes, asyncapi, brokerChannels, sources, diags, Options{TopN: topN, SlowThresholdMS: slowThreshold, ErrorRateThreshold: errorThreshold}), nil
}

func Build(openapi parser.OpenAPIContract, accessRoutes []observedRoute, asyncapi parser.AsyncAPIContract, brokerChannels []observedChannel, sources []string, diags []*diagnostics.ParserDiagnostics, opts Options) models.AnalysisResult {
	topN := opts.TopN
	if topN <= 0 {
		topN = 100
	}
	slowThreshold := opts.SlowThresholdMS
	if slowThreshold <= 0 {
		slowThreshold = 1000
	}
	errorThreshold := opts.ErrorRateThreshold
	if errorThreshold <= 0 {
		errorThreshold = 5
	}
	operationRows, undocumented, unused, slow, highError := compareHTTP(openapi.Operations, accessRoutes, slowThreshold, errorThreshold, topN)
	channelRows, undocumentedChannels, unusedChannels := compareChannels(asyncapi.Channels, brokerChannels, topN)

	result := models.New(ResultType, "api_contract_analysis")
	result.SourceFiles = sources
	result.Summary = map[string]any{
		"openapi_operation_count":          len(openapi.Operations),
		"observed_route_count":             len(accessRoutes),
		"matched_operation_count":          countMatchedOperations(operationRows),
		"undocumented_route_count":         len(undocumented),
		"unused_operation_count":           len(unused),
		"slow_operation_count":             len(slow),
		"high_error_operation_count":       len(highError),
		"asyncapi_channel_count":           len(asyncapi.Channels),
		"observed_broker_channel_count":    len(brokerChannels),
		"undocumented_event_channel_count": len(undocumentedChannels),
		"unused_event_channel_count":       len(unusedChannels),
	}
	result.Series = map[string]any{
		"operation_coverage": []map[string]any{
			{"status": "matched", "count": countMatchedOperations(operationRows)},
			{"status": "unused", "count": len(unused)},
			{"status": "undocumented", "count": len(undocumented)},
		},
		"event_channel_coverage": []map[string]any{
			{"status": "matched", "count": countMatchedChannels(channelRows)},
			{"status": "unused", "count": len(unusedChannels)},
			{"status": "undocumented", "count": len(undocumentedChannels)},
		},
	}
	result.Tables = map[string]any{
		"operations":                  operationRows,
		"observed_routes":             routeRows(accessRoutes, topN),
		"undocumented_routes":         undocumented,
		"unused_operations":           unused,
		"slow_operations":             slow,
		"high_error_operations":       highError,
		"event_channels":              channelRows,
		"observed_event_channels":     channelObservationRows(brokerChannels, topN),
		"undocumented_event_channels": undocumentedChannels,
		"unused_event_channels":       unusedChannels,
	}
	result.Metadata.SchemaVersion = "0.1.0"
	result.Metadata.Extra["openapi"] = map[string]any{"title": openapi.Title, "version": openapi.Version}
	result.Metadata.Extra["asyncapi"] = map[string]any{"title": asyncapi.Title, "version": asyncapi.Version}
	result.Metadata.Extra["thresholds"] = map[string]any{"slow_ms": slowThreshold, "error_rate_percent": errorThreshold}
	result.Metadata.Extra["diagnostics"] = diags
	addFindings(&result)
	return result
}

func parseOpenAPI(path string) (parser.OpenAPIContract, error) {
	contract, _, err := parser.ParseOpenAPIFile(path, parser.Options{})
	if err != nil {
		return parser.OpenAPIContract{}, err
	}
	return contract, nil
}

func parseAsyncAPI(path string) (parser.AsyncAPIContract, error) {
	contract, _, err := parser.ParseAsyncAPIFile(path, parser.Options{})
	if err != nil {
		return parser.AsyncAPIContract{}, err
	}
	return contract, nil
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
	return result, nil
}

func compareHTTP(operations []parser.Operation, observed []observedRoute, slowThreshold, errorThreshold float64, limit int) ([]map[string]any, []map[string]any, []map[string]any, []map[string]any, []map[string]any) {
	matchedObserved := map[int]bool{}
	matchedOperation := map[int]bool{}
	var rows []map[string]any
	for opIndex, op := range operations {
		var matches []observedRoute
		for obsIndex, route := range observed {
			if operationMatches(op, route) {
				matches = append(matches, route)
				matchedObserved[obsIndex] = true
				matchedOperation[opIndex] = true
			}
		}
		rows = append(rows, map[string]any{
			"method":         op.Method,
			"path":           op.Path,
			"operation_id":   op.OperationID,
			"summary":        op.Summary,
			"tags":           op.Tags,
			"observed":       len(matches) > 0,
			"observed_count": sumRouteCount(matches),
			"max_p95_ms":     maxRouteP95(matches),
			"max_error_rate": maxRouteErrorRate(matches),
			"matched_routes": routeRefs(matches),
		})
	}
	var undocumented []map[string]any
	for index, route := range observed {
		if !matchedObserved[index] {
			undocumented = append(undocumented, routeRow(route, "undocumented"))
		}
	}
	var unused []map[string]any
	for index, op := range operations {
		if !matchedOperation[index] {
			unused = append(unused, map[string]any{"method": op.Method, "path": op.Path, "operation_id": op.OperationID, "summary": op.Summary, "tags": op.Tags})
		}
	}
	var slow []map[string]any
	var highError []map[string]any
	for _, route := range observed {
		if maxFloat(route.P95MS, route.AvgMS, route.P99MS) >= slowThreshold {
			slow = append(slow, routeRow(route, "slow"))
		}
		if route.ErrorRatePct >= errorThreshold || route.ErrorCount > 0 && route.Count > 0 && float64(route.ErrorCount)*100/float64(route.Count) >= errorThreshold {
			highError = append(highError, routeRow(route, "high_error"))
		}
	}
	sortRouteRows(undocumented)
	sortOperationRows(unused)
	sortRouteRows(slow)
	sortRouteRows(highError)
	return rows, trimRows(undocumented, limit), trimRows(unused, limit), trimRows(slow, limit), trimRows(highError, limit)
}

func compareChannels(spec []parser.Channel, observed []observedChannel, limit int) ([]map[string]any, []map[string]any, []map[string]any) {
	matchedObserved := map[int]bool{}
	matchedSpec := map[int]bool{}
	var rows []map[string]any
	for specIndex, channel := range spec {
		var matches []observedChannel
		for obsIndex, obs := range observed {
			if channelMatches(channel.Name, obs.Name) {
				matches = append(matches, obs)
				matchedObserved[obsIndex] = true
				matchedSpec[specIndex] = true
			}
		}
		rows = append(rows, map[string]any{
			"channel":        channel.Name,
			"direction":      channel.Direction,
			"operation_id":   channel.OperationID,
			"summary":        channel.Summary,
			"message_names":  channel.MessageNames,
			"observed":       len(matches) > 0,
			"observed_count": sumChannelCount(matches),
			"matched_events": channelRefs(matches),
		})
	}
	var undocumented []map[string]any
	for index, obs := range observed {
		if !matchedObserved[index] {
			undocumented = append(undocumented, channelObservationRow(obs, "undocumented"))
		}
	}
	var unused []map[string]any
	for index, channel := range spec {
		if !matchedSpec[index] {
			unused = append(unused, map[string]any{"channel": channel.Name, "direction": channel.Direction, "operation_id": channel.OperationID, "summary": channel.Summary, "message_names": channel.MessageNames})
		}
	}
	sortChannelRows(undocumented)
	sortChannelRows(unused)
	return rows, trimRows(undocumented, limit), trimRows(unused, limit)
}

func extractAccessRoutes(result rawResult, limit int) []observedRoute {
	var routes []observedRoute
	addRoute := func(row map[string]any, table string, index int) {
		path := firstString(row, "uri", "path", "route", "endpoint")
		if path == "" {
			return
		}
		method := strings.ToUpper(firstString(row, "method", "http_method"))
		if method == "" {
			method = "ANY"
		}
		count := firstInt(row, "count", "request_count", "total_requests")
		if count <= 0 {
			count = 1
		}
		routes = append(routes, observedRoute{
			Method:       method,
			Path:         path,
			Template:     observedTemplate(path),
			Count:        count,
			AvgMS:        firstFloat(row, "avg_response_ms", "avg_duration_ms", "duration_ms"),
			P95MS:        firstFloat(row, "p95_response_ms", "p95_duration_ms"),
			P99MS:        firstFloat(row, "p99_response_ms", "p99_duration_ms"),
			ErrorCount:   firstInt(row, "error_count", "errors"),
			ErrorRatePct: normalizeErrorRate(firstFloat(row, "error_rate")),
			EvidenceRef:  fmt.Sprintf("tables.%s[%d]", table, index),
		})
	}
	for _, table := range []string{"url_stats", "route_stats", "sample_records", "slow_requests"} {
		for index, item := range array(result.Tables[table]) {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			addRoute(row, table, index)
			if limit > 0 && len(routes) >= limit {
				return routes
			}
		}
	}
	return routes
}

func extractBrokerChannels(result rawResult, limit int) []observedChannel {
	var channels []observedChannel
	addChannel := func(row map[string]any, table string, index int) {
		name := firstString(row, "topic", "queue", "channel", "subject", "destination", "resource", "callee")
		name = normalizeBrokerName(name)
		if name == "" {
			return
		}
		count := firstInt(row, "count", "call_count", "event_count")
		if count <= 0 {
			count = 1
		}
		channels = append(channels, observedChannel{Name: name, Kind: firstString(row, "event_type", "kind", "broker"), Count: count, ErrorCount: firstInt(row, "error_count"), EvidenceRef: fmt.Sprintf("tables.%s[%d]", table, index)})
	}
	for _, table := range []string{"events", "service_dependencies"} {
		for index, item := range array(result.Tables[table]) {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			addChannel(row, table, index)
			if limit > 0 && len(channels) >= limit {
				return channels
			}
		}
	}
	return channels
}

func operationMatches(op parser.Operation, route observedRoute) bool {
	if route.Method != "ANY" && op.Method != route.Method {
		return false
	}
	if op.Path == route.Path || op.Path == route.Template {
		return true
	}
	return pathTemplateRegex(op.Path).MatchString(route.Path)
}

func channelMatches(specName, observedName string) bool {
	specName = normalizeChannel(specName)
	observedName = normalizeChannel(observedName)
	if specName == observedName {
		return true
	}
	pattern := regexp.QuoteMeta(specName)
	pattern = strings.ReplaceAll(pattern, "\\{", "{")
	pattern = strings.ReplaceAll(pattern, "\\}", "}")
	pattern = regexp.MustCompile(`\{[^/]+\}`).ReplaceAllString(pattern, `[^/]+`)
	return regexp.MustCompile("^" + pattern + "$").MatchString(observedName)
}

func pathTemplateRegex(path string) *regexp.Regexp {
	pattern := regexp.QuoteMeta(path)
	pattern = strings.ReplaceAll(pattern, "\\{", "{")
	pattern = strings.ReplaceAll(pattern, "\\}", "}")
	pattern = regexp.MustCompile(`\{[^/]+\}`).ReplaceAllString(pattern, `[^/]+`)
	return regexp.MustCompile("^" + pattern + "$")
}

func observedTemplate(path string) string {
	parts := strings.Split(strings.TrimSpace(path), "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if isLikelyID(part) {
			parts[i] = "{id}"
		}
	}
	return strings.Join(parts, "/")
}

func isLikelyID(value string) bool {
	if regexp.MustCompile(`^[0-9]+$`).MatchString(value) {
		return true
	}
	if regexp.MustCompile(`^[0-9a-fA-F-]{16,}$`).MatchString(value) {
		return true
	}
	return false
}

func addFindings(result *models.AnalysisResult) {
	checks := []struct {
		key, code, msg, sev string
	}{
		{"undocumented_route_count", "UNDOCUMENTED_API_ROUTE", "Observed access-log routes are not documented by OpenAPI.", "warning"},
		{"unused_operation_count", "UNUSED_API_OPERATION", "OpenAPI operations were not observed in access-log evidence.", "info"},
		{"slow_operation_count", "SLOW_API_OPERATION", "Observed API routes exceed the configured slow-operation threshold.", "warning"},
		{"high_error_operation_count", "HIGH_ERROR_API_OPERATION", "Observed API routes exceed the configured error-rate threshold.", "critical"},
		{"undocumented_event_channel_count", "UNDOCUMENTED_EVENT_CHANNEL", "Observed broker channels are not documented by AsyncAPI.", "warning"},
		{"unused_event_channel_count", "UNUSED_EVENT_CHANNEL", "AsyncAPI channels were not observed in broker evidence.", "info"},
	}
	for _, check := range checks {
		if asInt(result.Summary[check.key]) > 0 {
			result.AddFinding(check.sev, check.code, check.msg, map[string]any{check.key: result.Summary[check.key]})
		}
	}
}

func routeRows(routes []observedRoute, limit int) []map[string]any {
	rows := make([]map[string]any, 0, len(routes))
	for _, route := range routes {
		rows = append(rows, routeRow(route, "observed"))
	}
	sortRouteRows(rows)
	return trimRows(rows, limit)
}

func routeRow(route observedRoute, status string) map[string]any {
	return map[string]any{"method": route.Method, "path": route.Path, "normalized_path": route.Template, "status": status, "count": route.Count, "avg_ms": route.AvgMS, "p95_ms": route.P95MS, "p99_ms": route.P99MS, "error_count": route.ErrorCount, "error_rate_percent": route.ErrorRatePct, "evidence_ref": route.EvidenceRef}
}

func channelObservationRows(channels []observedChannel, limit int) []map[string]any {
	rows := make([]map[string]any, 0, len(channels))
	for _, channel := range channels {
		rows = append(rows, channelObservationRow(channel, "observed"))
	}
	sortChannelRows(rows)
	return trimRows(rows, limit)
}

func channelObservationRow(channel observedChannel, status string) map[string]any {
	return map[string]any{"channel": channel.Name, "kind": channel.Kind, "status": status, "count": channel.Count, "error_count": channel.ErrorCount, "evidence_ref": channel.EvidenceRef}
}

func countMatchedOperations(rows []map[string]any) int {
	count := 0
	for _, row := range rows {
		if row["observed"] == true {
			count++
		}
	}
	return count
}

func countMatchedChannels(rows []map[string]any) int {
	count := 0
	for _, row := range rows {
		if row["observed"] == true {
			count++
		}
	}
	return count
}

func sumRouteCount(routes []observedRoute) int {
	total := 0
	for _, route := range routes {
		total += route.Count
	}
	return total
}

func maxRouteP95(routes []observedRoute) float64 {
	maxValue := 0.0
	for _, route := range routes {
		maxValue = maxFloat(maxValue, route.P95MS, route.AvgMS, route.P99MS)
	}
	return maxValue
}

func maxRouteErrorRate(routes []observedRoute) float64 {
	maxValue := 0.0
	for _, route := range routes {
		if route.ErrorRatePct > maxValue {
			maxValue = route.ErrorRatePct
		}
	}
	return maxValue
}

func sumChannelCount(channels []observedChannel) int {
	total := 0
	for _, channel := range channels {
		total += channel.Count
	}
	return total
}

func routeRefs(routes []observedRoute) []string {
	refs := make([]string, 0, len(routes))
	for _, route := range routes {
		refs = append(refs, route.EvidenceRef)
	}
	return refs
}

func channelRefs(channels []observedChannel) []string {
	refs := make([]string, 0, len(channels))
	for _, channel := range channels {
		refs = append(refs, channel.EvidenceRef)
	}
	return refs
}

func sortRouteRows(rows []map[string]any) {
	sort.SliceStable(rows, func(i, j int) bool {
		if asFloat(rows[i]["p95_ms"]) != asFloat(rows[j]["p95_ms"]) {
			return asFloat(rows[i]["p95_ms"]) > asFloat(rows[j]["p95_ms"])
		}
		return fmt.Sprint(rows[i]["path"]) < fmt.Sprint(rows[j]["path"])
	})
}

func sortOperationRows(rows []map[string]any) {
	sort.SliceStable(rows, func(i, j int) bool {
		if fmt.Sprint(rows[i]["path"]) != fmt.Sprint(rows[j]["path"]) {
			return fmt.Sprint(rows[i]["path"]) < fmt.Sprint(rows[j]["path"])
		}
		return fmt.Sprint(rows[i]["method"]) < fmt.Sprint(rows[j]["method"])
	})
}

func sortChannelRows(rows []map[string]any) {
	sort.SliceStable(rows, func(i, j int) bool {
		return fmt.Sprint(rows[i]["channel"]) < fmt.Sprint(rows[j]["channel"])
	})
}

func trimRows(rows []map[string]any, limit int) []map[string]any {
	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func normalizeErrorRate(value float64) float64 {
	if value <= 1 {
		return value * 100
	}
	return value
}

func normalizeBrokerName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "broker:")
	return value
}

func normalizeChannel(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "broker:")
	value = strings.Trim(value, "/")
	return value
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

func firstFloat(row map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if value := asFloat(row[key]); value != 0 {
			return value
		}
	}
	return 0
}

func firstInt(row map[string]any, keys ...string) int {
	for _, key := range keys {
		if value := asInt(row[key]); value != 0 {
			return value
		}
	}
	return 0
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

func maxFloat(values ...float64) float64 {
	maxValue := 0.0
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}
