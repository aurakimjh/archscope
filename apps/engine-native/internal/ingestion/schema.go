package ingestion

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

type bundledOnlyLoader struct{}

func (bundledOnlyLoader) Load(string) (any, error) {
	return nil, fmt.Errorf("external schema references are disabled")
}

// CompileSchema uses only the supplied bundled document and draft 2020-12.
// No file/URL from input data participates in schema loading.
func CompileSchema(definition json.RawMessage) (func(json.RawMessage) error, error) {
	var document any
	if e := wirejson.Decode(definition, &document); e != nil {
		return nil, e
	}
	if e := boundedSchemaNumbers(document); e != nil {
		return nil, e
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.UseLoader(bundledOnlyLoader{})
	const location = "urn:diagnostics:bundled-schema"
	if e := compiler.AddResource(location, document); e != nil {
		return nil, e
	}
	schema, e := compiler.Compile(location)
	if e != nil {
		return nil, e
	}
	return func(body json.RawMessage) error {
		var value any
		// Validation of an entire result permits its tree depth. Purpose-specific
		// parsers/options/metadata retain their stricter decoding before this call.
		if e := wirejson.DecodeWithLimits(body, &value, wirejson.LegacyResultLimits()); e != nil {
			return e
		}
		if e := boundedSchemaNumbers(value); e != nil {
			return e
		}
		return schema.Validate(value)
	}, nil
}

// Schema numeric predicates may construct big.Rat values internally. Bound both
// schema literals and conformance operands before invoking that arithmetic. This
// boundary is deliberately separate from opaque storage/round-trip decoding.
func boundedSchemaNumbers(value any) error {
	switch v := value.(type) {
	case json.Number:
		if _, e := wirejson.RationalNumber(v); e != nil {
			return fmt.Errorf("schema numeric arithmetic budget exceeded")
		}
	case map[string]any:
		for _, child := range v {
			if e := boundedSchemaNumbers(child); e != nil {
				return e
			}
		}
	case []any:
		for _, child := range v {
			if e := boundedSchemaNumbers(child); e != nil {
				return e
			}
		}
	}
	return nil
}
