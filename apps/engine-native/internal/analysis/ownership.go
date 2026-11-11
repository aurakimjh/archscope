package analysis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

type ResultLease struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
}

// AcquireResult grants one caller ownership, independently of content dedup.
// Possession of a handle alone does not authorize releasing somebody else's lease.
func (s *Service) AcquireResult(handle string) (ResultLease, error) {
	s.storeMu.Lock()
	defer s.storeMu.Unlock()
	return s.acquireResultLocked(handle)
}
func (s *Service) acquireResultLocked(handle string) (ResultLease, error) {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return ResultLease{}, ingestion.Problem("IMPORT_SERVICE_CLOSED", "evidence service is closed")
	}
	if _, ok := s.stored[handle]; !ok {
		return ResultLease{}, ingestion.Problem("IMPORT_RESULT_STALE", "reopen a saved case or result")
	}
	if len(s.leases) >= 4096 {
		return ResultLease{}, ingestion.Problem("IMPORT_STORE_FULL", "result lease limit; release unused caller leases")
	}
	key, e := randomID("lease-")
	if e != nil {
		return ResultLease{}, e
	}
	if s.leases == nil {
		s.leases = map[string]string{}
	}
	s.leases[key] = handle
	return ResultLease{key, handle}, nil
}

// ReleaseResultLease never deletes result bytes. Cache eviction is a separate
// administrative operation and cannot remove a snapshot with any live owner.
func (s *Service) ReleaseResultLease(leaseID string) error {
	s.storeMu.Lock()
	defer s.storeMu.Unlock()
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return ingestion.Problem("IMPORT_SERVICE_CLOSED", "evidence service is closed")
	}
	if _, ok := s.leases[leaseID]; !ok {
		return ingestion.Problem("IMPORT_LEASE_STALE", "caller lease missing or already released")
	}
	handle := s.leases[leaseID]
	delete(s.leases, leaseID)
	remaining := false
	for _, owner := range s.leases {
		remaining = remaining || owner == handle
	}
	if !remaining {
		s.dropProjectionLocked(handle)
	}
	if rev, ok := s.stored[handle]; ok && rev.reserved {
		for _, owner := range s.leases {
			if owner == handle {
				return nil
			}
		}
		if e := os.Remove(s.resultPathLocked(handle)); e != nil && !os.IsNotExist(e) {
			return e
		}
		delete(s.stored, handle)
		s.dropProjectionLocked(handle)
	}
	return nil
}

// LoadSnapshot rehydrates exact saved bytes after cache loss. Saved-case IDs and
// digests remain authoritative; an ephemeral cache handle is never persisted.
func (s *Service) LoadSnapshot(ctx context.Context, body []byte) (ResultView, error) {
	ctx, finish, e := s.beginLoad(ctx)
	if e != nil {
		return ResultView{}, e
	}
	defer finish()
	if int64(len(body)) > s.policy.DecodedBytes {
		return ResultView{}, fmt.Errorf("result byte budget exceeded")
	}
	if _, e = resultcontract.DecodeWithLimits(body, wirejson.ResultLimits(s.policy.DecodedBytes, s.policy.StackDepth)); e != nil {
		return ResultView{}, e
	}
	dir, e := os.MkdirTemp(filepath.Join(s.root, "staging"), "saved-case-")
	if e != nil {
		return ResultView{}, e
	}
	defer os.RemoveAll(dir)
	key, e := randomID("result-")
	if e != nil {
		return ResultView{}, e
	}
	p := filepath.Join(dir, "snapshot.json")
	if e = writeDurable(p, body); e != nil {
		return ResultView{}, e
	}
	key, e = s.publish(ctx, p, key, body)
	if e != nil {
		return ResultView{}, e
	}
	return s.View(key)
}
