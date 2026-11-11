package packs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

const MaxStateBytes = 2 << 20

type State struct {
	Version       string            `json:"version"`
	Revision      string            `json:"revision"`
	TrustSequence string            `json:"trust_sequence"`
	Keys          map[string]Key    `json:"keys"`
	Installed     map[string]Ref    `json:"installed"`
	Active        map[string]string `json:"active"`
	Choices       map[string]string `json:"choices"`
}
type Manager struct {
	mu     sync.Mutex
	root   *os.Root
	lock   *os.File
	state  State
	check  func(Snapshot) error
	closed bool
}

func Open(directory string, bootstrap []Key, check func(Snapshot) error) (*Manager, error) {
	if e := os.MkdirAll(directory, 0700); e != nil {
		return nil, e
	}
	info, e := os.Lstat(directory)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, problem("PACK_STORE_UNSAFE", "real app-data directory required")
	}
	root, e := os.OpenRoot(directory)
	if e != nil {
		return nil, e
	}
	m := &Manager{root: root, check: check}
	fail := func(e error) (*Manager, error) {
		if m.lock != nil {
			m.lock.Close()
		}
		root.Close()
		return nil, e
	}
	if i, e := root.Lstat(".lock"); e == nil && !i.Mode().IsRegular() {
		return fail(problem("PACK_STORE_UNSAFE", "invalid lock file"))
	} else if e != nil && !os.IsNotExist(e) {
		return fail(e)
	}
	m.lock, e = root.OpenFile(".lock", os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return fail(e)
	}
	if e = lockStore(m.lock); e != nil {
		return fail(problem("PACK_STORE_BUSY", "pack store is already open; close the owning app before CLI maintenance"))
	}
	for _, dir := range []string{"versions", "staging"} {
		if e = root.MkdirAll(dir, 0700); e != nil {
			return fail(e)
		}
		if i, e := root.Lstat(dir); e != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
			return fail(problem("PACK_STORE_UNSAFE", "invalid managed directory"))
		}
	}
	if e = root.RemoveAll("staging"); e != nil {
		return fail(e)
	}
	if e = root.Mkdir("staging", 0700); e != nil {
		return fail(e)
	}
	body, e := m.read("state.json", MaxStateBytes)
	if os.IsNotExist(e) {
		m.state = State{Version: Format, Revision: "0", TrustSequence: "0", Keys: map[string]Key{}, Installed: map[string]Ref{}, Active: map[string]string{}, Choices: map[string]string{}}
		for _, k := range bootstrap {
			if _, e = k.public(); e != nil {
				return fail(e)
			}
			if _, ok := m.state.Keys[k.ID]; ok {
				return fail(problem("PACK_TRUST_INVALID", "duplicate bootstrap key"))
			}
			m.state.Keys[k.ID] = k
		}
		if e = m.commit(context.Background(), m.state); e != nil {
			return fail(e)
		}
	} else if e != nil {
		return fail(e)
	} else {
		if e = Strict(body, &m.state); e != nil {
			return fail(problem("PACK_STORE_INVALID", "invalid state; preserve store and recover explicitly"))
		}
		if e = m.validateState(); e != nil {
			return fail(e)
		}
	}
	if e = m.recoverVersions(); e != nil {
		return fail(e)
	}
	return m, nil
}
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	return errors.Join(m.lock.Close(), m.root.Close())
}
func (m *Manager) read(p string, limit int) ([]byte, error) {
	i, e := m.root.Lstat(p)
	if e != nil {
		return nil, e
	}
	if !i.Mode().IsRegular() || i.Size() > int64(limit) {
		return nil, problem("PACK_STORE_UNSAFE", "bounded regular managed file required")
	}
	f, e := m.root.Open(p)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if len(b) > limit {
		return nil, problem("PACK_BUDGET_EXCEEDED", "managed file size")
	}
	return b, e
}
func (m *Manager) validateState() error {
	if m.state.Version != Format || m.state.Keys == nil || m.state.Installed == nil || m.state.Active == nil || m.state.Choices == nil || len(m.state.Keys) > 64 || len(m.state.Installed) > MaxVersions || len(m.state.Active) > 32 || len(m.state.Choices) > 256 {
		return problem("PACK_STORE_INVALID", "invalid state version/cardinality")
	}
	if _, e := nextRevision(m.state.Revision); e != nil {
		return e
	}
	if _, e := nextRevision(m.state.TrustSequence); e != nil {
		return e
	}
	for id, k := range m.state.Keys {
		if id != k.ID {
			return problem("PACK_TRUST_INVALID", "key identity mismatch")
		}
		if _, e := k.public(); e != nil {
			return e
		}
	}
	for key, ref := range m.state.Installed {
		if key != ref.Scope+"|"+ref.SHA256 || !scopeOK(ref.Scope) || !hashRE.MatchString(ref.SHA256) || !hashRE.MatchString(ref.ArchiveSHA256) {
			return problem("PACK_STORE_INVALID", "invalid installed reference")
		}
	}
	for key, hash := range m.state.Active {
		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 || !scopeOK(parts[0]) || !ID.MatchString(parts[1]) || !hashRE.MatchString(hash) {
			return problem("PACK_STORE_INVALID", "invalid active reference")
		}
		ref, ok := m.state.Installed[parts[0]+"|"+hash]
		if !ok || ref.ID != parts[1] {
			return problem("PACK_STORE_INVALID", "uninstalled active reference")
		}
	}
	for key, winner := range m.state.Choices {
		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 || !scopeOK(parts[0]) || !ID.MatchString(parts[1]) || !ID.MatchString(winner) {
			return problem("PACK_STORE_INVALID", "invalid alias choice")
		}
	}
	return nil
}
func (m *Manager) copyState() State {
	b, _ := json.Marshal(m.state)
	var s State
	json.Unmarshal(b, &s)
	return s
}
func (m *Manager) commit(ctx context.Context, s State) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if len(s.Active) > 32 || len(s.Keys) > 64 || len(s.Installed) > MaxVersions || len(s.Choices) > 256 {
		return problem("PACK_BUDGET_EXCEEDED", "pack state cardinality")
	}
	revision, e := nextRevision(m.state.Revision)
	if e != nil {
		return e
	}
	s.Revision = revision
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	if len(b) > MaxStateBytes {
		return problem("PACK_BUDGET_EXCEEDED", "pack state byte limit")
	}
	f, e := m.root.OpenFile("state.next", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(e) {
		if e = m.root.Remove("state.next"); e != nil {
			return e
		}
		f, e = m.root.OpenFile("state.next", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	}
	if e != nil {
		return e
	}
	defer m.root.Remove("state.next")
	_, e = f.Write(b)
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
	if e = m.root.Rename("state.next", "state.json"); e != nil {
		return e
	}
	m.state = s
	return nil
}
func (m *Manager) State() (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return State{}, problem("PACK_STORE_CLOSED", "pack store closed")
	}
	return m.copyState(), nil
}
func (m *Manager) Install(ctx context.Context, body []byte, opts InstallOptions) (Ref, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Ref{}, problem("PACK_STORE_CLOSED", "pack store closed")
	}
	r, e := Validate(ctx, body, m.state.Keys, opts)
	if e != nil {
		return Ref{}, e
	}
	key := r.Ref.Scope + "|" + r.Ref.SHA256
	if old, ok := m.state.Installed[key]; ok {
		return old, nil
	}
	if len(m.state.Installed) >= MaxVersions {
		return Ref{}, problem("PACK_BUDGET_EXCEEDED", "version history full; preserve/export before maintenance")
	}
	var random [16]byte
	if _, e = rand.Read(random[:]); e != nil {
		return Ref{}, e
	}
	dir := "staging/pack-" + hex.EncodeToString(random[:])
	if e = m.root.Mkdir(dir, 0700); e != nil {
		return Ref{}, e
	}
	defer m.root.RemoveAll(dir)
	staged := dir + "/archive.zip"
	f, e := m.root.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return Ref{}, e
	}
	_, e = f.Write(body)
	if e == nil {
		e = f.Sync()
	}
	e = errors.Join(e, f.Close())
	if e != nil {
		return Ref{}, e
	}
	if e = ctx.Err(); e != nil {
		return Ref{}, e
	}
	final := "versions/" + r.Ref.SHA256 + ".zip"
	added, committed := false, false
	defer func() {
		if added && !committed {
			_ = m.root.Remove(final)
		}
	}()
	if existing, e := m.read(final, MaxArchiveBytes); e == nil {
		old, e := Validate(ctx, existing, m.state.Keys, opts)
		if e != nil || old.Ref.SHA256 != r.Ref.SHA256 {
			return Ref{}, problem("PACK_STORE_INVALID", "existing immutable version changed")
		}
		r.Ref = old.Ref
	} else if os.IsNotExist(e) {
		if e = m.root.Rename(staged, final); e != nil {
			return Ref{}, e
		}
		added = true
	} else {
		return Ref{}, e
	}
	next := m.copyState()
	next.Installed[key] = r.Ref
	if e = m.commit(ctx, next); e != nil {
		return Ref{}, e
	}
	committed = true
	return r.Ref, nil
}
func (m *Manager) load(ctx context.Context, s State, scope, hash string) (Revision, error) {
	ref, ok := s.Installed[scope+"|"+hash]
	if !ok {
		return Revision{}, problem("PACK_NOT_INSTALLED", "install a verified revision before activation")
	}
	b, e := m.read("versions/"+hash+".zip", MaxArchiveBytes)
	if e != nil {
		return Revision{}, e
	}
	if Hash(b) != ref.ArchiveSHA256 {
		return Revision{}, problem("PACK_HASH_MISMATCH", "stored immutable archive changed")
	}
	r, e := Validate(ctx, b, s.Keys, InstallOptions{AllowUnsigned: ref.Origin == "locally_authored", Scope: scope})
	if e != nil {
		return r, e
	}
	if r.Ref != ref {
		return r, problem("PACK_STORE_INVALID", "stored revision identity changed")
	}
	return r, nil
}
func (m *Manager) snapshot(ctx context.Context, s State, scope string) (Snapshot, error) {
	if scope == "" {
		scope = "application"
	}
	if !scopeOK(scope) {
		return Snapshot{}, problem("PACK_SCOPE_INVALID", "select application/user/case scope explicitly")
	}
	out := Snapshot{Revision: s.Revision, TrustSequence: s.TrustSequence, Refs: []Ref{}, Mappings: []Mapping{}, Choices: map[string]string{}}
	keys := []string{}
	for key := range s.Active {
		if strings.HasPrefix(key, "application|") || strings.HasPrefix(key, scope+"|") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	total := 0
	ids := map[string]Ref{}
	for _, key := range keys {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		p := strings.SplitN(key, "|", 2)
		r, e := m.load(ctx, s, p[0], s.Active[key])
		if e != nil {
			return out, e
		}
		if r.Ref.ID != p[1] {
			return out, problem("PACK_STORE_INVALID", "active identity mismatch")
		}
		if _, ok := ids[r.Ref.ID]; ok {
			return out, problem("PACK_ALIAS_CONFLICT", "same pack ID in selected scopes; select a distinct local ID")
		}
		ids[r.Ref.ID] = r.Ref
		total += r.Ref.Bytes
		if total > MaxActiveBytes {
			return out, problem("PACK_BUDGET_EXCEEDED", "active pack memory budget")
		}
		out.Refs = append(out.Refs, r.Ref)
		out.Mappings = append(out.Mappings, r.Mappings...)
	}
	for _, key := range keys {
		p := strings.SplitN(key, "|", 2)
		r, e := m.load(ctx, s, p[0], s.Active[key])
		if e != nil {
			return out, e
		}
		for _, d := range r.Manifest.Dependencies {
			ref, ok := ids[d.ID]
			if !ok || ref.SHA256 != d.SHA256 || !d.Version.accepts(ref.Version) {
				return out, problem("PACK_DEPENDENCY_UNSATISFIED", "activate exact compatible dependency revisions first")
			}
		}
	}
	// Dependency cycles are forbidden even if every version is present.
	graph := map[string][]string{}
	for _, key := range keys {
		p := strings.SplitN(key, "|", 2)
		r, e := m.load(ctx, s, p[0], s.Active[key])
		if e != nil {
			return out, e
		}
		for _, d := range r.Manifest.Dependencies {
			graph[r.Ref.ID] = append(graph[r.Ref.ID], d.ID)
		}
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var walk func(string) bool
	walk = func(id string) bool {
		if visiting[id] {
			return false
		}
		if done[id] {
			return true
		}
		visiting[id] = true
		for _, dep := range graph[id] {
			if !walk(dep) {
				return false
			}
		}
		delete(visiting, id)
		done[id] = true
		return true
	}
	for id := range ids {
		if !walk(id) {
			return out, problem("PACK_DEPENDENCY_CYCLE", "dependency cycle")
		}
	}
	for key, v := range s.Choices {
		if strings.HasPrefix(key, "application|") {
			out.Choices[strings.TrimPrefix(key, "application|")] = v
		}
	}
	for key, v := range s.Choices {
		if strings.HasPrefix(key, scope+"|") {
			out.Choices[strings.TrimPrefix(key, scope+"|")] = v
		}
	}
	if m.check != nil {
		if e := m.check(out); e != nil {
			return out, e
		}
	}
	return out, nil
}
func (m *Manager) Snapshot(ctx context.Context, scope string) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Snapshot{}, problem("PACK_STORE_CLOSED", "pack store closed")
	}
	return m.snapshot(ctx, m.state, scope)
}
func (m *Manager) Activate(ctx context.Context, expected, scope, hash string, choices map[string]string) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Snapshot{}, problem("PACK_STORE_CLOSED", "pack store closed")
	}
	if expected != m.state.Revision {
		return Snapshot{}, problem("PACK_STATE_STALE", "reload pack state before activation")
	}
	if !scopeOK(scope) {
		return Snapshot{}, problem("PACK_SCOPE_INVALID", "invalid explicit scope")
	}
	r, e := m.load(ctx, m.state, scope, hash)
	if e != nil {
		return Snapshot{}, e
	}
	next := m.copyState()
	next.Active[scope+"|"+r.Ref.ID] = hash
	for key := range next.Choices {
		if strings.HasPrefix(key, scope+"|") {
			delete(next.Choices, key)
		}
	}
	for alias, winner := range choices {
		if !ID.MatchString(alias) || !ID.MatchString(winner) {
			return Snapshot{}, problem("PACK_ALIAS_CONFLICT", "invalid explicit alias selection")
		}
		next.Choices[scope+"|"+alias] = winner
	}
	if _, e = m.snapshot(ctx, next, scope); e != nil {
		return Snapshot{}, e
	}
	if e = m.commit(ctx, next); e != nil {
		return Snapshot{}, e
	}
	return m.snapshot(ctx, m.state, scope)
}

// Rollback validates trust/compatibility/conformance again; revoked old revisions
// cannot be reactivated. Retained results and already pinned jobs are not rewritten.
func (m *Manager) Rollback(ctx context.Context, expected, scope, hash string, choices map[string]string) (Snapshot, error) {
	return m.Activate(ctx, expected, scope, hash, choices)
}

// Reclaim only unreferenced immutable archives left between file publish and the
// state commit. All retained user-selected versions remain in Installed.
func (m *Manager) recoverVersions() error {
	f, e := m.root.Open("versions")
	if e != nil {
		return e
	}
	defer f.Close()
	entries, e := f.ReadDir(MaxVersions + 2)
	if e != nil && e != io.EOF {
		return e
	}
	if len(entries) > MaxVersions+1 {
		return problem("PACK_STORE_INVALID", "version directory entry limit")
	}
	needed := map[string]bool{}
	for _, r := range m.state.Installed {
		needed[r.SHA256+".zip"] = true
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".zip") || !hashRE.MatchString(strings.TrimSuffix(name, ".zip")) {
			return problem("PACK_STORE_INVALID", "unknown managed version entry")
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() || info.Size() > MaxArchiveBytes {
			return problem("PACK_STORE_UNSAFE", "invalid immutable archive")
		}
		if !needed[name] {
			if e = m.root.Remove("versions/" + name); e != nil {
				return e
			}
		}
	}
	return nil
}
