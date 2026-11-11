package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Budgets are operational ceilings fixed before a job starts. Importer/user
// requests may lower, but never raise, the service policy.
type Budgets struct {
	InputBytes     int64 `json:"inputBytes"`
	DecodedBytes   int64 `json:"decodedBytes"`
	RecordBytes    int64 `json:"recordBytes"`
	Records        int64 `json:"records"`
	StackDepth     int   `json:"stackDepth"`
	UniqueKeys     int   `json:"uniqueKeys"`
	TempBytes      int64 `json:"tempBytes"`
	TimeoutMS      int64 `json:"timeoutMs"`
	QueueTimeoutMS int64 `json:"queueTimeoutMs"`
	ProbeBytes     int   `json:"probeBytes"`
}

func DefaultBudgets() Budgets {
	return Budgets{64 << 20, 64 << 20, 1 << 20, 100_000, 1024, 100_000, 160 << 20, int64((5 * time.Minute) / time.Millisecond), int64((5 * time.Minute) / time.Millisecond), MaxProbeBytes}
}
func (b Budgets) Within(p Budgets) (Budgets, error) {
	ints := []struct {
		value *int64
		max   int64
	}{{&b.InputBytes, p.InputBytes}, {&b.DecodedBytes, p.DecodedBytes}, {&b.RecordBytes, p.RecordBytes}, {&b.Records, p.Records}, {&b.TempBytes, p.TempBytes}, {&b.TimeoutMS, p.TimeoutMS}, {&b.QueueTimeoutMS, p.QueueTimeoutMS}}
	for _, v := range ints {
		if *v.value == 0 {
			*v.value = v.max
		}
		if *v.value < 1 || *v.value > v.max {
			return b, Problem("IMPORT_BUDGET_EXCEEDED", "requested budget exceeds policy")
		}
	}
	small := []struct {
		value *int
		max   int
	}{{&b.StackDepth, p.StackDepth}, {&b.UniqueKeys, p.UniqueKeys}, {&b.ProbeBytes, p.ProbeBytes}}
	for _, v := range small {
		if *v.value == 0 {
			*v.value = v.max
		}
		if *v.value < 1 || *v.value > v.max {
			return b, Problem("IMPORT_BUDGET_EXCEEDED", "requested budget exceeds policy")
		}
	}
	return b, nil
}

type SourceLease struct {
	mu      sync.Mutex
	closed  bool
	readers map[*leaseReader]bool
	path    string
	info    SourceInfo
}

func (l *SourceLease) Info() SourceInfo { return l.info }
func (l *SourceLease) Open(ctx context.Context) (io.ReadCloser, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil, fmt.Errorf("source lease is closed")
	}
	f, e := os.Open(l.path)
	if e != nil {
		return nil, e
	}
	reader := &leaseReader{ReadCloser: &contextReadCloser{ctx, &limitedReadCloser{Reader: io.LimitReader(f, l.info.Size+1), Closer: f}}, owner: l}
	if l.readers == nil {
		l.readers = map[*leaseReader]bool{}
	}
	l.readers[reader] = true
	context.AfterFunc(ctx, func() { _ = reader.Close() })
	return reader, nil
}

type leaseReader struct {
	io.ReadCloser
	owner *SourceLease
	once  sync.Once
	err   error
}

func (r *leaseReader) Close() error {
	r.once.Do(func() {
		r.err = r.ReadCloser.Close()
		r.owner.mu.Lock()
		delete(r.owner.readers, r)
		r.owner.mu.Unlock()
	})
	return r.err
}
func (l *SourceLease) Close() error {
	l.mu.Lock()
	l.closed = true
	readers := make([]*leaseReader, 0, len(l.readers))
	for r := range l.readers {
		readers = append(readers, r)
	}
	l.mu.Unlock()
	var result error
	for _, r := range readers {
		if e := r.Close(); e != nil {
			result = e
		}
	}
	return result
}

type contextReadCloser struct {
	ctx context.Context
	io.ReadCloser
}

func (r *contextReadCloser) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.ReadCloser.Read(b)
}
func (l *SourceLease) Verify(ctx context.Context) error {
	f, e := l.Open(ctx)
	if e != nil {
		return e
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, f)
	if e != nil {
		return e
	}
	if n != l.info.Size || hex.EncodeToString(h.Sum(nil)) != l.info.SHA256 {
		return Problem("SOURCE_CHANGED", "owned snapshot changed")
	}
	return nil
}

// Acquire snapshots before probing. A second hash/stat pass detects in-place
// edits and path replacement while copying; later parsing only sees owned bytes.
func Acquire(ctx context.Context, path, directory, id string, b Budgets) (*SourceLease, error) {
	selected, statErr := os.Stat(path)
	if statErr != nil || !selected.Mode().IsRegular() {
		return nil, Problem("IMPORT_SOURCE_UNAVAILABLE", "select a regular local file")
	}
	source, e := os.Open(path)
	if e != nil {
		return nil, Problem("IMPORT_SOURCE_UNAVAILABLE", "cannot open selected source")
	}
	defer source.Close()
	stop := context.AfterFunc(ctx, func() { _ = source.Close() })
	defer stop()
	before, e := source.Stat()
	if e != nil || !before.Mode().IsRegular() {
		return nil, Problem("IMPORT_SOURCE_UNAVAILABLE", "source must be a regular file")
	}
	if before.Size() > b.InputBytes || before.Size() > b.TempBytes {
		return nil, Problem("IMPORT_BUDGET_EXCEEDED", "snapshot exceeds input/temp budget")
	}
	target := filepath.Join(directory, "source.snapshot")
	out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	h := sha256.New()
	reader := &contextReadCloser{ctx, source}
	n, e := io.Copy(io.MultiWriter(out, h), io.LimitReader(reader, b.InputBytes+1))
	if e == nil {
		e = out.Sync()
	}
	closeErr := out.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return nil, e
	}
	if n > b.InputBytes || n > b.TempBytes {
		return nil, Problem("IMPORT_BUDGET_EXCEEDED", "source grew beyond budget")
	}
	after, e := source.Stat()
	if e != nil {
		return nil, e
	}
	named, e := os.Stat(path)
	if e != nil || !os.SameFile(before, named) || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || n != before.Size() {
		return nil, Problem("SOURCE_CHANGED", "source changed while taking snapshot")
	}
	if _, e = source.Seek(0, io.SeekStart); e != nil {
		return nil, e
	}
	again := sha256.New()
	n2, e := io.Copy(again, io.LimitReader(reader, b.InputBytes+1))
	if e != nil {
		return nil, e
	}
	namedFinal, namedErr := os.Stat(path)
	final, e := source.Stat()
	if e != nil || namedErr != nil || !os.SameFile(before, namedFinal) || n != n2 || !final.ModTime().Equal(before.ModTime()) || hex.EncodeToString(h.Sum(nil)) != hex.EncodeToString(again.Sum(nil)) {
		return nil, Problem("SOURCE_CHANGED", "source content changed while taking snapshot")
	}
	return &SourceLease{path: target, info: SourceInfo{ID: id, Name: filepath.Base(path), Size: n, SHA256: fmt.Sprintf("%x", h.Sum(nil))}}, nil
}

type limitedReadCloser struct {
	io.Reader
	io.Closer
}
