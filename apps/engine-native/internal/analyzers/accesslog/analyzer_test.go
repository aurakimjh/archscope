package accesslog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func nginxLine(timestamp, uri, status, responseTimeSec string) string {
	if timestamp == "" {
		timestamp = "27/Apr/2026:10:00:01 +0900"
	}
	if uri == "" {
		uri = "/api/orders/1001"
	}
	if status == "" {
		status = "200"
	}
	if responseTimeSec == "" {
		responseTimeSec = "0.123"
	}
	return "127.0.0.1 - - [" + timestamp + "] " +
		`"GET ` + uri + ` HTTP/1.1" ` + status + ` 1234 "-" "Mozilla/5.0" ` + responseTimeSec
}

func TestAnalyzeFixtureSummaryParity(t *testing.T) {
	// Mirrors Python test_analyze_access_log_sample.
	path := "../../../../../examples/access-logs/sample-nginx-access.log"
	result, err := Analyze(path, "nginx", Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Type != "access_log" {
		t.Fatalf("Type = %q, want access_log", result.Type)
	}
	if got := asInt(result.Summary["total_requests"]); got != 6 {
		t.Errorf("total_requests = %d, want 6", got)
	}
	if got := asFloat(result.Summary["error_rate"]); got != 33.33 {
		t.Errorf("error_rate = %v, want 33.33", got)
	}
	if got := asInt(result.Summary["error_count"]); got != 2 {
		t.Errorf("error_count = %d, want 2", got)
	}
}

func TestAnalyzeIncludesDiagnosticsMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	body := nginxLine("", "", "", "") + "\nmalformed\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := Analyze(path, "nginx", Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := asInt(result.Summary["total_requests"]); got != 1 {
		t.Errorf("total_requests = %d, want 1", got)
	}
	if result.Metadata.Diagnostics == nil {
		t.Fatal("Diagnostics nil")
	}
	d := result.Metadata.Diagnostics
	if d.TotalLines != 2 {
		t.Errorf("TotalLines = %d, want 2", d.TotalLines)
	}
	if d.ParsedRecords != 1 {
		t.Errorf("ParsedRecords = %d, want 1", d.ParsedRecords)
	}
	if d.SkippedLines != 1 {
		t.Errorf("SkippedLines = %d, want 1", d.SkippedLines)
	}
	if d.SkippedByReason["NO_FORMAT_MATCH"] != 1 {
		t.Errorf("SkippedByReason mismatch: %+v", d.SkippedByReason)
	}
}

func TestAnalyzeRespectsMaxLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	body := strings.Join([]string{
		nginxLine("", "/one", "", ""),
		nginxLine("", "/two", "", ""),
		nginxLine("", "/three", "", ""),
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := Analyze(path, "nginx", Options{MaxLines: 2})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := asInt(result.Summary["total_requests"]); got != 2 {
		t.Errorf("total_requests = %d, want 2", got)
	}
	if result.Metadata.Diagnostics.TotalLines != 2 {
		t.Errorf("TotalLines = %d, want 2", result.Metadata.Diagnostics.TotalLines)
	}
	opts, _ := result.Metadata.Extra["analysis_options"].(map[string]any)
	if asInt(opts["max_lines"]) != 2 {
		t.Errorf("analysis_options.max_lines = %v, want 2", opts["max_lines"])
	}
}

func TestAnalyzeFiltersByTimeRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	body := strings.Join([]string{
		nginxLine("27/Apr/2026:10:00:00 +0900", "/early", "", ""),
		nginxLine("27/Apr/2026:10:01:00 +0900", "/included", "", ""),
		nginxLine("27/Apr/2026:10:02:00 +0900", "/late", "", ""),
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	bound := time.Date(2026, time.April, 27, 10, 1, 0, 0, time.FixedZone("UTC+9", 9*3600))
	result, err := Analyze(path, "nginx", Options{StartTime: &bound, EndTime: &bound})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := asInt(result.Summary["total_requests"]); got != 1 {
		t.Errorf("total_requests = %d, want 1", got)
	}
	tops := result.Series["top_urls_by_count"].([]map[string]any)
	if len(tops) != 1 || tops[0]["uri"] != "/included" || asInt(tops[0]["count"]) != 1 {
		t.Errorf("top_urls_by_count mismatch: %+v", tops)
	}
	if result.Metadata.Diagnostics.TotalLines != 3 {
		t.Errorf("TotalLines = %d, want 3", result.Metadata.Diagnostics.TotalLines)
	}
	if result.Metadata.Diagnostics.ParsedRecords != 1 {
		t.Errorf("ParsedRecords = %d, want 1", result.Metadata.Diagnostics.ParsedRecords)
	}
}

func TestAnalyzeReportsErrorAndSlowURLFindings(t *testing.T) {
	// Need ≥5 hits on a single URI so SLOW_URL_P95 fires.
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	lines := []string{nginxLine("", "/fast", "200", "0.100")}
	for i := 0; i < 5; i++ {
		lines = append(lines, nginxLine(
			fmt.Sprintf("27/Apr/2026:10:00:%02d +0900", 2+i),
			"/slow-error", "500", "2.500",
		))
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := Analyze(path, "nginx", Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	codes := map[string]bool{}
	for _, finding := range result.Metadata.Findings {
		codes[finding["code"].(string)] = true
	}
	want := []string{"HIGH_ERROR_RATE", "SERVER_ERRORS_PRESENT", "SLOW_URL_P95"}
	for _, code := range want {
		if !codes[code] {
			t.Errorf("missing finding %q in %v", code, codes)
		}
	}
}

func TestAnalyzeReportsNonStandardHTTPStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	if err := os.WriteFile(path, []byte(nginxLine("", "", "999", "")+"\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := Analyze(path, "nginx", Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	var ns map[string]any
	for _, finding := range result.Metadata.Findings {
		if finding["code"] == "NON_STANDARD_HTTP_STATUS" {
			ns = finding
			break
		}
	}
	if ns == nil {
		t.Fatalf("NON_STANDARD_HTTP_STATUS finding not emitted; got %+v", result.Metadata.Findings)
	}
	evidence := ns["evidence"].(map[string]any)
	if evidence["statuses"] != "999" || asInt(evidence["count"]) != 1 {
		t.Fatalf("evidence mismatch: %+v", evidence)
	}
}

func TestAnalyzeMarshalsToJSONWithExtraInline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	if err := os.WriteFile(path, []byte(nginxLine("", "", "", "")+"\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := Analyze(path, "nginx", Options{MaxLines: 10})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`"type":"access_log"`,
		`"parser":"nginx_combined_with_response_time"`,
		`"schema_version":"0.2.0"`,
		`"format":"nginx"`,
		`"analysis_options":`,
		`"diagnostics":`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("missing %q in marshalled output: %s", want, body)
		}
	}
}

func TestAnalyzeHandlesMoreRecordsThanReservoir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	lines := make([]string, 0, 10049)
	for i := 1; i < 10050; i++ {
		ts := fmt.Sprintf("27/Apr/2026:10:%02d:00 +0900", i%60)
		uri := fmt.Sprintf("/item/%d", i)
		rt := fmt.Sprintf("%.3f", float64(i)/1000.0)
		lines = append(lines, nginxLine(ts, uri, "", rt))
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := Analyze(path, "nginx", Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := asInt(result.Summary["total_requests"]); got != 10049 {
		t.Errorf("total_requests = %d, want 10049", got)
	}
	if got := asFloat(result.Summary["p95_response_ms"]); got <= 0 {
		t.Errorf("p95_response_ms = %v, want > 0", got)
	}
}

func TestStatusFamilyBoundaries(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{99, "other"},
		{200, "2xx"}, {299, "2xx"},
		{300, "3xx"}, {399, "3xx"},
		{400, "4xx"}, {499, "4xx"},
		{500, "5xx"}, {599, "5xx"}, {999, "5xx"},
	}
	for _, tc := range cases {
		if got := statusFamily(tc.status); got != tc.want {
			t.Errorf("statusFamily(%d) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestClassifyRequestStaticVsAPI(t *testing.T) {
	cases := []struct {
		uri  string
		want string
	}{
		{"/static/app.js", "static"},
		{"/assets/logo.png", "static"},
		{"/api/orders/1001", "api"},
		{"/health", "api"},
		{"/IMG/photo.JPEG", "static"},
		{"/foo.css?v=1", "static"},
	}
	for _, tc := range cases {
		if got := classifyRequest(tc.uri); got != tc.want {
			t.Errorf("classifyRequest(%q) = %q, want %q", tc.uri, got, tc.want)
		}
	}
}
