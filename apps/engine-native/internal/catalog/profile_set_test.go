package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"testing"
)

func TestProfileSetBundledConformance(t *testing.T) {
	_, schemas, _, e := profileRegistration()
	if e != nil {
		t.Fatal(e)
	}
	for _, schema := range schemas {
		definition, e := ingestion.CompileSchema(schema.Definition)
		if e != nil {
			t.Fatal(schema.Ref.ID, e)
		}
		for index, sample := range schema.Conformance {
			if e := schema.Validate(sample.Value); (e == nil) != sample.Valid {
				t.Fatalf("%s sample %d compiled: %v", schema.Ref.ID, index, e)
			}
			if e := definition(sample.Value); (e == nil) != sample.Valid {
				t.Fatalf("%s sample %d definition: %v", schema.Ref.ID, index, e)
			}
		}
	}
}

func TestProfileSetManifestExpectedValuesAndOrigin(t *testing.T) {
	raw, e := Contracts.ReadFile("contracts/profile-set.manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var manifest struct {
		Expected struct {
			Duration string `json:"duration_total"`
			Samples  string `json:"sample_total"`
			Memory   string `json:"memory_total"`
			Records  int    `json:"duration_records"`
			Profiles int    `json:"profiles"`
		} `json:"expected"`
	}
	if e = json.Unmarshal(raw, &manifest); e != nil {
		t.Fatal(e)
	}
	body, _ := Contracts.ReadFile("contracts/profile-set.json")
	set, e := profilemodel.Decode(body)
	if e != nil {
		t.Fatal(e)
	}
	origin, _ := Contracts.ReadFile("contracts/profile-origin.txt")
	sum := sha256.Sum256(origin)
	if set.SourceHashes[0] != hex.EncodeToString(sum[:]) {
		t.Fatal("fixture origin mismatch")
	}
	if len(set.Profiles) != manifest.Expected.Profiles {
		t.Fatal("profile count")
	}
	for id, want := range map[string]string{"duration": manifest.Expected.Duration, "samples": manifest.Expected.Samples, "memory": manifest.Expected.Memory} {
		tree, e := profilemodel.BuildTree(context.Background(), set, profilemodel.Selection{ProfileIDs: []string{"worker-a", "worker-b"}, MetricID: id})
		if e != nil || tree.Root.Inclusive != want {
			t.Fatal(id, tree, e)
		}
		if id == "duration" && tree.SampleRecords != manifest.Expected.Records {
			t.Fatal("zero sample record lost")
		}
	}
}
