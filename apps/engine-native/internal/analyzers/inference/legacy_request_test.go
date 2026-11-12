package inference

import (
	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"testing"
)

func TestR16PreservesOldEqualBoundarySemantics(t *testing.T) {
	v := n.Event{FirstToken: m.Pointer("2"), LastToken: m.Pointer("2"), OutputTokens: m.Pointer("3")}
	old := request(v, true)
	next := request(v, false)
	if old.TPOT == nil || *old.TPOT != "0" || next.TPOT != nil || next.TPOTStatus != "token_boundaries_not_distinct" {
		t.Fatal(old, next)
	}
}
