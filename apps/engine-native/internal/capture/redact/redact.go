// Package redact applies field-aware redaction to imported HTTP evidence.
// It intentionally does not operate on raw HAR JSON: callers pass parsed URL,
// header, cookie, body, and process fields so secrets cannot survive in an
// unvisited copy of the input tree.
package redact

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const (
	PolicyVersion          = "har_redaction_1.0.0"
	DefaultMaxPatterns     = 32
	DefaultMaxPatternBytes = 512
	DefaultMaxScanBytes    = 1 << 20
)

type Options struct {
	CustomPatterns  []string
	MaxPatterns     int
	MaxPatternBytes int
	MaxScanBytes    int
	RuleTimeLimit   time.Duration
}

type Warning struct {
	Code    string
	Message string
}

type Summary struct {
	Applied bool           `json:"applied"`
	Version string         `json:"version"`
	Rules   []string       `json:"rules"`
	Counts  map[string]int `json:"counts"`
}

type customRule struct {
	name     string
	compiled *regexp.Regexp
	disabled bool
}

type Policy struct {
	maxScanBytes  int
	ruleTimeLimit time.Duration
	custom        []customRule
	counts        map[string]int
	warnings      []Warning
}

var (
	bearerPattern      = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
	jwtPattern         = regexp.MustCompile(`\b[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{4,}\b`)
	awsKeyPattern      = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
	assignmentPattern  = regexp.MustCompile(`(?i)(password|passwd|pwd|access[_-]?token|refresh[_-]?token|api[_-]?key|service[_-]?key|client[_-]?secret|private[_-]?key|credential|secret|signature|credit[_-]?card|ssn)(\s*[=:]\s*)([^&\s,;"'}]+)`)
	cliArgumentPattern = regexp.MustCompile(`(?i)(--?(?:password|passwd|pwd|access[_-]?token|refresh[_-]?token|api[_-]?key|service[_-]?key|client[_-]?secret|secret|signature))(\s+)([^\s]+)`)
)

func NewPolicy(opts Options) *Policy {
	maxPatterns := boundedPositive(opts.MaxPatterns, DefaultMaxPatterns)
	maxPatternBytes := boundedPositive(opts.MaxPatternBytes, DefaultMaxPatternBytes)
	maxScanBytes := boundedPositive(opts.MaxScanBytes, DefaultMaxScanBytes)
	timeLimit := opts.RuleTimeLimit
	if timeLimit <= 0 {
		timeLimit = 50 * time.Millisecond
	}
	p := &Policy{
		maxScanBytes:  maxScanBytes,
		ruleTimeLimit: timeLimit,
		custom:        []customRule{},
		counts:        map[string]int{},
		warnings:      []Warning{},
	}
	for i, pattern := range opts.CustomPatterns {
		if i >= maxPatterns {
			p.warnings = append(p.warnings, Warning{Code: "HAR_REDACTION_RULE_DISABLED", Message: fmt.Sprintf("custom redaction rule cap %d reached", maxPatterns)})
			break
		}
		if len(pattern) == 0 || len(pattern) > maxPatternBytes {
			p.warnings = append(p.warnings, Warning{Code: "HAR_REDACTION_RULE_DISABLED", Message: fmt.Sprintf("custom redaction rule %d has invalid length", i+1)})
			continue
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			p.warnings = append(p.warnings, Warning{Code: "HAR_REDACTION_RULE_DISABLED", Message: fmt.Sprintf("custom redaction rule %d is invalid", i+1)})
			continue
		}
		p.custom = append(p.custom, customRule{name: fmt.Sprintf("custom_%d", i+1), compiled: compiled})
	}
	return p
}

func (p *Policy) Warnings() []Warning {
	return append([]Warning(nil), p.warnings...)
}

func (p *Policy) Summary() Summary {
	rules := make([]string, 0, len(p.counts))
	counts := make(map[string]int, len(p.counts))
	for rule, count := range p.counts {
		if count <= 0 {
			continue
		}
		rules = append(rules, rule)
		counts[rule] = count
	}
	sort.Strings(rules)
	return Summary{Applied: len(rules) > 0, Version: PolicyVersion, Rules: rules, Counts: counts}
}

func (p *Policy) RedactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return p.applyText(raw)
	}
	if parsed.User != nil {
		parsed.User = nil
		p.bump("url_user_info")
	}
	query := parsed.Query()
	for key, values := range query {
		for i, value := range values {
			if sensitiveKey(key) {
				values[i] = "[REDACTED]"
				p.bump("query_value")
			} else {
				values[i] = p.applyText(value)
			}
		}
		query[key] = values
	}
	parsed.RawQuery = query.Encode()
	return p.applyText(parsed.String())
}

func (p *Policy) RedactHeaders(headers []models.HeaderField) []models.HeaderField {
	out := make([]models.HeaderField, 0, len(headers))
	for _, header := range headers {
		next := header
		if sensitiveHeader(header.Name) {
			next.Value = "[REDACTED]"
			next.Redacted = true
			p.bump("header_value")
		} else {
			redacted := p.applyText(header.Value)
			next.Redacted = redacted != header.Value
			next.Value = redacted
		}
		out = append(out, next)
	}
	return out
}

func (p *Policy) RedactNamedValue(name, value string) (string, bool) {
	if sensitiveKey(name) {
		p.bump("named_value")
		return "[REDACTED]", true
	}
	redacted := p.applyText(value)
	return redacted, redacted != value
}

func (p *Policy) RedactBody(mimeType, text string) (string, bool) {
	if text == "" {
		return "", false
	}
	if !isTextMIME(mimeType) {
		p.bump("non_text_body_omitted")
		return "", true
	}
	limited := text
	if len(limited) > p.maxScanBytes {
		limited = limited[:p.maxScanBytes]
		p.bump("body_scan_truncated")
	}
	trimmed := strings.TrimSpace(limited)
	if strings.Contains(strings.ToLower(mimeType), "json") || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var value any
		if json.Unmarshal([]byte(trimmed), &value) == nil {
			before, _ := json.Marshal(value)
			p.redactJSON(&value, "")
			after, _ := json.Marshal(value)
			result := p.applyText(string(after))
			return result, string(before) != result
		}
	}
	if strings.Contains(strings.ToLower(mimeType), "x-www-form-urlencoded") {
		if values, err := url.ParseQuery(trimmed); err == nil {
			changed := false
			for key, items := range values {
				for i, item := range items {
					items[i], changed = p.redactValueWithChange(key, item, changed)
				}
				values[key] = items
			}
			return p.applyText(values.Encode()), changed
		}
	}
	result := p.applyText(limited)
	return result, result != text
}

func (p *Policy) RedactProcess(process *models.ProcessInstance) *models.ProcessInstance {
	if process == nil {
		return nil
	}
	next := *process
	if next.ExecPath != "" {
		base := filepath.Base(next.ExecPath)
		if base != next.ExecPath {
			next.ExecPath = ".../" + base
			p.bump("process_path")
		}
	}
	if next.CommandLine != "" {
		next.CommandLine = p.applyText(next.CommandLine)
		p.bump("process_command_line")
	}
	if next.User != "" {
		next.User = ""
		p.bump("process_user")
	}
	return &next
}

func (p *Policy) redactJSON(value *any, key string) {
	switch typed := (*value).(type) {
	case map[string]any:
		for childKey, child := range typed {
			if sensitiveKey(childKey) {
				typed[childKey] = "[REDACTED]"
				p.bump("body_field")
				continue
			}
			p.redactJSON(&child, childKey)
			typed[childKey] = child
		}
	case []any:
		for i := range typed {
			child := typed[i]
			p.redactJSON(&child, key)
			typed[i] = child
		}
	case string:
		*value = p.applyText(typed)
	}
}

func (p *Policy) redactValueWithChange(key, value string, changed bool) (string, bool) {
	redacted, itemChanged := p.RedactNamedValue(key, value)
	return redacted, changed || itemChanged
}

func (p *Policy) applyText(value string) string {
	if value == "" {
		return value
	}
	value = replaceAndCount(value, bearerPattern, "Bearer [REDACTED]", func(n int) { p.bumpN("bearer", n) })
	value = replaceAndCount(value, jwtPattern, "[REDACTED_JWT]", func(n int) { p.bumpN("jwt", n) })
	value = replaceAndCount(value, awsKeyPattern, "[REDACTED_AWS_KEY]", func(n int) { p.bumpN("aws_key", n) })
	matches := assignmentPattern.FindAllStringSubmatchIndex(value, -1)
	if len(matches) > 0 {
		value = assignmentPattern.ReplaceAllString(value, `${1}${2}[REDACTED]`)
		p.bumpN("key_value", len(matches))
	}
	cliMatches := cliArgumentPattern.FindAllStringSubmatchIndex(value, -1)
	if len(cliMatches) > 0 {
		value = cliArgumentPattern.ReplaceAllString(value, `${1}${2}[REDACTED]`)
		p.bumpN("cli_argument", len(cliMatches))
	}
	return p.applyCustom(value)
}

func (p *Policy) applyCustom(value string) string {
	if value == "" || len(p.custom) == 0 {
		return value
	}
	limitedLen := len(value)
	truncated := false
	if limitedLen > p.maxScanBytes {
		limitedLen = p.maxScanBytes
		truncated = true
	}
	prefix := value[:limitedLen]
	for i := range p.custom {
		rule := &p.custom[i]
		if rule.disabled {
			continue
		}
		started := time.Now()
		matches := rule.compiled.FindAllStringIndex(prefix, -1)
		if len(matches) > 0 {
			prefix = rule.compiled.ReplaceAllString(prefix, "[REDACTED_CUSTOM]")
			p.bumpN(rule.name, len(matches))
		}
		if time.Since(started) > p.ruleTimeLimit {
			rule.disabled = true
			p.warnings = append(p.warnings, Warning{Code: "HAR_REDACTION_RULE_DISABLED", Message: rule.name + " exceeded its execution budget"})
		}
	}
	if truncated {
		// Once custom rules are configured, retaining an unscanned suffix could
		// preserve a secret. Drop it instead of pretending it was inspected.
		p.bump("custom_scan_truncated")
		return prefix + "[TRUNCATED]"
	}
	return prefix
}

func (p *Policy) bump(rule string) {
	p.bumpN(rule, 1)
}

func (p *Policy) bumpN(rule string, count int) {
	if count > 0 {
		p.counts[rule] += count
	}
}

func replaceAndCount(value string, pattern *regexp.Regexp, replacement string, count func(int)) string {
	matches := pattern.FindAllStringIndex(value, -1)
	count(len(matches))
	if len(matches) == 0 {
		return value
	}
	return pattern.ReplaceAllString(value, replacement)
}

func sensitiveHeader(name string) bool {
	normalized := normalizedKey(name)
	switch normalized {
	case "authorization", "proxyauthorization", "cookie", "setcookie", "xapikey", "xauthtoken", "apikey":
		return true
	default:
		return sensitiveNormalizedKey(strings.TrimPrefix(normalized, "x"))
	}
}

func sensitiveKey(name string) bool {
	return sensitiveNormalizedKey(normalizedKey(name))
}

func sensitiveNormalizedKey(normalized string) bool {
	switch normalized {
	case "auth", "authorization", "token", "accesstoken", "refreshtoken", "apikey", "servicekey", "password", "passwd", "pwd", "secret", "signature", "code", "cookie", "setcookie", "creditcard", "ssn", "session":
		return true
	}
	return strings.HasSuffix(normalized, "password") ||
		strings.HasSuffix(normalized, "passwd") ||
		strings.HasSuffix(normalized, "token") ||
		strings.HasSuffix(normalized, "secret") ||
		strings.HasSuffix(normalized, "signature") ||
		strings.HasSuffix(normalized, "privatekey") ||
		strings.HasSuffix(normalized, "credential") ||
		normalized == "awssecretaccesskey"
}

func normalizedKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("-", "", "_", "", ".", "", " ", "").Replace(value)
	return value
}

func isTextMIME(value string) bool {
	value = strings.ToLower(value)
	return value == "" || strings.HasPrefix(value, "text/") || strings.Contains(value, "json") || strings.Contains(value, "xml") || strings.Contains(value, "javascript") || strings.Contains(value, "x-www-form-urlencoded") || strings.Contains(value, "graphql")
}

func boundedPositive(value, fallback int) int {
	if value <= 0 || value > fallback {
		return fallback
	}
	return value
}
