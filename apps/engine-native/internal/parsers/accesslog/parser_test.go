package accesslog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestParseLineNginxWithResponseTime(t *testing.T) {
	rec, perr := ParseLine(nginxLine("", "", "", ""))
	if perr != nil {
		t.Fatalf("unexpected parse error: %+v", perr)
	}
	if rec.Method != "GET" {
		t.Errorf("Method = %q, want GET", rec.Method)
	}
	if rec.URI != "/api/orders/1001" {
		t.Errorf("URI = %q, want /api/orders/1001", rec.URI)
	}
	if rec.Status != 200 {
		t.Errorf("Status = %d, want 200", rec.Status)
	}
	if rec.ResponseTimeMS != 123.0 {
		t.Errorf("ResponseTimeMS = %f, want 123.0", rec.ResponseTimeMS)
	}
	if rec.BytesSent != 1234 {
		t.Errorf("BytesSent = %d, want 1234", rec.BytesSent)
	}
	if rec.ClientIP != "127.0.0.1" {
		t.Errorf("ClientIP = %q, want 127.0.0.1", rec.ClientIP)
	}
	if rec.UserAgent != "Mozilla/5.0" {
		t.Errorf("UserAgent = %q", rec.UserAgent)
	}
	if rec.RawLine == "" {
		t.Errorf("RawLine should be populated")
	}
}

func TestParseLineRejectsBadTimestamp(t *testing.T) {
	line := `127.0.0.1 - - [bad-time] "GET /a HTTP/1.1" 200 1 "-" "x" 0.1`
	rec, perr := ParseLine(line)
	if rec != nil {
		t.Fatalf("expected nil record, got %+v", rec)
	}
	if perr.Reason != ReasonInvalidTimestamp {
		t.Fatalf("Reason = %q, want %q", perr.Reason, ReasonInvalidTimestamp)
	}
}

func TestParseLineRejectsBadNumber(t *testing.T) {
	line := `127.0.0.1 - - [27/Apr/2026:10:00:02 +0900] "GET /a HTTP/1.1" abc 1 "-" "x" 0.1`
	_, perr := ParseLine(line)
	if perr == nil || perr.Reason != ReasonInvalidNumber {
		t.Fatalf("expected INVALID_NUMBER, got %+v", perr)
	}
}

func TestParseLineRejectsNoFormat(t *testing.T) {
	_, perr := ParseLine("not an access log line")
	if perr == nil || perr.Reason != ReasonNoFormatMatch {
		t.Fatalf("expected NO_FORMAT_MATCH, got %+v", perr)
	}
}

func TestParseLineCommonFormatWithoutResponseTime(t *testing.T) {
	line := `127.0.0.1 - - [27/Apr/2026:10:00:01 +0900] "GET /health HTTP/1.1" 204 -`
	rec, perr := ParseLine(line)
	if perr != nil {
		t.Fatalf("unexpected parse error: %+v", perr)
	}
	if rec.URI != "/health" || rec.Status != 204 {
		t.Errorf("rec mismatch: %+v", rec)
	}
	if rec.BytesSent != 0 {
		t.Errorf("BytesSent should be 0 for `-`, got %d", rec.BytesSent)
	}
	if rec.ResponseTimeMS != 0 {
		t.Errorf("ResponseTimeMS should be 0 when format has none, got %f", rec.ResponseTimeMS)
	}
}

func TestParseFileReportsMalformedLineDiagnostics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	valid := nginxLine("", "", "", "")
	invalidTimestamp := `127.0.0.1 - - [bad-time] ` +
		`"GET /api/orders/1002 HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 0.123`
	invalidNumber := `127.0.0.1 - - [27/Apr/2026:10:00:02 +0900] ` +
		`"GET /api/orders/1003 HTTP/1.1" abc 1234 "-" "Mozilla/5.0" 0.123`
	noFormat := "not an nginx access log record"
	body := strings.Join([]string{valid, "", invalidTimestamp, invalidNumber, noFormat}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	records, diags, err := ParseFile(path, "nginx", Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if diags.TotalLines != 5 {
		t.Errorf("TotalLines = %d, want 5", diags.TotalLines)
	}
	if diags.ParsedRecords != 1 {
		t.Errorf("ParsedRecords = %d, want 1", diags.ParsedRecords)
	}
	if diags.SkippedLines != 3 {
		t.Errorf("SkippedLines = %d, want 3", diags.SkippedLines)
	}
	if diags.ErrorCount != 3 {
		t.Errorf("ErrorCount = %d, want 3", diags.ErrorCount)
	}
	wantReasons := map[string]int{
		ReasonInvalidTimestamp: 1,
		ReasonInvalidNumber:    1,
		ReasonNoFormatMatch:    1,
	}
	for k, v := range wantReasons {
		if diags.SkippedByReason[k] != v {
			t.Errorf("SkippedByReason[%q] = %d, want %d", k, diags.SkippedByReason[k], v)
		}
	}
	if len(diags.Samples) != 3 {
		t.Fatalf("Samples = %d, want 3", len(diags.Samples))
	}
	wantOrder := []string{ReasonInvalidTimestamp, ReasonInvalidNumber, ReasonNoFormatMatch}
	for i, want := range wantOrder {
		if diags.Samples[i].Reason != want {
			t.Errorf("Samples[%d].Reason = %q, want %q", i, diags.Samples[i].Reason, want)
		}
	}
	if diags.SourceFile == nil || *diags.SourceFile != path {
		t.Errorf("SourceFile = %v, want %q", diags.SourceFile, path)
	}
	if diags.Format != "nginx" {
		t.Errorf("Format = %q", diags.Format)
	}
}

func TestParseFileStrictModeFailsFast(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	body := "bad access log\n" + nginxLine("", "", "", "") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, _, err := ParseFile(path, "nginx", Options{Strict: true})
	if err == nil {
		t.Fatalf("expected error in strict mode")
	}
	if !strings.Contains(err.Error(), ReasonNoFormatMatch) {
		t.Fatalf("error should mention NO_FORMAT_MATCH; got %v", err)
	}
}

func TestParseFileRejectsUnsupportedFormat(t *testing.T) {
	_, _, err := ParseFile("/tmp/none.log", "iis", Options{})
	if err == nil || !strings.Contains(err.Error(), "Unsupported access log format") {
		t.Fatalf("expected format-rejection error, got %v", err)
	}
}

func TestParseFileEmptyFileWarns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	records, diags, err := ParseFile(path, "nginx", Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records non-empty for empty file: %+v", records)
	}
	if diags.WarningCount != 1 {
		t.Errorf("WarningCount = %d, want 1", diags.WarningCount)
	}
	if len(diags.Warnings) == 0 || diags.Warnings[0].Reason != "EMPTY_FILE" {
		t.Errorf("expected EMPTY_FILE warning, got %+v", diags.Warnings)
	}
}
