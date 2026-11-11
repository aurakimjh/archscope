package cases

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

const MaxCases = 128
const MaxRevisions = 1024
const MaxStoreBytes = int64(4 << 30)

type Store struct {
	mu     sync.Mutex
	root   *os.Root
	lock   *os.File
	closed bool
	page   *sourceIndex
}
type head struct {
	Revision string `json:"revision"`
	SHA256   string `json:"sha256"`
}
type Summary struct {
	ID           string `json:"id"`
	Revision     string `json:"revision"`
	Title        string `json:"title"`
	State        string `json:"state"`
	UpdatedAt    string `json:"updated_at"`
	RetentionDue bool   `json:"retention_due"`
	Problem      string `json:"problem,omitempty"`
}

// DefaultRoot is app data, never a disposable OS cache directory.
func DefaultRoot() (string, error) {
	if override := os.Getenv("ARCHSCOPE_STATE_ROOT"); override != "" {
		if !filepath.IsAbs(override) {
			return "", problem("CASE_STORE_UNSAFE", "ARCHSCOPE_STATE_ROOT must be absolute")
		}
		return override, nil
	}
	root, e := os.UserConfigDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(root, "ArchScope", "diagnostics"), nil
}
func Open(directory string) (*Store, error) {
	if e := os.MkdirAll(directory, 0700); e != nil {
		return nil, e
	}
	info, e := os.Lstat(directory)
	if e != nil {
		return nil, e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, problem("CASE_STORE_UNSAFE", "app-data root must be a real directory")
	}
	root, e := os.OpenRoot(directory)
	if e != nil {
		return nil, e
	}
	s := &Store{root: root}
	fail := func(e error) (*Store, error) {
		if s.lock != nil {
			s.lock.Close()
		}
		root.Close()
		return nil, e
	}
	if _, e = root.Lstat(".lock"); e == nil {
		if e = s.regular(".lock"); e != nil {
			return fail(e)
		}
	} else if !errors.Is(e, fs.ErrNotExist) {
		return fail(e)
	}
	s.lock, e = root.OpenFile(".lock", os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return fail(e)
	}
	if e = lockStore(s.lock); e != nil {
		return fail(problem("CASE_STORE_BUSY", "case storage is already open by another process; close the desktop app before CLI maintenance"))
	}
	for _, dir := range []string{"cases", "staging", "indexes"} {
		if e = root.MkdirAll(dir, 0700); e != nil {
			return fail(e)
		}
		if e = s.directory(dir); e != nil {
			return fail(e)
		}
	}
	// Kernel lock proves exclusive ownership. Only owned staging and head/index
	// temp files are disposable; authoritative manifests/blobs are preserved.
	if e = root.RemoveAll("staging"); e != nil {
		return fail(e)
	}
	if e = root.Mkdir("staging", 0700); e != nil {
		return fail(e)
	}
	if e = s.capacity(0); e != nil {
		return fail(e)
	}
	return s, nil
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.page = nil
	return errors.Join(s.lock.Close(), s.root.Close())
}
func (s *Store) check() error {
	if s.closed {
		return problem("CASE_STORE_CLOSED", "case store is closed")
	}
	return nil
}
func (s *Store) directory(p string) error {
	for part := p; part != "."; part = path.Dir(part) {
		i, e := s.root.Lstat(part)
		if e != nil {
			return e
		}
		if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
			return problem("CASE_STORE_UNSAFE", "non-directory or symbolic link in managed path")
		}
	}
	return nil
}
func (s *Store) regular(p string) error {
	if e := s.directory(path.Dir(p)); e != nil {
		return e
	}
	i, e := s.root.Lstat(p)
	if e != nil {
		return e
	}
	if !i.Mode().IsRegular() {
		return problem("CASE_STORE_UNSAFE", "managed file must be regular")
	}
	return nil
}
func (s *Store) read(p string, limit int64) ([]byte, error) {
	if e := s.regular(p); e != nil {
		return nil, e
	}
	f, e := s.root.Open(p)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return readBounded(f, limit)
}
func (s *Store) capacity(extra int64) error {
	var bytes int64
	count := 0
	e := fs.WalkDir(s.root.FS(), ".", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if p == ".lock" {
			return nil
		}
		count++
		if count > 32768+4*MaxCases {
			return problem("CASE_BUDGET_EXCEEDED", "case-store file limit")
		}
		i, e := d.Info()
		if e != nil {
			return e
		}
		if i.Mode()&os.ModeSymlink != 0 || !i.IsDir() && !i.Mode().IsRegular() {
			return problem("CASE_STORE_UNSAFE", "unsupported entry in case storage")
		}
		if i.Mode().IsRegular() {
			bytes += i.Size()
		}
		ceiling := MaxStoreBytes
		if extra == 0 {
			ceiling += int64(MaxCases * MaxManifestBytes)
		}
		if bytes+extra > ceiling {
			return problem("CASE_BUDGET_EXCEEDED", "case-store byte limit; archive/purge explicitly")
		}
		return nil
	})
	return e
}
func (s *Store) atomic(p string, body []byte) error {
	return s.atomicContext(context.Background(), p, body)
}
func (s *Store) atomicContext(ctx context.Context, p string, body []byte) error {
	if e := s.directory(path.Dir(p)); e != nil {
		return e
	}
	token, e := id("artifact")
	if e != nil {
		return e
	}
	tmp := path.Join(path.Dir(p), ".tmp-"+token)
	f, e := s.root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer s.root.Remove(tmp)
	_, e = f.Write(body)
	if e == nil {
		e = f.Sync()
	}
	e = errors.Join(e, f.Close())
	if e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	if e = s.root.Rename(tmp, p); e != nil {
		return e
	}
	if runtime.GOOS != "windows" {
		d, e := s.root.Open(path.Dir(p))
		if e != nil {
			return e
		}
		e = errors.Join(d.Sync(), d.Close())
		return e
	}
	return nil
}
func (s *Store) get(caseID string) (Manifest, error) {
	if e := s.check(); e != nil {
		return Manifest{}, e
	}
	if !validID(caseID, "case") {
		return Manifest{}, problem("CASE_INVALID", "invalid case identity")
	}
	b, e := s.read(path.Join("cases", caseID, "HEAD.json"), 1024)
	if e != nil {
		return Manifest{}, e
	}
	var h head
	if e = decodeStrict(b, &h); e != nil {
		return Manifest{}, e
	}
	if _, e = revision(h.Revision); e != nil || !digestRE.MatchString(h.SHA256) {
		return Manifest{}, problem("CASE_CORRUPT", "invalid committed head")
	}
	b, e = s.read(path.Join("cases", caseID, "revisions", h.Revision+".json"), MaxManifestBytes)
	if e != nil {
		return Manifest{}, e
	}
	if hash(b) != h.SHA256 {
		return Manifest{}, problem("CASE_HASH_MISMATCH", "manifest digest mismatch")
	}
	var m Manifest
	if e = decodeStrict(b, &m); e != nil {
		return m, e
	}
	if m.ID != caseID || m.Revision != h.Revision {
		return m, problem("CASE_CORRUPT", "head/manifest identity mismatch")
	}
	return m, m.validate()
}
func (s *Store) Get(caseID string) (Manifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.get(caseID)
	if e != nil {
		return m, e
	}
	s.availability(&m)
	return m, nil
}
func (s *Store) list() ([]Summary, error) {
	if e := s.check(); e != nil {
		return nil, e
	}
	entries, e := fs.ReadDir(s.root.FS(), "cases")
	if e != nil {
		return nil, e
	}
	out := []Summary{}
	for _, entry := range entries {
		if !validID(entry.Name(), "case") {
			continue
		}
		m, e := s.get(entry.Name())
		if e != nil {
			out = append(out, Summary{ID: entry.Name(), Problem: e.Error()})
			continue
		}
		due := false
		if m.RetentionDays != nil && m.State != "deleted" {
			days, _ := strconv.ParseUint(*m.RetentionDays, 10, 32)
			t, _ := time.Parse(time.RFC3339Nano, m.UpdatedAt)
			due = time.Since(t).Hours()/24 >= float64(days)
		}
		out = append(out, Summary{ID: m.ID, Revision: m.Revision, Title: m.Title, State: m.State, UpdatedAt: m.UpdatedAt, RetentionDue: due})
	}
	return out, nil
}
func (s *Store) List() ([]Summary, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.list() }
func (s *Store) Create(title string) (Manifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.check(); e != nil {
		return Manifest{}, e
	}
	list, e := s.list()
	if e != nil {
		return Manifest{}, e
	}
	if len(list) >= MaxCases {
		return Manifest{}, problem("CASE_BUDGET_EXCEEDED", "case count limit")
	}
	key, e := id("case")
	if e != nil {
		return Manifest{}, e
	}
	m := Manifest{SchemaVersion: Version, ID: key, Revision: "1", State: "active", Title: title, CreatedAt: now(), UpdatedAt: now(), Sources: []Source{}, Results: []ResultRef{}, Captures: []Capture{}, Artifacts: []Artifact{}, Baselines: []Baseline{}, Workspace: json.RawMessage("null")}
	e = s.commit(m, nil)
	if e != nil {
		if _, ge := s.get(key); ge != nil {
			s.root.RemoveAll(path.Join("cases", key))
		}
	}
	return m, e
}
func (s *Store) commit(m Manifest, objects map[string][]byte) error {
	return s.commitContext(context.Background(), m, objects)
}
func (s *Store) commitContext(ctx context.Context, m Manifest, objects map[string][]byte) error {
	s.page = nil
	if e := ctx.Err(); e != nil {
		return e
	}
	if e := m.validate(); e != nil {
		return e
	}
	n, _ := revision(m.Revision)
	if n > MaxRevisions && m.State != "deleted" && m.State != "archived" || n > MaxRevisions+2 {
		return problem("CASE_BUDGET_EXCEEDED", "revision history full; create/archive a case explicitly")
	}
	bytes := int64(0)
	required := map[string]bool{}
	for _, v := range m.Sources {
		if v.Storage == "managed" {
			required[v.SHA256] = true
		}
	}
	for _, v := range m.Results {
		required[v.SHA256] = true
	}
	for key := range required {
		if e := ctx.Err(); e != nil {
			return e
		}
		if m.State == "deleted" {
			continue
		}
		if b, ok := objects[key]; ok {
			if hash(b) != key || len(b) > MaxObjectBytes {
				return problem("CASE_HASH_MISMATCH", "object digest/size mismatch")
			}
			bytes += int64(len(b))
		} else {
			b, e := s.read(path.Join("cases", m.ID, "objects", key), MaxObjectBytes)
			if e != nil {
				return e
			}
			if hash(b) != key {
				return problem("CASE_HASH_MISMATCH", "stored object digest mismatch")
			}
			bytes += int64(len(b))
		}
	}
	if bytes > MaxCaseBytes {
		return problem("CASE_BUDGET_EXCEEDED", "referenced case byte limit")
	}
	body, e := json.Marshal(m)
	if e != nil {
		return e
	}
	if m.State != "deleted" {
		if e = s.capacity(bytes + int64(len(body)) + 1024); e != nil {
			return e
		}
	}
	base := path.Join("cases", m.ID)
	for _, d := range []string{"objects", "revisions"} {
		if e = s.root.MkdirAll(path.Join(base, d), 0700); e != nil {
			return e
		}
		if e = s.directory(path.Join(base, d)); e != nil {
			return e
		}
	}
	added := []string{}
	committed := false
	defer func() {
		if !committed {
			for _, p := range added {
				s.root.Remove(p)
			}
		}
	}()
	for key, b := range objects {
		if e := ctx.Err(); e != nil {
			return e
		}
		if !required[key] {
			continue
		}
		p := path.Join(base, "objects", key)
		old, e := s.read(p, MaxObjectBytes)
		if e == nil {
			if hash(old) != key {
				return problem("CASE_HASH_MISMATCH", "existing object changed")
			}
			continue
		}
		if !errors.Is(e, fs.ErrNotExist) {
			return e
		}
		if e = s.atomic(p, b); e != nil {
			return e
		}
		added = append(added, p)
	}
	p := path.Join(base, "revisions", m.Revision+".json")
	if _, e = s.root.Lstat(p); e == nil {
		// HEAD is the sole commit point. An unreferenced next revision is crash
		// residue and may be overwritten; committed history is never overwritten.
		old, ge := s.get(m.ID)
		if ge == nil {
			on, _ := revision(old.Revision)
			if n <= on {
				return problem("CASE_REVISION_STALE", "revision already committed")
			}
		} else if n != 1 {
			return ge
		}
	} else if !errors.Is(e, fs.ErrNotExist) {
		return e
	}
	if e = s.atomic(p, body); e != nil {
		return e
	}
	h, _ := json.Marshal(head{m.Revision, hash(body)})
	if e = s.atomicContext(ctx, path.Join(base, "HEAD.json"), h); e != nil {
		current, ce := s.root.ReadFile(path.Join(base, "HEAD.json"))
		committed = ce == nil && string(current) == string(h)
		return e
	}
	committed = true
	return nil
}
func (s *Store) expect(caseID, expected string) (Manifest, error) {
	m, e := s.get(caseID)
	if e != nil {
		return m, e
	}
	if expected != m.Revision {
		return m, problem("CASE_REVISION_STALE", "case changed; reload before editing")
	}
	return m, nil
}
func advance(m *Manifest) error {
	n, e := revision(m.Revision)
	if e != nil {
		return e
	}
	m.Revision = currentRevision(n + 1)
	m.UpdatedAt = now()
	return nil
}
func (s *Store) availability(m *Manifest) {
	available := map[string]bool{}
	for i := range m.Sources {
		v := &m.Sources[i]
		if v.Status == "failed" {
			continue
		}
		ok := false
		switch v.Storage {
		case "managed":
			b, e := s.read(path.Join("cases", m.ID, "objects", v.SHA256), MaxObjectBytes)
			ok = e == nil && hash(b) == v.SHA256
		case "external":
			b, e := readLocal(v.ExternalPath, MaxObjectBytes)
			ok = e == nil && hash(b) == v.SHA256
		}
		available[v.ID] = ok
		if !ok {
			v.Status = "missing"
			v.Reason = "Original unavailable or changed; stored result remains readable."
		} else {
			v.Status = "available"
			v.Reason = ""
		}
	}
	resultSources := map[string][]string{}
	for i := range m.Results {
		r := &m.Results[i]
		r.Availability = "snapshot-only"
		complete := len(r.SourceIDs) > 0
		for _, id := range r.SourceIDs {
			complete = complete && available[id]
		}
		if complete && !r.Stale {
			r.Availability = "originals-present-unverified"
		}
		resultSources[r.ID] = r.SourceIDs
	}
	for i := range m.Baselines {
		b := &m.Baselines[i]
		b.OriginalAvailable = len(resultSources[b.ResultID]) > 0
		for _, key := range resultSources[b.ResultID] {
			b.OriginalAvailable = b.OriginalAvailable && available[key]
		}
	}
}
func (s *Store) Object(caseID, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.get(caseID)
	if e != nil {
		return nil, e
	}
	if m.State == "deleted" {
		return nil, problem("CASE_DELETED", "restore the logical deletion before reading evidence")
	}
	if !digestRE.MatchString(key) {
		return nil, problem("CASE_INVALID", "invalid object digest")
	}
	referenced := false
	for _, r := range m.Results {
		referenced = referenced || r.SHA256 == key
	}
	for _, r := range m.Sources {
		referenced = referenced || r.Storage == "managed" && r.SHA256 == key
	}
	if !referenced {
		return nil, problem("CASE_INVALID", "unreferenced case object")
	}
	b, e := s.read(path.Join("cases", caseID, "objects", key), MaxObjectBytes)
	if e != nil {
		return nil, e
	}
	if hash(b) != key {
		return nil, problem("CASE_HASH_MISMATCH", "object changed")
	}
	return b, nil
}

// RebuildIndex is disposable. It cannot erase manifests, objects, or originals.
func (s *Store) RebuildIndex() ([]Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, e := s.list()
	if e != nil {
		return nil, e
	}
	b, e := json.Marshal(list)
	if e != nil {
		return nil, e
	}
	e = s.atomic("indexes/cases.json", b)
	return list, e
}
func (s *Store) CleanupCache() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.check(); e != nil {
		return e
	}
	s.page = nil
	for _, d := range []string{"indexes", "staging"} {
		if e := s.directory(d); e != nil {
			return e
		}
		if e := s.root.RemoveAll(d); e != nil {
			return e
		}
		if e := s.root.Mkdir(d, 0700); e != nil {
			return e
		}
	}
	return nil
}

type DeletionPlan struct {
	CaseID                    string   `json:"case_id"`
	Revision                  string   `json:"revision"`
	Token                     string   `json:"token"`
	Scopes                    []string `json:"scopes"`
	Excluded                  []string `json:"excluded"`
	PhysicalErasureGuaranteed bool     `json:"physical_erasure_guaranteed"`
}

func plan(m Manifest) DeletionPlan {
	b, _ := json.Marshal(struct{ ID, Revision string }{m.ID, m.Revision})
	return DeletionPlan{m.ID, m.Revision, hash(b), []string{"managed sources", "result snapshots", "revision history and workspace/AI specifications", "managed artifacts", "rebuildable case index and staging"}, []string{"external original sources", "separately exported files", "clipboard", "OS backups", "shared evidence-cache snapshots and caller leases (separate lifecycle)", "legacy cache retained by migration"}, false}
}
func (s *Store) DeletionPlan(caseID string) (DeletionPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.get(caseID)
	if e != nil {
		return DeletionPlan{}, e
	}
	return plan(m), nil
}
func (s *Store) Purge(caseID, expected, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.expect(caseID, expected)
	if e != nil {
		return e
	}
	if m.State != "deleted" || token != plan(m).Token {
		return problem("CASE_DELETE_SCOPE_REQUIRED", "logical deletion and current deletion-plan token required")
	}
	s.page = nil
	if e = s.root.RemoveAll(path.Join("cases", caseID)); e != nil {
		return e
	}
	return s.root.RemoveAll("indexes/cases.json")
}

// ReadSource preserves original bytes; caller chooses decoder explicitly. It
// never returns an external pathname as a URL or executes archive contents.
func (s *Store) ReadSource(caseID, sourceID string, opts DecodeOptions) (Decoded, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.get(caseID)
	if e != nil {
		return Decoded{}, e
	}
	if m.State == "deleted" {
		return Decoded{}, problem("CASE_DELETED", "restore the case first")
	}
	for _, v := range m.Sources {
		if v.ID != sourceID {
			continue
		}
		var b []byte
		if v.Storage == "external" {
			b, e = readLocal(v.ExternalPath, MaxObjectBytes)
		} else if v.Storage == "managed" {
			b, e = s.read(path.Join("cases", caseID, "objects", v.SHA256), MaxObjectBytes)
		} else {
			return Decoded{}, problem("CASE_SOURCE_MISSING", "source omitted")
		}
		if e != nil {
			return Decoded{}, e
		}
		if hash(b) != v.SHA256 {
			return Decoded{}, problem("CASE_HASH_MISMATCH", "source changed; reimport explicitly")
		}
		useSaved := opts.Encoding == "" && len(opts.Sniff) == 0 && v.Encoding != nil
		if useSaved {
			opts.Encoding = v.Encoding.Name
			opts.AllowReplacement = v.Encoding.Replacements != "0"
		}
		decoded, e := DecodeText(b, opts)
		if e == nil && useSaved {
			if decoded.Encoding.IndexRevision != v.Encoding.IndexRevision {
				return Decoded{}, problem("CASE_INDEX_STALE", "decoder changed; rebuild encoding/index explicitly")
			}
			decoded.Encoding = *v.Encoding
		}
		return decoded, e
	}
	return Decoded{}, problem("CASE_SOURCE_MISSING", "source not found")
}
