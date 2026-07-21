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
