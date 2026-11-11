package profile

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func TestT629NoObservationsAndKnownZeroHaveDifferentMeasurements(t *testing.T) {
	frame := profilemodel.Frame{Symbol: "synthetic", Display: "synthetic"}
	frame.ID = profilemodel.FrameID(frame)
	stack := profilemodel.Stack{Frames: []string{frame.ID}}
	stack.ID = profilemodel.StackID(stack.Frames)
	set := profilemodel.ProfileSet{SchemaVersion: profilemodel.Version, Producer: "synthetic", Format: "internal", SourceHashes: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, CaptureOptions: json.RawMessage(`{}`), Frames: []profilemodel.Frame{frame}, Stacks: []profilemodel.Stack{stack}, Metrics: []profilemodel.Metric{{ID: "duration", Kind: "wall_time", Unit: "seconds", Scale: "1"}}, Profiles: []profilemodel.Profile{{ID: "empty", Clock: "aggregate", Samples: []profilemodel.Sample{}}}, Quality: profilemodel.Quality{ClockConfidence: "unavailable"}}
	selection := profilemodel.Selection{ProfileIDs: []string{"empty"}, MetricID: "duration"}
	unknown, e := BuildSet(context.Background(), set, selection, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	if unknown.Summary["total_weight"] != nil {
		t.Fatal("unmeasured became zero")
	}
	set.Profiles[0].Samples = []profilemodel.Sample{{StackID: stack.ID, MetricID: "duration", Weight: "0"}}
	known, e := BuildSet(context.Background(), set, selection, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	if known.Summary["total_weight"] != "0" {
		t.Fatal("known zero lost")
	}
	registry, e := resultcontract.New(ModelDescriptor())
	if e != nil {
		t.Fatal(e)
	}
	if verdict := registry.Check(unknown, "save"); !verdict.Allowed {
		t.Fatal(verdict)
	}
	if verdict := registry.Check(known, "save"); !verdict.Allowed {
		t.Fatal(verdict)
	}
}
