package catalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"testing"
)

func TestActualSystemDBCorpusSchemaManifestAndModels(t *testing.T) {
	raw, e := Contracts.ReadFile("contracts/system-db-manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var manifest struct {
		ID    string `json:"id"`
		Files []struct {
			File string `json:"file"`
			SHA  string `json:"sha256"`
		} `json:"files"`
	}
	if e = json.Unmarshal(raw, &manifest); e != nil || manifest.ID != "diagnostics.system-db-snapshots.v1" || len(manifest.Files) != 13 {
		t.Fatal(manifest, e)
	}
	for _, f := range manifest.Files {
		data, e := Contracts.ReadFile("contracts/system-db-" + f.File)
		h := sha256.Sum256(data)
		if e != nil || hex.EncodeToString(h[:]) != f.SHA {
			t.Fatal(f.File, e)
		}
	}
	for _, name := range []string{"postgres", "linux-ss"} {
		schema, _ := Contracts.ReadFile("contracts/system-db-" + name + "-capture.schema.json")
		validate, e := ingestion.CompileSchema(schema)
		if e != nil {
			t.Fatal(e)
		}
		original, _ := Contracts.ReadFile("contracts/system-db-" + name + "-capture.json")
		if e = validate(original); e != nil {
			t.Fatal(name, e)
		}
		if validate(json.RawMessage(`{}`)) == nil {
			t.Fatal("schema accepts empty capture")
		}
		set, e := p.ParseBytes(context.Background(), original, name+"-capture.json", p.Options{})
		if e != nil {
			t.Fatal(e)
		}
		stored, _ := Contracts.ReadFile("contracts/system-db-" + name + "-model.json")
		model, e := m.Decode(stored)
		if e != nil {
			t.Fatal(e)
		}
		expectedNormalizer := m.ObservationNormalizer
		if name == "postgres" {
			expectedNormalizer = m.EpochNormalizer
		}
		if model.Normalizer != m.Normalizer || set.Normalizer != expectedNormalizer {
			t.Fatal("normalizer revisions")
		}
		model.Normalizer = expectedNormalizer
		a, _ := json.Marshal(set)
		b, _ := json.Marshal(model)
		if !bytes.Equal(a, b) {
			t.Fatal(name, "normalized fixture drift")
		}
	}
}
