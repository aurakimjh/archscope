package analysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

// Structural admission units include decoded values and column-index reserve;
// they are deliberately conservative, not a measurement of RSS. Cache eviction
// affects only projections, never result bytes or a caller's ownership lease.
const maxProjectionEntries = 4
const maxProjectionCharge = int64(512 << 20)

type changeStamp struct{ sec, nsec int64 }
type projectionEntry struct {
	projection *resultcontract.Projection
	info       os.FileInfo
	stamp      changeStamp
	reliable   bool
	used       uint64
}

func (s *Service) dropProjectionLocked(handle string) {
	if p := s.projections[handle]; p != nil {
		s.projectionCharge -= p.projection.Weight()
		delete(s.projections, handle)
	}
}
func (s *Service) projection(ctx context.Context, handle string) (*resultcontract.Projection, error) {
	if !handlePattern.MatchString(handle) {
		return nil, fmt.Errorf("invalid result handle")
	}
	s.storeMu.Lock()
	defer s.storeMu.Unlock()
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	rev, ok := s.stored[handle]
	if !ok {
		s.dropProjectionLocked(handle)
		return nil, ingestion.Problem("IMPORT_RESULT_STALE", "reopen a saved case or result file")
	}
	f, e := os.Open(s.resultPathLocked(handle))
	if e != nil {
		s.dropProjectionLocked(handle)
		if os.IsNotExist(e) {
			e = ingestion.Problem("IMPORT_RESULT_STALE", "result is no longer available; reopen the saved file")
		}
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > s.policy.DecodedBytes {
		s.dropProjectionLocked(handle)
		return nil, ingestion.Problem("IMPORT_RESULT_CORRUPT", "invalid stored result file")
	}
	stamp, reliable := fileChangeStamp(f, info)
	cached := s.projections[handle]
	matches := cached != nil && cached.projection.Digest() == rev.hash && os.SameFile(cached.info, info) && cached.info.Size() == info.Size() && cached.info.ModTime() == info.ModTime() && cached.stamp == stamp
	if matches && cached.reliable && reliable {
		s.projectionClock++
		cached.used = s.projectionClock
		return cached.projection, nil
	}
	// A cold/fingerprint-changed entry is fully hash checked before decoding. On
	// filesystems without a usable change stamp, every hit rehashes the bytes.
	body, e := io.ReadAll(io.LimitReader(f, s.policy.DecodedBytes+1))
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	if int64(len(body)) > s.policy.DecodedBytes {
		return nil, fmt.Errorf("stored result exceeds read budget")
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != rev.hash {
		s.dropProjectionLocked(handle)
		return nil, ingestion.Problem("IMPORT_RESULT_CORRUPT", "stored revision changed; reopen the saved case snapshot")
	}
	after, e := f.Stat()
	if e != nil {
		return nil, e
	}
	afterStamp, afterReliable := fileChangeStamp(f, after)
	if !os.SameFile(info, after) || info.Size() != after.Size() || info.ModTime() != after.ModTime() || stamp != afterStamp {
		s.dropProjectionLocked(handle)
		return nil, ingestion.Problem("IMPORT_RESULT_CORRUPT", "stored revision changed during read")
	}
	if matches {
		s.projectionClock++
		cached.used = s.projectionClock
		return cached.projection, nil
	}
	s.dropProjectionLocked(handle)
	p, e := s.results.PrepareBytes(ctx, body, wirejson.ResultLimits(s.policy.DecodedBytes, s.policy.StackDepth))
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	if p.Weight() > maxProjectionCharge {
		return p, nil
	}
	for len(s.projections) >= maxProjectionEntries || s.projectionCharge+p.Weight() > maxProjectionCharge {
		oldest := ""
		var used uint64
		for key, item := range s.projections {
			if oldest == "" || item.used < used {
				oldest = key
				used = item.used
			}
		}
		s.dropProjectionLocked(oldest)
	}
	if s.projections == nil {
		s.projections = map[string]*projectionEntry{}
	}
	s.projectionClock++
	s.projections[handle] = &projectionEntry{p, after, afterStamp, reliable && afterReliable, s.projectionClock}
	s.projectionCharge += p.Weight()
	return p, nil
}
