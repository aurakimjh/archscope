package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"time"
)

// Process-local admission units, not RSS: model JSON twice plus the index's
// conservative structural charge. Oversized valid entries are used uncached.
const metricCacheBytes = int64(192 << 20)
const metricCacheEntries = 2
const metricCacheLifetime = 2 * time.Minute

type metricSemantic struct {
	core  a.Core
	index *m.QueryIndex
}
type metricCacheEntry struct {
	digest, version string
	value           *metricSemantic
	charge          int64
	expires         time.Time
}

func (s *Service) preparedMetric(ctx context.Context, body []byte, digest string) (*metricSemantic, error) {
	select {
	case s.metricGate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.metricGate }()
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	now := time.Now()
	kept := s.metricCache[:0]
	s.metricCharge = 0
	for _, v := range s.metricCache {
		if now.Before(v.expires) {
			kept = append(kept, v)
			s.metricCharge += v.charge
		}
	}
	clear(s.metricCache[len(kept):])
	s.metricCache = kept
	for i, v := range s.metricCache {
		if v.digest == digest {
			// Versions are part of the validated immutable bytes. LRU promotion does not
			// extend the fixed lifetime or change the original verified result.
			s.metricCache = append(append(s.metricCache[:i:i], s.metricCache[i+1:]...), v)
			return v.value, nil
		}
	}
	result, e := resultcontract.DecodeWithLimits(body, wirejson.ResultLimits(s.policy.DecodedBytes, s.policy.StackDepth))
	if e != nil {
		return nil, e
	}
	if verdict := s.results.Check(result, "compute"); !verdict.Allowed {
		return nil, fmt.Errorf("%s", verdict.Reason)
	}
	core, e := a.ReadCore(result)
	if e != nil {
		return nil, e
	}
	index, e := m.PrepareQueries(ctx, core.Model)
	if e != nil {
		return nil, e
	}
	value := &metricSemantic{core, index}
	raw, e := json.Marshal(core)
	if e != nil {
		return nil, e
	}
	charge := 2*int64(len(raw)) + index.Charge()
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	if charge > metricCacheBytes {
		return value, nil
	}
	for len(s.metricCache) >= metricCacheEntries || s.metricCharge+charge > metricCacheBytes {
		s.metricCharge -= s.metricCache[0].charge
		s.metricCache[0] = metricCacheEntry{}
		s.metricCache = s.metricCache[1:]
	}
	s.metricCache = append(s.metricCache, metricCacheEntry{digest, result.Metadata.SchemaVersion, value, charge, time.Now().Add(metricCacheLifetime)})
	s.metricCharge += charge
	return value, nil
}
