package httpcapture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/halog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestNativeImportRedactionLimitsAndProvenance(t *testing.T) {
	rows := []models.CaptureTransaction{{ID: "test", URL: "http://example.test/path?token=private-query", Method: "POST", State: models.TxComplete, StatusCode: 200, StartedAt: "2026-11-10T00:00:00Z", TotalMS: 125, Comment: "token=private-comment", Request: models.HTTPMessage{Headers: []models.HeaderField{{Name: "Authorization", Value: "Bearer private-header"}, {Name: "Cookie", Value: "session=private-cookie"}}, ContentType: "application/json", BodyPreview: `{"token":"private-body"}`, BodyStorage: "inline"}, Response: models.HTTPMessage{BodyStorage: "inline", BodyPreview: "response"}}}
	data, err := halog.EncodeTransactionsCodePage(rows, 1252)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture.halog")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	x := parsed.Entries[0]
	if parsed.Format != FormatHALOG || x.CaptureMode != "halog_import" || x.Fidelity != "semantic" || x.ObservationPoint != "foreign_tool" || x.StreamProvenance != "reconstructed_semantic" {
		t.Fatal("incorrect native provenance")
	}
	if strings.Contains(x.URL+x.Comment+x.Request.BodyPreview+x.Request.Headers[0].Value+x.Request.Headers[1].Value, "private-") {
		t.Fatal("native import bypassed redaction")
	}
	if _, err = ParseFile(path, Options{MaxBodyBytes: 3}); err == nil {
		t.Fatal("configured body limit ignored")
	}
	if _, err = ParseFile(path, Options{MaxBytes: 10}); err == nil {
		t.Fatal("configured input limit ignored")
	}
	if _, err = ParseFile(path, Options{CustomRedactionPatterns: []string{"response"}}); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(data, 0), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ParseFile(path, Options{}); err == nil {
		t.Fatal("trailing native bytes accepted")
	}
}
