package httpcapture

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
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
	if entry.Request.BodyDecoded != -1 {
		t.Fatalf("request decoded body size must remain unknown: %+v", entry.Request)
	}
	if !parsed.Redaction.Applied || parsed.Redaction.Counts["query_value"] == 0 {
		t.Fatalf("missing redaction summary: %+v", parsed.Redaction)
	}
}

func TestParseHARHandlesUnparseableURLsWithoutPanic(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "invalid-escape", url: "https://example.test/%GG?token=URL-SECRET"},
		{name: "invalid-ipv6", url: "http://user:URL-SECRET@[::1"},
		{name: "control-character", url: "https://example.test/\x01?token=URL-SECRET"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeMinimalHAR(t, test.url, 0)
			parsed, err := ParseFile(path, Options{})
			if err != nil {
				t.Fatalf("unparseable URL should be retained with a warning: %v", err)
			}
			if len(parsed.Entries) != 1 || strings.Contains(parsed.Entries[0].URL, "URL-SECRET") {
				t.Fatalf("URL was not safely retained: %+v", parsed.Entries)
			}
			if !hasDiagnostic(parsed.Diagnostics, ReasonURLUnparsable) {
				t.Fatalf("missing %s diagnostic: %+v", ReasonURLUnparsable, parsed.Diagnostics)
			}
		})
	}
}

func TestParseHARReportsMalformedNegativeTimings(t *testing.T) {
	path := writeMinimalHAR(t, "https://example.test/", -2)
	parsed, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(parsed.Diagnostics, "HAR_TIMING_NEGATIVE") {
		t.Fatalf("missing negative-timing diagnostic: %+v", parsed.Diagnostics)
	}
}

func TestParseFilePanicRecoveryReturnsStructuralErrorWithoutPanicValue(t *testing.T) {
	result, err := func() (result ParseResult, err error) {
		diags := diagnostics.New(FormatHAR)
		defer recoverParsePanic(&result, &err, diags)
		panic("SENSITIVE-PANIC-VALUE")
	}()
	if !errors.Is(err, ErrStructuralHAR) || result.Diagnostics == nil || !hasDiagnostic(result.Diagnostics, ReasonStructural) {
		t.Fatalf("panic was not converted to a structural error: result=%+v err=%v", result, err)
	}
	if strings.Contains(err.Error(), "SENSITIVE-PANIC-VALUE") {
		t.Fatalf("panic value leaked through the parser error: %v", err)
	}
}

func TestInlinePreviewPreservesUTF8Boundary(t *testing.T) {
	value := strings.Repeat("a", inlineBodyPreviewBytes-1) + "😀tail"
	preview := inlinePreview(value)
	if !utf8.ValidString(preview) || len(preview) != inlineBodyPreviewBytes-1 {
		t.Fatalf("preview split a UTF-8 rune: length=%d valid=%v", len(preview), utf8.ValidString(preview))
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

func TestParseHARDropsWebSocketAndSSEPayloadsFromNormalizedOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.har")
	data := `{"log":{"creator":{"name":"Chrome"},"entries":[{"startedDateTime":"2026-07-20T10:00:00Z","time":1,"request":{"method":"GET","url":"https://example.test/socket","headers":[],"queryString":[],"cookies":[],"headersSize":-1,"bodySize":0},"response":{"status":101,"headers":[],"cookies":[],"content":{"size":0,"text":""},"headersSize":-1,"bodySize":0},"cache":{},"timings":{"blocked":0,"dns":-1,"connect":-1,"ssl":-1,"send":0,"wait":1,"receive":0},"_webSocketMessages":[{"data":"WS-SECRET-MUST-NOT-ESCAPE"}],"_eventSourceMessages":[{"data":"SSE-SECRET-MUST-NOT-ESCAPE"}]}]}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "WS-SECRET-MUST-NOT-ESCAPE") || strings.Contains(string(encoded), "SSE-SECRET-MUST-NOT-ESCAPE") {
		t.Fatalf("message payload escaped normalized output: %s", encoded)
	}
}

func writeMinimalHAR(t *testing.T, rawURL string, blocked float64) string {
	t.Helper()
	document := map[string]any{
		"log": map[string]any{
			"creator": map[string]any{"name": "Chrome"},
			"entries": []any{map[string]any{
				"startedDateTime": "2026-07-20T10:00:00Z",
				"time":            1,
				"request": map[string]any{
					"method": "GET", "url": rawURL, "headers": []any{}, "queryString": []any{}, "cookies": []any{}, "headersSize": -1, "bodySize": 0,
				},
				"response": map[string]any{
					"status": 200, "headers": []any{}, "cookies": []any{}, "content": map[string]any{"size": 0, "text": ""}, "headersSize": -1, "bodySize": 0,
				},
				"cache":   map[string]any{},
				"timings": map[string]any{"blocked": blocked, "dns": -1, "connect": -1, "ssl": -1, "send": 0, "wait": 1, "receive": 0},
			}},
		},
	}
	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "minimal.har")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func hasDiagnostic(diags *diagnostics.ParserDiagnostics, reason string) bool {
	if diags == nil {
		return false
	}
	for _, sample := range diags.Samples {
		if sample.Reason == reason {
			return true
		}
	}
	return false
}
