package ingestion

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

// BoundedSink is synchronous. Serialized records own their bytes before Write
// returns; no UI queue or caller-owned batch is retained.
type BoundedSink struct {
	mu           sync.Mutex
	sealed       bool
	File         *os.File
	Family       string
	Schema       SchemaRef
	Catalog      *Catalog
	Budget       Budgets
	TempUsed     int64
	Count, Bytes int64
	last         uint64
	keys         map[string]bool
}

func (s *BoundedSink) Write(ctx context.Context, b Batch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sealed {
		return Problem("IMPORT_RESULT_INVALID", "record sink is sealed")
	}
	if b.Family != s.Family {
		return Problem("IMPORT_RESULT_INVALID", "record family mismatch")
	}
	for _, r := range b.Records {
		if e := ctx.Err(); e != nil {
			return e
		}
		if r.Sequence <= s.last || r.Position.Byte < 0 || r.Position.Line < 0 || r.Position.Event < 0 {
			return Problem("IMPORT_RESULT_INVALID", "invalid record position/sequence")
		}
		if r.StackDepth > s.Budget.StackDepth {
			return Problem("IMPORT_BUDGET_EXCEEDED", "stack depth exceeded")
		}
		if int64(len(r.Payload)) > s.Budget.RecordBytes {
			return Problem("IMPORT_BUDGET_EXCEEDED", "record byte limit exceeded")
		}
		if e := s.Catalog.Validate(s.Schema, r.Payload); e != nil {
			return Problem("IMPORT_RESULT_INVALID", "record schema mismatch")
		}
		if s.keys == nil {
			s.keys = map[string]bool{}
		}
		for _, key := range r.Keys {
			s.keys[key] = true
			if len(s.keys) > s.Budget.UniqueKeys {
				return Problem("IMPORT_BUDGET_EXCEEDED", "unique key budget exceeded")
			}
		}
		data, e := json.Marshal(r)
		if e != nil {
			return e
		}
		size := int64(len(data) + 1)
		if size > s.Budget.RecordBytes {
			return Problem("IMPORT_BUDGET_EXCEEDED", "serialized record byte limit exceeded")
		}
		if s.Count >= s.Budget.Records || s.Bytes+size > s.Budget.DecodedBytes || s.TempUsed+s.Bytes+size > s.Budget.TempBytes {
			return Problem("IMPORT_BUDGET_EXCEEDED", "record/decoded/temp budget exceeded")
		}
		if _, e = s.File.Write(append(data, '\n')); e != nil {
			return e
		}
		s.Count++
		s.Bytes += size
		s.last = r.Sequence
	}
	return nil
}

type RecordFile struct {
	Path        string
	RecordBytes int64
}

func (s RecordFile) ForEach(ctx context.Context, fn func(Record) error) error {
	f, e := os.Open(s.Path)
	if e != nil {
		return e
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), int(s.RecordBytes*2+65536))
	for scanner.Scan() {
		if e = ctx.Err(); e != nil {
			return e
		}
		var r Record
		if e = wirejson.Decode(scanner.Bytes(), &r); e != nil {
			return e
		}
		if e = fn(r); e != nil {
			return e
		}
	}
	return scanner.Err()
}

type NormalizeSink struct {
	Family, Output string
	Normalize      func(context.Context, Record) (Record, error)
	Next           RecordSink
}

func (s NormalizeSink) Write(ctx context.Context, b Batch) error {
	if b.Family != s.Family {
		return Problem("IMPORT_RESULT_INVALID", "normalizer input family mismatch")
	}
	for _, r := range b.Records {
		next, e := s.Normalize(ctx, r)
		if e != nil {
			return e
		}
		if e = s.Next.Write(ctx, Batch{s.Output, []Record{next}}); e != nil {
			return e
		}
	}
	return nil
}

var _ io.Reader = (*contextReadCloser)(nil)

func (s *BoundedSink) Seal() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sealed = true
	return s.File.Sync()
}
