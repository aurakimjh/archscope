package catalog

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
)

// PackSession owns the persistent pack store and immutable compiled snapshots.
// Its provider returns the same frozen catalog to a running job after activation.
type PackSession struct {
	Manager *packs.Manager
	mu      sync.Mutex
	base    *ingestion.Catalog
	cache   map[string]*ingestion.Catalog
}

func OpenPackSession(ctx context.Context, root string, base *ingestion.Catalog) (*PackSession, error) {
	return OpenPackDirectory(ctx, filepath.Join(root, "packs"), base)
}
func OpenPackDirectory(ctx context.Context, root string, base *ingestion.Catalog) (*PackSession, error) {
	p := &PackSession{base: base, cache: map[string]*ingestion.Catalog{}}
	manager, e := packs.OpenDefault(ctx, root, func(snapshot packs.Snapshot) error { _, e := WithPacks(base, snapshot); return e })
	if e != nil {
		return nil, e
	}
	p.Manager = manager
	return p, nil
}
func (p *PackSession) Provider(scope string) (analysis.CatalogSnapshot, error) {
	snapshot, e := p.Manager.Snapshot(context.Background(), scope)
	if e != nil {
		return analysis.CatalogSnapshot{}, e
	}
	key := snapshot.Revision + "|" + scope
	p.mu.Lock()
	defer p.mu.Unlock()
	compiled := p.cache[key]
	if compiled == nil {
		compiled, e = WithPacks(p.base, snapshot)
		if e != nil {
			return analysis.CatalogSnapshot{}, e
		}
		if len(p.cache) > 32 {
			p.cache = map[string]*ingestion.Catalog{}
		}
		p.cache[key] = compiled
	}
	refs := []ingestion.PackPin{}
	for _, ref := range snapshot.Refs {
		refs = append(refs, ingestion.PackPin{ID: ref.ID, Version: ref.Version, SHA256: ref.SHA256, Origin: ref.Origin, Scope: ref.Scope})
	}
	return analysis.CatalogSnapshot{Catalog: compiled, Packs: refs, Revision: snapshot.Revision, TrustSequence: snapshot.TrustSequence}, nil
}
