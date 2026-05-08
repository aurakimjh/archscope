// ─────────────────────────────────────────────────────────────────────
// [한글] redaction — debug log / 진단 sample 의 민감정보 마스킹.
//
// 책임/목적
//   parser/analyzer 가 진단 sample 에 raw 입력을 일부 포함시키므로
//   토큰/쿠키/URL/이메일/경로/IP/긴 숫자 같은 잠재적 민감 정보를 안전한
//   placeholder 로 치환한다. Python 측 archscope_engine.common.redaction
//   과 동일한 정책 + 동일한 placeholder 형식 (cross-engine debug log 참조용).
//
// 마스킹 규칙 (적용 순서)
//   1) Authorization: Bearer/Basic <TOKEN> → "<TOKEN len=N>"
//   2) Cookie / Set-Cookie 값 → "<COOKIE len=N>"
//   3) URL query string → key 별 분류 후 "<TOKEN|NUMBER|QUERY_VALUE len=N>"
//      (key 에 token/secret/password/key/auth 들어있으면 TOKEN, 전부 숫자
//      면 NUMBER, 그 외 QUERY_VALUE)
//   4) 이메일 → "<EMAIL>"
//   5) 절대 경로 (/Users /home /var /opt /srv /app /data) → "<PATH>/leaf"
//   6) IPv4 → "<IPV4>"
//   7) /숫자4자리이상 (path 내) → "/<NUMBER len=N>"
//   8) 8자리 이상 숫자 → "<NUMBER len=N>"
//
// 트리키한 부분
//   • placeholder 의 "len=N" 은 byte length 이며 Python 측과 동일.
//   • redactPathNumbers 는 lookahead 가 흉내 내기 어려워 trailing 1글자를
//     보존하는 트릭 사용 (Python 정규식 동작 모사).
//   • url.Values 의 iteration 순서가 비결정적이라 keys 정렬 후 출력해
//     redacted URL 의 byte-level parity 를 보장.
//   • RedactionVersion 상수는 Python 정책 버전과 매칭되어야 함 (현재 0.1.0).
// ─────────────────────────────────────────────────────────────────────

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
//
// [한글] RedactText — 8단계 정규식 치환을 순차 적용하며 summary 카운터에
// 카테고리별 발생 횟수 누적. 빈 문자열은 즉시 반환. 정책 순서는 Python 과 동일.
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
