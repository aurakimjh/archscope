package main

import (
	"context"
	"encoding/json"
	"fmt"

	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/inference"
	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/inference"
)

func (s *EngineService) InspectInferenceSource(ctx context.Context, path, options string) (string, error) {
	o, e := p.DecodeOptions([]byte(options))
	if e != nil {
		return "", e
	}
	set, digest, e := p.ParseFile(ctx, path, o)
	if e != nil {
		return "", e
	}
	v := a.Inspect(set)
	v.SourceSHA256 = digest
	return caseJSON(v, nil)
}
func (s *EngineService) NormalizeInferenceSource(ctx context.Context, path, options, expectedSHA, output string) (string, error) {
	o, e := p.DecodeOptions([]byte(options))
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
	set, digest, e := p.ParseFile(ctx, path, o)
	if e != nil {
		return "", e
	}
	if expectedSHA == "" || digest != expectedSHA {
		return "", fmt.Errorf("SOURCE_CHANGED: inspect current original again")
	}
	raw, e := json.Marshal(set)
	if e != nil {
		return "", e
	}
	if e = localfiles.Publish(ctx, output, raw, roots...); e != nil {
		return "", e
	}
	v := a.Inspect(set)
	v.SourceSHA256 = digest
	return caseJSON(v, nil)
}
func (s *EngineService) DescribeInferenceEvidence(ctx context.Context, handle string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.InferenceInventory(ctx, handle))
}
func (s *EngineService) QueryInferenceMetric(ctx context.Context, handle, digest, signal, cursor string, limit int) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.QueryInferenceMetric(ctx, handle, digest, signal, cursor, limit))
}
func (s *EngineService) MaterializeInferenceMetrics(ctx context.Context, handle, digest, signalsJSON string) (string, error) {
	var ids []string
	if e := n.Strict([]byte(signalsJSON), &ids, 128<<10); e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	view, lease, e := service.MaterializeInferenceMetrics(ctx, handle, digest, ids)
	if e != nil {
		return "", e
	}
	return caseJSON(map[string]any{"view": view, "lease": lease}, nil)
}
