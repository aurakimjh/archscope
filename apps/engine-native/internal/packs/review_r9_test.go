package packs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestR901BootstrapRevocationKeepsRecoveryOpenAndPinsDormant(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	m, e := OpenDefault(ctx, root, nil)
	if e != nil {
		t.Fatal(e)
	}
	original, e := m.Snapshot(ctx, "")
	if e != nil {
		t.Fatal(e)
	}
	old := original.Refs[0]
	owner, secret := keypair(t, "owner.root", true)
	if e = m.RegisterKey(ctx, state(t, m).Revision, owner); e != nil {
		t.Fatal(e)
	}
	update := TrustUpdate{Version: Format, Sequence: "1", Revoke: []string{old.Signer}}
	body, sig, _ := SignTrust(update, owner.ID, secret)
	expected := state(t, m).Revision
	preview, e := m.PreviewTrustUpdate(ctx, expected, body, sig)
	if e != nil || len(preview.Affected) != 1 || preview.Affected[0].Pack != old {
		t.Fatal(preview, e)
	}
	if state(t, m).Revision != expected {
		t.Fatal("preview changed state")
	}
	if e = m.ApplyReviewedTrustUpdate(ctx, expected, body, sig, preview.ReviewSHA256); e != nil {
		t.Fatal(e)
	}
	revision := state(t, m).Revision
	m.Close()
	m, e = OpenDefault(ctx, root, nil)
	if e != nil {
		t.Fatal("revoked bootstrap prevented administration", e)
	}
	defer m.Close()
	current := state(t, m)
	if current.Revision != revision || !current.Keys[old.Signer].Revoked {
		t.Fatal("bootstrap resurrected or owner state rewritten")
	}
	snapshot, e := m.Snapshot(ctx, "")
	if e != nil || len(snapshot.Mappings) != 0 || len(snapshot.Refs) != 0 || len(snapshot.Unavailable) != 1 {
		t.Fatal(snapshot, e)
	}
	if _, e = original.Mappings[0].Evaluate([]byte(`{"producer":"synthetic.metrics","producer_version":"1.0.0","metric":"latency","value_ms":1}`)); e != nil {
		t.Fatal("already pinned mapping changed", e)
	}
	archived, e := m.Export(ctx, "application", old.SHA256)
	if e != nil || Hash(archived) != old.ArchiveSHA256 {
		t.Fatal("revoked archive backup unavailable", e)
	}
	if _, e = m.Rollback(ctx, current.Revision, "application", old.SHA256, nil); e == nil {
		t.Fatal("backup granted revoked activation")
	}
	spare, _ := keypair(t, "owner.spare", true)
	if e = m.RegisterKey(ctx, state(t, m).Revision, spare); e != nil {
		t.Fatal("recovery registration failed", e)
	}
	if e = m.Deactivate(ctx, state(t, m).Revision, "application", old.ID); e != nil {
		t.Fatal("recovery deactivation failed", e)
	}
	replacement, e := m.Install(ctx, specimen(t, owner, secret, "1.1.0", nil), InstallOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Activate(ctx, state(t, m).Revision, "application", replacement.SHA256, nil); e != nil {
		t.Fatal(e)
	}
	m.Close()
	m, e = OpenDefault(ctx, root, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	snapshot, e = m.Snapshot(ctx, "")
	if e != nil || len(snapshot.Refs) != 1 || snapshot.Refs[0] != replacement {
		t.Fatal("restart changed owner replacement", snapshot, e)
	}
	// Missing historical bundled trust must not be silently registered again.
	next := m.copyState()
	delete(next.Keys, old.Signer)
	if e = m.commit(ctx, next); e != nil {
		t.Fatal(e)
	}
	m.Close()
	m, e = OpenDefault(ctx, root, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if _, exists := state(t, m).Keys[old.Signer]; exists {
		t.Fatal("missing bootstrap trust was reintroduced")
	}
}
func TestR901ExactPreviewBindingStaleStateAndDependencyImpact(t *testing.T) {
	ctx := context.Background()
	signer, secret := keypair(t, "old.signer", true)
	nextSigner, _ := keypair(t, "next.manager", true)
	m, _ := manager(t, signer)
	dependency, e := m.Install(ctx, specimen(t, signer, secret, "1.0.0", func(manifest *Manifest, _ map[string][]byte) { manifest.ID = "dependency.metrics" }), InstallOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Activate(ctx, state(t, m).Revision, "application", dependency.SHA256, nil); e != nil {
		t.Fatal(e)
	}
	if e = m.RegisterKey(ctx, state(t, m).Revision, nextSigner); e != nil {
		t.Fatal(e)
	}
	// A locally authored dependent is suspended transitively when its signed
	// dependency is revoked; it is never reclassified as a trusted publisher.
	local := specimen(t, Key{}, nil, "1.0.0", func(manifest *Manifest, _ map[string][]byte) {
		manifest.ID = "local.dependent"
		manifest.Dependencies = []Dependency{{ID: dependency.ID, Version: Range{"1.0.0", "2.0.0"}, SHA256: dependency.SHA256}}
	})
	ref, e := m.Install(ctx, local, InstallOptions{AllowUnsigned: true, Scope: "user"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Activate(ctx, state(t, m).Revision, "user", ref.SHA256, nil); e != nil {
		t.Fatal(e)
	}
	body, sig, _ := SignTrust(TrustUpdate{Version: Format, Sequence: "1", Revoke: []string{signer.ID}}, signer.ID, secret)
	revision := state(t, m).Revision
	preview, e := m.PreviewTrustUpdate(ctx, revision, body, sig)
	if e != nil || len(preview.Affected) != 2 {
		t.Fatal(preview, e)
	}
	for _, hash := range []string{"", strings.Repeat("a", 64)} {
		if e = m.ApplyReviewedTrustUpdate(ctx, revision, body, sig, hash); e == nil || state(t, m).Revision != revision {
			t.Fatal("unreviewed impact applied", e)
		}
	}
	modified := append(append([]byte{}, body...), '\n')
	if e = m.ApplyReviewedTrustUpdate(ctx, revision, modified, sig, preview.ReviewSHA256); e == nil {
		t.Fatal("changed body passed signature")
	}
	modifiedSig := append(append([]byte{}, sig...), '\n')
	if e = m.ApplyReviewedTrustUpdate(ctx, revision, body, modifiedSig, preview.ReviewSHA256); e == nil {
		t.Fatal("changed signature envelope passed preview binding")
	}
	spare, _ := keypair(t, "spare.manager", true)
	if e = m.RegisterKey(ctx, revision, spare); e != nil {
		t.Fatal(e)
	}
	if e = m.ApplyReviewedTrustUpdate(ctx, revision, body, sig, preview.ReviewSHA256); e == nil {
		t.Fatal("stale revision applied")
	}
	if e = m.ApplyReviewedTrustUpdate(ctx, state(t, m).Revision, body, sig, preview.ReviewSHA256); e == nil {
		t.Fatal("old review hash reused after state changed")
	}
	revision = state(t, m).Revision
	preview, e = m.PreviewTrustUpdate(ctx, revision, body, sig)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ApplyReviewedTrustUpdate(ctx, revision, body, sig, preview.ReviewSHA256); e != nil {
		t.Fatal(e)
	}
	snapshot, e := m.Snapshot(ctx, "user")
	if e != nil || len(snapshot.Refs) != 0 || len(snapshot.Unavailable) != 2 {
		t.Fatal(snapshot, e)
	}
	if e = m.ApplyReviewedTrustUpdate(ctx, state(t, m).Revision, body, sig, preview.ReviewSHA256); e == nil {
		t.Fatal("revoked/replayed update accepted")
	}
}
func canonicalCheck(s Snapshot) error {
	ids := map[string]bool{}
	for _, mapping := range s.Mappings {
		if ids[mapping.ID] {
			return problem("PACK_ALIAS_CONFLICT", "duplicate canonical mapping")
		}
		ids[mapping.ID] = true
	}
	return nil
}
func TestR902BothScopeActivationOrdersAreAtomic(t *testing.T) {
	for _, scope := range []string{"user", "case:case-" + strings.Repeat("a", 32)} {
		for _, applicationFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/app-first-%t", scope, applicationFirst), func(t *testing.T) {
				ctx := context.Background()
				key, secret := keypair(t, "root.key", true)
				m, e := Open(t.TempDir(), []Key{key}, canonicalCheck)
				if e != nil {
					t.Fatal(e)
				}
				defer m.Close()
				local, e := m.Install(ctx, specimen(t, Key{}, nil, "1.0.0", func(manifest *Manifest, _ map[string][]byte) { manifest.ID = "local.only" }), InstallOptions{AllowUnsigned: true, Scope: scope})
				if e != nil {
					t.Fatal(e)
				}
				app, e := m.Install(ctx, specimen(t, key, secret, "1.0.0", func(manifest *Manifest, _ map[string][]byte) { manifest.ID = "global.only" }), InstallOptions{})
				if e != nil {
					t.Fatal(e)
				}
				first, second := local, app
				if applicationFirst {
					first, second = app, local
				}
				if _, e = m.Activate(ctx, state(t, m).Revision, first.Scope, first.SHA256, nil); e != nil {
					t.Fatal(e)
				}
				before := state(t, m)
				if _, e = m.Activate(ctx, before.Revision, second.Scope, second.SHA256, nil); e == nil {
					t.Fatal("cross-scope conflict committed")
				}
				after := state(t, m)
				a, _ := json.Marshal(before)
				b, _ := json.Marshal(after)
				if string(a) != string(b) {
					t.Fatal("failed activation changed state")
				}
				if _, e = m.Snapshot(ctx, scope); e != nil {
					t.Fatal("valid existing scope damaged", e)
				}
			})
		}
	}
}
func renameMetricFixture(manifest *Manifest, files map[string][]byte, id string) {
	var mapping Mapping
	Strict(files[manifest.Mappings[0]], &mapping)
	mapping.ID = id
	mapping.Aliases = []string{id}
	files[manifest.Mappings[0]], _ = json.Marshal(mapping)
	for i := range manifest.Fixtures {
		manifest.Fixtures[i].Mapping = id
	}
}
func largeFixture(t *testing.T, key Key, secret []byte, id string) []byte {
	return specimen(t, key, secret, "1.0.0", func(manifest *Manifest, files map[string][]byte) {
		manifest.ID = id
		renameMetricFixture(manifest, files, id)
		files["fixtures/padding-a.txt"] = []byte(strings.Repeat("synthetic padding data\n", 145000))
		files["fixtures/padding-b.txt"] = []byte(strings.Repeat("synthetic padding data\n", 145000))
	})
}
func TestR903WarmSnapshotsReuseVerifiedRevisionsAndStillHashArchives(t *testing.T) {
	ctx := context.Background()
	key, secret := keypair(t, "large.signer", true)
	m, root := manager(t, key)
	for _, id := range []string{"large.a", "large.b"} {
		ref, e := m.Install(ctx, largeFixture(t, key, secret, id), InstallOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = m.Activate(ctx, state(t, m).Revision, "application", ref.SHA256, nil); e != nil {
			t.Fatal(e)
		}
	}
	before := map[string]*verifiedEntry{}
	for key, entry := range m.verified {
		before[key] = entry
	}
	started := time.Now()
	var snapshot Snapshot
	for i := 0; i < 20; i++ {
		var e error
		snapshot, e = m.Snapshot(ctx, "")
		if e != nil {
			t.Fatal(e)
		}
		if len(snapshot.Refs) != 2 {
			t.Fatal(snapshot)
		}
	}
	elapsed := time.Since(started)
	t.Logf("expanded=%d entries=%d charge=%d warm snapshots=20 elapsed=%s", snapshot.Refs[0].Bytes+snapshot.Refs[1].Bytes, len(m.verified), m.verifiedCharge, elapsed)
	for key, entry := range before {
		if m.verified[key] != entry {
			t.Fatal("warm snapshot decompressed/revalidated an unchanged revision")
		}
	}
	if elapsed > time.Second {
		t.Fatal("warm snapshots exceeded CI headroom", elapsed)
	}
	snapshot.Mappings[0].Match["/producer"] = "mutated"
	snapshot.Mappings[0].Aliases[0] = "mutated"
	fresh, e := m.Snapshot(ctx, "")
	if e != nil || fresh.Mappings[0].Match["/producer"] == "mutated" || fresh.Mappings[0].Aliases[0] == "mutated" {
		t.Fatal("caller mutated verified cache", e)
	}
	ref := fresh.Refs[0]
	path := filepath.Join(root, "versions", ref.SHA256+".zip")
	original, _ := os.ReadFile(path)
	changed := append([]byte{}, original...)
	changed[len(changed)/2] ^= 1
	os.WriteFile(path, changed, 0600)
	if _, e = m.Snapshot(ctx, ""); e == nil {
		t.Fatal("warm cache hid archive corruption")
	}
	os.WriteFile(path, original, 0600)
	// Same trust sequence with different candidate key material cannot reuse a
	// signature result prepared for another candidate owner state.
	fork := m.copyState()
	other, _ := keypair(t, key.ID, true)
	fork.Keys[key.ID] = other
	if _, e = m.load(ctx, fork, "application", ref.SHA256); e == nil {
		t.Fatal("cache ignored candidate key material")
	}
	ctxCanceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = m.Snapshot(ctxCanceled, ""); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if len(m.verified) > maxVerifiedEntries || m.verifiedCharge > maxVerifiedCharge {
		t.Fatal("verified cache exceeded retention budget")
	}
	m.Close()
	if len(m.verified) != 0 || m.verifiedCharge != 0 {
		t.Fatal("close retained payload cache")
	}
}
func TestR904MetricNamesRejectControlsWithoutRewritingUnicode(t *testing.T) {
	raw, _ := Assets.ReadFile("assets/mapping.json")
	var mapping Mapping
	Strict(raw, &mapping)
	for _, name := range []string{"a\u202e\x07", "tab\t", "line\n", "c1\u0085", "isolate\u2066", "mark\u061c", "direction\u200f"} {
		body, _ := json.Marshal(map[string]any{"producer": "synthetic.metrics", "producer_version": "1.0.0", "metric": name, "value_ms": "0"})
		if _, e := mapping.Evaluate(body); e == nil {
			t.Fatal("unsafe metric name accepted", name)
		}
	}
	for _, name := range []string{"latency", "한글.지연", "café", "worker👩‍💻"} {
		body, _ := json.Marshal(map[string]any{"producer": "synthetic.metrics", "producer_version": "1.0.0", "metric": name, "value_ms": "9007199254740993"})
		record, e := mapping.Evaluate(body)
		if e != nil || record.Metric != name || record.Value != "9007199254740.993" {
			t.Fatal(record, e)
		}
	}
}
