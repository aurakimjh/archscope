package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/runtimeevidence"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/runtimeevidence"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
)

func (s *EngineService) InspectRuntimeSource(ctx context.Context, path, options string) (string, error) {
	o, e := p.DecodeOptions([]byte(options))
	if e != nil {
		return "", e
	}
	set, hash, e := p.ParseFile(ctx, path, o)
	if e != nil {
		return "", e
	}
	v := analysis.InspectRuntime(set)
	v.SourceSHA = hash
	return caseJSON(v, nil)
}
func (s *EngineService) NormalizeRuntimeSource(ctx context.Context, path, options, digest, output string) (string, error) {
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
	return caseJSON(analysis.InspectRuntime(set), nil)
}
func (s *EngineService) MergeRuntimeModels(ctx context.Context, pathsJSON, output string) (string, error) {
	var paths []string
	if e := r.Strict([]byte(pathsJSON), &paths, 16<<10); e != nil {
		return "", e
	}
	if len(paths) < 1 || len(paths) > 16 {
		return "", fmt.Errorf("RUNTIME_MERGE_BUDGET")
	}
	roots, e := assistanceRoots()
	if e != nil {
		return "", e
	}
	if _, e = localfiles.Preflight(output, roots...); e != nil {
		return "", e
	}
	sets := []r.Set{}
	for _, path := range paths {
		set, _, e := p.ParseFile(ctx, path, p.Options{Format: "runtime-evidence-json"})
		if e != nil {
			return "", e
		}
		sets = append(sets, set)
	}
	merged, e := r.Merge(ctx, sets)
	if e != nil {
		return "", e
	}
	raw, e := json.Marshal(merged)
	if e != nil {
		return "", e
	}
	if e = localfiles.Publish(ctx, output, raw, roots...); e != nil {
		return "", e
	}
	return caseJSON(analysis.InspectRuntime(merged), nil)
}
func (s *EngineService) DescribeRuntimeEvidence(ctx context.Context, handle string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.RuntimeInventory(ctx, handle))
}
func (s *EngineService) QueryRuntimeEvidence(ctx context.Context, handle, digest, query string) (string, error) {
	var q a.Query
	if e := r.Strict([]byte(query), &q, 16<<10); e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.QueryRuntime(ctx, handle, digest, q))
}

func (s *EngineService) MaterializeRuntimeAllocation(ctx context.Context, handle, digest, entity, source, metric string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	view, lease, e := service.MaterializeRuntimeAllocation(ctx, handle, digest, entity, source, metric)
	if e != nil {
		return "", e
	}
	return caseJSON(struct {
		View  analysis.ResultView  `json:"view"`
		Lease analysis.ResultLease `json:"lease"`
	}{view, lease}, nil)
}
