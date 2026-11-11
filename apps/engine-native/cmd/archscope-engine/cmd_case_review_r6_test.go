package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestR6ActualCLISaveAliasGuardCorruptPlanAndCacheCleanup(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "engine")
	if out, e := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	run := func(args ...string) []byte {
		t.Helper()
		out, e := exec.Command(binary, args...).CombinedOutput()
		if e != nil {
			t.Fatal(args, e, string(out))
		}
		return out
	}
	root := filepath.Join(t.TempDir(), "store")
	var m struct {
		ID, Revision string
		Results      []struct{ ID string }
	}
	if e := json.Unmarshal(run("case", "create", "--store", root), &m); e != nil {
		t.Fatal(e)
	}
	r := models.New("synthetic", "fixture")
	r.Summary["exact"] = json.Number("9007199254740993")
	body, _ := json.Marshal(r)
	snapshot := filepath.Join(t.TempDir(), "snapshot.json")
	os.WriteFile(snapshot, body, 0600)
	request := filepath.Join(t.TempDir(), "save.json")
	b, _ := json.Marshal(map[string]any{"case_id": m.ID, "expected_revision": m.Revision, "source_ids": []string{}, "selection": nil})
	os.WriteFile(request, b, 0600)
	saved := run("case", "save-result", "--store", root, "--request", request, "--in", snapshot)
	json.Unmarshal(saved, &m)
	req, _ := json.Marshal(map[string]any{"case_id": m.ID, "expected_revision": m.Revision, "source_ids": []string{}, "selection": nil})
	os.WriteFile(request, req, 0600)
	duplicate := run("case", "save-result", "--store", root, "--request", request, "--in", snapshot)
	var again struct {
		Revision string
		Results  []struct{ ID string }
	}
	json.Unmarshal(duplicate, &again)
	if again.Revision != m.Revision || len(again.Results) != 1 {
		t.Fatal(string(duplicate))
	}
	head := filepath.Join(root, "cases", m.ID, "HEAD.json")
	alias := filepath.Join(filepath.Dir(root), strings.ToUpper(filepath.Base(root)), "CASES", strings.ToUpper(m.ID), "HEAD.JSON")
	if _, e := os.Stat(alias); e == nil {
		out, e := exec.Command(binary, "case", "export-result", "--store", root, "--id", m.ID, "--result", m.Results[0].ID, "--out", alias).CombinedOutput()
		if e == nil || !strings.Contains(string(out), "CASE_EXPORT_DESTINATION_UNSAFE") {
			t.Fatal(string(out), e)
		}
	}
	os.WriteFile(head, []byte("corrupt synthetic head"), 0600)
	var plan struct{ Token string }
	json.Unmarshal(run("case", "corrupt-delete-plan", "--store", root, "--id", m.ID), &plan)
	if plan.Token == "" {
		t.Fatal("no corrupt plan token")
	}
	run("case", "purge-corrupt", "--store", root, "--id", m.ID, "--token", plan.Token)
	if out := run("case", "list", "--store", root); strings.TrimSpace(string(out)) != "[]" {
		t.Fatal(string(out))
	}
	cache := filepath.Join(t.TempDir(), "evidence-cache")
	c, registry, _ := catalog.Builtin()
	svc, e := analysis.New(cache, c, registry)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.LoadSnapshot(context.Background(), body); e != nil {
		t.Fatal(e)
	}
	svc.Close()
	cleaned := run("evidence", "cleanup-cache", "--cache-dir", cache)
	if !strings.Contains(string(cleaned), `"removed": 1`) {
		t.Fatal(string(cleaned))
	}
	if b, _ := os.ReadFile(snapshot); string(b) != string(body) {
		t.Fatal("external snapshot removed")
	}
}
