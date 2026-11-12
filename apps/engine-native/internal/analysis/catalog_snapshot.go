package analysis

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
)

type CatalogSnapshot struct {
	Mappings                []packs.Mapping
	Catalog                 *ingestion.Catalog
	Packs                   []ingestion.PackPin
	Revision, TrustSequence string
}
type CatalogProvider func(scope string) (CatalogSnapshot, error)

func (s *Service) SetCatalogProvider(provider CatalogProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.catalogProvider = provider
}
func (s *Service) CurrentImporters(scope string) ([]ingestion.ImporterDescriptor, error) {
	s.mu.Lock()
	provider := s.catalogProvider
	s.mu.Unlock()
	if provider == nil {
		return s.catalog.List(), nil
	}
	snapshot, e := provider(scope)
	if e != nil {
		return nil, e
	}
	return snapshot.Catalog.List(), nil
}
