package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
)

func TestR901PackBridgeReviewsImpactAndReopensRevokedBootstrap(t *testing.T) {
	ctx := context.Background()
	t.Setenv("ARCHSCOPE_STATE_ROOT", filepath.Join(t.TempDir(), "state"))
	service := &EngineService{}
	defer service.closeEvidence()
	m, e := service.packManager()
	if e != nil {
		t.Fatal(e)
	}
	public, private, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	owner := packs.Key{ID: "bridge.owner", Public: base64.StdEncoding.EncodeToString(public), ManageTrust: true}
	keyJSON, _ := json.Marshal(owner)
	state, _ := m.State()
	if e = service.RegisterPackTrustKey(ctx, state.Revision, string(keyJSON)); e != nil {
		t.Fatal(e)
	}
	keys, _ := packs.BootstrapKeys()
	body, sig, _ := packs.SignTrust(packs.TrustUpdate{Version: packs.Format, Sequence: "1", Revoke: []string{keys[0].ID}}, owner.ID, private)
	dir := t.TempDir()
	bodyPath, sigPath := filepath.Join(dir, "update.json"), filepath.Join(dir, "signature.json")
	os.WriteFile(bodyPath, body, 0600)
	os.WriteFile(sigPath, sig, 0600)
	state, _ = m.State()
	if e = service.ApplyOfflinePackTrust(ctx, state.Revision, bodyPath, sigPath); e == nil {
		t.Fatal("legacy bridge silently suspended active pack")
	}
	encoded, e := service.PreviewOfflinePackTrust(ctx, state.Revision, bodyPath, sigPath)
	if e != nil {
		t.Fatal(e)
	}
	var preview packs.TrustPreview
	if e = json.Unmarshal([]byte(encoded), &preview); e != nil || len(preview.Affected) != 1 {
		t.Fatal(preview, e)
	}
	if e = service.CommitOfflinePackTrust(ctx, state.Revision, bodyPath, sigPath, preview.ReviewSHA256); e != nil {
		t.Fatal(e)
	}
	service.closeEvidence()
	reopened := &EngineService{}
	defer reopened.closeEvidence()
	encoded, e = reopened.GetPackState(ctx)
	if e != nil {
		t.Fatal("administrative bridge did not reopen", e)
	}
	var status packs.Status
	if e = json.Unmarshal([]byte(encoded), &status); e != nil || len(status.Unavailable) != 1 || status.Unavailable[0].Code != "PACK_SIGNER_UNTRUSTED" {
		t.Fatal(status, e)
	}
	if _, e = reopened.ListEvidenceImporters(); e != nil {
		t.Fatal("builtin importer list failed", e)
	}
	if e = reopened.DeactivateDataPack(ctx, status.Revision, "application", status.Unavailable[0].Pack.ID); e != nil {
		t.Fatal(e)
	}
}
