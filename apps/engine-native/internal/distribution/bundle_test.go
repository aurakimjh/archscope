package distribution

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStandaloneFixturesRequiredAndComplete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ids, e := CheckFixtures(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if len(ids) != 6 {
		t.Fatal(ids)
	}
	b, _ := Assets.ReadFile("assets/fixtures.zip")
	if _, _, e = ReadFixtures(b, append(ids, "new.uncovered-importer")); e == nil {
		t.Fatal("uncovered importer accepted")
	}
	b[50] ^= 1
	if _, _, e = ReadFixtures(b, ids); e == nil {
		t.Fatal("corrupt bundle accepted")
	}
}
func TestCleanBundleAndPreflightRefuseMissingTamperAndUndeclared(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	source, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "install")
	if _, e := Build(ctx, BuildOptions{Engine: source, Out: out}); e != nil {
		t.Fatal(e)
	}
	if _, e := Verify(out, false); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(out, "resources/fixtures.zip")
	original, _ := os.ReadFile(p)
	os.Remove(p)
	if _, e := Verify(out, false); e == nil {
		t.Fatal("missing fixtures allowed")
	}
	os.WriteFile(p, append(original, 'x'), 0600)
	if _, e := Verify(out, false); e == nil {
		t.Fatal("tamper allowed")
	}
	os.WriteFile(p, original, 0600)
	// An attacker updating the local checksum list cannot replace a mandatory
	// resource pinned independently by the compiled distribution package.
	manifestPath := filepath.Join(out, "inventory.json")
	originalManifest, _ := os.ReadFile(manifestPath)
	var m Inventory
	json.Unmarshal(originalManifest, &m)
	b := []byte("forged")
	os.WriteFile(p, b, 0600)
	m.Files["resources/fixtures.zip"], _ = digestFile(p)
	raw, _ := json.Marshal(m)
	os.WriteFile(manifestPath, raw, 0600)
	if _, e := Verify(out, false); e == nil {
		t.Fatal("forged fixture+inventory allowed")
	}
	os.WriteFile(manifestPath, originalManifest, 0600)
	os.WriteFile(p, original, 0600)
	extra := filepath.Join(out, "unlisted.dll")
	os.WriteFile(extra, []byte("extra"), 0600)
	if _, e := Verify(out, false); e == nil {
		t.Fatal("unlisted runtime allowed")
	}
	os.Remove(extra)
	os.Remove(p)
	if e := os.Symlink(source, p); e == nil {
		if _, e := Verify(out, false); e == nil {
			t.Fatal("symlink allowed")
		}
	}
}
func TestFixedRuntimeMustBeCompleteAndArchitectureMatched(t *testing.T) {
	root := t.TempDir()
	if e := runtimePreflight(root, "amd64"); e == nil {
		t.Fatal("empty runtime accepted")
	}
	for _, n := range []string{"msedgewebview2.exe", "msedge.dll", "resources.pak", "icudtl.dat", "LICENSE.txt"} {
		os.WriteFile(filepath.Join(root, n), []byte("not a PE"), 0600)
	}
	os.Mkdir(filepath.Join(root, "locales"), 0700)
	os.WriteFile(filepath.Join(root, "locales/en-US.pak"), []byte("test"), 0600)
	if e := runtimePreflight(root, "amd64"); e == nil {
		t.Fatal("forged runtime executable accepted")
	}
}
