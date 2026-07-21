package httpcapture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestParseHARNormalizesDialectAndRedactsURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chrome.har")
	data := `{"log":{"version":"1.2","creator":{"name":"Chrome","version":"1"},"entries":[{"startedDateTime":"2026-07-20T10:00:00Z","time":42,"request":{"method":"GET","url":"https://api.example.test/orders?token=secret","httpVersion":"HTTP/2","headers":[],"queryString":[],"cookies":[],"headersSize":-1,"bodySize":0},"response":{"status":503,"statusText":"Unavailable","httpVersion":"HTTP/2","headers":[],"cookies":[],"content":{"size":12,"mimeType":"application/json","text":"{\"ok\":false}"},"redirectURL":"","headersSize":-1,"bodySize":12},"cache":{},"timings":{"blocked":0,"dns":-1,"connect":-1,"ssl":-1,"send":0,"wait":31,"receive":11}}]}}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Dialect != "chrome" || len(parsed.Entries) != 1 {
		t.Fatalf("unexpected parsed HAR: %+v", parsed)
	}
	entry := parsed.Entries[0]
	if strings.Contains(entry.URL, "secret") || !strings.Contains(entry.URL, "%5BREDACTED%5D") {
		t.Fatalf("URL was not safely redacted: %q", entry.URL)
	}
	if entry.Timings.ImportedHAR == nil || entry.Timings.ImportedHAR.Wait.MS != 31 || entry.Timings.ImportedHAR.Wait.State != models.TimingKnown || entry.Response.BodySize != 12 || entry.State != models.TxComplete || entry.StatusCode != 503 {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if !parsed.Redaction.Applied || parsed.Redaction.Counts["query_value"] == 0 {
		t.Fatalf("missing redaction summary: %+v", parsed.Redaction)
	}
}

func TestParseHARRejectsMissingEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.har")
	if err := os.WriteFile(path, []byte(`{"log":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseFile(path, Options{}); err == nil {
		t.Fatal("expected missing entries error")
	}
}

func TestParseHARRejectsEntriesObjectWithoutPartialSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad-entries.har")
	if err := os.WriteFile(path, []byte(`{"log":{"entries":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseFile(path, Options{})
	if err == nil {
		t.Fatal("expected structural error")
	}
	if len(parsed.Entries) != 0 {
		t.Fatalf("fatal structural error returned partial entries: %+v", parsed.Entries)
	}
}
