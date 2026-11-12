package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/traceimport"
	l "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/traceimport"
)

func (s *EngineService) InspectLLMTraceSource(ctx context.Context, path, options string) (string, error) {
	o, e := p.DecodeLLMOptions([]byte(options))
	if e != nil {
		return "", e
	}
	set, e := p.ParseLLMFile(ctx, path, o)
	if e != nil {
		return "", e
	}
	return caseJSON(analysis.InspectLLM(set), nil)
}
func (s *EngineService) NormalizeLLMTraceSource(ctx context.Context, path, options, digest, output string) (string, error) {
	o, e := p.DecodeLLMOptions([]byte(options))
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
	set, e := p.ParseLLMFile(ctx, path, o)
	if e != nil {
		return "", e
	}
	if digest == "" || set.Source.SHA256 != digest {
		return "", fmt.Errorf("SOURCE_CHANGED: inspect the original again")
	}
	raw, e := json.Marshal(set)
	if e != nil {
		return "", e
	}
	if e = localfiles.Publish(ctx, output, raw, roots...); e != nil {
		return "", e
	}
	return caseJSON(analysis.InspectLLM(set), nil)
}
func (s *EngineService) DescribeLLMTrace(ctx context.Context, handle string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.LLMInventory(ctx, handle))
}
func (s *EngineService) QueryLLMTrace(ctx context.Context, handle, digest, query string) (string, error) {
	var q a.LLMQuery
	if e := l.Strict([]byte(query), &q, 16<<10); e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.QueryLLMTrace(ctx, handle, digest, q))
}
