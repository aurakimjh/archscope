package runtimestack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const dotnetIisFixture = `System.NullReferenceException: Object reference not set to an instance of an object.
   at DemoSite.OrderController.Get(String id) in C:\demo-site\Controllers\OrderController.cs:line 37
   at DemoSite.Middleware.CorrelationMiddleware.Invoke(HttpContext context)

#Software: Microsoft Internet Information Services 10.0
#Fields: date time cs-method cs-uri-stem sc-status time-taken
2026-04-27 10:00:01 GET /api/orders/1001 200 123
2026-04-27 10:00:03 POST /api/payment 500 2300
2026-04-27 10:01:12 GET /api/orders/1003 404 340
`

func writeTempFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestParseDotnetFileFixtureHappyPath(t *testing.T) {
	path := writeTempFile(t, "dotnet.txt", dotnetIisFixture)
	exceptions, iis, diags, err := ParseDotnetFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseDotnetFile: %v", err)
	}
	if len(exceptions) != 1 {
		t.Fatalf("exceptions = %d, want 1", len(exceptions))
	}
	exc := exceptions[0]
	if exc.Runtime != "dotnet" {
		t.Errorf("Runtime = %q, want dotnet", exc.Runtime)
	}
	if exc.RecordType != "System.NullReferenceException" {
		t.Errorf("RecordType = %q", exc.RecordType)
	}
	if exc.Headline != "System.NullReferenceException" {
		t.Errorf("Headline = %q", exc.Headline)
	}
	if exc.Message == nil || *exc.Message != "Object reference not set to an instance of an object." {
		t.Errorf("Message = %v", exc.Message)
	}
	if len(exc.Stack) != 2 {
		t.Fatalf("Stack len = %d, want 2", len(exc.Stack))
	}
	if !strings.HasPrefix(exc.Stack[0], "DemoSite.OrderController.Get") {
		t.Errorf("Stack[0] = %q", exc.Stack[0])
	}
	wantSig := "System.NullReferenceException|" + exc.Stack[0]
	if exc.Signature != wantSig {
		t.Errorf("Signature = %q, want %q", exc.Signature, wantSig)
	}

	if len(iis) != 3 {
		t.Fatalf("iis records = %d, want 3", len(iis))
	}
	if iis[0].Method != "GET" || iis[0].URI != "/api/orders/1001" || iis[0].Status != 200 {
		t.Errorf("iis[0] = %+v", iis[0])
	}
	if iis[0].TimeTakenMS == nil || *iis[0].TimeTakenMS != 123 {
		t.Errorf("iis[0].TimeTakenMS = %v", iis[0].TimeTakenMS)
	}
	if iis[1].Status != 500 {
		t.Errorf("iis[1].Status = %d, want 500", iis[1].Status)
	}

	// Diagnostics: 4 parsed (1 exception + 3 IIS), no skips.
	if diags.ParsedRecords != 4 {
		t.Errorf("ParsedRecords = %d, want 4", diags.ParsedRecords)
	}
	if diags.SkippedLines != 0 {
		t.Errorf("SkippedLines = %d, want 0", diags.SkippedLines)
	}
	if diags.Format != "dotnet_exception_iis" {
		t.Errorf("Format = %q", diags.Format)
	}
}

func TestParseDotnetFileFlagsUnsupportedLine(t *testing.T) {
	body := "garbage line that is neither exception nor IIS\n"
	path := writeTempFile(t, "bad.txt", body)
	_, _, diags, err := ParseDotnetFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseDotnetFile: %v", err)
	}
	if diags.SkippedByReason[ReasonUnsupportedDotnetOrIisLine] != 1 {
		t.Errorf("expected 1 UNSUPPORTED_DOTNET_OR_IIS_LINE, got %v", diags.SkippedByReason)
	}
}

func TestParseDotnetFileStrictModeFailsFast(t *testing.T) {
	body := "garbage\n" + dotnetIisFixture
	path := writeTempFile(t, "strict.txt", body)
	_, _, _, err := ParseDotnetFile(path, Options{Strict: true})
	if err == nil {
		t.Fatalf("expected strict-mode error")
	}
	if !strings.Contains(err.Error(), ReasonUnsupportedDotnetOrIisLine) {
		t.Errorf("error should mention reason, got %v", err)
	}
}

func TestParseDotnetFileExceptionWithoutMessage(t *testing.T) {
	body := "System.OperationCanceledException\n   at MyApp.Worker.Run()\n"
	path := writeTempFile(t, "noMsg.txt", body)
	exceptions, _, _, err := ParseDotnetFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseDotnetFile: %v", err)
	}
	if len(exceptions) != 1 {
		t.Fatalf("exceptions = %d, want 1", len(exceptions))
	}
	if exceptions[0].Message != nil {
		t.Errorf("Message should be nil for header without colon, got %q", *exceptions[0].Message)
	}
	if len(exceptions[0].Stack) != 1 || exceptions[0].Stack[0] != "MyApp.Worker.Run()" {
		t.Errorf("Stack = %v", exceptions[0].Stack)
	}
}

func TestParseDotnetFileEmptyWarns(t *testing.T) {
	path := writeTempFile(t, "empty.txt", "")
	_, _, diags, err := ParseDotnetFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseDotnetFile: %v", err)
	}
	if diags.WarningCount != 1 || len(diags.Warnings) == 0 || diags.Warnings[0].Reason != "EMPTY_FILE" {
		t.Errorf("expected EMPTY_FILE warning, got %+v", diags.Warnings)
	}
}

func TestParseIisLineRequiresMandatoryFields(t *testing.T) {
	// Field schema is missing sc-status entirely — record should be
	// skipped (returns false).
	fields := []string{"date", "time", "cs-method", "cs-uri-stem"}
	if _, ok := parseIisLine("2026-04-27 10:00:01 GET /a", fields); !ok {
		// length matches but sc-status missing → false.
		// This actually returns false because hasStatus=false.
	} else {
		t.Errorf("expected miss when sc-status missing in fields")
	}
	// time-taken is "-" → TimeTakenMS=nil but record still produced.
	fields2 := []string{"cs-method", "cs-uri-stem", "sc-status", "time-taken"}
	rec, ok := parseIisLine("GET /a 200 -", fields2)
	if !ok {
		t.Fatalf("expected record")
	}
	if rec.TimeTakenMS != nil {
		t.Errorf("TimeTakenMS should be nil for `-`, got %v", *rec.TimeTakenMS)
	}
}
