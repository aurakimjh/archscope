// Package profileclassification ports
// archscope_engine.analyzers.profile_classification.
//
// It assigns a runtime-family label (e.g. "JVM", "Node.js",
// "Spring Framework") to a collapsed-stack string by scanning each
// rule's tokens for a case-insensitive substring match. The first
// matching rule wins; if nothing matches the fallback label
// "Application" is returned.
//
// Rules are config-driven: the canonical rule set ships as a JSON
// document inside the Python package
// (`archscope_engine/config/runtime_classification_rules.json`).
// The Go port embeds the same file via `embed` so the binary stays
// self-contained — no module path lookup, no working-directory
// assumptions, byte-for-byte parity with Python's
// `importlib.resources` loader.
//
// Three loaders mirror the Python public API:
//
//   - LoadPackagedRules — read the embedded JSON
//   - LoadRules — read a user-supplied JSON file (parity with
//     `load_stack_classification_rules(path)`)
//   - ParseRules — parse already-decoded JSON bytes
//
// DefaultRules is populated at package init from the embedded JSON
// (parity with Python's module-level
// `DEFAULT_STACK_CLASSIFICATION_RULES`). BuiltinRules is the same
// list expressed as a Go literal — kept around for callers that
// want to bypass the JSON path entirely (e.g. tests, embedded
// builds that strip resources).
package profileclassification

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// FallbackLabel matches Python's "Application" return when no
// rule matches.
const FallbackLabel = "Application"

// Rule mirrors Python's `StackClassificationRule` dataclass. Tokens
// are stored lower-cased so ClassifyStack only needs to lower-case
// the haystack once per call.
type Rule struct {
	Label    string
	Contains []string
}

// BuiltinRules is the Go-literal mirror of the JSON shipped under
// archscope_engine/config/runtime_classification_rules.json. Order
// matches Python's `BUILTIN_STACK_CLASSIFICATION_RULES` exactly —
// rule order is part of the contract because the first match wins.
//
// Tokens are stored lower-cased to match how the JSON loader
// normalises them (`token.lower()` in Python).
var BuiltinRules = []Rule{
	{Label: "Oracle JDBC", Contains: []string{"oracle.jdbc"}},
	{Label: "Spring Batch", Contains: []string{"springframework.batch"}},
	{Label: "Spring Framework", Contains: []string{"springframework"}},
	{Label: "Node.js", Contains: []string{"node:", "node_modules", "v8::", "uv_"}},
	{Label: "Python", Contains: []string{"python", "site-packages", ".py:"}},
	{Label: "Go", Contains: []string{"runtime.", "goroutine", ".go:"}},
	{Label: "ASP.NET / .NET", Contains: []string{"system.web", "system.net", "microsoft.", ".dll"}},
	{
		Label: "HTTP / Network",
		Contains: []string{
			"java.net.socket",
			"java.net.http",
			"sun.nio.ch.socketchannel",
			"okhttp3.",
			"org.apache.http.",
			"http.client",
			"urllib3.",
			"requests.sessions",
			"net/http",
			"system.net.http",
		},
	},
	{Label: "JVM", Contains: []string{"java.", "javax.", "jdk.", "sun."}},
}

//go:embed config/runtime_classification_rules.json
var packagedRulesJSON []byte

// DefaultRules is loaded from the embedded JSON at package init.
// Mirrors Python's module-level
// `DEFAULT_STACK_CLASSIFICATION_RULES = load_packaged_stack_classification_rules()`.
// If embedded JSON loading fails (which would mean the build is
// broken — the `embed` directive guarantees the bytes are present)
// the package falls back to BuiltinRules so callers never see nil.
var DefaultRules []Rule

func init() {
	rules, err := ParseRules(packagedRulesJSON)
	if err != nil {
		// Fall back so library consumers never crash at import. A
		// broken embed is a build-time bug, not a runtime one.
		DefaultRules = cloneRules(BuiltinRules)
		return
	}
	DefaultRules = rules
}

// ClassifyStack returns the rule label that matches `stack`, or
// FallbackLabel ("Application") if nothing matches. When `rules`
// is nil the package-level DefaultRules are used — same default
// as Python's `classify_stack(stack, rules=None)`.
//
// Matching is a case-insensitive substring scan. Python uses
// `str.casefold()` for unicode-correct lowering; Go has no direct
// equivalent, but every token in the shipped rule set is plain
// ASCII so `strings.ToLower` produces identical results.
func ClassifyStack(stack string, rules []Rule) string {
	if rules == nil {
		rules = DefaultRules
	}
	lowered := strings.ToLower(stack)
	for _, rule := range rules {
		for _, token := range rule.Contains {
			if strings.Contains(lowered, token) {
				return rule.Label
			}
		}
	}
	return FallbackLabel
}

// LoadRules reads a JSON file from disk and returns the parsed
// rule set. Parity with Python's
// `load_stack_classification_rules(path)`.
func LoadRules(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("profileclassification: read rules file: %w", err)
	}
	return ParseRules(data)
}

// LoadPackagedRules returns a fresh copy of the rules baked into
// the binary at build time. Parity with Python's
// `load_packaged_stack_classification_rules()`.
//
// We re-parse rather than returning DefaultRules so callers can
// mutate the result without affecting the package-level default.
func LoadPackagedRules() ([]Rule, error) {
	return ParseRules(packagedRulesJSON)
}

// ParseRules decodes a JSON document and validates the rule shape.
// Mirrors Python's `parse_stack_classification_rules` — including
// its error messages, which downstream code may match on.
func ParseRules(data []byte) ([]Rule, error) {
	// Use json.RawMessage at the top level so we can distinguish
	// "not a JSON array" from "valid array of garbage". Python's
	// loader raises ValueError for both but with different
	// messages.
	if !looksLikeJSONArray(data) {
		return nil, errors.New("Runtime classification rules must be a JSON array.")
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, errors.New("Runtime classification rules must be a JSON array.")
	}

	rules := make([]Rule, 0, len(raw))
	for index, item := range raw {
		var entry struct {
			Label    *string  `json:"label"`
			Contains []string `json:"contains"`
		}
		// json.Unmarshal will accept arrays as well as objects when
		// the target is `any`; constraining `entry` to a struct
		// rejects non-object items the same way Python's
		// `isinstance(item, dict)` does.
		if err := json.Unmarshal(item, &entry); err != nil {
			return nil, fmt.Errorf("Rule at index %d must be an object.", index)
		}

		// Reject `null` and bare arrays — `entry` would still be
		// zero-valued in those cases, and we want the same
		// "must be an object" error Python raises.
		if !looksLikeJSONObject(item) {
			return nil, fmt.Errorf("Rule at index %d must be an object.", index)
		}

		if entry.Label == nil || strings.TrimSpace(*entry.Label) == "" {
			return nil, fmt.Errorf("Rule at index %d must include a non-empty label.", index)
		}

		if len(entry.Contains) == 0 {
			return nil, fmt.Errorf("Rule at index %d must include contains tokens.", index)
		}

		tokens := make([]string, 0, len(entry.Contains))
		for _, token := range entry.Contains {
			if strings.TrimSpace(token) == "" {
				return nil, fmt.Errorf("Rule at index %d contains an invalid token.", index)
			}
			tokens = append(tokens, strings.ToLower(token))
		}

		rules = append(rules, Rule{Label: *entry.Label, Contains: tokens})
	}

	return rules, nil
}

// looksLikeJSONObject reports whether `data` is the textual form
// of a JSON object (after leading whitespace). We use it to reject
// JSON arrays / scalars / null at positions Python's `isinstance`
// check would catch.
func looksLikeJSONObject(data json.RawMessage) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{':
			return true
		default:
			return false
		}
	}
	return false
}

// looksLikeJSONArray is the array-side counterpart. Needed because
// `json.Unmarshal([]byte("null"), &slice)` succeeds with a nil
// slice — but Python's `isinstance(value, list)` returns False for
// None, so we need to reject that case explicitly to keep error
// messages aligned.
func looksLikeJSONArray(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '[':
			return true
		default:
			return false
		}
	}
	return false
}

// cloneRules returns a deep copy so downstream mutations don't
// reach BuiltinRules / DefaultRules.
func cloneRules(in []Rule) []Rule {
	out := make([]Rule, len(in))
	for i, rule := range in {
		tokens := make([]string, len(rule.Contains))
		copy(tokens, rule.Contains)
		out[i] = Rule{Label: rule.Label, Contains: tokens}
	}
	return out
}
