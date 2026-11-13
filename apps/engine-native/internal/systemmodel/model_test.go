package systemmodel_test

import (
	"context"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/modeltest"
	model "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"testing"
)

func TestDirectModelBoundaries(t *testing.T) {
	modeltest.Boundaries(t, "system-evidence-set.json", 16<<20, func(raw []byte) error { _, err := model.Decode(context.Background(), raw); return err }, []modeltest.Mutation{
		{Name: "version", Change: func(d map[string]any) { d["version"] = "9.0.0" }},
		{Name: "source_hash", Change: func(d map[string]any) { d["source_sha256"] = "bad" }},
		{Name: "missing_clock", Change: func(d map[string]any) { d["capture"].(map[string]any)["clock"] = "" }},
		{Name: "orphan_entity", Change: func(d map[string]any) { modeltest.Row(d, "records", 0)["entity_id"] = "missing" }},
		{Name: "noncanonical_time", Change: func(d map[string]any) { modeltest.Row(d, "records", 0)["time_ns"] = "01" }},
		{Name: "unknown_field", Change: func(d map[string]any) { d["unexpected"] = true }},
	})
}
