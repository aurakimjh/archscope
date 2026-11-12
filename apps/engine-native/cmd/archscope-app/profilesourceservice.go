package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
)

// Sources stay backend-owned; inspection returns bounded inventory only, and
// normalization writes a new model file for the existing generic import jobs.
func (s *EngineService) InspectProfileSource(ctx context.Context, path, optionsJSON string) (string, error) {
	options, e := parser.DecodeSourceOptions([]byte(optionsJSON))
	if e != nil {
		return "", e
	}
	model, e := parser.ParseSourceFile(ctx, path, options)
	if e != nil {
		return "", e
	}
	return caseJSON(parser.Inventory(model), nil)
}
func (s *EngineService) NormalizeProfileSource(ctx context.Context, path, optionsJSON, expectedSHA256, output string) (string, error) {
	options, e := parser.DecodeSourceOptions([]byte(optionsJSON))
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
	model, e := parser.ParseSourceFile(ctx, path, options)
	if e != nil {
		return "", e
	}
	if expectedSHA256 == "" || model.SourceHashes[0] != expectedSHA256 {
		return "", fmt.Errorf("SOURCE_CHANGED: inspect the current original source again before normalizing")
	}
	body, e := json.Marshal(model)
	if e != nil {
		return "", e
	}
	if e = localfiles.Publish(ctx, output, body, roots...); e != nil {
		return "", e
	}
	return caseJSON(parser.Inventory(model), nil)
}
