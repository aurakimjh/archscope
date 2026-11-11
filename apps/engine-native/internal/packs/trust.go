package packs

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
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
func (m *Manager) ApplyTrustUpdate(ctx context.Context, expected string, body, signature []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return problem("PACK_STORE_CLOSED", "pack store closed")
	}
	if expected != m.state.Revision {
		return problem("PACK_STATE_STALE", "reload state before trust update")
	}
	if len(body) > MaxManifestBytes || len(signature) > 2048 {
		return problem("PACK_BUDGET_EXCEEDED", "trust update byte limit")
	}
	var update TrustUpdate
	var sig Signature
	if Strict(body, &update) != nil || Strict(signature, &sig) != nil || update.Version != Format || len(update.Add) > 64 || len(update.Revoke) > 64 {
		return problem("PACK_TRUST_INVALID", "invalid signed trust update")
	}
	key, ok := m.state.Keys[sig.KeyID]
	if !ok || key.Revoked || !key.ManageTrust {
		return problem("PACK_TRUST_INVALID", "trusted management key required")
	}
	public, e := key.public()
	if e != nil {
		return e
	}
	raw, e := base64.StdEncoding.Strict().DecodeString(sig.Signature)
	if e != nil || !ed25519.Verify(public, body, raw) {
		return problem("PACK_SIGNATURE_INVALID", "trust signature invalid")
	}
	sequence, e := nextRevision(m.state.TrustSequence)
	if e != nil || update.Sequence != sequence {
		return problem("PACK_TRUST_STALE", "trust sequence must advance exactly once")
	}
	next := m.copyState()
	next.TrustSequence = sequence
	for _, k := range update.Add {
		if _, e = k.public(); e != nil {
			return e
		}
		if k.Revoked {
			return problem("PACK_TRUST_INVALID", "new key cannot be revoked")
		}
		if old, ok := next.Keys[k.ID]; ok && old != k {
			return problem("PACK_TRUST_CONFLICT", "rotation requires a distinct key ID")
		}
		next.Keys[k.ID] = k
	}
	for _, id := range update.Revoke {
		k, ok := next.Keys[id]
		if !ok {
			return problem("PACK_TRUST_INVALID", "revoked key is not registered")
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
		return problem("PACK_TRUST_INVALID", "at least one unrevoked management key must remain")
	}
	return m.commit(ctx, next)
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
	if _, e := m.load(ctx, m.state, scope, hash); e != nil {
		return nil, e
	}
	return m.read("versions/"+hash+".zip", MaxArchiveBytes)
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
