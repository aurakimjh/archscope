package exception

import (
	"os"
	"path/filepath"
	"testing"
)

// Mirrors test_jvm_analyzers.test_exception_parser_and_analyzer in
// the Python suite — two repeated IllegalStateException blocks, one
// of which has a SQLTimeoutException root cause.
func writeRepeatedException(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "exception.log")
	body := "" +
		"2026-04-27T10:00:00+09:00 java.lang.IllegalStateException: failed\n" +
		"    at com.example.OrderController.create(OrderController.java:15)\n" +
		"Caused by: java.sql.SQLTimeoutException: timeout\n" +
		"    at oracle.jdbc.driver.T4CPreparedStatement.executeQuery(T4CPreparedStatement.java:1)\n" +
		"java.lang.IllegalStateException: failed\n" +
		"    at com.example.OrderController.create(OrderController.java:15)\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestAnalyzeRepeatedException(t *testing.T) {
	path := writeRepeatedException(t)

	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.Type != ResultType {
		t.Fatalf("Type = %q, want %q", result.Type, ResultType)
	}
	if got := asInt(result.Summary["total_exceptions"]); got != 2 {
		t.Errorf("total_exceptions = %d, want 2", got)
	}
	if got := asInt(result.Summary["unique_signatures"]); got != 1 {
		t.Errorf("unique_signatures = %d, want 1", got)
	}
	if got := asInt(result.Summary["unique_exception_types"]); got != 1 {
		t.Errorf("unique_exception_types = %d, want 1", got)
	}
	if got, want := result.Summary["top_exception_type"], "java.lang.IllegalStateException"; got != want {
		t.Errorf("top_exception_type = %v, want %s", got, want)
	}

	codes := map[string]bool{}
	for _, finding := range result.Metadata.Findings {
		codes[finding["code"].(string)] = true
	}
	for _, want := range []string{"REPEATED_EXCEPTION_SIGNATURE", "ROOT_CAUSE_PRESENT"} {
		if !codes[want] {
			t.Errorf("expected finding %s, got %v", want, codes)
		}
	}
	if len(codes) != 2 {
		t.Errorf("expected exactly 2 findings, got %v", codes)
	}
}

func TestAnalyzeNoRepeats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "single.log")
	body := "java.lang.IllegalStateException: failed\n" +
		"    at com.example.OrderController.create(OrderController.java:15)\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := asInt(result.Summary["total_exceptions"]); got != 1 {
		t.Errorf("total_exceptions = %d, want 1", got)
	}
	codes := map[string]bool{}
	for _, finding := range result.Metadata.Findings {
		codes[finding["code"].(string)] = true
	}
	if codes["REPEATED_EXCEPTION_SIGNATURE"] {
		t.Errorf("did not expect REPEATED_EXCEPTION_SIGNATURE for single record")
	}
	if codes["ROOT_CAUSE_PRESENT"] {
		t.Errorf("did not expect ROOT_CAUSE_PRESENT (no Caused by chain)")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := asInt(result.Summary["total_exceptions"]); got != 0 {
		t.Errorf("total_exceptions = %d, want 0", got)
	}
	if got := asInt(result.Summary["unique_signatures"]); got != 0 {
		t.Errorf("unique_signatures = %d, want 0", got)
	}
	if result.Summary["top_exception_type"] != nil {
		t.Errorf("top_exception_type = %v, want nil", result.Summary["top_exception_type"])
	}
	if len(result.Metadata.Findings) != 0 {
		t.Errorf("expected no findings on empty input, got %v", result.Metadata.Findings)
	}
}

func TestAnalyzeSeriesShape(t *testing.T) {
	path := writeRepeatedException(t)
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	typeDist, ok := result.Series["exception_type_distribution"].([]map[string]any)
	if !ok {
		t.Fatalf("exception_type_distribution missing or wrong type: %T", result.Series["exception_type_distribution"])
	}
	if len(typeDist) != 1 {
		t.Fatalf("expected 1 distribution row, got %d", len(typeDist))
	}
	if typeDist[0]["exception_type"] != "java.lang.IllegalStateException" {
		t.Errorf("exception_type = %v", typeDist[0]["exception_type"])
	}
	if asInt(typeDist[0]["count"]) != 2 {
		t.Errorf("count = %v, want 2", typeDist[0]["count"])
	}

	rootDist, ok := result.Series["root_cause_distribution"].([]map[string]any)
	if !ok {
		t.Fatalf("root_cause_distribution missing")
	}
	// Two records: one root_cause = SQLTimeoutException, one with no
	// root_cause (falls back to ExceptionType=IllegalStateException).
	if len(rootDist) != 2 {
		t.Fatalf("expected 2 root rows, got %d: %v", len(rootDist), rootDist)
	}

	signatures, ok := result.Series["top_stack_signatures"].([]map[string]any)
	if !ok {
		t.Fatalf("top_stack_signatures missing")
	}
	if len(signatures) != 1 {
		t.Fatalf("expected 1 signature row, got %d", len(signatures))
	}
	if asInt(signatures[0]["count"]) != 2 {
		t.Errorf("signature count = %v, want 2", signatures[0]["count"])
	}
}

func TestAnalyzeTopNCapsTables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "many.log")
	// Three distinct exception blocks.
	body := "" +
		"java.lang.IllegalStateException: a\n" +
		"    at com.example.A.run(A.java:1)\n" +
		"java.lang.RuntimeException: b\n" +
		"    at com.example.B.run(B.java:2)\n" +
		"java.lang.Error: c\n" +
		"    at com.example.C.run(C.java:3)\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := Analyze(path, Options{TopN: 2})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := asInt(result.Summary["total_exceptions"]); got != 3 {
		t.Errorf("total_exceptions = %d, want 3", got)
	}
	rows, ok := result.Tables["exceptions"].([]map[string]any)
	if !ok {
		t.Fatalf("exceptions table missing")
	}
	if len(rows) != 2 {
		t.Errorf("table rows = %d, want 2 (TopN cap)", len(rows))
	}
}

func TestAnalyzeMissingFile(t *testing.T) {
	_, err := Analyze("/nonexistent/path.log", Options{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}
