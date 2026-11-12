package catalog

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/packs"
)

func TestR901BuiltinImportsContinueAfterRevocationAndRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	base, registry, e := Builtin()
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	session, e := OpenPackSession(ctx, root, base)
	if e != nil {
		t.Fatal(e)
	}
	old, e := session.Provider("")
	if e != nil {
		t.Fatal(e)
	}
	public, private, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	owner := packs.Key{ID: "review.owner", Public: base64.StdEncoding.EncodeToString(public), ManageTrust: true}
	state, _ := session.Manager.State()
	if e = session.Manager.RegisterKey(ctx, state.Revision, owner); e != nil {
		t.Fatal(e)
	}
	keys, _ := packs.BootstrapKeys()
	body, sig, _ := packs.SignTrust(packs.TrustUpdate{Version: packs.Format, Sequence: "1", Revoke: []string{keys[0].ID}}, owner.ID, private)
	state, _ = session.Manager.State()
	preview, e := session.Manager.PreviewTrustUpdate(ctx, state.Revision, body, sig)
	if e != nil {
		t.Fatal(e)
	}
	if e = session.Manager.ApplyReviewedTrustUpdate(ctx, state.Revision, body, sig, preview.ReviewSHA256); e != nil {
		t.Fatal(e)
	}
	session.Manager.Close()
	session, e = OpenPackSession(ctx, root, base)
	if e != nil {
		t.Fatal("reopen failed", e)
	}
	defer session.Manager.Close()
	current, e := session.Provider("")
	if e != nil || len(current.Packs) != 0 || len(current.Catalog.List()) != len(base.List()) {
		t.Fatal("builtin catalog blocked by revoked default", e)
	}
	if _, e = current.Catalog.Resolve("synthetic-metrics", nil, "test"); e == nil {
		t.Fatal("revoked data mapping remained available")
	}
	if _, e = old.Catalog.Resolve("synthetic-metrics", nil, "test"); e != nil {
		t.Fatal("pinned catalog changed", e)
	}
	service, e := analysis.New(filepath.Join(root, "jobs"), base, registry)
	if e != nil {
		t.Fatal(e)
	}
	defer service.Close()
	service.SetCatalogProvider(session.Provider)
	fixture, _ := Contracts.ReadFile("contracts/server.log")
	path := filepath.Join(t.TempDir(), "server.log")
	os.WriteFile(path, fixture, 0600)
	job, e := service.Start(analysis.Request{Path: path, Importer: "tomcat", Options: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	job, e = service.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
}
func TestR904NewMetricControlNamesFailWhileSavedMetricsStayReadable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	base, registry, e := Builtin()
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	session, e := OpenPackSession(ctx, root, base)
	if e != nil {
		t.Fatal(e)
	}
	defer session.Manager.Close()
	service, e := analysis.New(filepath.Join(root, "jobs"), base, registry)
	if e != nil {
		t.Fatal(e)
	}
	defer service.Close()
	service.SetCatalogProvider(session.Provider)
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	fixture, _ := packs.Assets.ReadFile("assets/input.jsonl")
	os.WriteFile(path, fixture, 0600)
	job, e := service.Start(analysis.Request{Path: path, Importer: "synthetic-metrics", Options: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	job, e = service.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	_, result, e := service.Read(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	// Legacy saved payloads are not rewritten by a stricter input-name policy.
	for _, row := range result.Tables["samples"].([]any) {
		row.(map[string]any)["metric"] = "legacy\u202e\x07"
	}
	body, _ := json.Marshal(result)
	view, e := service.LoadSnapshot(ctx, body)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	page, e := service.QueryResult(ctx, view.Handle, "data:metric-samples", 0, 25)
	if e != nil || !page.Escaped {
		t.Fatal(page, e)
	}
	exported, e := service.Export(view.Handle, "raw-json")
	if e != nil || string(exported) != string(body) {
		t.Fatal("legacy snapshot changed", e)
	}
	bad := []byte("{\"producer\":\"synthetic.metrics\",\"producer_version\":\"1.0.0\",\"metric\":\"bad\\u202E\\u0007\",\"value_ms\":0}\n")
	os.WriteFile(path, bad, 0600)
	job, e = service.Start(analysis.Request{Path: path, Importer: "synthetic-metrics", Options: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	job, e = service.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "failed" || job.ResultHandle != "" {
		t.Fatal("new unsafe metric was published", job, e)
	}
}
