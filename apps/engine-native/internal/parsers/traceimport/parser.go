// Package traceimport parses portable trace export files into a small
// canonical span model. It intentionally starts with local-first formats:
// OpenTelemetry OTLP JSON files, Zipkin v2 JSON, Elastic APM JSON exports,
// Jaeger QueryService JSON, and guarded SkyWalking GraphQL trace responses.
package traceimport

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

const (
	FormatAuto                = "auto"
	FormatOTLPJSON            = "otlp-json"
	FormatZipkinV2            = "zipkin-v2-json"
	FormatElasticSearchJSON   = "elastic-apm-search-json"
	FormatElasticSourceNDJSON = "elastic-apm-source-ndjson"
	FormatJaegerQueryJSON     = "jaeger-query-json"
	FormatSkyWalkingGraphQL   = "skywalking-graphql-json"

	ReasonInvalidJSON         = "INVALID_TRACE_JSON"
	ReasonUnsupportedFormat   = "UNSUPPORTED_TRACE_FORMAT"
	ReasonInvalidOTLPEnvelope = "INVALID_OTLP_JSON_ENVELOPE"
	ReasonInvalidZipkinSpan   = "INVALID_ZIPKIN_V2_SPAN"
	ReasonInvalidElasticEvent = "INVALID_ELASTIC_APM_EVENT"
	ReasonInvalidJaegerTrace  = "INVALID_JAEGER_TRACE"
	ReasonInvalidSkyWalking   = "INVALID_SKYWALKING_TRACE"
	ReasonSkyWalkingSchema    = "UNSUPPORTED_SKYWALKING_SCHEMA"
	ReasonSkyWalkingVersion   = "SKYWALKING_VERSION_UNVERIFIED"
)

type Options struct {
	Format string
}

type ParseResult struct {
	Format      string
	Spans       []Span
	Diagnostics *diagnostics.ParserDiagnostics
}

type Span struct {
	TraceID        string
	SpanID         string
	ParentSpanID   string
	Name           string
	ServiceName    string
	RemoteService  string
	Kind           string
	StartUnixNanos int64
	DurationNanos  int64
	StatusCode     string
	Error          bool
	Attributes     map[string]string
	SourceFormat   string
}

func ParseFile(path string, opts Options) (ParseResult, error) {
	format := strings.TrimSpace(opts.Format)
	if format == "" {
		format = FormatAuto
	}
	if format == FormatAuto {
		detected, err := detectFormat(path)
		if err != nil {
			return ParseResult{}, err
		}
		format = detected
	}

	switch format {
	case FormatOTLPJSON:
		return parseOTLPJSONFile(path)
	case FormatZipkinV2:
		return parseZipkinV2File(path)
	case FormatElasticSearchJSON:
		return parseElasticSearchJSONFile(path)
	case FormatElasticSourceNDJSON:
		return parseElasticSourceNDJSONFile(path)
	case FormatJaegerQueryJSON:
		return parseJaegerQueryJSONFile(path)
	case FormatSkyWalkingGraphQL:
		return parseSkyWalkingGraphQLFile(path)
	default:
		diags := diagnostics.New("trace_import")
		diags.SetSourceFile(path)
		diags.AddError(0, ReasonUnsupportedFormat, "Unsupported trace import format: "+format, "")
		return ParseResult{Format: format, Diagnostics: diags}, fmt.Errorf("unsupported trace import format %q", format)
	}
}

func detectFormat(path string) (string, error) {
	first, err := firstJSONValue(path)
	if err != nil {
		return "", err
	}
	switch v := first.(type) {
	case map[string]any:
		if isSkyWalkingGraphQLPayload(v) {
			return FormatSkyWalkingGraphQL, nil
		}
		if isJaegerPayload(v) {
			return FormatJaegerQueryJSON, nil
		}
		if hits, ok := v["hits"].(map[string]any); ok {
			if _, ok := hits["hits"]; ok {
				return FormatElasticSearchJSON, nil
			}
		}
		if isElasticSource(v) {
			return FormatElasticSourceNDJSON, nil
		}
		if source, ok := v["_source"].(map[string]any); ok && isElasticSource(source) {
			return FormatElasticSourceNDJSON, nil
		}
		if hasAnyKey(v, "resourceSpans", "resource_spans") {
			return FormatOTLPJSON, nil
		}
		if hasAnyKey(v, "traceId", "id") {
			return FormatZipkinV2, nil
		}
	case []any:
		if len(v) == 0 {
			return FormatZipkinV2, nil
		}
		if _, ok := v[0].(map[string]any); ok {
			if isJaegerPayload(v[0]) {
				return FormatJaegerQueryJSON, nil
			}
			return FormatZipkinV2, nil
		}
		if nested, ok := v[0].([]any); ok && (len(nested) == 0 || isObject(nested[0])) {
			return FormatZipkinV2, nil
		}
	}
	return "", fmt.Errorf("could not auto-detect trace import format for %s", path)
}

func firstJSONValue(path string) (any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var payload any
		dec := json.NewDecoder(strings.NewReader(line))
		dec.UseNumber()
		if err := dec.Decode(&payload); err == nil {
			return payload, nil
		}
		break
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	f2, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f2.Close()
	dec := json.NewDecoder(f2)
	dec.UseNumber()
	var payload any
	if err := dec.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func parseOTLPJSONFile(path string) (ParseResult, error) {
	diags := diagnostics.New(FormatOTLPJSON)
	diags.SetSourceFile(path)
	spans := make([]Span, 0, 1024)

	f, err := os.Open(path)
	if err != nil {
		return ParseResult{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		diags.TotalLines++
		if line == "" {
			continue
		}
		var envelope otlpEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			diags.AddSkipped(diags.TotalLines, ReasonInvalidJSON, err.Error(), line)
			continue
		}
		next, err := spansFromOTLP(envelope)
		if err != nil {
			diags.AddSkipped(diags.TotalLines, ReasonInvalidOTLPEnvelope, err.Error(), line)
			continue
		}
		diags.ParsedRecords += len(next)
		spans = append(spans, next...)
	}
	if err := scanner.Err(); err != nil {
		return ParseResult{}, err
	}
	if diags.TotalLines == 0 {
		diags.AddWarning(0, "EMPTY_FILE", "Trace import file is empty.", "", false)
	}
	return ParseResult{Format: FormatOTLPJSON, Spans: spans, Diagnostics: diags}, nil
}

func parseZipkinV2File(path string) (ParseResult, error) {
	diags := diagnostics.New(FormatZipkinV2)
	diags.SetSourceFile(path)

	f, err := os.Open(path)
	if err != nil {
		return ParseResult{}, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.UseNumber()
	var payload any
	if err := dec.Decode(&payload); err != nil {
		diags.AddError(0, ReasonInvalidJSON, err.Error(), "")
		return ParseResult{Format: FormatZipkinV2, Diagnostics: diags}, err
	}
	spans, skipped := spansFromZipkinPayload(payload)
	diags.TotalLines = 1
	diags.ParsedRecords = len(spans)
	for _, msg := range skipped {
		diags.AddSkipped(0, ReasonInvalidZipkinSpan, msg, "")
	}
	return ParseResult{Format: FormatZipkinV2, Spans: spans, Diagnostics: diags}, nil
}

func parseElasticSearchJSONFile(path string) (ParseResult, error) {
	diags := diagnostics.New(FormatElasticSearchJSON)
	diags.SetSourceFile(path)

	f, err := os.Open(path)
	if err != nil {
		return ParseResult{}, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.UseNumber()
	var payload any
	if err := dec.Decode(&payload); err != nil {
		diags.AddError(0, ReasonInvalidJSON, err.Error(), "")
		return ParseResult{Format: FormatElasticSearchJSON, Diagnostics: diags}, err
	}

	sources := flattenElasticSearchSources(payload)
	spans := make([]Span, 0, len(sources))
	diags.TotalLines = 1
	for _, source := range sources {
		span, ok, msg := spanFromElasticSource(source, FormatElasticSearchJSON)
		if !ok {
			diags.AddSkipped(0, ReasonInvalidElasticEvent, msg, "")
			continue
		}
		spans = append(spans, span)
		diags.ParsedRecords++
	}
	sortSpans(spans)
	return ParseResult{Format: FormatElasticSearchJSON, Spans: spans, Diagnostics: diags}, nil
}

func parseElasticSourceNDJSONFile(path string) (ParseResult, error) {
	diags := diagnostics.New(FormatElasticSourceNDJSON)
	diags.SetSourceFile(path)
	spans := make([]Span, 0, 1024)

	f, err := os.Open(path)
	if err != nil {
		return ParseResult{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		diags.TotalLines++
		if line == "" {
			continue
		}
		var payload any
		dec := json.NewDecoder(strings.NewReader(line))
		dec.UseNumber()
		if err := dec.Decode(&payload); err != nil {
			diags.AddSkipped(diags.TotalLines, ReasonInvalidJSON, err.Error(), line)
			continue
		}
		for _, source := range flattenElasticSourceLine(payload) {
			span, ok, msg := spanFromElasticSource(source, FormatElasticSourceNDJSON)
			if !ok {
				diags.AddSkipped(diags.TotalLines, ReasonInvalidElasticEvent, msg, line)
				continue
			}
			spans = append(spans, span)
			diags.ParsedRecords++
		}
	}
	if err := scanner.Err(); err != nil {
		return ParseResult{}, err
	}
	if diags.TotalLines == 0 {
		diags.AddWarning(0, "EMPTY_FILE", "Trace import file is empty.", "", false)
	}
	sortSpans(spans)
	return ParseResult{Format: FormatElasticSourceNDJSON, Spans: spans, Diagnostics: diags}, nil
}

func parseJaegerQueryJSONFile(path string) (ParseResult, error) {
	diags := diagnostics.New(FormatJaegerQueryJSON)
	diags.SetSourceFile(path)

	f, err := os.Open(path)
	if err != nil {
		return ParseResult{}, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.UseNumber()
	var payload any
	if err := dec.Decode(&payload); err != nil {
		diags.AddError(0, ReasonInvalidJSON, err.Error(), "")
		return ParseResult{Format: FormatJaegerQueryJSON, Diagnostics: diags}, err
	}

	traces, skipped := jaegerTracesFromPayload(payload)
	diags.TotalLines = 1
	if len(traces) == 0 {
		diags.AddError(0, ReasonInvalidJaegerTrace, "Jaeger payload must contain data[].spans or a trace object with spans.", "")
		return ParseResult{Format: FormatJaegerQueryJSON, Diagnostics: diags}, nil
	}

	spans := make([]Span, 0)
	for _, msg := range skipped {
		diags.AddSkipped(0, ReasonInvalidJaegerTrace, msg, "")
	}
	for _, trace := range traces {
		next, nextSkipped := spansFromJaegerTrace(trace)
		spans = append(spans, next...)
		for _, msg := range nextSkipped {
			diags.AddSkipped(0, ReasonInvalidJaegerTrace, msg, "")
		}
	}
	diags.ParsedRecords = len(spans)
	sortSpans(spans)
	return ParseResult{Format: FormatJaegerQueryJSON, Spans: spans, Diagnostics: diags}, nil
}

func parseSkyWalkingGraphQLFile(path string) (ParseResult, error) {
	diags := diagnostics.New(FormatSkyWalkingGraphQL)
	diags.SetSourceFile(path)

	f, err := os.Open(path)
	if err != nil {
		return ParseResult{}, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.UseNumber()
	var payload any
	if err := dec.Decode(&payload); err != nil {
		diags.AddError(0, ReasonInvalidJSON, err.Error(), "")
		return ParseResult{Format: FormatSkyWalkingGraphQL, Diagnostics: diags}, err
	}

	spans, schemaMatched, version, skipped := spansFromSkyWalkingPayload(payload)
	diags.TotalLines = 1
	if !schemaMatched {
		diags.AddError(0, ReasonSkyWalkingSchema, "SkyWalking GraphQL payload must contain data.queryTrace.spans or data.trace.spans.", "")
		return ParseResult{Format: FormatSkyWalkingGraphQL, Diagnostics: diags}, nil
	}
	if version == "" {
		diags.AddWarning(0, ReasonSkyWalkingVersion, "SkyWalking response does not expose an explicit schema/version marker; parsed using the guarded queryTrace.spans contract.", "", false)
	}
	for _, msg := range skipped {
		diags.AddSkipped(0, ReasonInvalidSkyWalking, msg, "")
	}
	diags.ParsedRecords = len(spans)
	sortSpans(spans)
	return ParseResult{Format: FormatSkyWalkingGraphQL, Spans: spans, Diagnostics: diags}, nil
}

type otlpEnvelope struct {
	ResourceSpans []otlpResourceSpans `json:"resourceSpans"`
}

type otlpResourceSpans struct {
	Resource                    otlpResource     `json:"resource"`
	ScopeSpans                  []otlpScopeSpans `json:"scopeSpans"`
	InstrumentationLibrarySpans []otlpScopeSpans `json:"instrumentationLibrarySpans"`
}

type otlpResource struct {
	Attributes []otlpKeyValue `json:"attributes"`
}

type otlpScopeSpans struct {
	Spans []otlpSpan `json:"spans"`
}

type otlpSpan struct {
	TraceID           string         `json:"traceId"`
	SpanID            string         `json:"spanId"`
	ParentSpanID      string         `json:"parentSpanId"`
	Name              string         `json:"name"`
	Kind              int            `json:"kind"`
	StartTimeUnixNano json.Number    `json:"startTimeUnixNano"`
	EndTimeUnixNano   json.Number    `json:"endTimeUnixNano"`
	Attributes        []otlpKeyValue `json:"attributes"`
	Status            otlpStatus     `json:"status"`
}

type otlpStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type otlpKeyValue struct {
	Key   string    `json:"key"`
	Value otlpValue `json:"value"`
}

type otlpValue struct {
	StringValue string      `json:"stringValue"`
	IntValue    json.Number `json:"intValue"`
	DoubleValue json.Number `json:"doubleValue"`
	BoolValue   *bool       `json:"boolValue"`
}

func spansFromOTLP(envelope otlpEnvelope) ([]Span, error) {
	if len(envelope.ResourceSpans) == 0 {
		return nil, errors.New("OTLP envelope has no resourceSpans")
	}
	out := make([]Span, 0)
	for _, resourceSpans := range envelope.ResourceSpans {
		resourceAttrs := attrsFromOTLP(resourceSpans.Resource.Attributes)
		serviceName := resourceAttrs["service.name"]
		scopeGroups := append([]otlpScopeSpans{}, resourceSpans.ScopeSpans...)
		scopeGroups = append(scopeGroups, resourceSpans.InstrumentationLibrarySpans...)
		for _, scopeSpans := range scopeGroups {
			for _, raw := range scopeSpans.Spans {
				if raw.TraceID == "" || raw.SpanID == "" {
					continue
				}
				spanAttrs := attrsFromOTLP(raw.Attributes)
				if serviceName == "" {
					serviceName = spanAttrs["service.name"]
				}
				start := numberToInt64(raw.StartTimeUnixNano)
				end := numberToInt64(raw.EndTimeUnixNano)
				duration := int64(0)
				if end > start {
					duration = end - start
				}
				status := otlpStatusName(raw.Status.Code)
				out = append(out, Span{
					TraceID:        strings.ToLower(raw.TraceID),
					SpanID:         strings.ToLower(raw.SpanID),
					ParentSpanID:   strings.ToLower(raw.ParentSpanID),
					Name:           raw.Name,
					ServiceName:    serviceName,
					RemoteService:  firstNonEmpty(spanAttrs["peer.service"], spanAttrs["server.address"], spanAttrs["net.peer.name"], spanAttrs["db.system"]),
					Kind:           otlpKindName(raw.Kind),
					StartUnixNanos: start,
					DurationNanos:  duration,
					StatusCode:     status,
					Error:          raw.Status.Code == 2 || strings.EqualFold(spanAttrs["error"], "true"),
					Attributes:     mergeAttrs(resourceAttrs, spanAttrs),
					SourceFormat:   FormatOTLPJSON,
				})
			}
		}
	}
	return out, nil
}

func attrsFromOTLP(kvs []otlpKeyValue) map[string]string {
	out := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		if kv.Key == "" {
			continue
		}
		out[kv.Key] = kv.Value.String()
	}
	return out
}

func (v otlpValue) String() string {
	switch {
	case v.StringValue != "":
		return v.StringValue
	case string(v.IntValue) != "":
		return v.IntValue.String()
	case string(v.DoubleValue) != "":
		return v.DoubleValue.String()
	case v.BoolValue != nil:
		return strconv.FormatBool(*v.BoolValue)
	default:
		return ""
	}
}

type zipkinEndpoint struct {
	ServiceName string `json:"serviceName"`
	IPv4        string `json:"ipv4"`
	IPv6        string `json:"ipv6"`
	Port        int    `json:"port"`
}

type zipkinSpan struct {
	TraceID        string            `json:"traceId"`
	ID             string            `json:"id"`
	ParentID       string            `json:"parentId"`
	Name           string            `json:"name"`
	Kind           string            `json:"kind"`
	Timestamp      json.Number       `json:"timestamp"`
	Duration       json.Number       `json:"duration"`
	LocalEndpoint  zipkinEndpoint    `json:"localEndpoint"`
	RemoteEndpoint zipkinEndpoint    `json:"remoteEndpoint"`
	Tags           map[string]string `json:"tags"`
}

func spansFromZipkinPayload(payload any) ([]Span, []string) {
	rawItems := flattenZipkinPayload(payload)
	out := make([]Span, 0, len(rawItems))
	skipped := []string{}
	for _, raw := range rawItems {
		body, err := json.Marshal(raw)
		if err != nil {
			skipped = append(skipped, err.Error())
			continue
		}
		var item zipkinSpan
		dec := json.NewDecoder(strings.NewReader(string(body)))
		dec.UseNumber()
		if err := dec.Decode(&item); err != nil {
			skipped = append(skipped, err.Error())
			continue
		}
		if item.TraceID == "" || item.ID == "" {
			skipped = append(skipped, "Zipkin span missing traceId or id")
			continue
		}
		attrs := map[string]string{}
		for k, v := range item.Tags {
			attrs[k] = v
		}
		serviceName := item.LocalEndpoint.ServiceName
		remoteService := item.RemoteEndpoint.ServiceName
		duration := numberToInt64(item.Duration) * 1000
		out = append(out, Span{
			TraceID:        strings.ToLower(item.TraceID),
			SpanID:         strings.ToLower(item.ID),
			ParentSpanID:   strings.ToLower(item.ParentID),
			Name:           item.Name,
			ServiceName:    serviceName,
			RemoteService:  remoteService,
			Kind:           strings.ToUpper(item.Kind),
			StartUnixNanos: numberToInt64(item.Timestamp) * 1000,
			DurationNanos:  duration,
			StatusCode:     statusFromZipkinTags(attrs),
			Error:          zipkinSpanHasError(attrs),
			Attributes:     attrs,
			SourceFormat:   FormatZipkinV2,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TraceID != out[j].TraceID {
			return out[i].TraceID < out[j].TraceID
		}
		if out[i].StartUnixNanos != out[j].StartUnixNanos {
			return out[i].StartUnixNanos < out[j].StartUnixNanos
		}
		return out[i].SpanID < out[j].SpanID
	})
	return out, skipped
}

func flattenZipkinPayload(payload any) []any {
	switch v := payload.(type) {
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			if nested, ok := item.([]any); ok {
				out = append(out, flattenZipkinPayload(nested)...)
			} else {
				out = append(out, item)
			}
		}
		return out
	case map[string]any:
		return []any{v}
	default:
		return nil
	}
}

type jaegerTrace struct {
	TraceID   string                   `json:"traceID"`
	Spans     []jaegerSpan             `json:"spans"`
	Processes map[string]jaegerProcess `json:"processes"`
}

type jaegerSpan struct {
	TraceID       string            `json:"traceID"`
	SpanID        string            `json:"spanID"`
	OperationName string            `json:"operationName"`
	Name          string            `json:"name"`
	Kind          string            `json:"kind"`
	References    []jaegerReference `json:"references"`
	StartTime     json.Number       `json:"startTime"`
	Duration      json.Number       `json:"duration"`
	Tags          []jaegerTag       `json:"tags"`
	ProcessID     string            `json:"processID"`
	Process       jaegerProcess     `json:"process"`
}

type jaegerReference struct {
	RefType string `json:"refType"`
	TraceID string `json:"traceID"`
	SpanID  string `json:"spanID"`
}

type jaegerProcess struct {
	ServiceName string      `json:"serviceName"`
	Tags        []jaegerTag `json:"tags"`
}

type jaegerTag struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

func jaegerTracesFromPayload(payload any) ([]jaegerTrace, []string) {
	rawItems := flattenJaegerTraceItems(payload)
	out := make([]jaegerTrace, 0, len(rawItems))
	skipped := []string{}
	for _, raw := range rawItems {
		if !isJaegerPayload(raw) {
			skipped = append(skipped, "Jaeger trace item missing spans")
			continue
		}
		body, err := json.Marshal(raw)
		if err != nil {
			skipped = append(skipped, err.Error())
			continue
		}
		var item jaegerTrace
		dec := json.NewDecoder(strings.NewReader(string(body)))
		dec.UseNumber()
		if err := dec.Decode(&item); err != nil {
			skipped = append(skipped, err.Error())
			continue
		}
		out = append(out, item)
	}
	return out, skipped
}

func flattenJaegerTraceItems(payload any) []any {
	switch v := payload.(type) {
	case map[string]any:
		if data, ok := v["data"]; ok {
			switch dataValue := data.(type) {
			case []any:
				out := make([]any, 0, len(dataValue))
				for _, item := range dataValue {
					out = append(out, flattenJaegerTraceItems(item)...)
				}
				return out
			case map[string]any:
				return flattenJaegerTraceItems(dataValue)
			}
		}
		if isJaegerPayload(v) {
			return []any{v}
		}
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, flattenJaegerTraceItems(item)...)
		}
		return out
	}
	return nil
}

func spansFromJaegerTrace(trace jaegerTrace) ([]Span, []string) {
	out := make([]Span, 0, len(trace.Spans))
	skipped := []string{}
	for _, raw := range trace.Spans {
		traceID := strings.ToLower(firstNonEmpty(raw.TraceID, trace.TraceID))
		spanID := strings.ToLower(raw.SpanID)
		if traceID == "" || spanID == "" {
			skipped = append(skipped, "Jaeger span missing traceID or spanID")
			continue
		}
		process := raw.Process
		if process.ServiceName == "" && raw.ProcessID != "" && trace.Processes != nil {
			process = trace.Processes[raw.ProcessID]
		}
		processAttrs := attrsFromJaegerTags(process.Tags)
		spanAttrs := attrsFromJaegerTags(raw.Tags)
		attrs := mergeAttrs(processAttrs, spanAttrs)
		serviceName := firstNonEmpty(process.ServiceName, attrs["service.name"])
		statusCode := firstNonEmpty(attrs["http.status_code"], attrs["otel.status_code"])
		kind := strings.ToUpper(firstNonEmpty(raw.Kind, attrs["span.kind"], "SPAN"))
		out = append(out, Span{
			TraceID:        traceID,
			SpanID:         spanID,
			ParentSpanID:   strings.ToLower(jaegerParentSpanID(raw.References)),
			Name:           firstNonEmpty(raw.OperationName, raw.Name, "(unnamed)"),
			ServiceName:    serviceName,
			RemoteService:  jaegerRemoteService(attrs),
			Kind:           kind,
			StartUnixNanos: numberToInt64(raw.StartTime) * 1000,
			DurationNanos:  numberToInt64(raw.Duration) * 1000,
			StatusCode:     statusCode,
			Error:          traceAttrsIndicateError(attrs, statusCode),
			Attributes:     attrs,
			SourceFormat:   FormatJaegerQueryJSON,
		})
	}
	return out, skipped
}

func attrsFromJaegerTags(tags []jaegerTag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		if tag.Key == "" {
			continue
		}
		out[tag.Key] = traceValueString(tag.Value)
	}
	return out
}

func jaegerParentSpanID(refs []jaegerReference) string {
	for _, ref := range refs {
		if strings.EqualFold(ref.RefType, "CHILD_OF") && ref.SpanID != "" {
			return ref.SpanID
		}
	}
	for _, ref := range refs {
		if ref.SpanID != "" {
			return ref.SpanID
		}
	}
	return ""
}

func jaegerRemoteService(attrs map[string]string) string {
	return firstNonEmpty(
		attrs["peer.service"],
		attrs["server.address"],
		attrs["net.peer.name"],
		attrs["net.peer.ip"],
		attrs["peer.hostname"],
		attrs["http.host"],
		attrs["db.system"],
	)
}

func isJaegerPayload(payload any) bool {
	root, ok := payload.(map[string]any)
	if !ok {
		return false
	}
	if _, ok := root["spans"]; ok {
		return true
	}
	data, ok := root["data"]
	if !ok {
		return false
	}
	switch v := data.(type) {
	case []any:
		if len(v) == 0 {
			return false
		}
		return isJaegerPayload(v[0])
	case map[string]any:
		return isJaegerPayload(v)
	default:
		return false
	}
}

func spansFromSkyWalkingPayload(payload any) ([]Span, bool, string, []string) {
	rawSpans, matched := flattenSkyWalkingSpanItems(payload)
	if !matched {
		return nil, false, skyWalkingVersion(payload), nil
	}
	out := make([]Span, 0, len(rawSpans))
	skipped := []string{}
	for _, raw := range rawSpans {
		span, ok, msg := spanFromSkyWalkingSpan(raw)
		if !ok {
			skipped = append(skipped, msg)
			continue
		}
		out = append(out, span)
	}
	return out, true, skyWalkingVersion(payload), skipped
}

func flattenSkyWalkingSpanItems(payload any) ([]map[string]any, bool) {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil, false
	}
	data, ok := root["data"].(map[string]any)
	if !ok {
		return nil, false
	}
	for _, key := range []string{"queryTrace", "trace"} {
		if candidate, ok := data[key]; ok {
			spans, matched := skyWalkingSpansFromCandidate(candidate)
			if matched {
				return spans, true
			}
		}
	}
	if _, ok := data["queryBasicTraces"]; ok {
		return nil, false
	}
	return nil, false
}

func skyWalkingSpansFromCandidate(candidate any) ([]map[string]any, bool) {
	switch v := candidate.(type) {
	case map[string]any:
		rawSpans, ok := v["spans"]
		if !ok {
			return nil, false
		}
		return skyWalkingSpanList(rawSpans), true
	case []any:
		out := []map[string]any{}
		matched := false
		for _, item := range v {
			next, ok := skyWalkingSpansFromCandidate(item)
			if ok {
				matched = true
				out = append(out, next...)
			}
		}
		return out, matched
	default:
		return nil, false
	}
}

func skyWalkingSpanList(raw any) []map[string]any {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if span, ok := item.(map[string]any); ok {
			out = append(out, span)
		}
	}
	return out
}

func spanFromSkyWalkingSpan(raw map[string]any) (Span, bool, string) {
	traceID := strings.ToLower(firstNonEmpty(
		traceMapString(raw, "traceId"),
		traceMapString(raw, "traceID"),
	))
	segmentID := traceMapString(raw, "segmentId")
	spanIDValue := traceMapString(raw, "spanId")
	if traceID == "" || segmentID == "" || spanIDValue == "" {
		return Span{}, false, "SkyWalking span missing traceId, segmentId, or spanId"
	}

	attrs := attrsFromSkyWalkingTags(raw["tags"])
	for _, key := range []string{"component", "layer", "serviceInstanceName"} {
		if value := traceMapString(raw, key); value != "" {
			attrs[key] = value
		}
	}
	start := skyWalkingTimeToNanos(firstNumber(raw["startTime"]))
	end := skyWalkingTimeToNanos(firstNumber(raw["endTime"]))
	duration := int64(0)
	if end > start {
		duration = end - start
	} else if rawDuration := firstNumber(raw["duration"]); rawDuration > 0 {
		duration = skyWalkingDurationToNanos(rawDuration)
	}
	statusCode := firstNonEmpty(attrs["http.status_code"], attrs["status_code"])
	span := Span{
		TraceID:        traceID,
		SpanID:         skyWalkingCanonicalSpanID(segmentID, spanIDValue),
		ParentSpanID:   skyWalkingParentSpanID(segmentID, raw),
		Name:           firstNonEmpty(traceMapString(raw, "endpointName"), traceMapString(raw, "operationName"), "(unnamed)"),
		ServiceName:    firstNonEmpty(traceMapString(raw, "serviceCode"), traceMapString(raw, "serviceName"), traceMapString(raw, "service")),
		RemoteService:  firstNonEmpty(traceMapString(raw, "peer"), attrs["peer.service"], attrs["networkAddress"]),
		Kind:           strings.ToUpper(firstNonEmpty(traceMapString(raw, "type"), traceMapString(raw, "spanType"), "SPAN")),
		StartUnixNanos: start,
		DurationNanos:  duration,
		StatusCode:     statusCode,
		Error:          traceValueBool(raw["isError"]) || traceAttrsIndicateError(attrs, statusCode),
		Attributes:     attrs,
		SourceFormat:   FormatSkyWalkingGraphQL,
	}
	return span, true, ""
}

func attrsFromSkyWalkingTags(raw any) map[string]string {
	out := map[string]string{}
	switch tags := raw.(type) {
	case []any:
		for _, item := range tags {
			tag, ok := item.(map[string]any)
			if !ok {
				continue
			}
			key := traceValueString(tag["key"])
			if key == "" {
				continue
			}
			out[key] = traceValueString(tag["value"])
		}
	case map[string]any:
		for key, value := range tags {
			out[key] = traceValueString(value)
		}
	}
	return out
}

func skyWalkingParentSpanID(segmentID string, raw map[string]any) string {
	parentValue := traceMapString(raw, "parentSpanId")
	if parentValue != "" && parentValue != "-1" {
		return skyWalkingCanonicalSpanID(segmentID, parentValue)
	}
	refs, ok := raw["refs"].([]any)
	if !ok {
		return ""
	}
	for _, item := range refs {
		ref, ok := item.(map[string]any)
		if !ok {
			continue
		}
		parentSegment := firstNonEmpty(
			traceMapString(ref, "parentTraceSegmentId"),
			traceMapString(ref, "parentSegmentId"),
			traceMapString(ref, "traceSegmentId"),
		)
		parentSpan := traceMapString(ref, "parentSpanId")
		if parentSegment != "" && parentSpan != "" && parentSpan != "-1" {
			return skyWalkingCanonicalSpanID(parentSegment, parentSpan)
		}
	}
	return ""
}

func skyWalkingCanonicalSpanID(segmentID, spanID string) string {
	return strings.ToLower(segmentID + ":" + spanID)
}

func isSkyWalkingGraphQLPayload(payload any) bool {
	root, ok := payload.(map[string]any)
	if !ok {
		return false
	}
	data, ok := root["data"].(map[string]any)
	if !ok {
		return false
	}
	for _, key := range []string{"queryTrace", "trace", "queryBasicTraces"} {
		if _, ok := data[key]; ok {
			return true
		}
	}
	return false
}

func skyWalkingVersion(payload any) string {
	root, ok := payload.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"version", "skyWalkingVersion", "schemaVersion"} {
		if value := traceValueString(root[key]); value != "" {
			return value
		}
	}
	if extensions, ok := root["extensions"].(map[string]any); ok {
		for _, key := range []string{"version", "skyWalkingVersion", "schemaVersion"} {
			if value := traceValueString(extensions[key]); value != "" {
				return value
			}
		}
	}
	return ""
}

func flattenElasticSearchSources(payload any) []map[string]any {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	hitsObj, _ := root["hits"].(map[string]any)
	rawHits, _ := hitsObj["hits"].([]any)
	out := make([]map[string]any, 0, len(rawHits))
	for _, raw := range rawHits {
		hit, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		source, ok := hit["_source"].(map[string]any)
		if !ok {
			continue
		}
		out = append(out, source)
	}
	return out
}

func flattenElasticSourceLine(payload any) []map[string]any {
	switch v := payload.(type) {
	case map[string]any:
		if source, ok := v["_source"].(map[string]any); ok {
			return []map[string]any{source}
		}
		if sources := flattenElasticSearchSources(v); len(sources) > 0 {
			return sources
		}
		return []map[string]any{v}
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			out = append(out, flattenElasticSourceLine(item)...)
		}
		return out
	default:
		return nil
	}
}

func spanFromElasticSource(source map[string]any, sourceFormat string) (Span, bool, string) {
	traceID := strings.ToLower(elasticNestedString(source, "trace", "id"))
	if traceID == "" {
		return Span{}, false, "Elastic APM event missing trace.id"
	}
	kind, id := elasticSpanKindAndID(source)
	if id == "" {
		return Span{}, false, "Elastic APM event missing transaction.id or span.id"
	}
	attrs := flattenStringAttrs(source)
	durationMicros := firstNumber(
		elasticNestedAny(source, "transaction", "duration", "us"),
		elasticNestedAny(source, "span", "duration", "us"),
		elasticNestedAny(source, "duration", "us"),
	)
	if durationMicros == 0 {
		// ECS event.duration is nanoseconds, while Elastic APM duration.us is
		// microseconds.
		durationMicros = firstNumber(elasticNestedAny(source, "event", "duration")) / 1000
	}
	name := firstNonEmpty(
		elasticNestedString(source, "transaction", "name"),
		elasticNestedString(source, "span", "name"),
		elasticNestedString(source, "message"),
		"(unnamed)",
	)
	serviceName := elasticNestedString(source, "service", "name")
	statusCode := firstNonEmpty(
		elasticNestedString(source, "event", "outcome"),
		elasticNestedString(source, "transaction", "result"),
		elasticNestedString(source, "http", "response", "status_code"),
	)
	errorFlag := elasticEventHasError(source, statusCode)
	span := Span{
		TraceID:        traceID,
		SpanID:         strings.ToLower(id),
		ParentSpanID:   strings.ToLower(elasticNestedString(source, "parent", "id")),
		Name:           name,
		ServiceName:    serviceName,
		RemoteService:  elasticRemoteService(source),
		Kind:           kind,
		StartUnixNanos: parseElasticTimestampNanos(elasticNestedString(source, "@timestamp")),
		DurationNanos:  durationMicros * 1000,
		StatusCode:     statusCode,
		Error:          errorFlag,
		Attributes:     attrs,
		SourceFormat:   sourceFormat,
	}
	return span, true, ""
}

func elasticSpanKindAndID(source map[string]any) (string, string) {
	if spanID := elasticNestedString(source, "span", "id"); spanID != "" {
		kind := strings.ToUpper(firstNonEmpty(
			elasticNestedString(source, "span", "type"),
			elasticNestedString(source, "span", "subtype"),
			"SPAN",
		))
		return kind, spanID
	}
	if txID := elasticNestedString(source, "transaction", "id"); txID != "" {
		kind := strings.ToUpper(firstNonEmpty(elasticNestedString(source, "transaction", "type"), "TRANSACTION"))
		return kind, txID
	}
	return "", ""
}

func elasticRemoteService(source map[string]any) string {
	return firstNonEmpty(
		elasticNestedString(source, "span", "destination", "service", "resource"),
		elasticNestedString(source, "span", "destination", "service", "name"),
		elasticNestedString(source, "service", "target", "name"),
		elasticNestedString(source, "destination", "service", "resource"),
		elasticNestedString(source, "destination", "address"),
		elasticNestedString(source, "url", "domain"),
		elasticNestedString(source, "span", "subtype"),
	)
}

func elasticEventHasError(source map[string]any, status string) bool {
	if strings.EqualFold(status, "failure") || strings.EqualFold(status, "error") {
		return true
	}
	if elasticNestedAny(source, "error") != nil {
		return true
	}
	if code, err := strconv.Atoi(status); err == nil && code >= 500 {
		return true
	}
	return false
}

func isElasticSource(source map[string]any) bool {
	return elasticNestedString(source, "trace", "id") != "" &&
		(elasticNestedString(source, "transaction", "id") != "" || elasticNestedString(source, "span", "id") != "")
}

func elasticNestedString(m map[string]any, path ...string) string {
	value := elasticNestedAny(m, path...)
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return ""
	}
}

func elasticNestedAny(m map[string]any, path ...string) any {
	var current any = m
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = obj[key]
		if current == nil {
			return nil
		}
	}
	return current
}

func firstNumber(values ...any) int64 {
	for _, value := range values {
		switch v := value.(type) {
		case json.Number:
			if i, err := v.Int64(); err == nil {
				return i
			}
			if f, err := strconv.ParseFloat(v.String(), 64); err == nil {
				return int64(f)
			}
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return int64(f)
			}
		}
	}
	return 0
}

func parseElasticTimestampNanos(value string) int64 {
	if value == "" {
		return 0
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000-07:00",
		"2006-01-02T15:04:05.000000-07:00",
	} {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts.UnixNano()
		}
	}
	return 0
}

func flattenStringAttrs(source map[string]any) map[string]string {
	out := map[string]string{}
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				next := key
				if prefix != "" {
					next = prefix + "." + key
				}
				walk(next, child)
			}
		case string:
			out[prefix] = v
		case json.Number:
			out[prefix] = v.String()
		case float64:
			out[prefix] = strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			out[prefix] = strconv.FormatBool(v)
		}
	}
	walk("", source)
	return out
}

func hasAnyKey(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}

func isObject(v any) bool {
	_, ok := v.(map[string]any)
	return ok
}

func numberToInt64(v json.Number) int64 {
	if string(v) == "" {
		return 0
	}
	if i, err := v.Int64(); err == nil {
		return i
	}
	f, err := strconv.ParseFloat(v.String(), 64)
	if err != nil {
		return 0
	}
	return int64(f)
}

func otlpKindName(kind int) string {
	switch kind {
	case 1:
		return "INTERNAL"
	case 2:
		return "SERVER"
	case 3:
		return "CLIENT"
	case 4:
		return "PRODUCER"
	case 5:
		return "CONSUMER"
	default:
		return "UNSPECIFIED"
	}
}

func otlpStatusName(code int) string {
	switch code {
	case 1:
		return "OK"
	case 2:
		return "ERROR"
	default:
		return "UNSET"
	}
}

func traceMapString(m map[string]any, key string) string {
	return traceValueString(m[key])
}

func traceValueString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		return strconv.FormatBool(v)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func traceValueBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		normalized := strings.ToLower(strings.TrimSpace(v))
		return normalized == "true" || normalized == "1" || normalized == "error"
	case json.Number:
		return v.String() == "1"
	case float64:
		return v == 1
	case int:
		return v == 1
	default:
		return false
	}
}

func traceAttrsIndicateError(attrs map[string]string, statusCode string) bool {
	if traceValueBool(attrs["error"]) {
		return true
	}
	if strings.EqualFold(statusCode, "ERROR") || strings.EqualFold(statusCode, "failure") {
		return true
	}
	if code, err := strconv.Atoi(statusCode); err == nil && code >= 500 {
		return true
	}
	return false
}

func skyWalkingTimeToNanos(value int64) int64 {
	switch {
	case value >= 10_000_000_000_000_000:
		return value
	case value >= 100_000_000_000_000:
		return value * 1000
	case value > 0:
		return value * 1_000_000
	default:
		return 0
	}
}

func skyWalkingDurationToNanos(value int64) int64 {
	if value <= 0 {
		return 0
	}
	if value >= 10_000_000_000_000 {
		return value
	}
	return value * 1_000_000
}

func statusFromZipkinTags(attrs map[string]string) string {
	if v := attrs["http.status_code"]; v != "" {
		return v
	}
	if v := attrs["error"]; v != "" {
		return "ERROR"
	}
	return ""
}

func zipkinSpanHasError(attrs map[string]string) bool {
	if attrs["error"] != "" {
		return true
	}
	code := attrs["http.status_code"]
	if code == "" {
		return false
	}
	n, err := strconv.Atoi(code)
	return err == nil && n >= 500
}

func sortSpans(spans []Span) {
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].TraceID != spans[j].TraceID {
			return spans[i].TraceID < spans[j].TraceID
		}
		if spans[i].StartUnixNanos != spans[j].StartUnixNanos {
			return spans[i].StartUnixNanos < spans[j].StartUnixNanos
		}
		return spans[i].SpanID < spans[j].SpanID
	})
}

func mergeAttrs(left, right map[string]string) map[string]string {
	out := make(map[string]string, len(left)+len(right))
	for k, v := range left {
		out[k] = v
	}
	for k, v := range right {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// DecodeOne is kept for tests and future structured importers.
func DecodeOne(r io.Reader) (any, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	var payload any
	err := dec.Decode(&payload)
	return payload, err
}
