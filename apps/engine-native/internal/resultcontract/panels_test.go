package resultcontract_test

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	c "github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"testing"
)

func TestPanelCapabilitiesAreCompiledFrozenAndFailClosed(t *testing.T) {
	panel := &c.Panel{ID: "execution", Version: "1.0.0", Actions: []string{"details", "compare"}}
	descriptor := c.Descriptor{Type: "panel_fixture", Payload: c.Range{Min: "1.0.0", Before: "1.1.0"}, Envelope: c.Range{Min: "1.0.0", Before: "2.0.0"}, Panel: panel}
	registry, err := c.New(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	panel.ID = "forged"
	panel.Actions[0] = "forged"
	result := models.New("panel_fixture", "test")
	result.Metadata.SchemaVersion = "1.0.0"
	result.Metadata.Extra["panel"] = map[string]any{"id": "forged", "version": "9.0.0"}
	caps, err := registry.Capabilities(context.Background(), result)
	if err != nil || !caps.Compatibility.Allowed || caps.Panel == nil || caps.Panel.ID != "execution" || caps.Panel.Actions[0] != "details" {
		t.Fatal(caps, err)
	}
	caps.Panel.Actions[0] = "mutated"
	again, err := registry.Capabilities(context.Background(), result)
	if err != nil || again.Panel.Actions[0] != "details" {
		t.Fatal(again, err)
	}
	body, _ := json.Marshal(result)
	snapshot, err := registry.PrepareBytes(context.Background(), body, wirejson.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	copy := snapshot.Capabilities()
	copy.Panel.Actions[0] = "mutated"
	if snapshot.Capabilities().Panel.Actions[0] != "details" {
		t.Fatal("projection alias")
	}
	result.Metadata.SchemaVersion = "9.0.0"
	unknown, err := registry.Capabilities(context.Background(), result)
	if err != nil || unknown.Compatibility.Allowed || unknown.Panel != nil {
		t.Fatal(unknown, err)
	}
	result.Metadata.SchemaVersion = "1.0.0"
	descriptor.Panel = nil
	generic, err := c.New(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	caps, err = generic.Capabilities(context.Background(), result)
	if err != nil || caps.Panel != nil {
		t.Fatal("imported metadata granted panel", caps, err)
	}
	for _, bad := range []*c.Panel{{ID: "execution", Version: "2.0.0", Actions: []string{"details"}}, {ID: "../execution", Version: "1.0.0", Actions: []string{"details"}}, {ID: "execution", Version: "1.0.0", Actions: []string{"details", "details"}}, {ID: "execution", Version: "1.0.0"}} {
		descriptor.Panel = bad
		if _, err = c.New(descriptor); err == nil {
			t.Fatal(bad)
		}
	}
}
