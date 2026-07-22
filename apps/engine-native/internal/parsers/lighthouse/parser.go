// Package lighthouse parses local Lighthouse report JSON files into a bounded,
// redacted browser-audit model. It deliberately keeps parsing separate from
// scoring and finding generation, which belong to the analyzer package.
package lighthouse

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

const (
	FormatLighthouseJSON = "lighthouse-json"
	DefaultMaxBytes      = int64(64 << 20)
	MaxOutputStringBytes = 4 << 10
	MaxIdentifierBytes   = 256
	MaxCategories        = 100
	MaxAudits            = 1_000
	MaxResources         = 100_000
	MaxResourceSummaries = 100

	ReasonInvalidJSON     = "INVALID_LIGHTHOUSE_JSON"
	ReasonInvalidEnvelope = "INVALID_LIGHTHOUSE_ENVELOPE"
	ReasonInputTooLarge   = "LIGHTHOUSE_INPUT_TOO_LARGE"
	ReasonAuditLimit      = "LIGHTHOUSE_AUDIT_LIMIT_REACHED"
	ReasonCategoryLimit   = "LIGHTHOUSE_CATEGORY_LIMIT_REACHED"
	ReasonResourceLimit   = "LIGHTHOUSE_RESOURCE_LIMIT_REACHED"
	ReasonScoreInvalid    = "LIGHTHOUSE_SCORE_INVALID"
)

type Options struct {
	MaxBytes int64
}

type ParseResult struct {
	Report      Report
	Diagnostics *diagnostics.ParserDiagnostics
}

type Report struct {
	LighthouseVersion string
	RequestedURL      string
	FinalURL          string
	FetchTime         string
	UserAgent         string
	FormFactor        string
	ThrottlingMethod  string
	Categories        []Category
	Audits            []Audit
	Resources         []Resource
	ResourceSummaries []ResourceSummary
	RunWarnings       []string
	RuntimeError      string
	Redaction         redact.Summary
}

type Category struct {
	ID    string
	Title string
	Score *float64
}

type Audit struct {
	ID               string
	Title            string
	Description      string
	Score            *float64
	ScoreDisplayMode string
	NumericValue     *float64
	NumericUnit      string
	DisplayValue     string
}

type Resource struct {
	URL           string
	Protocol      string
	StatusCode    int
	MIMEType      string
	ResourceType  string
	TransferBytes int64
	ResourceBytes int64
	StartMS       float64
	EndMS         float64
	DurationMS    float64
	InitiatorType string
}

type ResourceSummary struct {
	ResourceType  string
	RequestCount  int
	TransferBytes int64
}

type rawReport struct {
	LighthouseVersion string                 `json:"lighthouseVersion"`
	RequestedURL      string                 `json:"requestedUrl"`
	FinalURL          string                 `json:"finalUrl"`
	FinalDisplayedURL string                 `json:"finalDisplayedUrl"`
	FetchTime         string                 `json:"fetchTime"`
	UserAgent         string                 `json:"userAgent"`
	Environment       rawEnvironment         `json:"environment"`
	ConfigSettings    rawConfigSettings      `json:"configSettings"`
	Categories        map[string]rawCategory `json:"categories"`
	Audits            map[string]rawAudit    `json:"audits"`
	RunWarnings       []any                  `json:"runWarnings"`
	RuntimeError      *rawRuntimeError       `json:"runtimeError"`
}

type rawEnvironment struct {
	HostUserAgent    string `json:"hostUserAgent"`
	NetworkUserAgent string `json:"networkUserAgent"`
}

type rawConfigSettings struct {
	FormFactor       string `json:"formFactor"`
	ThrottlingMethod string `json:"throttlingMethod"`
}

type rawCategory struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Score *float64 `json:"score"`
}

type rawAudit struct {
	ID               string          `json:"id"`
	Title            string          `json:"title"`
	Description      string          `json:"description"`
	Score            *float64        `json:"score"`
	ScoreDisplayMode string          `json:"scoreDisplayMode"`
	NumericValue     *float64        `json:"numericValue"`
	NumericUnit      string          `json:"numericUnit"`
	DisplayValue     string          `json:"displayValue"`
	Details          json.RawMessage `json:"details"`
}

type rawRuntimeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type rawDetails struct {
	Type  string            `json:"type"`
	Items []json.RawMessage `json:"items"`
}

type rawNetworkRequest struct {
	URL                string  `json:"url"`
	Protocol           string  `json:"protocol"`
	StatusCode         int     `json:"statusCode"`
	MIMEType           string  `json:"mimeType"`
	ResourceType       string  `json:"resourceType"`
	TransferSize       float64 `json:"transferSize"`
	ResourceSize       float64 `json:"resourceSize"`
	StartTime          float64 `json:"startTime"`
	EndTime            float64 `json:"endTime"`
	NetworkEndTime     float64 `json:"networkEndTime"`
	NetworkRequestTime float64 `json:"networkRequestTime"`
	InitiatorType      string  `json:"initiatorType"`
}

type rawResourceSummary struct {
	ResourceType string  `json:"resourceType"`
	RequestCount int     `json:"requestCount"`
	TransferSize float64 `json:"transferSize"`
}

func ParseFile(path string, opts Options) (ParseResult, error) {
	diags := diagnostics.New(FormatLighthouseJSON)
	diags.SetSourceFile(path)
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	info, err := os.Stat(path)
	if err != nil {
		return ParseResult{}, err
	}
	if info.Size() > maxBytes {
		message := fmt.Sprintf("Lighthouse report is %d bytes; limit is %d bytes", info.Size(), maxBytes)
		diags.AddError(0, ReasonInputTooLarge, message, "")
		return ParseResult{Diagnostics: diags}, fmt.Errorf("%s", message)
	}

	f, err := os.Open(path)
	if err != nil {
		return ParseResult{}, err
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, maxBytes+1))
	var raw rawReport
	if err := dec.Decode(&raw); err != nil {
		diags.AddError(0, ReasonInvalidJSON, err.Error(), "")
		return ParseResult{Diagnostics: diags}, err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		message := "Lighthouse report must contain exactly one JSON value"
		diags.AddError(0, ReasonInvalidJSON, message, "")
		return ParseResult{Diagnostics: diags}, fmt.Errorf("%s", message)
	}
	diags.TotalLines = 1
	if strings.TrimSpace(raw.LighthouseVersion) == "" || len(raw.Categories) == 0 || len(raw.Audits) == 0 {
		message := "JSON is not a Lighthouse result: lighthouseVersion, categories, and audits are required"
		diags.AddError(0, ReasonInvalidEnvelope, message, "")
		return ParseResult{Diagnostics: diags}, fmt.Errorf("%s", message)
	}

	policy := redact.NewPolicy(redact.Options{})
	report := Report{
		LighthouseVersion: boundedString(strings.TrimSpace(raw.LighthouseVersion), MaxIdentifierBytes),
		RequestedURL:      boundedString(policy.RedactURL(raw.RequestedURL), MaxOutputStringBytes),
		FinalURL:          boundedString(policy.RedactURL(firstNonEmpty(raw.FinalDisplayedURL, raw.FinalURL)), MaxOutputStringBytes),
		FetchTime:         boundedString(strings.TrimSpace(raw.FetchTime), MaxIdentifierBytes),
		UserAgent:         boundedString(firstNonEmpty(raw.UserAgent, raw.Environment.HostUserAgent, raw.Environment.NetworkUserAgent), MaxOutputStringBytes),
		FormFactor:        boundedString(strings.TrimSpace(raw.ConfigSettings.FormFactor), MaxIdentifierBytes),
		ThrottlingMethod:  boundedString(strings.TrimSpace(raw.ConfigSettings.ThrottlingMethod), MaxIdentifierBytes),
		Categories:        categories(raw.Categories, policy, diags),
		Audits:            audits(raw.Audits, policy, diags),
		RunWarnings:       warningStrings(raw.RunWarnings, policy),
	}
	if raw.RuntimeError != nil {
		message, _ := policy.RedactNamedValue("", raw.RuntimeError.Message)
		report.RuntimeError = boundedString(strings.TrimSpace(raw.RuntimeError.Code+": "+message), MaxOutputStringBytes)
	}
	report.Resources = resources(raw.Audits["network-requests"], policy, diags)
	report.ResourceSummaries = resourceSummaries(raw.Audits["resource-summary"], policy, diags)
	report.Redaction = policy.Summary()
	diags.ParsedRecords = len(report.Audits)
	return ParseResult{Report: report, Diagnostics: diags}, nil
}

func categories(values map[string]rawCategory, policy *redact.Policy, diags *diagnostics.ParserDiagnostics) []Category {
	keys := sortedKeys(values)
	if len(keys) > MaxCategories {
		diags.AddWarning(0, ReasonCategoryLimit, fmt.Sprintf("kept %d of %d categories", MaxCategories, len(keys)), "", false)
		keys = keys[:MaxCategories]
	}
	out := make([]Category, 0, len(keys))
	for _, key := range keys {
		value := values[key]
		id := safeIdentifier(policy, firstNonEmpty(value.ID, key))
		out = append(out, Category{ID: id, Title: safeText(policy, value.Title), Score: validScore(value.Score, id, diags)})
	}
	return out
}

func audits(values map[string]rawAudit, policy *redact.Policy, diags *diagnostics.ParserDiagnostics) []Audit {
	keys := sortedKeys(values)
	if len(keys) > MaxAudits {
		diags.AddWarning(0, ReasonAuditLimit, fmt.Sprintf("kept %d of %d audits", MaxAudits, len(keys)), "", false)
		keys = keys[:MaxAudits]
	}
	out := make([]Audit, 0, len(keys))
	for _, key := range keys {
		value := values[key]
		id := safeIdentifier(policy, firstNonEmpty(value.ID, key))
		out = append(out, Audit{
			ID: id, Title: safeText(policy, value.Title), Description: safeText(policy, value.Description),
			Score: validScore(value.Score, id, diags), ScoreDisplayMode: safeIdentifier(policy, value.ScoreDisplayMode),
			NumericValue: value.NumericValue, NumericUnit: safeIdentifier(policy, value.NumericUnit), DisplayValue: safeText(policy, value.DisplayValue),
		})
	}
	return out
}

func resources(audit rawAudit, policy *redact.Policy, diags *diagnostics.ParserDiagnostics) []Resource {
	var details rawDetails
	if len(audit.Details) == 0 || json.Unmarshal(audit.Details, &details) != nil {
		return []Resource{}
	}
	limit := len(details.Items)
	if limit > MaxResources {
		limit = MaxResources
		diags.AddWarning(0, ReasonResourceLimit, fmt.Sprintf("kept %d of %d network requests", limit, len(details.Items)), "", false)
	}
	out := make([]Resource, 0, limit)
	for _, item := range details.Items[:limit] {
		var raw rawNetworkRequest
		if json.Unmarshal(item, &raw) != nil {
			continue
		}
		end := raw.EndTime
		if end == 0 {
			end = raw.NetworkEndTime
		}
		start := raw.StartTime
		if start == 0 {
			start = raw.NetworkRequestTime
		}
		out = append(out, Resource{
			URL: boundedString(policy.RedactURL(raw.URL), MaxOutputStringBytes), Protocol: safeIdentifier(policy, raw.Protocol), StatusCode: raw.StatusCode,
			MIMEType: safeIdentifier(policy, raw.MIMEType), ResourceType: safeIdentifier(policy, raw.ResourceType),
			TransferBytes: nonNegativeInt64(raw.TransferSize), ResourceBytes: nonNegativeInt64(raw.ResourceSize),
			StartMS: start, EndMS: end, DurationMS: nonNegative(end - start), InitiatorType: safeIdentifier(policy, raw.InitiatorType),
		})
	}
	return out
}

func resourceSummaries(audit rawAudit, policy *redact.Policy, diags *diagnostics.ParserDiagnostics) []ResourceSummary {
	var details rawDetails
	if len(audit.Details) == 0 || json.Unmarshal(audit.Details, &details) != nil {
		return []ResourceSummary{}
	}
	limit := len(details.Items)
	if limit > MaxResourceSummaries {
		limit = MaxResourceSummaries
		diags.AddWarning(0, ReasonResourceLimit, fmt.Sprintf("kept %d of %d resource summaries", limit, len(details.Items)), "", false)
	}
	out := make([]ResourceSummary, 0, limit)
	for _, item := range details.Items[:limit] {
		var raw rawResourceSummary
		if json.Unmarshal(item, &raw) != nil || strings.TrimSpace(raw.ResourceType) == "" {
			continue
		}
		out = append(out, ResourceSummary{ResourceType: safeIdentifier(policy, raw.ResourceType), RequestCount: nonNegativeInt(raw.RequestCount), TransferBytes: nonNegativeInt64(raw.TransferSize)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ResourceType < out[j].ResourceType })
	return out
}

func warningStrings(values []any, policy *redact.Policy) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			redacted, _ := policy.RedactNamedValue("", text)
			out = append(out, boundedString(redacted, MaxOutputStringBytes))
		}
	}
	return out
}

func safeText(policy *redact.Policy, value string) string {
	redacted, _ := policy.RedactNamedValue("", value)
	return boundedString(redacted, MaxOutputStringBytes)
}

func safeIdentifier(policy *redact.Policy, value string) string {
	return boundedString(safeText(policy, strings.TrimSpace(value)), MaxIdentifierBytes)
}

func validScore(value *float64, id string, diags *diagnostics.ParserDiagnostics) *float64 {
	if value == nil {
		return nil
	}
	if *value < 0 || *value > 1 {
		diags.AddWarning(0, ReasonScoreInvalid, fmt.Sprintf("ignored out-of-range score for %s", boundedString(id, 80)), "", false)
		return nil
	}
	return value
}

func boundedString(value string, limit int) string {
	value = strings.ToValidUTF8(value, "�")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	prefix := value[:limit]
	for len(prefix) > 0 && !utf8.ValidString(prefix) {
		prefix = prefix[:len(prefix)-1]
	}
	return prefix + "…"
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nonNegative(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func nonNegativeInt64(value float64) int64 {
	if value <= 0 {
		return 0
	}
	return int64(value)
}

func nonNegativeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
