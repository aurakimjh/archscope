package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/systemtrace"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/systemtrace"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
)

func (s *EngineService) InspectSystemSource(ctx context.Context, path, options string) (string, error) {
	o, e := p.DecodeOptions([]byte(options))
	if e != nil {
		return "", e
	}
	model, e := p.ParseFile(ctx, path, o)
	if e != nil {
		return "", e
	}
	return caseJSON(a.Inventory(model), nil)
}
func (s *EngineService) NormalizeSystemSource(ctx context.Context, path, options, expected, output string) (string, error) {
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
	model, e := p.ParseFile(ctx, path, o)
	if e != nil {
		return "", e
	}
	if expected == "" || model.SourceSHA256 != expected {
		return "", fmt.Errorf("SOURCE_CHANGED: inspect this source again")
	}
	body, e := json.Marshal(model)
	if e != nil {
		return "", e
	}
	if e = localfiles.Publish(ctx, output, body, roots...); e != nil {
		return "", e
	}
	return caseJSON(a.Inventory(model), nil)
}
func (s *EngineService) DescribeSystemEvidence(ctx context.Context, handle string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.SystemInventory(ctx, handle))
}
func (s *EngineService) MaterializeSystemProfile(ctx context.Context, handle, digest, metric, entitiesJSON string) (string, error) {
	var entities []string
	if e := m.Strict([]byte(entitiesJSON), &entities, 64<<10); e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	view, lease, e := service.MaterializeSystemProfile(ctx, handle, digest, metric, entities)
	if e != nil {
		return "", e
	}
	return caseJSON(struct {
		View  analysis.ResultView  `json:"view"`
		Lease analysis.ResultLease `json:"lease"`
	}{view, lease}, nil)
}
func (s *EngineService) OpenSystemEvidence(ctx context.Context, caseID, revision, resultID, resultSHA, sourceID, sourceSHA, eventID string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(service.OpenSystemEvidence(ctx, store, caseID, revision, resultID, resultSHA, sourceID, sourceSHA, eventID))
}
func (s *EngineService) QuerySystemHistogram(ctx context.Context, handle, digest, eventID string, offset, limit int) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.SystemHistogram(ctx, handle, digest, eventID, offset, limit))
}
