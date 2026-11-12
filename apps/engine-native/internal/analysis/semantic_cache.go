package analysis

import (
	"context"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"time"
)

// Each request first acquires its parent and verifies the projection fingerprint.
// Cache keys include the verified immutable digest, extension and payload version.
// Cached semantic models are private read-only values, never exposed to IPC.
type semanticEntry struct {
	key     string
	value   any
	charge  int
	expires time.Time
}

func (s *Service) semantic(ctx context.Context, p *resultcontract.Projection, name string, decode func([]byte) (any, int, error)) (any, error) {
	key := p.Digest() + ":" + p.PayloadVersion() + ":" + name
	s.semanticMu.Lock()
	defer s.semanticMu.Unlock()
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	now := time.Now()
	for _, entry := range s.semanticCache {
		if entry.key == key && now.Before(entry.expires) {
			return entry.value, nil
		}
	}
	raw, e := p.ExtensionJSON(ctx, name, 32<<20)
	if e != nil {
		return nil, e
	}
	v, extra, e := decode(raw)
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	charge := 2*len(raw) + extra
	if charge > 96<<20 {
		return v, nil
	}
	kept := []semanticEntry{}
	for _, entry := range s.semanticCache {
		if entry.key != key && now.Before(entry.expires) && len(kept) < 1 && charge+entry.charge <= 96<<20 {
			kept = append(kept, entry)
			charge += entry.charge
		}
	}
	s.semanticCache = append(kept, semanticEntry{key, v, 2*len(raw) + extra, now.Add(2 * time.Minute)})
	return v, nil
}
func withSemantic[T any](s *Service, parent context.Context, handle, digest, typeName, extension string, decode func(context.Context, []byte) (any, int, error), run func(context.Context, any, string) (T, error)) (out T, err error) {
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return out, e
	}
	defer finish()
	owner, e := s.AcquireResult(handle)
	if e != nil {
		return out, e
	}
	defer s.ReleaseResultLease(owner.ID)
	p, e := s.projection(ctx, handle)
	if e != nil {
		return out, e
	}
	if p.Type() != typeName {
		return out, fmt.Errorf("SEMANTIC_RESULT_TYPE_MISMATCH")
	}
	if digest != "" && digest != p.Digest() {
		return out, fmt.Errorf("SEMANTIC_RESULT_STALE")
	}
	value, e := s.semantic(ctx, p, extension, func(raw []byte) (any, int, error) { return decode(ctx, raw) })
	if e != nil {
		return out, e
	}
	return run(ctx, value, p.Digest())
}
