package main

import (
	"context"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnnotationsKeepIdentityAcrossSaveReopen(t *testing.T) {
	root := t.TempDir()
	persisted, err := store.New(store.Config{Root: root, SessionID: "cap-notes-fixture", ReserveBytes: 0})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		if _, err = persisted.Append(models.CaptureTransaction{ID: id, URL: "http://example.test/" + id, State: models.TxComplete}); err != nil {
			t.Fatal(err)
		}
	}
	if err = persisted.Close(); err != nil {
		t.Fatal(err)
	}
	service := newCaptureServiceForPlatform(root, certstore.New(memoryTrustBackend{}, nil), "windows")
	saved, err := service.SetCaptureAnnotation("cap-notes-fixture", "b", models.HTTPAnnotation{Starred: true, Comment: "password=private"})
	if err != nil || strings.Contains(saved.Comment, "private") {
		t.Fatalf("%+v %v", saved, err)
	}
	reopened, err := service.GetCaptureAnnotations("cap-notes-fixture")
	if err != nil || !reopened["b"].Starred || reopened["a"].Starred {
		t.Fatalf("%+v %v", reopened, err)
	}
	destination := filepath.Join(root, "notes.httpsession")
	_, err = service.ExportCaptureFile(CaptureExportRequest{Source: CaptureFileSource{SessionID: "cap-notes-fixture"}, Destination: destination, Format: "session", Scope: "filtered", IDs: []string{"b", "a"}})
	if err != nil {
		t.Fatal(err)
	}
	doc, _, err := sessionfile.Read(destination)
	if err != nil || doc.Transactions[0].ID != "b" || !doc.Transactions[0].Starred || doc.Transactions[0].Comment != saved.Comment || doc.Transactions[1].Starred {
		t.Fatalf("%+v %v", doc, err)
	}
	if _, err = service.SetCaptureAnnotation("cap-notes-fixture", "missing", models.HTTPAnnotation{Starred: true}); err == nil {
		t.Fatal("missing exchange accepted")
	}
	doc.Transactions[0].Comment = "local-file edit"
	if err = sessionfile.Write(context.Background(), destination, doc, "xml"); err != nil {
		t.Fatal(err)
	}
	doc, _, err = sessionfile.Read(destination)
	if err != nil || doc.Transactions[0].Comment != "local-file edit" {
		t.Fatal(err)
	}
}
