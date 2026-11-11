package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

type EvidenceImportRequest struct {
	Path         string            `json:"path"`
	Importer     string            `json:"importer"`
	OptionsJSON  string            `json:"optionsJSON"`
	Budgets      ingestion.Budgets `json:"budgets"`
	AllowPartial bool              `json:"allowPartial"`
}

func (s *EngineService) evidence() (*analysis.Service, error) {
	s.evidenceMu.Lock()
	defer s.evidenceMu.Unlock()
	if s.evidenceService != nil {
		return s.evidenceService, nil
	}
	root, e := os.UserCacheDir()
	if e != nil {
		return nil, e
	}
	c, r, e := catalog.Builtin()
	if e != nil {
		return nil, e
	}
	durable, e := cases.DefaultRoot()
	if e != nil {
		return nil, e
	}
	service, e := analysis.OpenDurable(context.Background(), filepath.Join(root, "ArchScope", "evidence-imports"), filepath.Join(durable, "evidence-cache"), c, r)
	if e != nil {
		return nil, e
	}
	s.evidenceService = service
	return service, nil
}
func (s *EngineService) closeEvidence() {
	s.evidenceMu.Lock()
	service := s.evidenceService
	caseStore := s.caseService
	s.evidenceMu.Unlock()
	if service != nil {
		service.Close()
	}
	if caseStore != nil {
		caseStore.Close()
	}
}
func (s *EngineService) ListEvidenceImporters() ([]ingestion.ImporterDescriptor, error) {
	service, e := s.evidence()
	if e != nil {
		return nil, e
	}
	return service.ListImporters(), nil
}
func (s *EngineService) ImportEvidence(req EvidenceImportRequest) (analysis.JobSnapshot, error) {
	service, e := s.evidence()
	if e != nil {
		return analysis.JobSnapshot{}, e
	}
	return service.Start(analysis.Request{Path: req.Path, Importer: req.Importer, Options: json.RawMessage(req.OptionsJSON), Budgets: req.Budgets, AllowPartial: req.AllowPartial})
}
func (s *EngineService) GetImportJob(id string, generation uint64) (analysis.JobSnapshot, error) {
	service, e := s.evidence()
	if e != nil {
		return analysis.JobSnapshot{}, e
	}
	return service.Get(id, generation)
}
func (s *EngineService) SelectImporter(id string, generation uint64, importer string) (analysis.JobSnapshot, error) {
	service, e := s.evidence()
	if e != nil {
		return analysis.JobSnapshot{}, e
	}
	return service.Select(id, generation, importer)
}
func (s *EngineService) CancelImport(id string, generation uint64) (analysis.CancelOutcome, error) {
	service, e := s.evidence()
	if e != nil {
		return analysis.CancelOutcome{}, e
	}
	return service.Cancel(id, generation)
}
func (s *EngineService) LoadEvidenceResult(path string) (analysis.ResultView, error) {
	service, e := s.evidence()
	if e != nil {
		return analysis.ResultView{}, e
	}
	return service.Load(context.Background(), path)
}
func (s *EngineService) GetEvidenceResult(handle string) (analysis.ResultView, error) {
	service, e := s.evidence()
	if e != nil {
		return analysis.ResultView{}, e
	}
	return service.View(handle)
}

// JSON text is intentionally opaque across the Wails/JavaScript boundary.
func (s *EngineService) GetEvidenceResultJSON(handle string) (string, error) {
	service, e := s.evidence()
	if e != nil {
		return "", e
	}
	return service.ResultJSON(handle)
}
func (s *EngineService) GetEvidenceScalar(handle, path string) (resultcontract.ScalarView, error) {
	service, e := s.evidence()
	if e != nil {
		return resultcontract.ScalarView{}, e
	}
	return service.Scalar(handle, path)
}

// CompareEvidenceScalar uses caller-selected baseline and target revisions.
func (s *EngineService) CompareEvidenceScalar(baseline, target, path string) (resultcontract.Delta, error) {
	service, e := s.evidence()
	if e != nil {
		return resultcontract.Delta{}, e
	}
	return service.Compare(baseline, target, path)
}
func (s *EngineService) ExportEvidenceResult(handle, path, format string) error {
	service, e := s.evidence()
	if e != nil {
		return e
	}
	body, e := service.Export(handle, format)
	if e != nil {
		return e
	}
	return analysis.AtomicExport(path, body)
}
