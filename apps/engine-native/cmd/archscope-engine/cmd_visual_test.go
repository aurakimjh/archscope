package main

import (
	"context"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/visuals"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestT636ActualCLIPinnedSpecsAndExplicitDesktopRefusals(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "engine")
	if out, e := exec.Command("go", "build", "-o", executable, ".").CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	run := func(args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		out, e := exec.CommandContext(ctx, executable, args...).CombinedOutput()
		if e != nil {
			t.Fatal(args, e, string(out))
		}
		return out
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "profile.json")
	saved := filepath.Join(dir, "result.json")
	fixture, _ := catalog.Contracts.ReadFile("contracts/profile-set.json")
	os.WriteFile(input, fixture, 0600)
	run("evidence", "import", "--in", input, "--importer", "profile-set", "--options", `{"profile_ids":["worker-a","worker-b"],"metric_id":"duration"}`, "--out", saved)
	for _, kind := range []string{"profile_top", "profile_flamegraph", "profile_diff", "provider_table"} {
		specPath := filepath.Join(dir, kind+".json")
		q := visuals.Query{Kind: kind, Offset: 0, Limit: 25}
		if kind == "provider_table" {
			q.Provider = "generic:summary"
		}
		query, _ := json.Marshal(q)
		run("visual", "create-spec", "--in", saved, "--target-in", saved, "--query", string(query), "--out", specPath)
		out := run("visual", "validate", "--in", specPath)
		if !strings.Contains(string(out), `"valid":true`) {
			t.Fatal(string(out))
		}
		if out, e := exec.Command(executable, "visual", "create-spec", "--in", saved, "--target-in", saved, "--query", string(query), "--out", specPath).CombinedOutput(); e == nil || !strings.Contains(string(out), "EXISTS") {
			t.Fatal("spec overwrite", e, string(out))
		}
		var doc visualSpecFile
		body, _ := os.ReadFile(specPath)
		json.Unmarshal(body, &doc)
		doc.View.Denominator = "wrong"
		body, _ = json.Marshal(doc)
		tampered := filepath.Join(dir, "tampered-"+kind+".json")
		os.WriteFile(tampered, body, 0600)
		if out, e := exec.Command(executable, "visual", "validate", "--in", tampered).CombinedOutput(); e == nil {
			t.Fatal("changed view accepted", string(out))
		}
	}
	for operation, code := range map[string]string{"render": "VISUAL_DESKTOP_RENDERER_REQUIRED", "clipboard": "VISUAL_CLIPBOARD_UNSUPPORTED"} {
		out, e := exec.Command(executable, "visual", operation).CombinedOutput()
		if e == nil || !strings.Contains(string(out), code) {
			t.Fatal(operation, e, string(out))
		}
	}
}
