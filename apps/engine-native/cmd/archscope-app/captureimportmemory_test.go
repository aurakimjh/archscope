package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestCaptureOpenFingerprintTracksExactProjectedRows(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "same.httpsession")
	doc := sessionfile.Document{Type: sessionfile.Kind, Version: 1, Scope: "all", Transactions: []models.CaptureTransaction{{ID: "one", URL: "http://example.test/one", State: models.TxComplete, Comment: "one"}}}
	write := func() {
		t.Helper()
		if err := sessionfile.Write(context.Background(), file, doc, "session"); err != nil {
			t.Fatal(err)
		}
	}
	write()
	info, _ := os.Stat(file)
	service := newCaptureServiceForPlatform(root, certstore.New(memoryTrustBackend{}, nil), "windows")
	open := func() CaptureOpenResult {
		t.Helper()
		value, err := service.OpenCaptureFile(file)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	first, again := open(), open()
	fingerprint := func(value CaptureOpenResult) string {
		t.Helper()
		s, _ := value.Result.Metadata.Extra["workspace_transaction_rows_fingerprint"].(string)
		if !strings.HasPrefix(s, "v1:") || len(s) != 67 {
			t.Fatalf("missing projection fingerprint: %q", s)
		}
		return s
	}
	if fingerprint(first) != fingerprint(again) {
		t.Fatal("identical transactions must share a fingerprint despite new analysis metadata")
	}
	doc.Transactions[0].Comment = "two"
	write()
	if err := os.Chtimes(file, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	edited := open()
	if fingerprint(first) == fingerprint(edited) {
		t.Fatal("same-size, same-mtime changed annotations must not reuse old rows")
	}
	doc.Transactions = append(doc.Transactions, models.CaptureTransaction{ID: "two", URL: "http://example.test/two", State: models.TxComplete})
	write()
	twoRows := open()
	if fingerprint(edited) == fingerprint(twoRows) {
		t.Fatal("row count must be part of table identity")
	}
	doc.Transactions[0], doc.Transactions[1] = doc.Transactions[1], doc.Transactions[0]
	write()
	if fingerprint(twoRows) == fingerprint(open()) {
		t.Fatal("row order must be part of table identity")
	}
}
