package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/execution"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/execution"
)

func (s *EngineService) InspectExecutionSource(ctx context.Context, path, options string) (string, error) {
	o, e := p.DecodeOptions([]byte(options))
	if e != nil {
		return "", e
	}
	set, hash, e := p.ParseFile(ctx, path, o)
	if e != nil {
		return "", e
	}
	v := analysis.InspectExecution(set)
	v.SourceSHA = hash
	return caseJSON(v, nil)
}
func (s *EngineService) NormalizeExecutionSource(ctx context.Context, path, options, digest, output string) (string, error) {
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
	return caseJSON(analysis.InspectExecution(set), nil)
}
func (s *EngineService) DescribeExecutionEvidence(ctx context.Context, handle string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.ExecutionInventory(ctx, handle))
}
func (s *EngineService) QueryExecutionEvidence(ctx context.Context, handle, digest, query string) (string, error) {
	var q a.Query
	if e := r.Strict([]byte(query), &q, 16<<10); e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.QueryExecution(ctx, handle, digest, q))
}

func (s *EngineService) CompareExecutionEvidence(ctx context.Context, left, right, leftSHA, rightSHA, query string) (string, error) {
	var q a.CompareQuery
	if err := r.Strict([]byte(query), &q, 4096); err != nil {
		return "", err
	}
	service, err := s.evidence()
	if err != nil {
		return "", err
	}
	return caseJSON(service.CompareExecution(ctx, left, right, leftSHA, rightSHA, q))
}
func (s *EngineService) ExportExecutionComparison(ctx context.Context, left, right, leftSHA, rightSHA, query, output string) (string, error) {
	body, err := s.CompareExecutionEvidence(ctx, left, right, leftSHA, rightSHA, query)
	if err != nil {
		return "", err
	}
	roots, err := assistanceRoots()
	if err != nil {
		return "", err
	}
	if err = localfiles.Publish(ctx, output, []byte(body), roots...); err != nil {
		return "", err
	}
	return body, nil
}
