package packs

import (
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"sync"
)

var schemaOnce sync.Once
var manifestSchema, mappingSchema func(json.RawMessage) error
var schemaError error

func validateSchema(name string, body []byte) error {
	schemaOnce.Do(func() {
		a, e := Assets.ReadFile("assets/data-pack.schema.json")
		if e != nil {
			schemaError = e
			return
		}
		b, e := Assets.ReadFile("assets/metric-mapping.schema.json")
		if e != nil {
			schemaError = e
			return
		}
		manifestSchema, schemaError = ingestion.CompileSchema(a)
		if schemaError == nil {
			mappingSchema, schemaError = ingestion.CompileSchema(b)
		}
	})
	if schemaError != nil {
		return schemaError
	}
	if name == "manifest" {
		return manifestSchema(body)
	}
	return mappingSchema(body)
}
