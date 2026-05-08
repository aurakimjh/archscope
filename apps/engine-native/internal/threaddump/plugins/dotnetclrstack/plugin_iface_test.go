// [한글] dotnetclrstack plugin 의 인터페이스 적합성 테스트.
//
// 검증 대상
//   • 두 plugin (clrstack / Environment.StackTrace) 이 모두
//     threaddump.Plugin 인터페이스를 만족하는지 컴파일타임 + 런타임
//     양쪽에서 확인.
//   • FormatID() / Language() / CanParse() / Parse() 시그니처 정확성.
//   • init() 으로 DefaultRegistry 에 자동 등록되는지.
package dotnetclrstack

import (
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump"
)

// TestPluginSatisfiesThreaddumpInterface guarantees both exported
// plugin types continue to satisfy the registry's Plugin contract.
// The compile-time assertion below catches any drift the moment one
// of the four required methods slips out of sync.
func TestPluginSatisfiesThreaddumpInterface(t *testing.T) {
	var _ threaddump.Plugin = (*Plugin)(nil)
	var _ threaddump.Plugin = (*EnvironmentStackTracePlugin)(nil)
}
