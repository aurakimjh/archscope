//go:build releasequalification

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
)

// External inputs are the unmodified synthetic files used by the ordinary
// desktop screen. This headless check is separate from native memory budgets.
func TestQualifiedImportWorkloadProjection(t *testing.T) {
	manifestPath := os.Getenv("ARCHSCOPE_IMPORT_WORKLOAD_MANIFEST")
	if manifestPath == "" {
		t.Skip("explicit synthetic workload manifest required")
	}
	var manifest struct {
		Files []string `json:"files"`
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 216 {
		t.Fatal("expected 108 unique and 108 body-rich inputs")
	}
	out := os.Getenv("ARCHSCOPE_IMPORT_WORKLOAD_OUT")
	if out == "" {
		t.Fatal("explicit receipt path required")
	}
	modelDir := out + "-models"
	if err = os.Mkdir(modelDir, 0700); err != nil {
		t.Fatal(err)
	}
	svc := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), runtime.GOOS)
	defer svc.shutdownAutomation()
	type item struct {
		InputSHA256      string `json:"inputSha256"`
		Rows             int    `json:"rows"`
		ProjectionSHA256 string `json:"projectionSha256"`
		Fingerprint      string `json:"fingerprint"`
	}
	items := make([]item, 0, len(manifest.Files))
	seen := map[string]bool{}
	for index, name := range manifest.Files {
		if !filepath.IsAbs(name) {
			t.Fatal("absolute owned fixture path required")
		}
		input, e := os.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		doc, _, e := sessionfile.Read(name)
		if e != nil {
			t.Fatal(e)
		}
		expected := 512
		if index >= 108 {
			expected = 128
		}
		if len(doc.Transactions) != expected {
			t.Fatal("input row count")
		}
		opened, e := svc.OpenCaptureFile(name)
		if e != nil {
			t.Fatal(e)
		}
		if opened.SelectedID != doc.SelectedID || opened.Result.Summary["total_transactions"] != expected {
			t.Fatal("selected ID or summary count")
		}
		projection, e := json.Marshal(opened.Result.Tables["transactions"])
		if e != nil {
			t.Fatal(e)
		}
		model, e := json.Marshal(opened.Result)
		if e != nil {
			t.Fatal(e)
		}
		modelName := filepath.Base(name) + ".json"
		if e = os.WriteFile(filepath.Join(modelDir, modelName), model, 0600); e != nil {
			t.Fatal(e)
		}
		var rows []map[string]json.RawMessage
		if e = json.Unmarshal(projection, &rows); e != nil || len(rows) != expected {
			t.Fatal("projection rows", e)
		}
		for row, fields := range rows {
			tx := doc.Transactions[row]
			for key, value := range map[string]any{"id": tx.ID, "comment": tx.Comment, "starred": tx.Starred, "request": tx.Request, "response": tx.Response} {
				want, e := json.Marshal(value)
				if e != nil {
					t.Fatal(e)
				}
				// Compare JSON values, independent of object key order.
				var a, b any
				if json.Unmarshal(want, &a) != nil || json.Unmarshal(fields[key], &b) != nil {
					t.Fatal("invalid projected field")
				}
				ca, _ := json.Marshal(a)
				cb, _ := json.Marshal(b)
				if !bytes.Equal(ca, cb) {
					t.Fatalf("projection changed synthetic %s at file %d row %d", key, index, row)
				}
			}
		}
		digest := sha256.Sum256(projection)
		fingerprint, _ := opened.Result.Metadata.Extra["workspace_transaction_rows_fingerprint"].(string)
		// The contract hashes count plus each encoded row. Preserve nested struct
		// field ordering from the actual wire projection, rather than inventing a
		// recursively key-sorted representation after generic JSON decoding.
		rowHash := sha256.New()
		encoder := json.NewEncoder(rowHash)
		if e = encoder.Encode(len(rows)); e != nil {
			t.Fatal(e)
		}
		for _, row := range rows {
			if e = encoder.Encode(row); e != nil {
				t.Fatal(e)
			}
		}
		if fingerprint != "v1:"+hex.EncodeToString(rowHash.Sum(nil)) || seen[fingerprint] {
			t.Fatal("invalid or repeated exact-row fingerprint")
		}
		seen[fingerprint] = true
		inputDigest := sha256.Sum256(input)
		items = append(items, item{hex.EncodeToString(inputDigest[:]), expected, hex.EncodeToString(digest[:]), fingerprint})
	}
	receipt, e := json.MarshalIndent(map[string]any{"passed": true, "scope": "Headless actual CaptureService projections of ordinary-run synthetic inputs; not native memory/GTK qualification", "files": items}, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(out, append(receipt, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("PASS 216 distinct sanitized projections preserve annotations and HTTP messages")
}
