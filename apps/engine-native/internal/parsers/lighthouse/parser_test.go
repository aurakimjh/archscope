package lighthouse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFileExtractsAndRedactsLighthouseEvidence(t *testing.T) {
	path := writeFixture(t, `{
  "lighthouseVersion":"13.0.1",
  "requestedUrl":"https://example.test/dashboard?token=secret-value",
  "finalDisplayedUrl":"https://example.test/dashboard?user=12345678",
  "fetchTime":"2026-07-21T00:00:00.000Z",
  "configSettings":{"formFactor":"mobile","throttlingMethod":"simulate"},
  "categories":{"performance":{"id":"performance","title":"Performance","score":0.42}},
  "audits":{
    "largest-contentful-paint":{"id":"largest-contentful-paint","title":"Largest Contentful Paint token=hidden","description":"See https://example.test/help?session=private","score":0.31,"numericValue":3100,"numericUnit":"millisecond","displayValue":"3.1 s"},
    "network-requests":{"id":"network-requests","title":"Network Requests","scoreDisplayMode":"informative","details":{"type":"table","items":[{"url":"https://api.example.test/data?api_key=abc","protocol":"h2","statusCode":200,"mimeType":"application/json","resourceType":"Fetch","transferSize":1200,"resourceSize":3000,"startTime":10,"endTime":25,"initiatorType":"fetch"}]}},
    "resource-summary":{"id":"resource-summary","title":"Resource Summary","details":{"type":"table","items":[{"resourceType":"script","requestCount":2,"transferSize":5000}]}}
  }
}`)
	parsed, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Report.LighthouseVersion != "13.0.1" || len(parsed.Report.Audits) != 3 {
		t.Fatalf("unexpected report: %+v", parsed.Report)
	}
	if strings.Contains(parsed.Report.RequestedURL, "secret-value") || !strings.Contains(parsed.Report.RequestedURL, "REDACTED") {
		t.Fatalf("requested URL was not redacted: %s", parsed.Report.RequestedURL)
	}
	if len(parsed.Report.Resources) != 1 || strings.Contains(parsed.Report.Resources[0].URL, "abc") {
		t.Fatalf("resource URL was not redacted: %+v", parsed.Report.Resources)
	}
	for _, audit := range parsed.Report.Audits {
		if strings.Contains(audit.Title+audit.Description, "hidden") || strings.Contains(audit.Title+audit.Description, "private") {
			t.Fatalf("audit text was not redacted: %+v", audit)
		}
	}
	if parsed.Diagnostics.ParsedRecords != 3 {
		t.Fatalf("parsed records = %d", parsed.Diagnostics.ParsedRecords)
	}
}

func TestParseFileRejectsNonLighthouseJSON(t *testing.T) {
	path := writeFixture(t, `{"categories":{},"audits":{}}`)
	parsed, err := ParseFile(path, Options{})
	if err == nil {
		t.Fatal("expected invalid envelope error")
	}
	if parsed.Diagnostics == nil || parsed.Diagnostics.ErrorCount != 1 {
		t.Fatalf("missing diagnostic: %+v", parsed.Diagnostics)
	}
}

func TestParseFileRejectsTrailingJSONValue(t *testing.T) {
	path := writeFixture(t, `{"lighthouseVersion":"1","categories":{"performance":{"score":1}},"audits":{"x":{"score":1}}} {}`)
	parsed, err := ParseFile(path, Options{})
	if err == nil {
		t.Fatal("expected trailing JSON error")
	}
	if parsed.Diagnostics == nil || parsed.Diagnostics.ErrorCount != 1 {
		t.Fatalf("missing diagnostic: %+v", parsed.Diagnostics)
	}
}

func TestParseFileEnforcesMaxBytes(t *testing.T) {
	path := writeFixture(t, `{"lighthouseVersion":"1","categories":{"performance":{"score":1}},"audits":{"x":{"score":1}}}`)
	parsed, err := ParseFile(path, Options{MaxBytes: 8})
	if err == nil {
		t.Fatal("expected size error")
	}
	if parsed.Diagnostics == nil || parsed.Diagnostics.ErrorCount != 1 {
		t.Fatalf("missing diagnostic: %+v", parsed.Diagnostics)
	}
}

func TestParseFileBoundsOutputStringsAndRejectsInvalidScores(t *testing.T) {
	longTitle := strings.Repeat("x", MaxOutputStringBytes+100)
	path := writeFixture(t, `{"lighthouseVersion":"1","categories":{"performance":{"score":7}},"audits":{"x":{"title":"`+longTitle+`","score":-1}}}`)
	parsed, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Report.Categories[0].Score != nil || parsed.Report.Audits[0].Score != nil {
		t.Fatalf("invalid scores survived: %+v %+v", parsed.Report.Categories, parsed.Report.Audits)
	}
	if len(parsed.Report.Audits[0].Title) > MaxOutputStringBytes+len("…") {
		t.Fatalf("title was not bounded: %d", len(parsed.Report.Audits[0].Title))
	}
	if parsed.Diagnostics.WarningCount != 2 {
		t.Fatalf("warnings = %d, want 2", parsed.Diagnostics.WarningCount)
	}
}

func writeFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
