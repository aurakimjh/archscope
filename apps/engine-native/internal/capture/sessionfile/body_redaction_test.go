package sessionfile

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestBodyRedactionProvenanceSurvivesRepeatedExports(t *testing.T) {
	doc := Document{Type: Kind, Version: 1, Scope: "all", SelectedID: "masked", Transactions: []models.CaptureTransaction{{ID: "masked", URL: "http://example.test/", Request: models.HTTPMessage{ContentType: "application/json", BodyStorage: "inline", BodyPreview: `{"value":"retained","password":"fixture"}`}}}}
	for i := 0; i < 3; i++ {
		file := filepath.Join(t.TempDir(), "masked.httpsession")
		if err := Write(context.Background(), file, doc, "session"); err != nil {
			t.Fatal(err)
		}
		loaded, _, err := Read(file)
		if err != nil {
			t.Fatal(err)
		}
		message := loaded.Transactions[0].Request
		if !message.Redacted || message.BodyPreview != `{"password":"[REDACTED]","value":"retained"}` || message.BodyStorage != "inline" {
			t.Fatalf("export/open cycle%d lost redaction provenance: %+v", i, message)
		}
		doc = loaded
	}
}
