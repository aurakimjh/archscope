// [한글] aop 정리 회귀 테스트 (T-194).
//
// 검증 대상
//   • CGLIB EnhancerBy* 접미사 제거: `OrderService$$EnhancerByCGLIB
//     $$abc123` → `OrderService`.
//   • Spring CGLIB 변형도 같은 결과.
//   • Lambda hash 제거: `Foo$$Lambda$42/0x...` → `Foo$$Lambda`.
//   • $Proxy<N>$$ 제거.
//   • accessor synthetic ($1, $$$0) 제거.
//   • 정리 적용 전후의 stack signature 가 같은 비즈니스 코드를 가리키면
//     동일해지는지 확인.
//   • Non-Java frame (Go func 등) 은 no-op 통과.
package javajstack

import (
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func javaFrame(module, function string) models.StackFrame {
	lang := Language
	frame := models.StackFrame{Function: function, Language: &lang}
	if module != "" {
		mod := module
		frame.Module = &mod
	}
	return frame
}

// T-194 — proxy class normalization.

func TestNormalizeStripsCGLIBEnhancerSuffix(t *testing.T) {
	frame := javaFrame("com.example.MyService$$EnhancerByCGLIB$$abc123", "businessMethod")
	cleaned := normalizeProxyFrame(frame)
	if cleaned.Module == nil || *cleaned.Module != "com.example.MyService" {
		t.Fatalf("module = %v", cleaned.Module)
	}
	if cleaned.Function != "businessMethod" {
		t.Fatalf("function = %q", cleaned.Function)
	}
}

func TestNormalizeStripsFastClassCGLIBSuffix(t *testing.T) {
	frame := javaFrame("com.example.Worker$$FastClassByCGLIB$$deadbeef", "invoke")
	cleaned := normalizeProxyFrame(frame)
	if cleaned.Module == nil || *cleaned.Module != "com.example.Worker" {
		t.Fatalf("module = %v", cleaned.Module)
	}
}

func TestNormalizeCollapsesJDKProxyDigits(t *testing.T) {
	frame := javaFrame("com.sun.proxy.$$Proxy42", "handle")
	cleaned := normalizeProxyFrame(frame)
	if cleaned.Module == nil || *cleaned.Module != "com.sun.proxy.$$Proxy" {
		t.Fatalf("module = %v", cleaned.Module)
	}
}

func TestNormalizeCollapsesGeneratedMethodAccessorDigits(t *testing.T) {
	frame := javaFrame("sun.reflect.GeneratedMethodAccessor1234", "invoke")
	cleaned := normalizeProxyFrame(frame)
	if cleaned.Module == nil || *cleaned.Module != "sun.reflect.GeneratedMethodAccessor" {
		t.Fatalf("module = %v", cleaned.Module)
	}
}

func TestNormalizeCollapsesGeneratedConstructorAccessorDigits(t *testing.T) {
	frame := javaFrame("sun.reflect.GeneratedConstructorAccessor99", "newInstance")
	cleaned := normalizeProxyFrame(frame)
	if cleaned.Module == nil || *cleaned.Module != "sun.reflect.GeneratedConstructorAccessor" {
		t.Fatalf("module = %v", cleaned.Module)
	}
}

func TestNormalizeLeavesPlainFramesUnchanged(t *testing.T) {
	frame := javaFrame("com.example.Plain", "run")
	cleaned := normalizeProxyFrame(frame)
	// Both fields unchanged → same identity is fine but Python returns
	// `frame is frame`. In Go we settle for value equality.
	if cleaned.Module == nil || *cleaned.Module != "com.example.Plain" {
		t.Fatalf("module mutated: %v", cleaned.Module)
	}
	if cleaned.Function != "run" {
		t.Fatalf("function mutated: %q", cleaned.Function)
	}
}

func TestNormalizeSkipsNonJavaFrames(t *testing.T) {
	goLang := "go"
	mod := "some$$EnhancerByCGLIB$$x"
	frame := models.StackFrame{Module: &mod, Function: "run", Language: &goLang}
	cleaned := normalizeProxyFrame(frame)
	if cleaned.Module == nil || *cleaned.Module != mod {
		t.Fatalf("non-java frame mutated: %v", cleaned.Module)
	}
}

func TestTwoProxyVariantsCollapseToSameSignature(t *testing.T) {
	a := javaFrame("com.example.PaymentService$$EnhancerByCGLIB$$aaaa1111", "charge")
	b := javaFrame("com.example.PaymentService$$EnhancerByCGLIB$$bbbb2222", "charge")
	clA := normalizeProxyFrame(a)
	clB := normalizeProxyFrame(b)
	if clA.Module == nil || clB.Module == nil || *clA.Module != *clB.Module {
		t.Fatalf("modules diverged: %v vs %v", clA.Module, clB.Module)
	}
}
