package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
)

// Queries retain exact numbers as JSON text across Wails; existing typed legacy
// endpoints remain compatibility facades. Generic screens belong to T-631.
func (s *EngineService) GetProfileSetTree(ctx context.Context, handle string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	tree, e := service.ProfileTree(ctx, handle)
	if e != nil {
		return "", e
	}
	body, e := json.Marshal(tree)
	return string(body), e
}
func (s *EngineService) CompareProfileSets(ctx context.Context, before, after string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	diff, e := service.ProfileCompare(ctx, before, after)
	if e != nil {
		return "", e
	}
	body, e := json.Marshal(diff)
	return string(body), e
}
func (s *EngineService) DrilldownProfileSet(ctx context.Context, handle, filtersJSON string) (string, error) {
	if len(filtersJSON) > 64<<10 {
		return "", fmt.Errorf("filter byte budget exceeded")
	}
	filters, e := profilemodel.DecodeFilters([]byte(filtersJSON))
	if e != nil {
		return "", e
	}

	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	stages, e := service.ProfileDrilldown(ctx, handle, filters)
	if e != nil {
		return "", e
	}
	body, e := json.Marshal(stages)
	return string(body), e
}
func (s *EngineService) ExportProfileSetPprof(ctx context.Context, handle, tickScale, path string) error {
	service, e := s.evidence()
	if e != nil {
		return e
	}
	body, e := service.ProfilePprof(ctx, handle, tickScale)
	if e != nil {
		return e
	}
	return analysis.AtomicExport(path, body)
}

func (s *EngineService) NormalizeLegacyProfileSet(ctx context.Context, path, format string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	set, e := service.NormalizeLegacyProfileSet(ctx, path, format)
	if e != nil {
		return "", e
	}
	body, e := json.Marshal(set)
	return string(body), e
}
