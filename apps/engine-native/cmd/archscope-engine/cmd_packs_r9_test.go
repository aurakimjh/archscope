package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
)

func TestR901CLIReviewedRevocationAndRecoveryAcrossProcesses(t *testing.T) {
	bin := buildBinary(t)
	root := t.TempDir()
	t.Setenv("ARCHSCOPE_STATE_ROOT", filepath.Join(root, "state"))
	store := filepath.Join(root, "packs")
	run := func(args ...string) []byte {
		t.Helper()
		out, e := exec.Command(bin, args...).CombinedOutput()
		if e != nil {
			t.Fatalf("%v: %v\n%s", args, e, out)
		}
		return out
	}
	list := func() packs.Status {
		t.Helper()
		var status packs.Status
		if e := json.Unmarshal(run("packs", "--store", store, "list"), &status); e != nil {
			t.Fatal(e)
		}
		return status
	}
	write := func(name string, body []byte) string {
		t.Helper()
		path := filepath.Join(root, name)
		if e := os.WriteFile(path, body, 0600); e != nil {
			t.Fatal(e)
		}
		return path
	}
	status := list()
	public, private, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	owner := packs.Key{ID: "cli.owner", Public: base64.StdEncoding.EncodeToString(public), ManageTrust: true}
	keyJSON, _ := json.Marshal(owner)
	run("packs", "--store", store, "register-key", "--revision", status.Revision, "--in", write("owner-public.json", keyJSON))
	status = list()
	keys, e := packs.BootstrapKeys()
	if e != nil {
		t.Fatal(e)
	}
	body, sig, e := packs.SignTrust(packs.TrustUpdate{Version: packs.Format, Sequence: "1", Revoke: []string{keys[0].ID}}, owner.ID, private)
	if e != nil {
		t.Fatal(e)
	}
	args := []string{"packs", "--store", store, "update-trust", "--revision", status.Revision, "--in", write("update.json", body), "--signature", write("signature.json", sig)}
	var preview packs.TrustPreview
	if e = json.Unmarshal(run(append(append([]string{}, args...), "--preview")...), &preview); e != nil || len(preview.Affected) != 1 {
		t.Fatal(preview, e)
	}
	if list().Revision != status.Revision {
		t.Fatal("preview mutated state")
	}
	if out, e := exec.Command(bin, args...).CombinedOutput(); e == nil || !strings.Contains(string(out), "PACK_TRUST_REVIEW_REQUIRED") {
		t.Fatalf("unreviewed update: %v %s", e, out)
	}
	run(append(args, "--reviewed-sha256", preview.ReviewSHA256)...)
	status = list() // A new CLI process reopens the revoked bootstrap every call.
	if len(status.Unavailable) != 1 || !status.Keys[keys[0].ID].Revoked {
		t.Fatal(status)
	}
	ref := status.Unavailable[0].Pack
	backup := filepath.Join(root, "backup.zip")
	run("packs", "--store", store, "export", "--scope", "application", "--sha256", ref.SHA256, "--out", backup)
	want, _ := packs.Assets.ReadFile("assets/bootstrap.zip")
	got, e := os.ReadFile(backup)
	if e != nil || !bytes.Equal(want, got) {
		t.Fatal("retained backup changed", e)
	}
	run("evidence", "formats", "--pack-store", store)
	fixture, _ := catalog.Contracts.ReadFile("contracts/server.log")
	log := write("server.log", fixture)
	run("evidence", "import", "--pack-store", store, "--in", log, "--importer", "tomcat", "--out", filepath.Join(root, "result.json"))
	run("packs", "--store", store, "deactivate", "--revision", status.Revision, "--scope", "application", "--id", ref.ID)
	status = list()
	if len(status.Active) != 0 || len(status.Unavailable) != 0 {
		t.Fatal(status)
	}
	spare := owner
	spare.ID = "cli.spare"
	keyJSON, _ = json.Marshal(spare)
	run("packs", "--store", store, "register-key", "--revision", status.Revision, "--in", write("spare-public.json", keyJSON))
	if _, ok := list().Keys[spare.ID]; !ok {
		t.Fatal("recovery registration failed")
	}
}
