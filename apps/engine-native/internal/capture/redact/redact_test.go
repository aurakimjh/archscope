package redact

import (
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestPolicyRedactsEverySupportedFieldClass(t *testing.T) {
	policy := NewPolicy(Options{})
	url := policy.RedactURL("https://user:EXAMPLE-URL-PASSWORD@example.test/login?api_key=EXAMPLE-KEY&note=Bearer+abc.def.ghi#client_secret=EXAMPLE-FRAGMENT")
	headers := policy.RedactHeaders([]models.HeaderField{{Name: "Authorization", Value: "Bearer abc.def.ghi"}, {Name: "X-Client-Secret", Value: "EXAMPLE-HEADER"}})
	cookie, cookieChanged := policy.RedactNamedValue("cookie", "EXAMPLE-COOKIE")
	body, bodyChanged := policy.RedactBody("application/json", `{"password":"EXAMPLE-PASSWORD","nested":{"service_key":"EXAMPLE-SERVICE"},"note":"Bearer abc.def.ghi"}`)
	process := policy.RedactProcess(&models.ProcessInstance{ExecPath: "/Users/example/bin/client", CommandLine: "client --password EXAMPLE-CLI-PASSWORD", User: "example"})
	joined := url + headers[0].Value + headers[1].Value + cookie + body + process.ExecPath + process.CommandLine + process.User
	for _, secret := range []string{"EXAMPLE-URL-PASSWORD", "EXAMPLE-FRAGMENT", "EXAMPLE-KEY", "EXAMPLE-HEADER", "EXAMPLE-PASSWORD", "EXAMPLE-CLI-PASSWORD", "EXAMPLE-COOKIE", "EXAMPLE-SERVICE", "abc.def.ghi", "/Users/example"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("secret survived redaction: %q in %q", secret, joined)
		}
	}
	if !cookieChanged || !bodyChanged || !policy.Summary().Applied {
		t.Fatalf("redaction summary incomplete: %+v", policy.Summary())
	}
}

func TestCustomRulesAreBoundedAndDropUnscannedSuffix(t *testing.T) {
	policy := NewPolicy(Options{CustomPatterns: []string{`CUSTOM-[0-9]+`}, MaxScanBytes: 16})
	value := policy.applyText("CUSTOM-1234567890-SECRET-BEYOND-SCAN")
	if strings.Contains(value, "SECRET-BEYOND-SCAN") || !strings.HasSuffix(value, "[TRUNCATED]") {
		t.Fatalf("unscanned suffix was retained: %q", value)
	}
	if policy.Summary().Counts["custom_scan_truncated"] != 1 {
		t.Fatalf("missing truncation accounting: %+v", policy.Summary())
	}
}

func TestInvalidCustomRuleIsDisabled(t *testing.T) {
	policy := NewPolicy(Options{CustomPatterns: []string{"["}})
	if len(policy.Warnings()) != 1 || policy.Warnings()[0].Code != "HAR_REDACTION_RULE_DISABLED" {
		t.Fatalf("invalid rule was not disabled: %+v", policy.Warnings())
	}
}

func TestFreeTextChannelsUseStructuredSensitiveKeyCoverage(t *testing.T) {
	policy := NewPolicy(Options{})
	body, changed := policy.RedactBody("text/plain", "token=BODY-TOKEN sessionId=BODY-SESSION code=BODY-CODE authToken=BODY-AUTH card=4111-1111-1111-1111")
	fragment := policy.RedactURL("https://example.test/#token=FRAGMENT-TOKEN&session=FRAGMENT-SESSION&code=FRAGMENT-CODE")
	headers := policy.RedactHeaders([]models.HeaderField{{Name: "X-Debug", Value: "authToken=HEADER-AUTH sessionId=HEADER-SESSION"}})
	combined := body + fragment + headers[0].Value
	for _, secret := range []string{"BODY-TOKEN", "BODY-SESSION", "BODY-CODE", "BODY-AUTH", "4111-1111-1111-1111", "FRAGMENT-TOKEN", "FRAGMENT-SESSION", "FRAGMENT-CODE", "HEADER-AUTH", "HEADER-SESSION"} {
		if strings.Contains(combined, secret) {
			t.Fatalf("free-text secret survived redaction: %q in %q", secret, combined)
		}
	}
	if !changed || !headers[0].Redacted || policy.Summary().Counts["payment_card"] != 1 {
		t.Fatalf("free-text redaction accounting incomplete: headers=%+v summary=%+v", headers, policy.Summary())
	}
}

func TestPaymentCardRequiresValidLuhnChecksum(t *testing.T) {
	policy := NewPolicy(Options{})
	value := policy.applyText("valid 4111 1111 1111 1111 invalid 4111 1111 1111 1112")
	if strings.Contains(value, "4111 1111 1111 1111") {
		t.Fatalf("valid card was not redacted: %q", value)
	}
	if !strings.Contains(value, "4111 1111 1111 1112") {
		t.Fatalf("invalid checksum was over-redacted: %q", value)
	}
}
