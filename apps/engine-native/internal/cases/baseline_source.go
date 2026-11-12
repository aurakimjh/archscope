package cases

import (
	"context"
	"path"
	"strconv"
)

// SourceSnapshot binds bytes to a current case revision. It never returns a
// caller-selected path, an omitted source, or an unreferenced managed object.
func (s *Store) SourceSnapshot(ctx context.Context, caseID, expected, sourceID string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.expect(caseID, expected)
	if e != nil {
		return nil, e
	}
	if m.State == "deleted" {
		return nil, problem("CASE_DELETED", "restore the case first")
	}
	for _, v := range m.Sources {
		if v.ID != sourceID {
			continue
		}
		if v.Storage == "omitted" || v.Status == "failed" {
			return nil, problem("CASE_SOURCE_MISSING", "original omitted or failed")
		}
		if e = s.verifyPageSource(ctx, m.ID, v); e != nil {
			return nil, e
		}
		var b []byte
		if v.Storage == "external" {
			b, e = readLocal(v.ExternalPath, MaxObjectBytes)
		} else {
			b, e = s.read(path.Join("cases", m.ID, "objects", v.SHA256), MaxObjectBytes)
		}
		if e != nil {
			return nil, e
		}
		if hash(b) != v.SHA256 || strconv.Itoa(len(b)) != v.Size {
			return nil, problem("CASE_HASH_MISMATCH", "source changed during snapshot")
		}
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		return b, nil
	}
	return nil, problem("CASE_SOURCE_MISSING", "source not in case")
}
