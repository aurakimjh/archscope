package main

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
)

func (s *EngineService) packManager() (*packs.Manager, error) {
	if _, e := s.evidence(); e != nil {
		return nil, e
	}
	s.evidenceMu.Lock()
	defer s.evidenceMu.Unlock()
	if s.packSession == nil {
		return nil, &packs.Error{Code: "PACK_STORE_UNAVAILABLE", Message: "pack session unavailable"}
	}
	return s.packSession.Manager, nil
}
func packFile(p string, limit int) ([]byte, error) {
	info, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return nil, &packs.Error{Code: "PACK_INPUT_INVALID", Message: "bounded regular local file required; resolve links explicitly"}
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if len(b) > limit {
		return nil, &packs.Error{Code: "PACK_BUDGET_EXCEEDED", Message: "input byte limit"}
	}
	return b, e
}
func (s *EngineService) GetPackState(ctx context.Context) (string, error) {
	m, e := s.packManager()
	if e != nil {
		return "", e
	}
	return caseJSON(m.Status(ctx))
}
func (s *EngineService) InstallDataPack(ctx context.Context, path, optionsJSON string) (string, error) {
	var opts packs.InstallOptions
	if e := packs.Strict([]byte(optionsJSON), &opts); e != nil {
		return "", e
	}
	body, e := packFile(path, packs.MaxArchiveBytes)
	if e != nil {
		return "", e
	}
	m, e := s.packManager()
	if e != nil {
		return "", e
	}
	return caseJSON(m.Install(ctx, body, opts))
}
func (s *EngineService) ActivateDataPack(ctx context.Context, requestJSON string) (string, error) {
	var req struct {
		Expected string            `json:"expected_revision"`
		Scope    string            `json:"scope"`
		SHA256   string            `json:"sha256"`
		Choices  map[string]string `json:"choices"`
		Rollback bool              `json:"rollback"`
	}
	if e := packs.Strict([]byte(requestJSON), &req); e != nil {
		return "", e
	}
	m, e := s.packManager()
	if e != nil {
		return "", e
	}
	if req.Rollback {
		return caseJSON(m.Rollback(ctx, req.Expected, req.Scope, req.SHA256, req.Choices))
	}
	return caseJSON(m.Activate(ctx, req.Expected, req.Scope, req.SHA256, req.Choices))
}
func (s *EngineService) DeactivateDataPack(ctx context.Context, expected, scope, id string) error {
	m, e := s.packManager()
	if e != nil {
		return e
	}
	return m.Deactivate(ctx, expected, scope, id)
}
func (s *EngineService) RegisterPackTrustKey(ctx context.Context, expected, keyJSON string) error {
	var key packs.Key
	if e := packs.Strict([]byte(keyJSON), &key); e != nil {
		return e
	}
	m, e := s.packManager()
	if e != nil {
		return e
	}
	return m.RegisterKey(ctx, expected, key)
}
func trustFiles(path, signaturePath string) ([]byte, []byte, error) {
	body, e := packFile(path, packs.MaxManifestBytes)
	if e != nil {
		return nil, nil, e
	}
	signature, e := packFile(signaturePath, 2048)
	return body, signature, e
}
func (s *EngineService) PreviewOfflinePackTrust(ctx context.Context, expected, path, signaturePath string) (string, error) {
	body, signature, e := trustFiles(path, signaturePath)
	if e != nil {
		return "", e
	}
	m, e := s.packManager()
	if e != nil {
		return "", e
	}
	return caseJSON(m.PreviewTrustUpdate(ctx, expected, body, signature))
}

// Keep the legacy entry point for non-impacting updates, while refusing silent
// suspension of active packs. New clients review and commit exact bytes below.
func (s *EngineService) ApplyOfflinePackTrust(ctx context.Context, expected, path, signaturePath string) error {
	body, signature, e := trustFiles(path, signaturePath)
	if e != nil {
		return e
	}
	m, e := s.packManager()
	if e != nil {
		return e
	}
	preview, e := m.PreviewTrustUpdate(ctx, expected, body, signature)
	if e != nil {
		return e
	}
	if len(preview.Affected) > 0 || len(preview.Problems) > 0 {
		return &packs.Error{Code: "PACK_TRUST_REVIEW_REQUIRED", Message: "preview affected packs, then commit the reviewed update"}
	}
	return m.ApplyReviewedTrustUpdate(ctx, expected, body, signature, preview.ReviewSHA256)
}
func (s *EngineService) CommitOfflinePackTrust(ctx context.Context, expected, path, signaturePath, reviewedSHA string) error {
	body, signature, e := trustFiles(path, signaturePath)
	if e != nil {
		return e
	}
	m, e := s.packManager()
	if e != nil {
		return e
	}
	return m.ApplyReviewedTrustUpdate(ctx, expected, body, signature, reviewedSHA)
}
func (s *EngineService) ExportDataPack(ctx context.Context, scope, hash, destination string) error {
	m, e := s.packManager()
	if e != nil {
		return e
	}
	body, e := m.Export(ctx, scope, hash)
	if e != nil {
		return e
	}
	root, e := m.Directory()
	if e != nil {
		return e
	}
	target, e := filepath.Abs(destination)
	if e != nil {
		return e
	}
	if e = packs.CheckExport(root, target); e != nil {
		return e
	}
	return analysis.AtomicExport(target, body)
}

func (s *EngineService) ListEvidenceImportersForScope(scope string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return caseJSON(service.CurrentImporters(scope))
}
