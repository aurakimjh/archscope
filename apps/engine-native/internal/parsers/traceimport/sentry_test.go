package traceimport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestActualSentrySDKAndExactTime(t *testing.T) {
	got, e := ParseFile("testdata/sentry/actual.json", Options{Format: FormatSentryTransaction})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Spans) != 3 || got.Diagnostics.ParsedRecords != 3 || got.Spans[0].DurationNanos != 4757000 {
		t.Fatal(got)
	}
	if got.Spans[1].ParentSpanID != got.Spans[0].SpanID || got.Spans[1].Kind != "CLIENT" || got.Spans[0].ServiceName != "unknown-sentry-service" {
		t.Fatal(got.Spans)
	}
	for _, s := range got.Spans {
		if s.Name != "sentry.http.server" && s.Name != "sentry.db" && s.Name != "sentry.http.client" {
			t.Fatal("raw name escaped")
		}
	}
	for _, raw := range []string{`1794530035.095574`, `"2026-11-13T00:33:55.095574Z"`} {
		if n, e := sentryTime(json.RawMessage(raw)); e != nil || n != 1794530035095574000 {
			t.Fatal(n, e)
		}
	}
	for _, raw := range []string{`1e999999999`, `0.0000000001`, `"2026-11-13T00:33:55.0000000001Z"`, `-1`, `null`} {
		if _, e := sentryTime(json.RawMessage(raw)); e == nil {
			t.Fatal("accepted timestamp", raw)
		}
	}
	body, _ := os.ReadFile("testdata/sentry/actual.json")
	var f map[string]any
	json.Unmarshal(body, &f)
	f["sdk"].(map[string]any)["version"] = "unknown"
	bad, _ := json.Marshal(f)
	path := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(path, bad, 0600)
	if r, e := ParseFile(path, Options{Format: FormatSentryTransaction}); e == nil || len(r.Spans) != 0 {
		t.Fatal("partial/unqualified accepted")
	}
}
