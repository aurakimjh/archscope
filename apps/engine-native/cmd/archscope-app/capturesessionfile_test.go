package main

import (
	"context"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"path/filepath"
	"testing"
)

func TestCaptureFileFilteredOrderReopenAndCancel(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.httpsession")
	doc := sessionfile.Document{Type: sessionfile.Kind, Version: 1, Scope: "all", Transactions: []models.CaptureTransaction{{ID: "a", URL: "http://example.test/one", State: models.TxComplete}, {ID: "b", URL: "http://example.test/two", State: models.TxComplete}}}
	if err := sessionfile.Write(context.Background(), source, doc, "session"); err != nil {
		t.Fatal(err)
	}
	service := newCaptureServiceForPlatform(root, certstore.New(memoryTrustBackend{}, nil), "windows")
	request := CaptureExportRequest{Source: CaptureFileSource{Path: source}, Destination: filepath.Join(root, "filtered.httpsession"), Scope: "filtered", Format: "session", IDs: []string{"b", "a"}, SelectedID: "a"}
	count, err := service.ExportCaptureFile(request)
	if err != nil || count != 2 {
		t.Fatalf("%d %v", count, err)
	}
	loaded, _, err := sessionfile.Read(request.Destination)
	if err != nil || loaded.Transactions[0].ID != "b" || loaded.SelectedID != "a" {
		t.Fatalf("%+v %v", loaded, err)
	}
	opened, err := service.OpenCaptureFile(request.Destination)
	if err != nil || opened.SelectedID != "a" || opened.Result.Summary["total_transactions"] != 2 {
		t.Fatalf("%+v %v", opened, err)
	}
	request.IDs = []string{"missing"}
	if _, err = service.ExportCaptureFile(request); err == nil {
		t.Fatal("stale IDs silently skipped")
	}
	request.Destination = ""
	if _, err = service.ExportCaptureFile(request); err != context.Canceled {
		t.Fatal(err)
	}
}
