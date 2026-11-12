package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gputrace"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/gputrace"
)

func (s *EngineService) InspectGPUExecutionSource(ctx context.Context, path, options string) (string, error) {
	o, e := p.DecodeOptions([]byte(options))
	if e != nil {
		return "", e
	}
	set, hash, e := p.ParseFile(ctx, path, o)
	if e != nil {
		return "", e
	}
	v := analysis.InspectGPUExecution(set)
	v.SourceSHA = hash
	return caseJSON(v, nil)
}
func (s *EngineService) NormalizeGPUExecutionSource(ctx context.Context, path, options, digest, output string) (string, error) {
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
	set, hash, e := p.ParseFile(ctx, path, o)
	if e != nil {
		return "", e
	}
	if digest == "" || digest != hash {
		return "", fmt.Errorf("SOURCE_CHANGED: inspect the original again")
	}
	raw, e := json.Marshal(set)
	if e != nil {
		return "", e
	}
	if e = localfiles.Publish(ctx, output, raw, roots...); e != nil {
		return "", e
	}
	return caseJSON(analysis.InspectGPUExecution(set), nil)
}
func (s *EngineService) DescribeGPUExecution(ctx context.Context, handle string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.GPUExecutionInventory(ctx, handle))
}
func (s *EngineService) QueryGPUExecution(ctx context.Context, handle, digest, query string) (string, error) {
	var q a.Query
	if e := r.Strict([]byte(query), &q, 16<<10); e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.QueryGPUExecution(ctx, handle, digest, q))
}

func (s *EngineService) ExportGPUExecutionSVG(ctx context.Context, handle, digest, query, output string) (string, error) {
	var q a.Query
	if e := r.Strict([]byte(query), &q, 16<<10); e != nil {
		return "", e
	}
	roots, e := assistanceRoots()
	if e != nil {
		return "", e
	}
	if _, e = localfiles.Preflight(output, roots...); e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	body, e := service.ExportGPUExecutionSVG(ctx, handle, digest, q)
	if e != nil {
		return "", e
	}
	if e = localfiles.Publish(ctx, output, body, roots...); e != nil {
		return "", e
	}
	return output, nil
}
