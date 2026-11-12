package analysis

import (
	"context"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/inputassist"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
)

func (s *Service) DiscoverInput(ctx context.Context, path, sidecar, scope string) (inputassist.Report, error) {
	s.mu.Lock()
	provider := s.catalogProvider
	catalog := s.catalog
	s.mu.Unlock()
	var mappings []packs.Mapping
	if provider != nil {
		snapshot, e := provider(scope)
		if e != nil {
			return inputassist.Report{}, e
		}
		catalog = snapshot.Catalog
		mappings = snapshot.Mappings
	}
	return inputassist.Discover(ctx, catalog, path, sidecar, mappings...)
}
