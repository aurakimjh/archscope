package httpcapture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseHARNormalizesDialectAndRedactsURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chrome.har")
	data := `{"log":{"creator":{"name":"Chrome","version":"1"},"entries":[{"startedDateTime":"2026-07-20T10:00:00Z","time":42,"request":{"method":"GET","url":"https://api.example.test/orders?token=secret"},"response":{"status":503,"bodySize":12,"content":{"mimeType":"application/json"}},"timings":{"wait":31,"receive":11}}]}}`
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
	if entry.URL != "https://api.example.test/orders?token=<TOKEN len=6>" || entry.WaitMS != 31 || entry.ResponseBytes != 12 || !entry.Error {
		t.Fatalf("unexpected entry: %+v", entry)
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
