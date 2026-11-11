package cases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"path"
	"strconv"
)

type CorruptDeletionPlan struct {
	CaseID                    string   `json:"case_id"`
	Directory                 string   `json:"directory"`
	Token                     string   `json:"token"`
	Files                     int      `json:"files"`
	Bytes                     string   `json:"bytes"`
	Scopes                    []string `json:"scopes"`
	Excluded                  []string `json:"excluded"`
	PhysicalErasureGuaranteed bool     `json:"physical_erasure_guaranteed"`
}

// A corrupt manifest cannot authorize normal lifecycle operations. This separate
// explicit plan binds deletion to the entire current managed tree, never to
// external paths recovered from corrupt bytes or an unverified manifest.
func (s *Store) corruptPlan(ctx context.Context, caseID string) (CorruptDeletionPlan, error) {
	out := CorruptDeletionPlan{}
	if e := s.check(); e != nil {
		return out, e
	}
	if !validID(caseID, "case") {
		return out, problem("CASE_INVALID", "invalid case ID")
	}
	base := path.Join("cases", caseID)
	if e := s.directory(base); e != nil {
		return out, e
	}
	if _, e := s.get(caseID); e == nil {
		return out, problem("CASE_NOT_CORRUPT", "use logical deletion and the normal deletion plan for readable cases")
	}
	out = CorruptDeletionPlan{CaseID: caseID, Directory: base, Scopes: []string{"all managed files in this corrupt case directory"}, Excluded: []string{"external originals", "exports", "other cases", "shared/legacy evidence caches", "OS backups"}}
	digest := sha256.New()
	entries := 0
	var size int64
	e := fs.WalkDir(s.root.FS(), base, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		entries++
		if entries > 32768+4*MaxCases {
			return problem("CASE_BUDGET_EXCEEDED", "corrupt-case tree entry limit")
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return problem("CASE_STORE_UNSAFE", "corrupt-case plan requires regular managed files; preserve and recover manually")
		}
		header, _ := json.Marshal([]string{p, info.Mode().String(), strconv.FormatInt(info.Size(), 10)})
		digest.Write(header)
		if info.IsDir() {
			return nil
		}
		out.Files++
		if info.Size() > MaxStoreBytes-size {
			return problem("CASE_BUDGET_EXCEEDED", "corrupt-case tree byte limit")
		}
		size += info.Size()
		if e = s.regular(p); e != nil {
			return e
		}
		f, e := s.root.Open(p)
		if e != nil {
			return e
		}
		defer f.Close()
		n, e := io.Copy(digest, &cancelReader{ctx: ctx, r: io.LimitReader(f, info.Size()+1)})
		if e != nil {
			return e
		}
		if n != info.Size() {
			return problem("CASE_CORRUPT_CHANGED", "managed tree changed during deletion planning")
		}
		return nil
	})
	if e != nil {
		return out, e
	}
	out.Bytes = strconv.FormatInt(size, 10)
	token, _ := json.Marshal([]string{"corrupt-case-purge-v1", caseID, hex.EncodeToString(digest.Sum(nil))})
	out.Token = hash(token)
	return out, nil
}

type cancelReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *cancelReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(b)
}
func (s *Store) CorruptDeletionPlan(ctx context.Context, caseID string) (CorruptDeletionPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.corruptPlan(ctx, caseID)
}
func (s *Store) PurgeCorrupt(ctx context.Context, caseID, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, e := s.corruptPlan(ctx, caseID)
	if e != nil {
		return e
	}
	if token == "" || token != plan.Token {
		return problem("CASE_DELETE_SCOPE_REQUIRED", "current corrupt-case tree plan token required")
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	s.page = nil
	if e = s.root.RemoveAll(plan.Directory); e != nil {
		return e
	}
	return s.root.RemoveAll("indexes/cases.json")
}
