package executionmodel_test

import (
	"context"
	model "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/modeltest"
	"testing"
)

func TestDirectModelBoundaries(t *testing.T) {
	modeltest.Boundaries(t, "execution-evidence-k6-set.json", 16<<20, func(raw []byte) error { _, err := model.Decode(context.Background(), raw); return err }, []modeltest.Mutation{
		{Name: "version", Change: func(d map[string]any) { d["schema_version"] = "9.0.0" }},
		{Name: "source_hash", Change: func(d map[string]any) { d["source"].(map[string]any)["sha256"] = "bad" }},
		{Name: "benchmark_context", Change: func(d map[string]any) { d["context"] = nil }},
		{Name: "duplicate_metric", Change: func(d map[string]any) { d["metrics"] = append(d["metrics"].([]any), modeltest.Row(d, "metrics", 0)) }},
		{Name: "negative_duration", Change: func(d map[string]any) { d["context"].(map[string]any)["configured_duration_ns"] = "-1" }},
		{Name: "unknown_field", Change: func(d map[string]any) { d["unexpected"] = true }},
	})
}
