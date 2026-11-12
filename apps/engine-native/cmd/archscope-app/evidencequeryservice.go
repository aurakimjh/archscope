package main

import (
	"context"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
)

func (s *EngineService) DescribeProfileQuery(ctx context.Context, handle string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.ProfileInventory(ctx, handle))
}
func (s *EngineService) QueryProfileEvidence(ctx context.Context, handle, digest, requestJSON string) (string, error) {
	q, e := analysis.DecodeProfileQuery([]byte(requestJSON))
	if e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.QueryProfile(ctx, handle, digest, q))
}
func (s *EngineService) SearchCaseEvidence(ctx context.Context, requestJSON string) (string, error) {
	var q cases.SearchQuery
	if e := cases.DecodeQuery([]byte(requestJSON), &q); e != nil {
		return "", e
	}
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(store.Search(ctx, q))
}
func (s *EngineService) PinVerifiedCaseBaseline(ctx context.Context, requestJSON string) (string, error) {
	var q analysis.BaselineRequest
	if e := cases.DecodeQuery([]byte(requestJSON), &q); e != nil {
		return "", e
	}
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.PinCaseBaseline(ctx, store, q))
}
func (s *EngineService) CompareVerifiedCaseBaseline(ctx context.Context, requestJSON string) (string, error) {
	var q analysis.BaselineComparisonRequest
	if e := cases.DecodeQuery([]byte(requestJSON), &q); e != nil {
		return "", e
	}
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.CompareCaseBaseline(ctx, store, q))
}

func (s *EngineService) MaterializeProfileView(ctx context.Context, handle, digest, selectionJSON string) (string, error) {
	selection, e := profilemodel.DecodeSelection([]byte(selectionJSON))
	if e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	view, lease, e := service.MaterializeProfileSelection(ctx, handle, digest, selection)
	if e != nil {
		return "", e
	}
	return caseJSON(struct {
		View  analysis.ResultView  `json:"view"`
		Lease analysis.ResultLease `json:"lease"`
	}{view, lease}, nil)
}
func (s *EngineService) OpenCaseEvidence(ctx context.Context, caseID, revision, sourceID, sourceSHA, line string) (string, error) {
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(store.OpenEvidence(ctx, caseID, revision, sourceID, sourceSHA, line))
}

func (s *EngineService) OpenProfileEvidence(ctx context.Context, caseID, revision, resultID, resultSHA, sourceID, sourceSHA, reference string) (string, error) {
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.OpenProfileEvidence(ctx, store, caseID, revision, resultID, resultSHA, sourceID, sourceSHA, reference))
}
