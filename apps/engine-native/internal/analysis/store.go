package analysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
)

// Operational retention limits. Revisions are never silently evicted: callers
// release unused handles or export saved files before reaching these ceilings.
const maxStoredResults = 1024
const maxStoredBytes int64 = 512 << 20

type storedRevision struct {
	hash     string
	bytes    int64
	reserved bool
}

func (s *Service) initStore() error {
	entries, e := os.ReadDir(filepath.Join(s.root, "results"))
	if e != nil {
		return e
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || len(name) < 5 || !handlePattern.MatchString(name[:len(name)-5]) || filepath.Ext(name) != ".json" {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() || info.Size() > maxStoredBytes || s.storedBytes > maxStoredBytes-info.Size() || len(s.stored) >= maxStoredResults {
			return ingestion.Problem("IMPORT_STORE_FULL", "stored evidence exceeds retention policy; release unused revisions")
		}
		f, e := os.Open(filepath.Join(s.root, "results", name))
		if e != nil {
			return e
		}
		hash := sha256.New()
		_, readErr := io.Copy(hash, io.LimitReader(f, info.Size()+1))
		closeErr := f.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		s.stored[name[:len(name)-5]] = storedRevision{hash: hex.EncodeToString(hash.Sum(nil)), bytes: info.Size()}
		s.storedBytes += info.Size()
	}
	return nil
}

// publish owns the atomic commit and retention accounting for both Load and
// import. Identical snapshots reuse a handle without changing provenance.
func (s *Service) publish(ctx context.Context, staged, handle string, body []byte) (string, error) {
	s.storeMu.Lock()
	defer s.storeMu.Unlock()
	if e := ctx.Err(); e != nil {
		return "", e
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	for prior, revision := range s.stored {
		if !revision.reserved && revision.hash == hash {
			return prior, nil
		}
	}
	size := int64(len(body))
	if s.ordinaryCountLocked() >= maxStoredResults || size > maxStoredBytes-s.storedBytes {
		return "", ingestion.Problem("IMPORT_STORE_FULL", "save/export needed results, release their screen leases, then use CleanupEvidenceCache or evidence cleanup-cache")
	}
	if e := os.Rename(staged, s.resultPathLocked(handle)); e != nil {
		return "", e
	}
	s.stored[handle] = storedRevision{hash: hash, bytes: size}
	s.storedBytes += size
	return handle, nil
}

// Release removes only the named immutable backend revision. Persisted files
// and other handles are unaffected. This administrative backend operation is
// absent from Wails. Leases guard shared snapshots; saved cases own durable copies.
func (s *Service) Release(handle string) error {
	if !handlePattern.MatchString(handle) {
		return fmt.Errorf("invalid result handle")
	}
	s.storeMu.Lock()
	defer s.storeMu.Unlock()
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return ingestion.Problem("IMPORT_SERVICE_CLOSED", "evidence service is closed")
	}
	for _, owned := range s.leases {
		if owned == handle {
			return ingestion.Problem("IMPORT_RESULT_IN_USE", "a caller still owns a result lease")
		}
	}
	revision, exists := s.stored[handle]
	if !exists {
		return ingestion.Problem("IMPORT_RESULT_STALE", "result is no longer available; reopen the saved file")
	}
	if e := os.Remove(s.resultPathLocked(handle)); e != nil && !os.IsNotExist(e) {
		return ingestion.Problem("IMPORT_IO_FAILED", "result release failed")
	}
	delete(s.stored, handle)
	s.dropProjectionLocked(handle)
	if !revision.reserved {
		s.storedBytes -= revision.bytes
	}
	return nil
}

// beginLoad joins service lifetime/concurrency accounting so Close cancels and
// waits for loads as well as imports. Queue waiting has its own deadline.
func (s *Service) beginLoad(parent context.Context) (context.Context, func(), error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, nil, ingestion.Problem("IMPORT_SERVICE_CLOSED", "evidence service is closed")
	}
	s.wait.Add(1)
	s.mu.Unlock()
	ctx, cancelCause := context.WithCancelCause(parent)
	cancel := func() { cancelCause(context.Canceled) }
	stop := context.AfterFunc(s.ctx, cancel)
	queue := timeAfter(s.policy.QueueTimeoutMS, func() { cancelCause(context.DeadlineExceeded) })
	queueError := func() error {
		if errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
			return ingestion.Problem("IMPORT_QUEUE_TIMEOUT", "load queue deadline exceeded")
		}
		return context.Cause(ctx)
	}
	select {
	case s.slots <- struct{}{}:
		queue.Stop()
		if e := ctx.Err(); e != nil {
			<-s.slots
			cancel()
			stop()
			s.wait.Done()
			return nil, nil, queueError()
		}
	case <-ctx.Done():
		queue.Stop()
		cancel()
		stop()
		s.wait.Done()
		return nil, nil, queueError()
	}
	execution := timeAfter(s.policy.TimeoutMS, func() { cancelCause(context.DeadlineExceeded) })
	return ctx, func() { execution.Stop(); cancel(); stop(); <-s.slots; s.wait.Done() }, nil
}

func timeAfter(ms int64, cancel context.CancelFunc) *time.Timer {
	return time.AfterFunc(time.Duration(ms)*time.Millisecond, cancel)
}
