package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestBundledConformanceAssetsMatchManifest(t *testing.T) {
	raw, e := Contracts.ReadFile("contracts/manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var manifest struct {
		ID     string            `json:"id"`
		Files  map[string]string `json:"files"`
		Schema string            `json:"measurement_schema_sha256"`
	}
	if e = json.Unmarshal(raw, &manifest); e != nil {
		t.Fatal(e)
	}
	if manifest.ID != "diagnostics.ingestion.foundation.v1" || len(manifest.Files) != 2 {
		t.Fatal("incomplete fixture manifest")
	}
	for name, digest := range manifest.Files {
		body, e := Contracts.ReadFile("contracts/" + name)
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != digest {
			t.Fatal("fixture hash mismatch", name)
		}
	}
	body, e := Contracts.ReadFile("contracts/measurement-descriptor.schema.json")
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != manifest.Schema {
		t.Fatal("measurement schema hash mismatch")
	}
	c, _, e := Builtin()
	if e != nil {
		t.Fatal(e)
	}
	list := c.List()
	want := []string{"metrics.history-json", "profile.set-json", "profile.source-set-json", "server.tomcat"}
	if len(list) != len(want) {
		t.Fatal("unexpected importer count", len(list))
	}
	for i, id := range want {
		if list[i].ID != id || list[i].SupportStatus != "limited" {
			t.Fatal("unqualified product activated", list[i])
		}
	}
}
