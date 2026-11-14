package sessionfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestReadRejectsAnnotationGrowthAfterRedaction(t *testing.T) {
	for _, test := range []struct {
		name, comment string
		refused       bool
	}{
		{"ascii-boundary", strings.Repeat("a", 4096), false},
		{"hangul-boundary", strings.Repeat("가", 1365) + "a", false},
		{"emoji-boundary", strings.Repeat("😀", 1024), false},
		{"redaction-expands-past-limit", strings.Repeat("password=x ", 300), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := Document{Type: Kind, Version: 1, Scope: "all", SelectedID: "annotation", Transactions: []models.CaptureTransaction{{ID: "annotation", URL: "http://example.test/", Comment: test.comment}}}
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), "historical.httpsession")
			if err = os.WriteFile(file, raw, 0600); err != nil {
				t.Fatal(err)
			}
			loaded, _, err := Read(file)
			if test.refused {
				if err == nil {
					t.Fatalf("accepted annotation with %d sanitized bytes", len(loaded.Transactions[0].Comment))
				}
			} else if err != nil || loaded.Transactions[0].Comment != test.comment || loaded.SelectedID != "annotation" {
				t.Fatalf("valid boundary changed: %v", err)
			}
			after, err := os.ReadFile(file)
			if err != nil || string(after) != string(raw) {
				t.Fatal("read changed historical source bytes", err)
			}
		})
	}
}
