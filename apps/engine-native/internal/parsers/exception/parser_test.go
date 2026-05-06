package exception

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleBlock = "2026-04-27T10:00:00+09:00 java.lang.IllegalStateException: failed\n" +
	"    at com.example.OrderController.create(OrderController.java:15)\n" +
	"Caused by: java.sql.SQLTimeoutException: timeout\n" +
	"    at oracle.jdbc.driver.T4CPreparedStatement.executeQuery(T4CPreparedStatement.java:1)\n" +
	"java.lang.IllegalStateException: failed\n" +
	"    at com.example.OrderController.create(OrderController.java:15)\n"

func TestParseFileMirrorsPythonAnalyzerCase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exception.log")
	if err := os.WriteFile(path, []byte(sampleBlock), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	records, diags, err := ParseFile(path, "", Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	if diags.ParsedRecords != 2 {
		t.Errorf("ParsedRecords = %d, want 2", diags.ParsedRecords)
	}
	if diags.SkippedLines != 0 {
		t.Errorf("SkippedLines = %d, want 0", diags.SkippedLines)
	}
	if diags.Format != FormatJavaExceptionStack {
		t.Errorf("Format = %q, want %q", diags.Format, FormatJavaExceptionStack)
	}
	if diags.SourceFile == nil || *diags.SourceFile != path {
		t.Errorf("SourceFile = %v, want %q", diags.SourceFile, path)
	}

	first := records[0]
	if first.ExceptionType != "java.lang.IllegalStateException" {
		t.Errorf("ExceptionType = %q", first.ExceptionType)
	}
	if first.RootCause == nil || *first.RootCause != "java.sql.SQLTimeoutException" {
		t.Errorf("RootCause = %v, want java.sql.SQLTimeoutException", first.RootCause)
	}
	if first.Language != "java" {
		t.Errorf("Language = %q, want java", first.Language)
	}
	if first.Message == nil || *first.Message != "failed" {
		t.Errorf("Message = %v, want \"failed\"", first.Message)
	}
	if first.Timestamp == nil {
		t.Errorf("Timestamp should be parsed for ISO-8601 header")
	}
	if first.Signature != "java.lang.IllegalStateException|com.example.OrderController.create(OrderController.java:15)" {
		t.Errorf("Signature = %q", first.Signature)
	}
	// Python: every `at`-prefixed line in the block (including those
	// after Caused-by separators) is appended to the same stack list.
	// We mirror that — analyzers downstream rely on the flattened view.
	wantStack := []string{
		"com.example.OrderController.create(OrderController.java:15)",
		"oracle.jdbc.driver.T4CPreparedStatement.executeQuery(T4CPreparedStatement.java:1)",
	}
	if len(first.Stack) != len(wantStack) {
		t.Errorf("Stack length = %d, want %d", len(first.Stack), len(wantStack))
	} else {
		for i, frame := range wantStack {
			if first.Stack[i] != frame {
				t.Errorf("Stack[%d] = %q, want %q", i, first.Stack[i], frame)
			}
		}
	}

	second := records[1]
	if second.RootCause != nil {
		t.Errorf("second.RootCause = %v, want nil (no Caused-by chain)", second.RootCause)
	}
	if second.Timestamp != nil {
		t.Errorf("second.Timestamp = %v, want nil (no leading timestamp)", second.Timestamp)
	}
	if first.Signature != second.Signature {
		t.Errorf("repeated headers should share a signature: %q vs %q", first.Signature, second.Signature)
	}
}

func TestParseFileNoExceptionHeaderRecordedAsSkip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exception.log")
	body := "this is not an exception line\n" +
		"java.lang.RuntimeException: hi\n" +
		"    at com.example.Foo.bar(Foo.java:1)\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	records, diags, err := ParseFile(path, "", Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if diags.SkippedLines != 1 {
		t.Errorf("SkippedLines = %d, want 1", diags.SkippedLines)
	}
	if diags.SkippedByReason[ReasonNoExceptionHeader] != 1 {
		t.Errorf("SkippedByReason[NO_EXCEPTION_HEADER] = %d", diags.SkippedByReason[ReasonNoExceptionHeader])
	}
	if len(diags.Samples) != 1 || diags.Samples[0].Reason != ReasonNoExceptionHeader {
		t.Errorf("expected single NO_EXCEPTION_HEADER sample, got %+v", diags.Samples)
	}
}

func TestParseFileEmptyFileWarns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	records, diags, err := ParseFile(path, "", Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records non-empty for empty file: %+v", records)
	}
	if diags.WarningCount != 1 || diags.Warnings[0].Reason != ReasonEmptyFile {
		t.Errorf("expected EMPTY_FILE warning, got %+v", diags.Warnings)
	}
}

func TestParseFileNoExceptionBlocksWarns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noisy.log")
	body := "INFO some app log line\n" + "WARN another non-exception line\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	records, diags, err := ParseFile(path, "", Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records = %d, want 0", len(records))
	}
	if diags.WarningCount != 1 {
		t.Errorf("WarningCount = %d, want 1", diags.WarningCount)
	}
	if len(diags.Warnings) == 0 || diags.Warnings[0].Reason != ReasonNoExceptionBlocks {
		t.Errorf("expected NO_EXCEPTION_BLOCKS warning, got %+v", diags.Warnings)
	}
}

func TestParseFileStrictModeFailsFastOnOrphanLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exception.log")
	body := "noise that opens nothing\n" + "java.lang.RuntimeException\n    at Foo.bar(Foo.java:1)\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, _, err := ParseFile(path, "", Options{Strict: true})
	if err == nil {
		t.Fatalf("expected error in strict mode")
	}
	if !strings.Contains(err.Error(), ReasonNoExceptionHeader) {
		t.Fatalf("error should mention NO_EXCEPTION_HEADER; got %v", err)
	}
}

func TestParseFileSampleFixture(t *testing.T) {
	// Run against the workspace fixture so the Go parser stays
	// diff-able with the Python reference output.
	fixture, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "examples", "exceptions", "sample-java-exception.txt"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(fixture); err != nil {
		t.Skipf("fixture not available: %v", err)
	}

	records, diags, err := ParseFile(fixture, "", Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2 (matching sample-java-exception.txt)", len(records))
	}
	if records[0].RootCause == nil || *records[0].RootCause != "java.sql.SQLTimeoutException" {
		t.Errorf("first.RootCause = %v, want java.sql.SQLTimeoutException", records[0].RootCause)
	}
	if records[1].RootCause != nil {
		t.Errorf("second.RootCause = %v, want nil", records[1].RootCause)
	}
	if diags.ParsedRecords != 2 {
		t.Errorf("ParsedRecords = %d, want 2", diags.ParsedRecords)
	}
}

func TestParseFileMaxLinesTruncates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exception.log")
	body := "java.lang.RuntimeException: a\n" +
		"    at Foo.bar(Foo.java:1)\n" +
		"java.lang.RuntimeException: b\n" +
		"    at Foo.baz(Foo.java:2)\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	records, diags, err := ParseFile(path, "", Options{MaxLines: 2})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if diags.TotalLines != 2 {
		t.Errorf("TotalLines = %d, want 2", diags.TotalLines)
	}
}

func TestParseFileRejectsUnsupportedFormat(t *testing.T) {
	_, _, err := ParseFile("/tmp/none.log", "javascript_stack", Options{})
	if err == nil || !strings.Contains(err.Error(), "Unsupported exception stack format") {
		t.Fatalf("expected unsupported-format error, got %v", err)
	}
}

func TestParseLineSingleHeaderOnly(t *testing.T) {
	rec, perr := ParseLine("java.lang.NullPointerException")
	if perr != nil {
		t.Fatalf("unexpected parse error: %+v", perr)
	}
	if rec.ExceptionType != "java.lang.NullPointerException" {
		t.Errorf("ExceptionType = %q", rec.ExceptionType)
	}
	if rec.Message != nil {
		t.Errorf("Message = %v, want nil for header-only line", rec.Message)
	}
	if rec.Signature != "java.lang.NullPointerException|(no-frame)" {
		t.Errorf("Signature = %q", rec.Signature)
	}
}

func TestParseLineRejectsCausedByPrefix(t *testing.T) {
	// "Caused by:" headers can only appear inside a block, never as
	// a standalone start. ParseLine must reject them — same as the
	// in-file logic that won't open a new block on these.
	_, perr := ParseLine("Caused by: java.lang.RuntimeException: x")
	if perr == nil || perr.Reason != ReasonNoExceptionHeader {
		t.Fatalf("expected NO_EXCEPTION_HEADER, got %+v", perr)
	}
}

func TestParseLineRejectsBlank(t *testing.T) {
	_, perr := ParseLine("   ")
	if perr == nil || perr.Reason != ReasonNoExceptionHeader {
		t.Fatalf("expected NO_EXCEPTION_HEADER for blank line, got %+v", perr)
	}
}

func TestParseLineExplicitEmptyMessage(t *testing.T) {
	// `Type:` (colon, no body) -> Python re.match captures group as ""
	// rather than None. Mirror that distinction.
	rec, perr := ParseLine("java.lang.RuntimeException:")
	if perr != nil {
		t.Fatalf("unexpected parse error: %+v", perr)
	}
	if rec.Message == nil {
		t.Fatalf("Message should be non-nil empty string for explicit `:`")
	}
	if *rec.Message != "" {
		t.Errorf("Message = %q, want empty string", *rec.Message)
	}
}

func TestParseBlockReturnsInvalidWhenHeaderMissing(t *testing.T) {
	_, perr := ParseBlock([]string{"not an exception", "    at Foo.bar(Foo.java:1)"})
	if perr == nil || perr.Reason != ReasonInvalidBlock {
		t.Fatalf("expected INVALID_EXCEPTION_BLOCK, got %+v", perr)
	}
}

func TestParseBlockMultipleCausedByKeepsLast(t *testing.T) {
	// Python: root_cause = caused_by[-1][0] — last cause wins.
	// ParseBlock's contract matches Python `_parse_exception_block`:
	// it expects already-stripped lines (the file walker hands stripped
	// lines in). Indented `at` frames have no leading whitespace here.
	block := []string{
		"java.lang.IllegalStateException: outer",
		"at Outer.boom(Outer.java:1)",
		"Caused by: java.io.IOException: middle",
		"at Middle.boom(Middle.java:2)",
		"Caused by: java.sql.SQLException: inner",
		"at Inner.boom(Inner.java:3)",
	}
	rec, perr := ParseBlock(block)
	if perr != nil {
		t.Fatalf("unexpected parse error: %+v", perr)
	}
	if rec.RootCause == nil || *rec.RootCause != "java.sql.SQLException" {
		t.Errorf("RootCause = %v, want java.sql.SQLException (deepest cause)", rec.RootCause)
	}
	if len(rec.Stack) != 3 {
		t.Errorf("Stack length = %d, want 3 (one per `at` line)", len(rec.Stack))
	}
}

func TestParseFileTimestampParsing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exception.log")
	body := "2026-04-27T10:00:00+09:00 java.lang.IllegalStateException: x\n" +
		"    at Foo.bar(Foo.java:1)\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	records, _, err := ParseFile(path, "", Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 1 || records[0].Timestamp == nil {
		t.Fatalf("expected one record with a parsed timestamp, got %+v", records)
	}
	got := records[0].Timestamp.Format("2006-01-02T15:04:05Z07:00")
	if got != "2026-04-27T10:00:00+09:00" {
		t.Errorf("Timestamp.Format = %q, want %q", got, "2026-04-27T10:00:00+09:00")
	}
}
