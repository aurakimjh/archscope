package inferencemodel_test

import (
	"context"
	model "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/modeltest"
	"testing"
)

func TestDirectModelBoundaries(t *testing.T) {
	modeltest.Boundaries(t, "inference-evidence-set.json", 32<<20, func(raw []byte) error { _, err := model.Decode(context.Background(), raw); return err }, []modeltest.Mutation{
		{Name: "version", Change: func(d map[string]any) { d["schema_version"] = "9.0.0" }},
		{Name: "source_hash", Change: func(d map[string]any) { modeltest.Row(d["metrics"].(map[string]any), "sources", 0)["sha256"] = "bad" }},
		{Name: "orphan_signal", Change: func(d map[string]any) { modeltest.Row(d, "signals", 0)["entity_id"] = "missing" }},
		{Name: "unknown_conversion", Change: func(d map[string]any) { modeltest.Row(d, "signals", 0)["conversion"] = "guess" }},
		{Name: "duplicate_entity", Change: func(d map[string]any) { d["entities"] = append(d["entities"].([]any), modeltest.Row(d, "entities", 0)) }},
		{Name: "unknown_field", Change: func(d map[string]any) { d["unexpected"] = true }},
	})
}
