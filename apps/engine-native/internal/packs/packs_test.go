package packs

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func keypair(t *testing.T, id string, manage bool) (Key, ed25519.PrivateKey) {
	t.Helper()
	public, private, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	return Key{ID: id, Public: base64.StdEncoding.EncodeToString(public), ManageTrust: manage}, private
}
func specimen(t *testing.T, key Key, private ed25519.PrivateKey, version string, mutate func(*Manifest, map[string][]byte)) []byte {
	t.Helper()
	m := Manifest{Format: Format, ID: "fixture.metrics", Version: version, Signer: key.ID, App: Range{"0.3.0", "0.4.0"}, Adapter: Range{"1.0.0", "1.1.0"}, Extension: Range{"1.0.0", "1.1.0"}, Dependencies: []Dependency{}, License: "CC0-1.0", LicenseFile: "licenses/CC0.txt", Changes: "CHANGELOG.md", Mappings: []string{"mappings/metrics.json"}, Fixtures: []Fixture{{"data.synthetic-metrics", "fixtures/input.jsonl", "fixtures/expected.json", true}, {"data.synthetic-metrics", "fixtures/invalid.jsonl", "fixtures/rejected.json", false}}}
	files := map[string][]byte{}
	for dst, src := range map[string]string{"mappings/metrics.json": "mapping.json", "fixtures/input.jsonl": "input.jsonl", "fixtures/expected.json": "expected.json", "fixtures/invalid.jsonl": "invalid.jsonl", "fixtures/rejected.json": "rejected.json", "licenses/CC0.txt": "LICENSE.txt"} {
		b, e := Assets.ReadFile("assets/" + src)
		if e != nil {
			t.Fatal(e)
		}
		files[dst] = b
	}
	files["CHANGELOG.md"] = []byte("synthetic revision\n")
	if mutate != nil {
		mutate(&m, files)
	}
	b, e := Archive(m, files, private)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func manager(t *testing.T, keys ...Key) (*Manager, string) {
	t.Helper()
	root := t.TempDir()
	m, e := Open(root, keys, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { m.Close() })
	return m, root
}
func rewriteZip(t *testing.T, b []byte, change func(string, []byte) (string, []byte)) []byte {
	t.Helper()
	r, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for _, f := range r.File {
		rd, _ := f.Open()
		v := new(bytes.Buffer)
		v.ReadFrom(rd)
		rd.Close()
		name, data := change(f.Name, v.Bytes())
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0600)
		w, _ := z.CreateHeader(h)
		w.Write(data)
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func state(t *testing.T, m *Manager) State {
	t.Helper()
	s, e := m.State()
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestExactSignaturesDataBudgetsAndNoSelfTrust(t *testing.T) {
	key, private := keypair(t, "fixture.signer", true)
	m, _ := manager(t, key)
	valid := specimen(t, key, private, "1.0.0", nil)
	ref, e := m.Install(context.Background(), valid, InstallOptions{})
	if e != nil {
		t.Fatal(e)
	}
	revision := state(t, m).Revision
	for name, body := range map[string][]byte{
		"manifest whitespace": rewriteZip(t, valid, func(p string, b []byte) (string, []byte) {
			if p == "manifest.json" {
				b = append([]byte(" "), b...)
			}
			return p, b
		}),
		"file hash": rewriteZip(t, valid, func(p string, b []byte) (string, []byte) {
			if p == "CHANGELOG.md" {
				b = []byte("changed")
			}
			return p, b
		}),
		"traversal": rewriteZip(t, valid, func(p string, b []byte) (string, []byte) {
			if p == "CHANGELOG.md" {
				p = "../escape.md"
			}
			return p, b
		}),
		"case duplicate": rewriteZip(t, valid, func(p string, b []byte) (string, []byte) {
			if p == "CHANGELOG.md" {
				p = "MANIFEST.JSON"
			}
			return p, b
		}),
		"trailing":        append(append([]byte{}, valid...), []byte("extra")...),
		"executable":      specimen(t, key, private, "1.0.1", func(_ *Manifest, f map[string][]byte) { f["payload.dll"] = []byte("not data") }),
		"fixture failure": specimen(t, key, private, "1.0.1", func(_ *Manifest, f map[string][]byte) { f["fixtures/expected.json"] = []byte(`[]`) }),
		"unknown syntax": specimen(t, key, private, "1.0.1", func(_ *Manifest, f map[string][]byte) {
			var v map[string]any
			json.Unmarshal(f["mappings/metrics.json"], &v)
			v["script"] = "run()"
			f["mappings/metrics.json"], _ = json.Marshal(v)
		}),
	} {
		t.Run(name, func(t *testing.T) {
			if _, e = m.Install(context.Background(), body, InstallOptions{}); e == nil {
				t.Fatal("invalid pack installed")
			}
			if state(t, m).Revision != revision {
				t.Fatal("failed intake mutated state")
			}
		})
	}
	foreign, secret := keypair(t, "self.offered", true)
	untrusted := specimen(t, foreign, secret, "1.0.0", func(_ *Manifest, f map[string][]byte) { f["fixtures/offered-key.json"], _ = json.Marshal(foreign) })
	if _, e = m.Install(context.Background(), untrusted, InstallOptions{}); e == nil {
		t.Fatal("pack registered its own signer")
	}
	if len(state(t, m).Keys) != 1 {
		t.Fatal("key store changed")
	}
	if _, e = m.Activate(context.Background(), revision, "application", ref.SHA256, nil); e != nil {
		t.Fatal(e)
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	before := state(t, m).Revision
	if _, e = m.Install(canceled, specimen(t, key, private, "1.0.2", nil), InstallOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if state(t, m).Revision != before {
		t.Fatal("canceled install committed")
	}
}
func TestActivationRollbackPinsAndOfflineRevocation(t *testing.T) {
	key, private := keypair(t, "root.old", true)
	nextKey, _ := keypair(t, "root.new", true)
	m, _ := manager(t, key)
	first, e := m.Install(context.Background(), specimen(t, key, private, "1.0.0", nil), InstallOptions{})
	if e != nil {
		t.Fatal(e)
	}
	old, e := m.Activate(context.Background(), state(t, m).Revision, "application", first.SHA256, nil)
	if e != nil {
		t.Fatal(e)
	}
	second, e := m.Install(context.Background(), specimen(t, key, private, "1.1.0", nil), InstallOptions{})
	if e != nil {
		t.Fatal(e)
	}
	current, e := m.Activate(context.Background(), state(t, m).Revision, "application", second.SHA256, nil)
	if e != nil || current.Refs[0].Version != "1.1.0" || old.Refs[0].Version != "1.0.0" {
		t.Fatal(current, e)
	}
	if _, e = m.Rollback(context.Background(), state(t, m).Revision, "application", first.SHA256, nil); e != nil {
		t.Fatal(e)
	}
	update := TrustUpdate{Version: Format, Sequence: "1", Add: []Key{nextKey}, Revoke: []string{key.ID}}
	body, sig, e := SignTrust(update, key.ID, private)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ApplyTrustUpdate(context.Background(), state(t, m).Revision, body, sig); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Snapshot(context.Background(), ""); e == nil {
		t.Fatal("revoked active signer remained available to new jobs")
	}
	if _, e = m.Rollback(context.Background(), state(t, m).Revision, "application", first.SHA256, nil); e == nil {
		t.Fatal("revoked rollback accepted")
	}
	if e = m.ApplyTrustUpdate(context.Background(), state(t, m).Revision, body, sig); e == nil {
		t.Fatal("replayed trust list accepted")
	}
	// Already pinned bytes remain usable; their old trust sequence is disclosed.
	if _, e = old.Mappings[0].Evaluate([]byte(`{"producer":"synthetic.metrics","producer_version":"1.0.0","metric":"latency","value_ms":1}`)); e != nil {
		t.Fatal(e)
	}
	if e = m.RegisterKey(context.Background(), state(t, m).Revision, Key{ID: key.ID, Public: nextKey.Public}); e == nil {
		t.Fatal("key ID was rebound")
	}
}
func TestUnsignedScopeDependencyAndRestart(t *testing.T) {
	key, private := keypair(t, "fixture.signer", true)
	m, root := manager(t, key)
	unsigned := specimen(t, Key{}, nil, "1.0.0", nil)
	if _, e := m.Install(context.Background(), unsigned, InstallOptions{}); e == nil {
		t.Fatal("unsigned pack trusted implicitly")
	}
	ref, e := m.Install(context.Background(), unsigned, InstallOptions{AllowUnsigned: true, Scope: "user"})
	if e != nil || ref.Origin != "locally_authored" {
		t.Fatal(ref, e)
	}
	if _, e = m.Activate(context.Background(), state(t, m).Revision, "application", ref.SHA256, nil); e == nil {
		t.Fatal("local preset promoted to application scope")
	}
	if _, e = m.Activate(context.Background(), state(t, m).Revision, "user", ref.SHA256, nil); e != nil {
		t.Fatal(e)
	}
	application, e := m.Snapshot(context.Background(), "")
	if e != nil || len(application.Refs) != 0 {
		t.Fatal(application, e)
	}
	selected, e := m.Snapshot(context.Background(), "user")
	if e != nil || len(selected.Refs) != 1 {
		t.Fatal(selected, e)
	}
	dependent := specimen(t, key, private, "2.0.0", func(manifest *Manifest, _ map[string][]byte) {
		manifest.ID = "fixture.dependent"
		manifest.Dependencies = []Dependency{{ID: "missing.pack", Version: Range{"1.0.0", "2.0.0"}, SHA256: strings.Repeat("a", 64)}}
	})
	dep, e := m.Install(context.Background(), dependent, InstallOptions{})
	if e != nil {
		t.Fatal(e)
	}
	before := state(t, m).Revision
	if _, e = m.Activate(context.Background(), before, "application", dep.SHA256, nil); e == nil || state(t, m).Revision != before {
		t.Fatal("unsatisfied dependency activated", e)
	}
	m.Close()
	m, e = Open(root, []Key{key}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	selected, e = m.Snapshot(context.Background(), "user")
	if e != nil || selected.Refs[0] != ref {
		t.Fatal(selected, e)
	}
	// Interrupted file publication is disposable until its state reference commits.
	orphan := filepath.Join(root, "versions", strings.Repeat("b", 64)+".zip")
	os.WriteFile(orphan, []byte("orphan"), 0600)
	m.Close()
	recovered, e := Open(root, []Key{key}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer recovered.Close()
	if _, e = os.Stat(orphan); !os.IsNotExist(e) {
		t.Fatal("uncommitted archive retained", e)
	}
	if _, e = recovered.Snapshot(context.Background(), "user"); e != nil {
		t.Fatal("valid retained version lost", e)
	}
}
func TestMappingExactnessAliasesAndBounds(t *testing.T) {
	raw, _ := Assets.ReadFile("assets/mapping.json")
	var m Mapping
	Strict(raw, &m)
	for value, want := range map[string]string{`9007199254740993`: "9007199254740.993", `"0.123456789012345678901234567890"`: "0.00012345678901234567890123456789", `0`: "0", `-1`: "-0.001"} {
		body := []byte(`{"producer":"synthetic.metrics","producer_version":"1.0.0","metric":"test","value_ms":` + value + `}`)
		record, e := m.Evaluate(body)
		if e != nil || record.Value != want {
			t.Fatal(record, e)
		}
	}
	for _, body := range []string{`{"producer":{},"producer_version":"1.0.0","metric":"test","value_ms":1}`, `{"producer":"synthetic.metrics","producer_version":"1.0.0","metric":"test","value_ms":null}`, `{"producer":"synthetic.metrics","producer_version":"1.0.0","metric":"test","value_ms":1,"duration_ms":2}`, `{"producer":"synthetic.metrics","producer_version":"99","metric":"test","value_ms":1}`} {
		if _, e := m.Evaluate([]byte(body)); e == nil {
			t.Fatal("malformed/ambiguous producer accepted")
		}
	}
	if _, e := m.Evaluate(bytes.Repeat([]byte{'x'}, 65537)); e == nil {
		t.Fatal("record budget ignored")
	}
}

func TestSharedLocalManifestKeepsPublishedArchiveIdentityAcrossScopes(t *testing.T) {
	m, _ := manager(t)
	body := specimen(t, Key{}, nil, "1.0.0", nil)
	first, e := m.Install(context.Background(), body, InstallOptions{AllowUnsigned: true, Scope: "user"})
	if e != nil {
		t.Fatal(e)
	}
	changed := rewriteZip(t, body, func(name string, b []byte) (string, []byte) {
		if name == "signature.json" {
			b = append(b, '\n')
		}
		return name, b
	})
	scope := "case:case-" + strings.Repeat("a", 32)
	second, e := m.Install(context.Background(), changed, InstallOptions{AllowUnsigned: true, Scope: scope})
	if e != nil {
		t.Fatal(e)
	}
	if second.ArchiveSHA256 != first.ArchiveSHA256 || second.Bytes != first.Bytes {
		t.Fatal("published identity changed")
	}
	if _, e = m.Activate(context.Background(), state(t, m).Revision, scope, second.SHA256, nil); e != nil {
		t.Fatal(e)
	}
}
