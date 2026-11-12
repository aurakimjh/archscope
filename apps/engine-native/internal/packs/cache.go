package packs

import (
	"context"
	"encoding/json"
)

const maxVerifiedEntries = 64
const maxVerifiedCharge = int64(80 << 20)

type verifiedEntry struct {
	revision Revision
	charge   int64
	used     uint64
}

// Reads still hash the compressed archive on every access. Only decompression,
// signature/schema/conformance work is memoized; a cache hit never trusts disk
// metadata in place of content. Candidate trust states use a key fingerprint too.
func verificationKey(s State, ref Ref) string {
	key, _ := json.Marshal(s.Keys[ref.Signer])
	return ref.Scope + "|" + ref.ArchiveSHA256 + "|" + s.TrustSequence + "|" + Hash(key)
}
func (m *Manager) clearVerified() { m.verified = nil; m.verifiedCharge = 0 }
func (m *Manager) cacheRevision(ctx context.Context, key string, r Revision) {
	if ctx.Err() != nil {
		return
	}
	charge := int64(r.Ref.Bytes)*2 + (64 << 10)
	if charge > maxVerifiedCharge {
		return
	}
	if m.verified == nil {
		m.verified = map[string]*verifiedEntry{}
	}
	if prior := m.verified[key]; prior != nil {
		m.verifiedCharge -= prior.charge
		delete(m.verified, key)
	}
	for len(m.verified) >= maxVerifiedEntries || m.verifiedCharge+charge > maxVerifiedCharge {
		oldest := ""
		var age uint64
		for k, v := range m.verified {
			if oldest == "" || v.used < age {
				oldest = k
				age = v.used
			}
		}
		m.verifiedCharge -= m.verified[oldest].charge
		delete(m.verified, oldest)
	}
	m.verifiedClock++
	m.verified[key] = &verifiedEntry{r, charge, m.verifiedClock}
	m.verifiedCharge += charge
}
