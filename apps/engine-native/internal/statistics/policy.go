package statistics

import (
	_ "embed"
	"encoding/json"
)

//go:embed assets/percentile-policies.json
var percentilePolicies []byte

// PercentilePolicy is additive disclosure for newly computed legacy results.
// A fresh map prevents callers from mutating the compiled shared-asset mirror.
func PercentilePolicy(family string) map[string]any {
	var catalog struct {
		Policies map[string]map[string]any `json:"policies"`
	}
	if err := json.Unmarshal(percentilePolicies, &catalog); err != nil {
		panic(err)
	}
	return catalog.Policies[family]
}
