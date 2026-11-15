//go:build releasequalification

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestStablePreparationImportModels(t *testing.T) {
	root := t.TempDir()
	out := os.Getenv("ARCHSCOPE_STABLE_PREPARATION_OUT")
	if out == "" {
		out = root
	}
	out = filepath.Join(out, "models")
	if e := os.Mkdir(out, 0700); e != nil {
		t.Fatal(e)
	}
	svc := newCaptureServiceForPlatform(root, certstore.New(memoryTrustBackend{}, nil), runtime.GOOS)
	defer svc.shutdownAutomation()
	generate := func(name string, size, nonce int, body bool) string {
		t.Helper()
		doc := sessionfile.Document{Type: sessionfile.Kind, Version: 1, Scope: "all", SelectedID: "row-0"}
		for i := 0; i < size; i++ {
			tx := models.CaptureTransaction{ID: fmt.Sprintf("row-%d", i), Sequence: i + 1, Method: "POST", URL: fmt.Sprintf("http://example.test/synthetic?row=%d", i), State: models.TxComplete, Comment: fmt.Sprintf("synthetic-%d", nonce), Starred: i == 0, Request: models.HTTPMessage{BodyStorage: "omitted"}, Response: models.HTTPMessage{BodyStorage: "omitted"}}
			if body {
				text := fmt.Sprintf("synthetic-body-%03d-%03d-", nonce, i) + strings.Repeat("x", 1024)
				tx.Request = models.HTTPMessage{BodyStorage: "inline", BodyPreview: text, BodySize: int64(len(text)), ContentType: "text/plain"}
				tx.Response = tx.Request
			}
			doc.Transactions = append(doc.Transactions, tx)
		}
		file := filepath.Join(root, "input.httpsession")
		if e := sessionfile.Write(context.Background(), file, doc, "session"); e != nil {
			t.Fatal(e)
		}
		opened, e := svc.OpenCaptureFile(file)
		if e != nil {
			t.Fatal(e)
		}
		if opened.Result.Summary["total_transactions"] != size {
			t.Fatal("rows", opened.Result.Summary)
		}
		fingerprint, _ := opened.Result.Metadata.Extra["workspace_transaction_rows_fingerprint"].(string)
		if !strings.HasPrefix(fingerprint, "v1:") || len(fingerprint) != 67 {
			t.Fatal("fingerprint missing")
		}
		b, e := json.Marshal(opened.Result)
		if e != nil {
			t.Fatal(e)
		}
		if body && !strings.Contains(string(b), "synthetic-body-") {
			t.Fatal("body fixture did not retain synthetic preview")
		}
		filename := name + ".json"
		if e = os.WriteFile(filepath.Join(out, filename), b, 0600); e != nil {
			t.Fatal(e)
		}
		return filename
	}
	repeated := []string{}
	for _, size := range []int{4096, 512, 32} {
		repeated = append(repeated, generate(fmt.Sprintf("repeated-%d", size), size, 0, false))
	}
	manifest := map[string]any{"source": "actual CaptureService.OpenCaptureFile sanitized projections", "cycles": 36, "warmupCycles": 28, "historyCapacity": 80, "forcedGC": "diagnostic attribution only; not natural pressure or native memory qualification", "scenarios": map[string][]string{"repeated": repeated}}
	scenarios := manifest["scenarios"].(map[string][]string)
	for _, spec := range []struct {
		name string
		rows int
		body bool
	}{{"unique", 512, false}, {"body-rich", 128, true}} {
		for i := 0; i < 108; i++ {
			scenarios[spec.name] = append(scenarios[spec.name], generate(fmt.Sprintf("%s-%03d", spec.name, i), spec.rows, i, spec.body))
		}
	}
	b, _ := json.MarshalIndent(manifest, "", "  ")
	if e := os.WriteFile(filepath.Join(out, "manifest.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("PASS 219 actual sanitized backend model fixtures; bodies are synthetic offline inputs")
}
