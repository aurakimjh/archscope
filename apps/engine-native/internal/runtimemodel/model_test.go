package runtimemodel_test

import (
	"context"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/modeltest"
	model "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	"testing"
)

func TestDirectModelBoundaries(t *testing.T) {
	modeltest.Boundaries(t, "runtime-evidence-set.json", 32<<20, func(raw []byte) error { _, err := model.Decode(context.Background(), raw); return err }, []modeltest.Mutation{
		{Name: "version", Change: func(d map[string]any) { d["schema_version"] = "9.0.0" }},
		{Name: "source_hash", Change: func(d map[string]any) { modeltest.Row(d, "sources", 0)["sha256"] = "bad" }},
		{Name: "orphan_process", Change: func(d map[string]any) { modeltest.Row(d, "entities", 0)["process_id"] = "missing" }},
		{Name: "orphan_entity", Change: func(d map[string]any) { modeltest.Row(d, "observations", 0)["entity_id"] = "missing" }},
		{Name: "duplicate_observation", Change: func(d map[string]any) {
			d["observations"] = append(d["observations"].([]any), modeltest.Row(d, "observations", 0))
		}},
		{Name: "unknown_field", Change: func(d map[string]any) { d["unexpected"] = true }},
	})
}
