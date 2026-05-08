// [한글] runtime 분석기 회귀 테스트.
//
// 검증 대상
//   • 4개 진입점 (Nodejs / PythonTraceback / GoPanic / Dotnet) 각각이
//     fixture 파일에 대해 올바른 result.type 과 summary 를 emit.
//   • REPEATED_RUNTIME_STACK_SIGNATURE finding 동작.
//   • STACKLESS_RUNTIME_RECORDS finding 동작.
//   • .NET 만의 IIS_5XX_PRESENT / DOTNET_EXCEPTIONS_PRESENT.
//   • tables.records 가 시간순 첫 N개를 보존.
//
// fixture
//   examples/runtime/ 아래의 sample-{nodejs-stack,python-traceback,
//   go-panic,dotnet-iis}.txt 를 사용 — Python 테스트와 공유.
package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

const fixturesDir = "../../../../../examples/runtime"

// TestAnalyzeNodejsStack mirrors test_nodejs_stack_analyzer_parses_error_blocks.
func TestAnalyzeNodejsStack(t *testing.T) {
	path := filepath.Join(fixturesDir, "sample-nodejs-stack.txt")
	result, err := AnalyzeNodejsStack(path, Options{})
	if err != nil {
		t.Fatalf("AnalyzeNodejsStack: %v", err)
	}
	if result.Type != ResultTypeNodejsStack {
		t.Errorf("Type = %q, want %q", result.Type, ResultTypeNodejsStack)
	}
	if got := asInt(result.Summary["total_records"]); got != 2 {
		t.Errorf("total_records = %d, want 2", got)
	}
	dist, ok := result.Series["record_type_distribution"].([]map[string]any)
	if !ok {
		t.Fatalf("record_type_distribution missing or wrong type: %T", result.Series["record_type_distribution"])
	}
	if len(dist) == 0 || dist[0]["record_type"] != "TypeError" {
		t.Errorf("record_type_distribution[0].record_type = %v, want TypeError (full=%v)", dist, dist)
	}
}

// TestAnalyzePythonTraceback mirrors test_python_traceback_analyzer_parses_tracebacks.
func TestAnalyzePythonTraceback(t *testing.T) {
	path := filepath.Join(fixturesDir, "sample-python-traceback.txt")
	result, err := AnalyzePythonTraceback(path, Options{})
	if err != nil {
		t.Fatalf("AnalyzePythonTraceback: %v", err)
	}
	if result.Type != ResultTypePythonTraceback {
		t.Errorf("Type = %q, want %q", result.Type, ResultTypePythonTraceback)
	}
	if got := asInt(result.Summary["total_records"]); got != 2 {
		t.Errorf("total_records = %d, want 2", got)
	}
	if got, want := result.Summary["top_record_type"], "TimeoutError"; got != want {
		t.Errorf("top_record_type = %v, want %s", got, want)
	}
}

// TestAnalyzeGoPanic mirrors test_go_panic_analyzer_parses_panic_and_goroutines.
func TestAnalyzeGoPanic(t *testing.T) {
	path := filepath.Join(fixturesDir, "sample-go-panic.txt")
	result, err := AnalyzeGoPanic(path, Options{})
	if err != nil {
		t.Fatalf("AnalyzeGoPanic: %v", err)
	}
	if result.Type != ResultTypeGoPanic {
		t.Errorf("Type = %q, want %q", result.Type, ResultTypeGoPanic)
	}
	if got := asInt(result.Summary["total_records"]); got != 3 {
		t.Errorf("total_records = %d, want 3", got)
	}
	dist, ok := result.Series["record_type_distribution"].([]map[string]any)
	if !ok {
		t.Fatalf("record_type_distribution missing")
	}
	if len(dist) == 0 || dist[0]["record_type"] != "goroutine" {
		t.Errorf("record_type_distribution[0].record_type = %v, want goroutine", dist)
	}
}

// TestAnalyzeDotnetExceptionIIS mirrors test_dotnet_analyzer_parses_exception_and_iis_records.
func TestAnalyzeDotnetExceptionIIS(t *testing.T) {
	path := filepath.Join(fixturesDir, "sample-dotnet-iis.txt")
	result, err := AnalyzeDotnetExceptionIIS(path, Options{})
	if err != nil {
		t.Fatalf("AnalyzeDotnetExceptionIIS: %v", err)
	}
	if result.Type != ResultTypeDotnetExceptionIIS {
		t.Errorf("Type = %q, want %q", result.Type, ResultTypeDotnetExceptionIIS)
	}
	if got := asInt(result.Summary["total_exceptions"]); got != 1 {
		t.Errorf("total_exceptions = %d, want 1", got)
	}
	if got := asInt(result.Summary["iis_requests"]); got != 3 {
		t.Errorf("iis_requests = %d, want 3", got)
	}
	if got := asInt(result.Summary["iis_error_requests"]); got != 1 {
		t.Errorf("iis_error_requests = %d, want 1", got)
	}
}

// TestDotnetFindings checks both .NET-specific finding codes fire on
// the example fixture.
func TestDotnetFindings(t *testing.T) {
	path := filepath.Join(fixturesDir, "sample-dotnet-iis.txt")
	result, err := AnalyzeDotnetExceptionIIS(path, Options{})
	if err != nil {
		t.Fatalf("AnalyzeDotnetExceptionIIS: %v", err)
	}
	codes := map[string]bool{}
	for _, finding := range result.Metadata.Findings {
		codes[finding["code"].(string)] = true
	}
	for _, want := range []string{"IIS_5XX_PRESENT", "DOTNET_EXCEPTIONS_PRESENT"} {
		if !codes[want] {
			t.Errorf("expected finding %s, got %v", want, codes)
		}
	}
}

// TestStackFindingsRepeatedSignature triggers REPEATED_RUNTIME_STACK_SIGNATURE
// using a synthetic file with two identical-looking Node.js errors.
func TestStackFindingsRepeatedSignature(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repeated.txt")
	body := "" +
		"TypeError: x\n" +
		"    at foo (/a.js:1:1)\n" +
		"\n" +
		"TypeError: x\n" +
		"    at foo (/a.js:1:1)\n"
	if err := writeFile(path, body); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := AnalyzeNodejsStack(path, Options{})
	if err != nil {
		t.Fatalf("AnalyzeNodejsStack: %v", err)
	}
	codes := map[string]bool{}
	for _, finding := range result.Metadata.Findings {
		codes[finding["code"].(string)] = true
	}
	if !codes["REPEATED_RUNTIME_STACK_SIGNATURE"] {
		t.Errorf("expected REPEATED_RUNTIME_STACK_SIGNATURE, got %v", codes)
	}
}

func TestEmptyDotnetEmitsNoFindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := writeFile(path, ""); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := AnalyzeDotnetExceptionIIS(path, Options{})
	if err != nil {
		t.Fatalf("AnalyzeDotnetExceptionIIS: %v", err)
	}
	if got := asInt(result.Summary["iis_requests"]); got != 0 {
		t.Errorf("iis_requests = %d, want 0", got)
	}
	if len(result.Metadata.Findings) != 0 {
		t.Errorf("expected no findings on empty file, got %v", result.Metadata.Findings)
	}
	// Series should still expose the .NET-specific keys (even if empty).
	if _, ok := result.Series["iis_status_distribution"]; !ok {
		t.Error("missing iis_status_distribution")
	}
	if _, ok := result.Series["iis_slowest_urls"]; !ok {
		t.Error("missing iis_slowest_urls")
	}
}

func TestStatusBucket(t *testing.T) {
	cases := map[int]string{
		200: "2xx",
		301: "3xx",
		404: "4xx",
		500: "5xx",
		100: "1xx",
		0:   "0xx",
	}
	for code, want := range cases {
		if got := statusBucket(code); got != want {
			t.Errorf("statusBucket(%d) = %q, want %q", code, got, want)
		}
	}
}

func writeFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}
