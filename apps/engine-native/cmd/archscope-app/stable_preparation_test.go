//go:build releasequalification

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
)

// Real renderer state -> loopback test transport -> real Wails service methods.
// Only synthetic files and an owned temporary case/cache store are reachable.
// This is headless integration, not native WebView/window acceptance.
func TestStablePreparationWorkspaceRoundTrip(t *testing.T) {
	root := t.TempDir()
	svc := evidenceBridge(t, nil)
	casePath := filepath.Join(root, "cases")
	store, err := cases.Open(casePath)
	if err != nil {
		t.Fatal(err)
	}
	svc.caseService = store
	t.Cleanup(func() { svc.caseService.Close(); svc.evidenceService.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	inputs := map[string]map[string]any{}
	for _, fixture := range []struct{ name, file, importer, options string }{
		{"profile", "profile-set.json", "profile-set", `{"profile_ids":["worker-a","worker-b"],"metric_id":"memory"}`},
		{"server", "server.log", "tomcat", `{"top_n":3}`},
		{"thread", "runtime-evidence-thread-set.json", "runtime.evidence-json", `{}`},
	} {
		body, e := catalog.Contracts.ReadFile("contracts/" + fixture.file)
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(root, fixture.file)
		if e = os.WriteFile(path, body, 0600); e != nil {
			t.Fatal(e)
		}
		job, e := svc.ImportEvidence(EvidenceImportRequest{Path: path, Importer: fixture.importer, OptionsJSON: fixture.options})
		if e != nil {
			t.Fatal(e)
		}
		job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
		if e != nil || job.State != "completed" {
			t.Fatal(job, e)
		}
		raw, e := svc.GetEvidenceResultJSON(job.ResultHandle)
		if e != nil {
			t.Fatal(e)
		}
		inputs[fixture.name] = map[string]any{"handle": job.ResultHandle, "raw": raw}
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	token := hex.EncodeToString(secret)
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "refused", 403)
			return
		}
		var request struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); e != nil {
			http.Error(w, "invalid", 400)
			return
		}
		args := request.Args
		var value any
		var e error
		arg := func(i int) string {
			if i >= len(args) {
				return ""
			}
			return args[i]
		}
		switch request.Command {
		case "inputs":
			value = inputs
		case "create":
			value, e = svc.CreateCase(arg(0))
		case "get":
			value, e = svc.GetCase(arg(0))
		case "save":
			value, e = svc.SaveCaseResult(arg(0), arg(1))
		case "reopen":
			value, e = svc.ReopenCaseResult(arg(0), arg(1))
		case "acquire":
			value, e = svc.AcquireEvidenceResult(arg(0))
		case "release":
			e = svc.ReleaseEvidenceLease(arg(0))
		case "export":
			if filepath.Base(arg(1)) != arg(1) || arg(1) == "" {
				e = fmt.Errorf("basename required")
				break
			}
			path := filepath.Join(root, arg(1))
			e = svc.ExportEvidenceResult(arg(0), path, arg(2))
			if e == nil {
				b, readErr := os.ReadFile(path)
				e = readErr
				value = string(b)
			}
		case "restart":
			svc.evidenceService.Close()
			svc.caseService.Close()
			cat, registry, buildErr := catalog.Builtin()
			if buildErr != nil {
				e = buildErr
				break
			}
			svc.evidenceService, e = analysis.New(filepath.Join(root, "fresh-cache"), cat, registry)
			if e == nil {
				svc.caseService, e = cases.Open(casePath)
			}
		default:
			e = fmt.Errorf("unknown test command")
		}
		if e != nil {
			w.WriteHeader(422)
			json.NewEncoder(w).Encode(map[string]any{"error": e.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"value": value})
	}))
	defer server.Close()
	script, err := filepath.Abs("../../../../scripts/verify-stable-workspace.mjs")
	if err != nil {
		t.Fatal(err)
	}
	module, err := filepath.Abs("frontend/.tmp/state-tests/state/analysisWorkspace.js")
	if err != nil {
		t.Fatal(err)
	}
	out := os.Getenv("ARCHSCOPE_STABLE_PREPARATION_OUT")
	if out == "" {
		out = root
	}
	cmd := exec.CommandContext(ctx, "node", script, module, filepath.Join(out, "workspace.json"))
	cmd.Env = append(os.Environ(), "ARCHSCOPE_STABLE_TEST_URL="+server.URL, "ARCHSCOPE_STABLE_TEST_TOKEN="+token)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("renderer/backend workflow: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "PASS") {
		t.Fatalf("missing result: %s", output)
	}
	t.Log(string(output))
}
