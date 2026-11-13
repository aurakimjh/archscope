package llmmodel_test

import (
	"context"
	model "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/modeltest"
	"testing"
)

func TestDirectModelBoundaries(t *testing.T) {
	modeltest.Boundaries(t, "llm-trace-set.json", 32<<20, func(raw []byte) error { _, err := model.Decode(context.Background(), raw); return err }, []modeltest.Mutation{
		{Name: "version", Change: func(d map[string]any) { d["schema_version"] = "9.0.0" }},
		{Name: "privacy_policy", Change: func(d map[string]any) { d["source"].(map[string]any)["privacy_policy"] = "keep-bodies" }},
		{Name: "unknown_clock", Change: func(d map[string]any) { d["source"].(map[string]any)["clock"] = "guessed-clock" }},
		{Name: "orphan_context", Change: func(d map[string]any) { modeltest.Row(d, "spans", 0)["context_id"] = "missing" }},
		{Name: "duplicate_span", Change: func(d map[string]any) { d["spans"] = append(d["spans"].([]any), modeltest.Row(d, "spans", 0)) }},
		{Name: "unknown_field", Change: func(d map[string]any) { d["unexpected"] = true }},
	})
}
