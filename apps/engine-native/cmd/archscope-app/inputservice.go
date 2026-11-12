package main

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/inputassist"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localconvert"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	"os"
	"path/filepath"
)

func (s *EngineService) DiscoverEvidenceInput(ctx context.Context, path, sidecar, scope string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.DiscoverInput(ctx, path, sidecar, scope))
}
func assistanceRoots() ([]string, error) {
	root, e := cases.DefaultRoot()
	if e != nil {
		return nil, e
	}
	cache, e := os.UserCacheDir()
	if e != nil {
		return nil, e
	}
	return []string{root, filepath.Join(cache, "ArchScope", "evidence-imports")}, nil
}
func (s *EngineService) ExportCollectionKit(ctx context.Context, path string) error {
	roots, e := assistanceRoots()
	if e != nil {
		return e
	}
	body, e := inputassist.Kit()
	if e != nil {
		return e
	}
	return localfiles.Publish(ctx, path, body, roots...)
}
func (s *EngineService) ExportCaptureDraft(ctx context.Context, path, sidecar, scope, output string) error {
	service, e := s.evidence()
	if e != nil {
		return e
	}
	report, e := service.DiscoverInput(ctx, path, sidecar, scope)
	if e != nil {
		return e
	}
	body, e := json.MarshalIndent(report.Draft, "", "  ")
	if e != nil {
		return e
	}
	roots, e := assistanceRoots()
	if e != nil {
		return e
	}
	return localfiles.Publish(ctx, output, append(body, '\n'), roots...)
}
func (s *EngineService) ConvertEvidenceJFR(ctx context.Context, path, tool, output string) (string, error) {
	roots, e := assistanceRoots()
	if e != nil {
		return "", e
	}
	converted, e := localconvert.ConvertNew(ctx, path, tool, output, roots...)
	if e != nil {
		return "", e
	}
	return caseJSON(converted, nil)
}
