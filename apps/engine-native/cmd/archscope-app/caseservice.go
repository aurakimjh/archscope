package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

func (s *EngineService) cases() (*cases.Store, error) {
	s.evidenceMu.Lock()
	defer s.evidenceMu.Unlock()
	if s.caseService != nil {
		return s.caseService, nil
	}
	root, e := cases.DefaultRoot()
	if e != nil {
		return nil, e
	}
	store, e := cases.Open(filepath.Join(root, "case-store"))
	if e == nil {
		s.caseService = store
	}
	return store, e
}
func caseJSON(v any, e error) (string, error) {
	if e != nil {
		return "", e
	}
	body, e := json.Marshal(v)
	return string(body), e
}

// Case DTOs cross Wails as JSON text: revision/count/byte/clock values never
// travel through a JavaScript floating point projection as authoritative data.
func (s *EngineService) CreateCase(title string) (string, error) {
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(store.Create(title))
}
func (s *EngineService) ListCases() (string, error) {
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(store.List())
}
func (s *EngineService) GetCase(caseID string) (string, error) {
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(store.Get(caseID))
}
func (s *EngineService) EditCase(ctx context.Context, requestJSON string) (string, error) {
	var req cases.Edit
	if e := wirejson.Decode([]byte(requestJSON), &req); e != nil {
		return "", e
	}
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(store.Apply(ctx, req))
}
func (s *EngineService) SaveCaseResult(handle, requestJSON string) (string, error) {
	var req cases.SaveResult
	if e := wirejson.Decode([]byte(requestJSON), &req); e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	owner, e := service.AcquireResult(handle)
	if e != nil {
		return "", e
	}
	defer service.ReleaseResultLease(owner.ID)
	body, _, e := service.Read(handle)
	if e != nil {
		return "", e
	}
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(store.SaveResult(req, body))
}
func (s *EngineService) ReopenCaseResult(caseID, resultID string) (string, error) {
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	body, e := store.ExportSnapshot(caseID, resultID)
	if e != nil {
		return "", e
	}
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	view, owner, e := service.LoadOwnedSnapshot(context.Background(), body)
	if e != nil {
		return "", e
	}
	return caseJSON(struct {
		View  analysis.ResultView  `json:"view"`
		Lease analysis.ResultLease `json:"lease"`
	}{view, owner}, nil)
}
func (s *EngineService) AcquireEvidenceResult(handle string) (analysis.ResultLease, error) {
	service, e := s.evidence()
	if e != nil {
		return analysis.ResultLease{}, e
	}
	return service.AcquireResult(handle)
}
func (s *EngineService) ReleaseEvidenceLease(leaseID string) error {
	service, e := s.evidence()
	if e != nil {
		return e
	}
	return service.ReleaseResultLease(leaseID)
}
func (s *EngineService) PreviewCaseArchive(ctx context.Context, path string) (string, error) {
	return caseJSON(cases.PreviewArchive(ctx, path))
}
func (s *EngineService) RestoreCaseArchive(ctx context.Context, path, digest string) (string, error) {
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(store.Restore(ctx, path, digest))
}
func (s *EngineService) ExportCaseArchive(ctx context.Context, caseID, optionsJSON, path string) error {
	var opts cases.ExportOptions
	if e := wirejson.Decode([]byte(optionsJSON), &opts); e != nil {
		return e
	}
	store, e := s.cases()
	if e != nil {
		return e
	}
	if e = store.CheckExportDestination(caseID, path); e != nil {
		return e
	}
	body, e := store.Portable(ctx, caseID, opts)
	if e != nil {
		return e
	}
	return analysis.AtomicExport(path, body)
}
func (s *EngineService) ExportCaseResult(caseID, resultID, path string) error {
	store, e := s.cases()
	if e != nil {
		return e
	}
	if e = store.CheckExportDestination(caseID, path); e != nil {
		return e
	}
	body, e := store.ExportSnapshot(caseID, resultID)
	if e != nil {
		return e
	}
	return analysis.AtomicExport(path, body)
}
func (s *EngineService) GetCaseDeletionPlan(caseID string) (string, error) {
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(store.DeletionPlan(caseID))
}
func (s *EngineService) PurgeCase(caseID, revision, token string) error {
	store, e := s.cases()
	if e != nil {
		return e
	}
	return store.Purge(caseID, revision, token)
}
func (s *EngineService) RebuildCaseIndex() (string, error) {
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(store.RebuildIndex())
}
func (s *EngineService) CleanupCaseCache() error {
	store, e := s.cases()
	if e != nil {
		return e
	}
	return store.CleanupCache()
}

// A bounded source page, never a 64 MiB WebView payload. Byte offsets are decimal
// strings into the selected source revision, not offsets into rendered text.
func (s *EngineService) GetCaseSourcePage(ctx context.Context, caseID, sourceID, optionsJSON string, start, count int) (string, error) {
	if start < 0 || count < 1 || count > 200 {
		return "", fmt.Errorf("source page requires start >=0 and count 1..200")
	}
	var opts cases.DecodeOptions
	if e := wirejson.Decode([]byte(optionsJSON), &opts); e != nil {
		return "", e
	}
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(store.SourcePage(ctx, caseID, sourceID, opts, start, count))
}

// Explicit cache cleanup removes unowned result copies only, never case data.
func (s *EngineService) CleanupEvidenceCache() (analysis.CacheCleanup, error) {
	service, e := s.evidence()
	if e != nil {
		return analysis.CacheCleanup{}, e
	}
	return service.CleanupEvidenceCache()
}

func (s *EngineService) GetCorruptCaseDeletionPlan(ctx context.Context, caseID string) (string, error) {
	store, e := s.cases()
	if e != nil {
		return "", e
	}
	return caseJSON(store.CorruptDeletionPlan(ctx, caseID))
}
func (s *EngineService) PurgeCorruptCase(ctx context.Context, caseID, token string) error {
	store, e := s.cases()
	if e != nil {
		return e
	}
	return store.PurgeCorrupt(ctx, caseID, token)
}
