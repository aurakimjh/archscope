// [한글] aop.go — Java 의 AOP/Proxy synthetic 접미사 정리 (T-194).
//
// 문제
//   Spring CGLIB / JDK Dynamic Proxy / accessor synthetic class 는
//   런타임에 클래스명에 해시를 붙여 매번 다른 이름으로 등장:
//     OrderService$$EnhancerBySpringCGLIB$$abc123def
//     OrderService$$EnhancerByCGLIB$$xyz789ghi
//     com.foo.Bar$$Lambda$42/0x000000800120cda0
//
//   같은 비즈니스 로직을 가리키는 frame 이 매번 다른 이름이 되어
//   stack signature 가 폭발 → multi-dump correlator 의 dedup 무력화.
//
// 정리 규칙 (정규식)
//   `$$EnhancerBy(Spring)?CGLIB$$<hash>` 제거.
//   `$$Lambda$N/0x...` → `$$Lambda` (해시만 제거, 식별자는 유지).
//   `$Proxy<N>$$...` 제거.
//   accessor `$1`, `$$$0` 등 synthetic 접미사 제거.
//
// 적용 대상
//   StackFrame.Module / Function 만. 다른 런타임 frame 은 정규식이
//   매칭되지 않아 no-op 통과.
//
// 효과
//   같은 위치를 가리키는 N개 변형 frame 이 한 stack signature 로
//   collapse → multi-dump finding 에서 정확한 그룹화 가능.
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
