package profileclassification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Mirror tests/test_profiler_analyzer.py:
//
//   - test_profile_classification_supports_common_runtime_families
//   - test_profile_classification_uses_application_fallback
//   - test_profile_classification_rule_order_prefers_specific_rules
//   - test_profile_classification_is_case_insensitive
//   - test_profile_classification_loads_packaged_rules
//   - test_profile_classification_loads_external_config

func TestClassifyStackSupportsCommonRuntimeFamilies(t *testing.T) {
	cases := []struct {
		stack string
		want  string
	}{
		{"java.lang.Thread;com.example.Service", "JVM"},
		{"node:internal;node_modules/app/index.js", "Node.js"},
		{"python;site-packages/django/core", "Python"},
		{"runtime.goexit;main.handle /app/main.go:42", "Go"},
		{"System.Web;Microsoft.Data.SqlClient", "ASP.NET / .NET"},
	}
	for _, c := range cases {
		if got := ClassifyStack(c.stack, nil); got != c.want {
			t.Errorf("ClassifyStack(%q) = %q, want %q", c.stack, got, c.want)
		}
	}
}

func TestClassifyStackUsesApplicationFallback(t *testing.T) {
	for _, stack := range []string{
		"com.mycompany.internal.Worker",
		"com.mycompany.HttpClientFacade",
	} {
		if got := ClassifyStack(stack, nil); got != FallbackLabel {
			t.Errorf("ClassifyStack(%q) = %q, want %q", stack, got, FallbackLabel)
		}
	}
}

func TestClassifyStackRuleOrderPrefersSpecificRules(t *testing.T) {
	// "Spring Batch" must beat "Spring Framework" because it sits
	// earlier in the rule list — the first match wins.
	stack := "org.springframework.batch.core.Job;org.springframework.Service"
	if got := ClassifyStack(stack, nil); got != "Spring Batch" {
		t.Errorf("ClassifyStack(%q) = %q, want %q", stack, got, "Spring Batch")
	}
}

func TestClassifyStackIsCaseInsensitive(t *testing.T) {
	stack := "JAVA.LANG.THREAD;COM.EXAMPLE.SERVICE"
	if got := ClassifyStack(stack, nil); got != "JVM" {
		t.Errorf("ClassifyStack(%q) = %q, want %q", stack, got, "JVM")
	}
}

func TestLoadPackagedRulesMatchesBuiltin(t *testing.T) {
	rules, err := LoadPackagedRules()
	if err != nil {
		t.Fatalf("LoadPackagedRules: %v", err)
	}
	if len(rules) != len(BuiltinRules) {
		t.Fatalf("len(rules)=%d, len(BuiltinRules)=%d", len(rules), len(BuiltinRules))
	}
	for i, want := range BuiltinRules {
		got := rules[i]
		if got.Label != want.Label {
			t.Errorf("rule[%d] label = %q, want %q", i, got.Label, want.Label)
		}
		if len(got.Contains) != len(want.Contains) {
			t.Errorf("rule[%d] tokens = %v, want %v", i, got.Contains, want.Contains)
			continue
		}
		for j, token := range want.Contains {
			if got.Contains[j] != token {
				t.Errorf("rule[%d].Contains[%d] = %q, want %q", i, j, got.Contains[j], token)
			}
		}
	}
	// Spot-check parity with Python test (oracle.jdbc → Oracle JDBC)
	if got := ClassifyStack("oracle.jdbc.driver.OracleDriver", rules); got != "Oracle JDBC" {
		t.Errorf("ClassifyStack(oracle.jdbc.*) = %q, want Oracle JDBC", got)
	}
}

func TestLoadRulesFromExternalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime_rules.json")
	body := `[{"label": "Vendor Runtime", "contains": ["vendor.special"]}]`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	rules, err := LoadRules(path)
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	if got := ClassifyStack("vendor.special.Handler", rules); got != "Vendor Runtime" {
		t.Errorf("ClassifyStack with external rules = %q, want Vendor Runtime", got)
	}
}

func TestLoadRulesFileNotFound(t *testing.T) {
	if _, err := LoadRules(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("LoadRules(missing) returned nil error")
	}
}

func TestParseRulesNormalisesTokensToLowercase(t *testing.T) {
	body := []byte(`[{"label": "Custom", "contains": ["VENDOR.Special", "FooBar"]}]`)
	rules, err := ParseRules(body)
	if err != nil {
		t.Fatalf("ParseRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
	wantTokens := []string{"vendor.special", "foobar"}
	for i, want := range wantTokens {
		if rules[0].Contains[i] != want {
			t.Errorf("Contains[%d] = %q, want %q", i, rules[0].Contains[i], want)
		}
	}
	// The original-case label is preserved.
	if rules[0].Label != "Custom" {
		t.Errorf("Label = %q, want Custom", rules[0].Label)
	}
}

func TestParseRulesRejectsNonArray(t *testing.T) {
	bodies := []string{
		`{"label": "X", "contains": ["y"]}`,
		`"a string"`,
		`null`,
		`42`,
	}
	for _, body := range bodies {
		_, err := ParseRules([]byte(body))
		if err == nil {
			t.Errorf("ParseRules(%q) returned nil error", body)
			continue
		}
		if !strings.Contains(err.Error(), "must be a JSON array") {
			t.Errorf("ParseRules(%q) err = %v, want JSON-array error", body, err)
		}
	}
}

func TestParseRulesRejectsNonObjectItem(t *testing.T) {
	bodies := []string{
		`["not-an-object"]`,
		`[42]`,
		`[null]`,
		`[["nested-array"]]`,
	}
	for _, body := range bodies {
		_, err := ParseRules([]byte(body))
		if err == nil {
			t.Errorf("ParseRules(%q) returned nil error", body)
			continue
		}
		if !strings.Contains(err.Error(), "must be an object") {
			t.Errorf("ParseRules(%q) err = %v, want object error", body, err)
		}
	}
}

func TestParseRulesRejectsBlankLabel(t *testing.T) {
	bodies := []string{
		`[{"label": "", "contains": ["x"]}]`,
		`[{"label": "   ", "contains": ["x"]}]`,
		`[{"contains": ["x"]}]`,
	}
	for _, body := range bodies {
		_, err := ParseRules([]byte(body))
		if err == nil {
			t.Errorf("ParseRules(%q) returned nil error", body)
			continue
		}
		if !strings.Contains(err.Error(), "non-empty label") {
			t.Errorf("ParseRules(%q) err = %v, want label error", body, err)
		}
	}
}

func TestParseRulesRejectsMissingOrEmptyContains(t *testing.T) {
	bodies := []string{
		`[{"label": "X", "contains": []}]`,
		`[{"label": "X"}]`,
	}
	for _, body := range bodies {
		_, err := ParseRules([]byte(body))
		if err == nil {
			t.Errorf("ParseRules(%q) returned nil error", body)
			continue
		}
		if !strings.Contains(err.Error(), "contains tokens") {
			t.Errorf("ParseRules(%q) err = %v, want contains error", body, err)
		}
	}
}

func TestParseRulesRejectsBlankToken(t *testing.T) {
	body := `[{"label": "X", "contains": ["valid", "   "]}]`
	_, err := ParseRules([]byte(body))
	if err == nil {
		t.Fatal("ParseRules with blank token returned nil error")
	}
	if !strings.Contains(err.Error(), "invalid token") {
		t.Errorf("err = %v, want invalid-token error", err)
	}
}

func TestClassifyStackEmptyStackHitsFallback(t *testing.T) {
	if got := ClassifyStack("", nil); got != FallbackLabel {
		t.Errorf("ClassifyStack(\"\") = %q, want %q", got, FallbackLabel)
	}
}

func TestClassifyStackEmptyRulesHitsFallback(t *testing.T) {
	if got := ClassifyStack("java.lang.Thread", []Rule{}); got != FallbackLabel {
		t.Errorf("ClassifyStack with empty rules = %q, want %q", got, FallbackLabel)
	}
}

func TestClassifyStackHTTPNetworkRule(t *testing.T) {
	// HTTP/Network sits before JVM, so a stack hitting both should
	// resolve to HTTP/Network.
	stack := "java.net.SocketInputStream.read;java.lang.Thread.run"
	if got := ClassifyStack(stack, nil); got != "HTTP / Network" {
		t.Errorf("ClassifyStack(%q) = %q, want HTTP / Network", stack, got)
	}
}

func TestDefaultRulesIsImmutableFromBuiltinPerspective(t *testing.T) {
	// Mutating LoadPackagedRules' result must not poison
	// DefaultRules — they are independent slices.
	rules, err := LoadPackagedRules()
	if err != nil {
		t.Fatalf("LoadPackagedRules: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected non-empty rule set")
	}
	rules[0].Label = "MUTATED"
	if DefaultRules[0].Label == "MUTATED" {
		t.Error("mutation leaked into DefaultRules")
	}
}
