package resultcontract

import (
	"context"
	"encoding/json"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

type referenceIndex map[string]map[string]bool

// Decode each reference list once per validation, never once per record.
func references(ctx context.Context, result models.AnalysisResult) (referenceIndex, error) {
	out := referenceIndex{}
	for _, key := range []string{"clock_ids", "evidence_ids", "population_ids"} {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		set := map[string]bool{}
		add := func(id string) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if id != "" {
				set[id] = true
			}
			return nil
		}
		// Wire-decoded arrays and compiled typed arrays require no JSON round
		// trip. The fallback preserves other JSON-serializable Go slice types.
		switch ids := result.Metadata.Extra[key].(type) {
		case []string:
			for _, id := range ids {
				if e := add(id); e != nil {
					return nil, e
				}
			}
		case []any:
			valid := true
			for _, value := range ids {
				id, ok := value.(string)
				if !ok {
					valid = false
					break
				}
				if e := add(id); e != nil {
					return nil, e
				}
			}
			if !valid {
				set = map[string]bool{}
			}
		case nil:
		default:
			b, e := json.Marshal(ids)
			var values []string
			if e == nil && wirejson.Decode(b, &values) == nil {
				for _, id := range values {
					if e := add(id); e != nil {
						return nil, e
					}
				}
			}
		}
		out[key] = set
	}
	return out, nil
}
func (r referenceIndex) has(key, id string) bool { return id != "" && r[key][id] }
