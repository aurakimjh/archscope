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
