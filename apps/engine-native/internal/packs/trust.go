package packs

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"
)

type TrustUpdate struct {
	Version  string   `json:"version"`
	Sequence string   `json:"sequence"`
	Add      []Key    `json:"add"`
	Revoke   []string `json:"revoke"`
}

// RegisterKey is an explicit owner/admin operation, separate from pack intake.
// A key embedded in an archive can never invoke or replace this registration.
func (m *Manager) RegisterKey(ctx context.Context, expected string, key Key) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return problem("PACK_STORE_CLOSED", "pack store closed")
	}
	if expected != m.state.Revision {
		return problem("PACK_STATE_STALE", "reload state before trust registration")
	}
	if _, e := key.public(); e != nil {
		return e
	}
	if key.Revoked {
		return problem("PACK_TRUST_INVALID", "cannot register a revoked key")
	}
	if old, ok := m.state.Keys[key.ID]; ok {
		if old != key {
			return problem("PACK_TRUST_CONFLICT", "key ID already exists; use a signed rotation with a new ID")
		}
		return nil
	}
	if len(m.state.Keys) >= 64 {
		return problem("PACK_BUDGET_EXCEEDED", "trusted key count")
	}
	next := m.copyState()
	next.Keys[key.ID] = key
	return m.commit(ctx, next)
}

type TrustPreview struct {
	ExpectedRevision string            `json:"expected_revision"`
	Sequence         string            `json:"sequence"`
	ReviewSHA256     string            `json:"review_sha256"`
	Signer           string            `json:"signer"`
	Add              []Key             `json:"add"`
	Revoke           []string          `json:"revoke"`
	Affected         []UnavailablePack `json:"affected"`
	Problems         []ScopeProblem    `json:"problems"`
}

func trustReviewHash(expected string, body, signature []byte) string {
	return Hash([]byte("archscope.offline.trust.v1\n" + expected + "\n" + Hash(body) + "\n" + Hash(signature)))
}
func (m *Manager) PreviewTrustUpdate(ctx context.Context, expected string, body, signature []byte) (TrustPreview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, preview, e := m.prepareTrustUpdate(ctx, expected, body, signature)
	return preview, e
}

// Low-level owner API preserves explicit programmatic administration. Interactive
// entry points use ApplyReviewedTrustUpdate after showing the exact-byte preview.
func (m *Manager) ApplyTrustUpdate(ctx context.Context, expected string, body, signature []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next, _, e := m.prepareTrustUpdate(ctx, expected, body, signature)
	if e != nil {
		return e
	}
	return m.commit(ctx, next)
}
func (m *Manager) ApplyReviewedTrustUpdate(ctx context.Context, expected string, body, signature []byte, reviewedSHA string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next, preview, e := m.prepareTrustUpdate(ctx, expected, body, signature)
	if e != nil {
		return e
	}
	if reviewedSHA != preview.ReviewSHA256 {
		return problem("PACK_TRUST_REVIEW_REQUIRED", "preview this exact update and signature, then provide review_sha256 before applying")
	}
	return m.commit(ctx, next)
}
func (m *Manager) prepareTrustUpdate(ctx context.Context, expected string, body, signature []byte) (State, TrustPreview, error) {
	fail := func(e error) (State, TrustPreview, error) { return State{}, TrustPreview{}, e }
	if m.closed {
		return fail(problem("PACK_STORE_CLOSED", "pack store closed"))
	}
	if expected != m.state.Revision {
		return fail(problem("PACK_STATE_STALE", "reload state before trust update"))
	}
	if len(body) > MaxManifestBytes || len(signature) > 2048 {
		return fail(problem("PACK_BUDGET_EXCEEDED", "trust update byte limit"))
	}
	var update TrustUpdate
	var sig Signature
	if Strict(body, &update) != nil || Strict(signature, &sig) != nil || update.Version != Format || len(update.Add) > 64 || len(update.Revoke) > 64 {
		return fail(problem("PACK_TRUST_INVALID", "invalid signed trust update"))
	}
	key, ok := m.state.Keys[sig.KeyID]
	if !ok || key.Revoked || !key.ManageTrust {
		return fail(problem("PACK_TRUST_INVALID", "trusted management key required"))
	}
	public, e := key.public()
	if e != nil {
		return fail(e)
	}
	raw, e := base64.StdEncoding.Strict().DecodeString(sig.Signature)
	if e != nil || !ed25519.Verify(public, body, raw) {
		return fail(problem("PACK_SIGNATURE_INVALID", "trust signature invalid"))
	}
	sequence, e := nextRevision(m.state.TrustSequence)
	if e != nil || update.Sequence != sequence {
		return fail(problem("PACK_TRUST_STALE", "trust sequence must advance exactly once"))
	}
	next := m.copyState()
	next.TrustSequence = sequence
	for _, k := range update.Add {
		if _, e = k.public(); e != nil {
			return fail(e)
		}
		if k.Revoked {
			return fail(problem("PACK_TRUST_INVALID", "new key cannot be revoked"))
		}
		if old, ok := next.Keys[k.ID]; ok && old != k {
			return fail(problem("PACK_TRUST_CONFLICT", "rotation requires a distinct key ID"))
		}
		next.Keys[k.ID] = k
	}
	for _, id := range update.Revoke {
		k, ok := next.Keys[id]
		if !ok {
			return fail(problem("PACK_TRUST_INVALID", "revoked key is not registered"))
		}
		k.Revoked = true
		next.Keys[id] = k
	}
	managers := 0
	for _, k := range next.Keys {
		if k.ManageTrust && !k.Revoked {
			managers++
		}
	}
	if managers == 0 || len(next.Keys) > 64 {
		return fail(problem("PACK_TRUST_INVALID", "at least one unrevoked management key must remain"))
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	preview := TrustPreview{ExpectedRevision: expected, Sequence: update.Sequence, ReviewSHA256: trustReviewHash(expected, body, signature), Signer: sig.KeyID, Add: update.Add, Revoke: update.Revoke, Affected: []UnavailablePack{}, Problems: []ScopeProblem{}}
	affected := map[string]UnavailablePack{}
	for key, hash := range m.state.Active {
		parts := strings.SplitN(key, "|", 2)
		ref := m.state.Installed[parts[0]+"|"+hash]
		before, exists := m.state.Keys[ref.Signer]
		after := next.Keys[ref.Signer]
		if ref.Origin == "signed" && exists && !before.Revoked && after.Revoked {
			affected[ref.Scope+"|"+ref.ID] = UnavailablePack{Pack: ref, Code: "PACK_SIGNER_UNTRUSTED"}
		}
	}
	for _, scope := range activeScopes(m.state) {
		before, _ := m.snapshot(ctx, m.state, scope)
		after, e := m.snapshot(ctx, next, scope)
		if ctx.Err() != nil {
			return fail(ctx.Err())
		}
		prior := map[string]string{}
		for _, issue := range before.Unavailable {
			prior[issue.Pack.Scope+"|"+issue.Pack.ID] = issue.Code + "|" + issue.Dependency
		}
		for _, issue := range after.Unavailable {
			key := issue.Pack.Scope + "|" + issue.Pack.ID
			if prior[key] != issue.Code+"|"+issue.Dependency {
				affected[key] = issue
			}
		}
		if e != nil {
			preview.Problems = append(preview.Problems, ScopeProblem{Scope: scope, Code: "PACK_SCOPE_UNAVAILABLE"})
		}
	}
	keys := []string{}
	for key := range affected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		preview.Affected = append(preview.Affected, affected[key])
	}
	return next, preview, nil
}
func (m *Manager) Deactivate(ctx context.Context, expected, scope, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return problem("PACK_STORE_CLOSED", "pack store closed")
	}
	if expected != m.state.Revision {
		return problem("PACK_STATE_STALE", "reload state before deactivation")
	}
	if !scopeOK(scope) || !ID.MatchString(id) {
		return problem("PACK_SCOPE_INVALID", "invalid scope/ID")
	}
	next := m.copyState()
	delete(next.Active, scope+"|"+id)
	return m.commit(ctx, next)
}
func (m *Manager) Export(ctx context.Context, scope, hash string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, problem("PACK_STORE_CLOSED", "pack store closed")
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	ref, ok := m.state.Installed[scope+"|"+hash]
	if !ok {
		return nil, problem("PACK_NOT_INSTALLED", "retained archive is not installed in this scope")
	}
	body, e := m.read("versions/"+hash+".zip", MaxArchiveBytes)
	if e != nil {
		return nil, e
	}
	if Hash(body) != ref.ArchiveSHA256 {
		return nil, problem("PACK_HASH_MISMATCH", "stored immutable archive changed")
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	// Exact retained bytes remain exportable for owner backup after revocation.
	// Export is not a trust grant: install/activation still enforce current keys.
	return body, nil
}

// SignTrust signs exact bytes for an external offline administration tool. The
// private key is owned by the caller and is never part of application state.
func SignTrust(update TrustUpdate, id string, key ed25519.PrivateKey) ([]byte, []byte, error) {
	body, e := json.Marshal(update)
	if e != nil {
		return nil, nil, e
	}
	if len(key) != ed25519.PrivateKeySize {
		return nil, nil, problem("PACK_SIGNATURE_INVALID", "Ed25519 key required")
	}
	sig, e := json.Marshal(Signature{id, base64.StdEncoding.EncodeToString(ed25519.Sign(key, body))})
	return body, sig, e
}
