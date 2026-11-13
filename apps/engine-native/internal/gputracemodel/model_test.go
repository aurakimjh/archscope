package gputracemodel_test

import (
	"context"
	model "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/modeltest"
	"testing"
)

func TestDirectModelBoundaries(t *testing.T) {
	modeltest.Boundaries(t, "gpu-execution-set.json", 32<<20, func(raw []byte) error { _, err := model.Decode(context.Background(), raw); return err }, []modeltest.Mutation{
		{Name: "version", Change: func(d map[string]any) { d["schema_version"] = "9.0.0" }},
		{Name: "source_hash", Change: func(d map[string]any) { modeltest.Row(d, "sources", 0)["sha256"] = "bad" }},
		{Name: "orphan_track", Change: func(d map[string]any) { modeltest.Row(d, "tracks", 0)["source_id"] = "missing" }},
		{Name: "reversed_interval", Change: func(d map[string]any) {
			modeltest.Row(d, "events", 0)["from_ns"] = "18446744073709551615"
			modeltest.Row(d, "events", 0)["to_ns"] = "0"
		}},
		{Name: "duplicate_event", Change: func(d map[string]any) { d["events"] = append(d["events"].([]any), modeltest.Row(d, "events", 0)) }},
		{Name: "unknown_field", Change: func(d map[string]any) { d["unexpected"] = true }},
	})
}
