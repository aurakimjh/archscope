package wirejson_test

import (
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	i "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	l "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	s "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	v "github.com/aurakimjh/archscope/apps/engine-native/internal/visuals"
	"strings"
	"testing"
)

func TestDomainStrictUsesExactCallerLimit(t *testing.T) {
	raw := []byte(`{"n":0}`)
	for _, c := range []struct {
		name, code string
		decode     func([]byte, any, int) error
	}{
		{"execution", "EXECUTION_BYTES_BUDGET", e.Strict}, {"gpu", "GPU_TRACE_BYTES_BUDGET", g.Strict},
		{"inference", "INFERENCE_BUDGET", func(b []byte, out any, n int) error { return i.Strict(b, out, n) }},
		{"llm", "LLMTRACE_BUDGET", func(b []byte, out any, n int) error { return l.Strict(b, out, n) }},
		{"runtime", "RUNTIME_BYTES_BUDGET", func(b []byte, out any, n int) error { return r.Strict(b, out, n) }},
		{"system", "SYSTEM_BUDGET", s.Strict}, {"visuals", "VISUAL_BUDGET_EXCEEDED", v.Strict},
	} {
		t.Run(c.name, func(t *testing.T) {
			var out map[string]any
			if err := c.decode(raw, &out, len(raw)); err != nil {
				t.Fatal(err)
			}
			if err := c.decode(raw, &out, len(raw)-1); err == nil || !strings.Contains(err.Error(), c.code) || strings.Contains(err.Error(), "METRIC_") {
				t.Fatal(err)
			}
			var closed struct {
				N int `json:"n"`
			}
			if err := c.decode([]byte(`{"n":0,"other":0}`), &closed, 100); err == nil {
				t.Fatal("unknown field")
			}
		})
	}
}
