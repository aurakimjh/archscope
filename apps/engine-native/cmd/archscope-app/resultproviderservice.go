package main

import (
	"context"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
)

// AnalyzeEvidence and ImportEvidence share catalog orchestration. Existing
// Analyze* endpoints keep their compatibility contracts.
func (s *EngineService) AnalyzeEvidence(req EvidenceImportRequest) (analysis.JobSnapshot, error) {
	return s.ImportEvidence(req)
}

// Context is supplied by Wails and canceled by the per-call cancel action.
// The older context-free endpoint is retained for Go callers.
func (s *EngineService) OpenEvidenceResult(ctx context.Context, path string) (analysis.ResultView, error) {
	service, e := s.evidence()
	if e != nil {
		return analysis.ResultView{}, e
	}
	return service.Load(ctx, path)
}
func (s *EngineService) DescribeEvidenceResult(ctx context.Context, handle string) (analysis.ResultCapabilities, error) {
	service, e := s.evidence()
	if e != nil {
		return analysis.ResultCapabilities{}, e
	}
	return service.DescribeResult(ctx, handle)
}
func (s *EngineService) QueryEvidenceProvider(ctx context.Context, handle, provider string, offset, limit int) (analysis.ProviderPage, error) {
	service, e := s.evidence()
	if e != nil {
		return analysis.ProviderPage{}, e
	}
	return service.QueryResult(ctx, handle, provider, offset, limit)
}
func (s *EngineService) CompareEvidenceProvider(ctx context.Context, baseline, target, provider string) (analysis.ProviderComparison, error) {
	service, e := s.evidence()
	if e != nil {
		return analysis.ProviderComparison{}, e
	}
	return service.CompareResult(ctx, baseline, target, provider)
}
