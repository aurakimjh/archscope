package httpcapture

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHARResourceGuardsRejectBeforePartialSuccess(t *testing.T) {
	validEntry := `{"startedDateTime":"2026-01-01T00:00:00Z","time":1,"request":{"method":"GET","url":"https://example.test/","headers":[],"queryString":[],"cookies":[],"headersSize":-1,"bodySize":0},"response":{"status":200,"headers":[],"cookies":[],"content":{"size":0,"text":""},"headersSize":-1,"bodySize":0},"cache":{},"timings":{"blocked":0,"dns":-1,"connect":-1,"ssl":-1,"send":0,"wait":1,"receive":0}}`
	tests := []struct {
		name string
		data string
		opts Options
	}{
		{"entry-cap", `{"log":{"creator":{"name":"Chrome"},"entries":[` + validEntry + `,` + validEntry + `]}}`, Options{MaxEntries: 1}},
		{"depth-cap", `{"log":{"entries":[]}}`, Options{MaxDepth: 2}},
		{"string-cap", `{"log":{"creator":{"name":"Chrome"},"entries":[]}}`, Options{MaxStringBytes: 4}},
		{"body-cap", `{"log":{"creator":{"name":"Chrome"},"entries":[` + strings.Replace(validEntry, `"text":""`, `"text":"0123456789"`, 1) + `]}}`, Options{MaxBodyBytes: 5}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), test.name+".har")
			if err := os.WriteFile(path, []byte(test.data), 0o600); err != nil {
				t.Fatal(err)
			}
			parsed, err := ParseFile(path, test.opts)
			if err == nil {
				t.Fatal("expected resource guard rejection")
			}
			if len(parsed.Entries) != 0 {
				t.Fatalf("resource guard returned partial entries: %d", len(parsed.Entries))
			}
		})
	}
}

func TestHARGzipExpansionRatioGuard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expansion.har.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := gzip.NewWriter(file)
	payload := `{"log":{"creator":{"name":"` + strings.Repeat("A", 32_000) + `"},"entries":[]}}`
	if _, err := writer.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseFile(path, Options{MaxDecompressionRatio: 2})
	if err == nil {
		t.Fatal("expected gzip expansion rejection")
	}
	if len(parsed.Entries) != 0 {
		t.Fatal("gzip rejection returned partial entries")
	}
}
