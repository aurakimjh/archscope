package models

import (
	"encoding/json"
	"testing"
)

func TestMetadataFlattenedExtensionRoundTrip(t *testing.T) {
	input := `{"parser":"p","schema_version":"1","findings":[],"drilldown":{"version":"1.0.0","selection":{"guid":"g"}},"other":[1,true,"x"]}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(input), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Parser != "p" || metadata.Extra["drilldown"] == nil || metadata.Extra["parser"] != nil {
		t.Fatal(metadata)
	}
	wire, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var before, after any
	json.Unmarshal([]byte(input), &before)
	json.Unmarshal(wire, &after)
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatalf("%s != %s", a, b)
	}
	if err = json.Unmarshal([]byte(`{"parser":"new","findings":[]}`), &metadata); err != nil {
		t.Fatal(err)
	}
	if len(metadata.Extra) != 0 {
		t.Fatal("reused metadata retained stale extensions")
	}
	if err = json.Unmarshal([]byte(`{"parser":123}`), &metadata); err == nil || metadata.Parser != "new" {
		t.Fatal("invalid metadata changed previous value")
	}
}
