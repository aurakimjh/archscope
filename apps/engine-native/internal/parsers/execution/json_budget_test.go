package execution_test

import (
	"context"
	"encoding/json"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/execution"
	"strings"
	"testing"
)

func TestT648K6FortyTwoMiBWithinSourceBudget(t *testing.T) {
	raw := fixture(t, "k6-1.3.0.json")
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	var metrics map[string]json.RawMessage
	if err := json.Unmarshal(root["metrics"], &metrics); err != nil {
		t.Fatal(err)
	}
	padding, _ := json.Marshal(strings.Repeat("x", 3<<20))
	for _, key := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n"} {
		metrics["custom_"+key] = padding
	}
	root["metrics"], _ = json.Marshal(metrics)
	raw, _ = json.Marshal(root)
	if len(raw) <= 32<<20 || len(raw) >= 64<<20 {
		t.Fatal(len(raw))
	}
	o, err := p.DecodeOptions(fixture(t, "k6-options.json"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := p.ParseBytes(context.Background(), raw, o)
	if err != nil || set.IgnoredMetrics < 14 {
		t.Fatal(len(raw), set.IgnoredMetrics, err)
	}
	var out any
	if err = e.Strict(raw, &out, 32<<20); err == nil || !strings.Contains(err.Error(), "EXECUTION_BYTES_BUDGET") || strings.Contains(err.Error(), "METRIC_") {
		t.Fatal(err)
	}
	// Key/string/depth limits remain separate from the caller's byte allowance.
	oversized, _ := json.Marshal(map[string]string{"x": strings.Repeat("x", (4<<20)+1)})
	if err = e.Strict(oversized, &out, 64<<20); err == nil {
		t.Fatal("string budget bypass")
	}
}
