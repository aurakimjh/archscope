package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
)

// Sources stay backend-owned; inspection returns bounded inventory only, and
// normalization writes a new model file for the existing generic import jobs.
func (s *EngineService) InspectMetricSource(ctx context.Context, path, optionsJSON string) (string, error) {
	options, e := parser.DecodeOptions([]byte(optionsJSON))
	if e != nil {
		return "", e
	}
	model, e := parser.ParseFile(ctx, path, options)
	if e != nil {
		return "", e
	}
	return metricInventoryJSON(parser.Inspect(model), nil)
}
func (s *EngineService) NormalizeMetricSource(ctx context.Context, path, optionsJSON, expectedSHA256, output string) (string, error) {
	options, e := parser.DecodeOptions([]byte(optionsJSON))
	if e != nil {
		return "", e
	}
	roots, e := assistanceRoots()
	if e != nil {
		return "", e
	}
	if _, e = localfiles.Preflight(output, roots...); e != nil {
		return "", e
	}
	model, e := parser.ParseFile(ctx, path, options)
	if e != nil {
		return "", e
	}
	if expectedSHA256 == "" || model.Sources[0].SHA256 != expectedSHA256 {
		return "", fmt.Errorf("SOURCE_CHANGED: inspect the current original source again before normalizing")
	}
	body, e := json.Marshal(model)
	if e != nil {
		return "", e
	}
	if e = localfiles.Publish(ctx, output, body, roots...); e != nil {
		return "", e
	}
	return metricInventoryJSON(parser.Inspect(model), nil)
}

// Identity JSON is opaque to JavaScript so large numeric attributes cannot be
// rounded by JSON.parse during display. Selection still uses backend series IDs.
func metricInventoryJSON(value any, failure error) (string, error) {
	if failure != nil {
		return "", failure
	}
	var series []m.Series
	switch inv := value.(type) {
	case parser.Inventory:
		series = inv.Series
	case analysis.MetricInventory:
		series = inv.Series
	default:
		return "", fmt.Errorf("METRIC_INVENTORY_INVALID")
	}
	type entry struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Unit         string `json:"unit"`
		Kind         string `json:"kind"`
		Temporality  string `json:"temporality"`
		Monotonic    bool   `json:"monotonic"`
		Origin       string `json:"origin"`
		IdentityJSON string `json:"identity_json"`
	}
	views := make([]entry, 0, len(series))
	for _, s := range series {
		raw, e := json.MarshalIndent(s, "", "  ")
		if e != nil {
			return "", e
		}
		views = append(views, entry{s.ID, s.Name, s.Unit, s.Kind, s.Temporality, s.Monotonic, s.Origin, string(raw)})
	}
	body, e := json.Marshal(value)
	if e != nil {
		return "", e
	}
	var object map[string]json.RawMessage
	if e = json.Unmarshal(body, &object); e != nil {
		return "", e
	}
	object["series"], e = json.Marshal(views)
	if e != nil {
		return "", e
	}
	return caseJSON(object, nil)
}
