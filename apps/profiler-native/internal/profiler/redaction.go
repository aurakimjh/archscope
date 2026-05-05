package profiler

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// RedactionVersion mirrors the Python redaction policy version so debug logs
// from either implementation can be cross-referenced.
const RedactionVersion = "0.1.0"

// RedactionResult is the output of `RedactText`.
type RedactionResult struct {
	Text    string
	Summary map[string]int
}

var (
	redactAuthRE       = regexp.MustCompile(`(?i)(\bAuthorization:\s*)(Bearer|Basic)\s+([^\s"']+)`)
	redactCookieRE     = regexp.MustCompile(`(?i)(\b(?:Cookie|Set-Cookie):\s*)([^"'\n]+)`)
	redactURLRE        = regexp.MustCompile(`(?:https?://[^\s"']+)|(?:/[^\s"']*\?[^\s"']+)`)
	redactEmailRE      = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
	redactPathRE       = regexp.MustCompile(`/(?:Users|home|var|opt|srv|app|data)/[^\s"']+`)
	redactIPv4RE       = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	redactPathNumberRE = regexp.MustCompile(`/(\d{4,})(?:[/?\s"']|$)`)
	redactLongNumberRE = regexp.MustCompile(`\b\d{8,}\b`)
)

// RedactText scrubs likely-sensitive substrings from `value` while preserving
// parseable structure: tokens become `<TOKEN len=N>`, cookies `<COOKIE>`,
// query string values are classified by key (`<TOKEN|NUMBER|QUERY_VALUE
// len=N>`), absolute paths collapse to their leaf, IPv4s become `<IPV4>`,
// long path/number components become `<NUMBER len=N>`.
//
// Mirrors `archscope_engine.common.redaction.redact_text` so cross-engine
// debug logs use the same placeholder shape.
func RedactText(value string) RedactionResult {
	if value == "" {
		return RedactionResult{Text: "", Summary: map[string]int{}}
	}
	summary := map[string]int{}
	text := value
	text = redactAuth(text, summary)
	text = redactCookies(text, summary)
	text = redactURLs(text, summary)
	text = redactEmail(text, summary)
	text = redactAbsolutePaths(text, summary)
	text = redactIPv4(text, summary)
	text = redactPathNumbers(text, summary)
	text = redactLongNumbers(text, summary)
	return RedactionResult{Text: text, Summary: summary}
}

// MergeRedactionSummaries adds counts from multiple `Summary` maps.
func MergeRedactionSummaries(summaries ...map[string]int) map[string]int {
	merged := map[string]int{}
	for _, s := range summaries {
		for k, v := range s {
			merged[k] += v
		}
	}
	return merged
}

func bumpRedaction(summary map[string]int, key string) {
	summary[key]++
}

func placeholder(kind, value string) string {
	return fmt.Sprintf("<%s len=%d>", kind, len(value))
}

func redactAuth(text string, summary map[string]int) string {
	return redactAuthRE.ReplaceAllStringFunc(text, func(match string) string {
		groups := redactAuthRE.FindStringSubmatch(match)
		bumpRedaction(summary, "TOKEN")
		return groups[1] + groups[2] + " " + placeholder("TOKEN", groups[3])
	})
}

func redactCookies(text string, summary map[string]int) string {
	return redactCookieRE.ReplaceAllStringFunc(text, func(match string) string {
		groups := redactCookieRE.FindStringSubmatch(match)
		bumpRedaction(summary, "COOKIE")
		return groups[1] + placeholder("COOKIE", groups[2])
	})
}

func redactURLs(text string, summary map[string]int) string {
	return redactURLRE.ReplaceAllStringFunc(text, func(match string) string {
		return redactURL(match, summary)
	})
}

func redactURL(raw string, summary map[string]int) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.RawQuery == "" {
		return raw
	}
	values, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return raw
	}
	parts := []string{}
	// Keep Go's url.Values iteration deterministic by sorting keys.
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for _, key := range keys {
		for _, value := range values[key] {
			kind := classifyQueryValue(key, value)
			bumpRedaction(summary, kind)
			parts = append(parts, fmt.Sprintf("%s=<%s len=%d>", url.QueryEscape(key), kind, len(value)))
		}
	}
	parsed.RawQuery = strings.Join(parts, "&")
	return parsed.String()
}

func classifyQueryValue(key, value string) string {
	lowered := strings.ToLower(key)
	for _, hint := range []string{"token", "secret", "password", "key", "auth"} {
		if strings.Contains(lowered, hint) {
			return "TOKEN"
		}
	}
	allDigits := value != ""
	for _, r := range value {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return "NUMBER"
	}
	return "QUERY_VALUE"
}

func redactEmail(text string, summary map[string]int) string {
	return redactEmailRE.ReplaceAllStringFunc(text, func(match string) string {
		bumpRedaction(summary, "EMAIL")
		return "<EMAIL>"
	})
}

func redactAbsolutePaths(text string, summary map[string]int) string {
	return redactPathRE.ReplaceAllStringFunc(text, func(match string) string {
		bumpRedaction(summary, "ABSOLUTE_PATH")
		trimmed := strings.TrimRight(match, "/")
		leaf := trimmed
		if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
			leaf = trimmed[idx+1:]
		}
		if leaf == "" {
			return "<PATH>"
		}
		return "<PATH>/" + leaf
	})
}

func redactIPv4(text string, summary map[string]int) string {
	return redactIPv4RE.ReplaceAllStringFunc(text, func(match string) string {
		bumpRedaction(summary, "IPV4")
		return "<IPV4>"
	})
}

func redactPathNumbers(text string, summary map[string]int) string {
	return redactPathNumberRE.ReplaceAllStringFunc(text, func(match string) string {
		groups := redactPathNumberRE.FindStringSubmatch(match)
		bumpRedaction(summary, "NUMBER")
		// Preserve trailing terminator that triggered the lookahead.
		trailer := match[len(groups[0])-1:]
		if trailer == groups[1] {
			trailer = ""
		}
		return fmt.Sprintf("/<NUMBER len=%d>%s", len(groups[1]), trailer)
	})
}

func redactLongNumbers(text string, summary map[string]int) string {
	return redactLongNumberRE.ReplaceAllStringFunc(text, func(match string) string {
		bumpRedaction(summary, "LONG_IDENTIFIER")
		return placeholder("NUMBER", match)
	})
}
