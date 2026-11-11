package analysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

// Saved-case reopens have a separate bounded session reserve. Ordinary imported
// snapshots are never evicted to make room; the reserve owns only disposable
// copies of durable case data and expires after its final caller lease.
const maxReopenedResults = 32
const maxReopenedBytes = int64(64 << 20)

func (s *Service) ordinaryCountLocked() int {
	n := 0
	for _, v := range s.stored {
		if !v.reserved {
			n++
		}
	}
	return n
}
func (s *Service) resultPathLocked(handle string) string {
	dir := "results"
	if s.stored[handle].reserved {
		dir = "reopened"
	}
	return filepath.Join(s.root, dir, handle+".json")
}

type CacheCleanup struct {
	Removed       int   `json:"removed"`
	BytesRemoved  int64 `json:"bytes_removed"`
	RetainedOwned int   `json:"retained_owned"`
}

// CleanupEvidenceCache is explicit, aggregate and owner-aware; it never exposes
// arbitrary shared-handle deletion and never touches saved cases or originals.
func (s *Service) CleanupEvidenceCache() (CacheCleanup, error) {
	ctx, finish, e := s.beginLoad(context.Background())
	if e != nil {
		return CacheCleanup{}, e
	}
	defer finish()
	s.storeMu.Lock()
	defer s.storeMu.Unlock()
	out := CacheCleanup{}
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return out, ingestion.Problem("IMPORT_SERVICE_CLOSED", "evidence service is closed")
	}
	owned := map[string]bool{}
	for _, handle := range s.leases {
		owned[handle] = true
	}
	keys := []string{}
	for key := range s.stored {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if owned[key] {
			out.RetainedOwned++
			continue
		}
		rev := s.stored[key]
		if e := os.Remove(s.resultPathLocked(key)); e != nil && !os.IsNotExist(e) {
			return out, ingestion.Problem("IMPORT_IO_FAILED", "cache cleanup partially completed; retry after closing unused screens")
		}
		delete(s.stored, key)
		if !rev.reserved {
			s.storedBytes -= rev.bytes
		}
		out.Removed++
		out.BytesRemoved += rev.bytes
	}
	return out, nil
}

func (s *Service) LoadOwnedSnapshot(parent context.Context, body []byte) (ResultView, ResultLease, error) {
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return ResultView{}, ResultLease{}, e
	}
	defer finish()
	if int64(len(body)) > s.policy.DecodedBytes {
		return ResultView{}, ResultLease{}, fmt.Errorf("result byte budget exceeded")
	}
	if _, e = resultcontract.DecodeWithLimits(body, wirejson.ResultLimits(s.policy.DecodedBytes, s.policy.StackDepth)); e != nil {
		return ResultView{}, ResultLease{}, e
	}
	dir, e := os.MkdirTemp(filepath.Join(s.root, "staging"), "reopen-")
	if e != nil {
		return ResultView{}, ResultLease{}, e
	}
	defer os.RemoveAll(dir)
	key, e := randomID("result-")
	if e != nil {
		return ResultView{}, ResultLease{}, e
	}
	staged := filepath.Join(dir, "snapshot.json")
	if e = writeDurable(staged, body); e != nil {
		return ResultView{}, ResultLease{}, e
	}
	lease, e := s.publishOwnedSnapshot(ctx, staged, key, body)
	if e != nil {
		return ResultView{}, ResultLease{}, e
	}
	view, e := s.View(lease.Handle)
	if e != nil {
		_ = s.ReleaseResultLease(lease.ID)
		return ResultView{}, ResultLease{}, e
	}
	return view, lease, nil
}
func (s *Service) publishOwnedSnapshot(ctx context.Context, staged, key string, body []byte) (ResultLease, error) {
	s.storeMu.Lock()
	defer s.storeMu.Unlock()
	if e := ctx.Err(); e != nil {
		return ResultLease{}, e
	}
	if len(s.leases) >= 4096 {
		return ResultLease{}, ingestion.Problem("IMPORT_STORE_FULL", "release unused screen leases before reopening")
	}
	digest := sha256.Sum256(body)
	sum := hex.EncodeToString(digest[:])
	for handle, rev := range s.stored {
		if rev.hash == sum {
			return s.acquireResultLocked(handle)
		}
	}
	reserved := s.ordinaryCountLocked() >= maxStoredResults || int64(len(body)) > maxStoredBytes-s.storedBytes
	if reserved {
		n := 0
		bytes := int64(0)
		for _, rev := range s.stored {
			if rev.reserved {
				n++
				bytes += rev.bytes
			}
		}
		if n >= maxReopenedResults || int64(len(body)) > maxReopenedBytes-bytes {
			return ResultLease{}, ingestion.Problem("IMPORT_REOPEN_FULL", "saved-case session reserve full; close unused saved results or explicitly clean the evidence cache")
		}
	}
	leaseID, e := randomID("lease-")
	if e != nil {
		return ResultLease{}, e
	}
	directory := "results"
	if reserved {
		directory = "reopened"
	}
	if e = os.Rename(staged, filepath.Join(s.root, directory, key+".json")); e != nil {
		return ResultLease{}, e
	}
	s.stored[key] = storedRevision{hash: sum, bytes: int64(len(body)), reserved: reserved}
	if !reserved {
		s.storedBytes += int64(len(body))
	}
	s.leases[leaseID] = key
	return ResultLease{ID: leaseID, Handle: key}, nil
}
