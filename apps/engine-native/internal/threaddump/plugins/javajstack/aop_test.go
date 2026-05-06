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
