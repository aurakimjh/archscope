package common

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

const IndexRedactionVersion = "0.2.0"

//go:embed assets/text-index-redaction-v2.json
var indexPolicy []byte

var IndexRedactionSHA256 = func() string { sum := sha256.Sum256(indexPolicy); return hex.EncodeToString(sum[:]) }()

type indexRule struct {
	Pattern     string
	Replacement string
	re          *regexp.Regexp
}

var indexMaskedAuth = regexp.MustCompile(`(?i)\bAuthorization:\s*(?:Bearer|Basic)\s+<REDACTED_AUTH>`)

var indexRules = func() []indexRule {
	var policy struct{ Rules []indexRule }
	if err := json.Unmarshal(indexPolicy, &policy); err != nil {
		panic(err)
	}
	for i := range policy.Rules {
		policy.Rules[i].re = regexp.MustCompile(policy.Rules[i].Pattern)
	}
	return policy.Rules
}()

// RedactIndexText is a new, explicitly versioned display/index policy. Legacy
// capture and snapshot redaction retain RedactText and RedactionVersion.
func RedactIndexText(value string) string {
	for _, rule := range indexRules {
		value = rule.re.ReplaceAllString(value, rule.Replacement)
	}
	// Shield the complete normalized header from the legacy token-length rule,
	// which otherwise masks its own placeholder on each cache/display pass.
	headers := []string{}
	value = indexMaskedAuth.ReplaceAllStringFunc(value, func(header string) string {
		headers = append(headers, header)
		return "\x00ARCHSCOPE_MASKED_AUTH_" + strconv.Itoa(len(headers)-1) + "\x00"
	})
	value = RedactText(value).Text
	for i, header := range headers {
		value = strings.ReplaceAll(value, "\x00ARCHSCOPE_MASKED_AUTH_"+strconv.Itoa(i)+"\x00", header)
	}
	return value
}
