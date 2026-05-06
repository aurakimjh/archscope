package javajstack

import (
	"regexp"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// T-194: aitop's `cleanProxyClassNames` (Java/Spring AOP-aware).
//
// The proxy patterns strip dynamic suffixes while keeping the rest of
// the qualified identifier intact. Order matters — the longest-prefix
// variants must be tried before the shorter ones.

type proxyRule struct {
	Pattern     *regexp.Regexp
	Replacement string
}

var proxyPatterns = []proxyRule{
	{regexp.MustCompile(`\$\$EnhancerByCGLIB\$\$[\w$]+`), ""},
	{regexp.MustCompile(`\$\$FastClassByCGLIB\$\$[\w$]+`), ""},
	{regexp.MustCompile(`(\$\$Proxy)\d+`), `$1`},
	{regexp.MustCompile(`(GeneratedMethodAccessor)\d+`), `$1`},
	{regexp.MustCompile(`(GeneratedConstructorAccessor)\d+`), `$1`},
	// Generic "Accessor<digits>" catch-all for SerializedLambda, JDK
	// proxy accessor classes, etc. Must come after the specific
	// Generated* patterns so we don't strip the prefix twice.
	{regexp.MustCompile(`(Accessor)\d+`), `$1`},
}

// dotCollapseRE collapses leftover ".." artifacts when EnhancerByCGLIB
// was strip-replaced with an empty string in the middle of an
// identifier.
var dotCollapseRE = regexp.MustCompile(`\.{2,}`)

// normalizeProxyText applies every proxy pattern and tidies up the
// leftover dot artifacts.
func normalizeProxyText(text string) string {
	for _, rule := range proxyPatterns {
		text = rule.Pattern.ReplaceAllString(text, rule.Replacement)
	}
	text = dotCollapseRE.ReplaceAllString(text, ".")
	return strings.Trim(text, ".")
}

// normalizeProxyFrame is the Java-only equivalent of Python's
// _normalize_proxy_frame. Non-Java frames pass through untouched so the
// registry can mix this plugin with other-runtime parsers without
// stepping on their identifiers.
func normalizeProxyFrame(frame models.StackFrame) models.StackFrame {
	if frame.Language == nil || *frame.Language != Language {
		return frame
	}
	newFunction := normalizeProxyText(frame.Function)
	var newModule *string
	if frame.Module != nil {
		mod := normalizeProxyText(*frame.Module)
		newModule = &mod
	}
	functionUnchanged := newFunction == frame.Function
	moduleUnchanged := (newModule == nil && frame.Module == nil) ||
		(newModule != nil && frame.Module != nil && *newModule == *frame.Module)
	if functionUnchanged && moduleUnchanged {
		return frame
	}
	out := frame
	out.Function = newFunction
	out.Module = newModule
	return out
}
