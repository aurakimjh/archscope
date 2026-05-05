package profiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/korean"
)

func TestJenniferCSVReconstructsTreeWithVirtualRoot(t *testing.T) {
	sample := filepath.Join("..", "..", "..", "..", "examples", "profiler", "sample-jennifer-flame.csv")
	result, err := ParseJenniferFlamegraphCSV(sample)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if got, want := result.Root.Name, "All"; got != want {
		t.Fatalf("root name = %q, want %q", got, want)
	}
	if got, want := result.Root.Samples, 115; got != want {
		t.Fatalf("root samples = %d, want %d", got, want)
	}
	if got, want := len(result.Root.Children), 2; got != want {
		t.Fatalf("root children count = %d, want %d", got, want)
	}
	if got, want := result.Root.Children[0].Name, "com.example.checkout.CheckoutController.checkout"; got != want {
		t.Fatalf("first root child = %q, want %q", got, want)
	}
	if got, want := result.Root.Children[1].Name, "java.lang.Thread.sleep"; got != want {
		t.Fatalf("second root child = %q, want %q", got, want)
	}
	checkout := result.Root.Children[0]
	if len(checkout.Children) == 0 || checkout.Children[0].Name != "com.example.checkout.CheckoutService.placeOrder" {
		t.Fatalf("expected placeOrder under checkout; got %+v", checkout.Children)
	}
	if got, want := result.Diagnostics.ParsedRecords, 7; got != want {
		t.Fatalf("parsed_records = %d, want %d", got, want)
	}
}

func TestJenniferCSVReportsDuplicateKey(t *testing.T) {
	path := writeTempCSV(t, "key,parent_key,method_name,ratio,sample_count\n"+
		"root,,Root,100,10\n"+
		"root,,Duplicate,100,5\n")
	result, err := ParseJenniferFlamegraphCSV(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if got, want := result.Root.Samples, 10; got != want {
		t.Fatalf("root samples = %d, want %d (first row should win)", got, want)
	}
	if got := result.Diagnostics.SkippedByReason["DUPLICATE_KEY"]; got != 1 {
		t.Fatalf("DUPLICATE_KEY skip count = %d, want 1", got)
	}
	if got := result.Diagnostics.ErrorCount; got != 1 {
		t.Fatalf("error_count = %d, want 1", got)
	}
}

func TestJenniferCSVReportsMissingParentAsWarning(t *testing.T) {
	path := writeTempCSV(t, "key,parent_key,method_name,ratio,sample_count\n"+
		"child,missing,Child,100,5\n")
	result, err := ParseJenniferFlamegraphCSV(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if got, want := result.Root.Name, "Child"; got != want {
		t.Fatalf("root name = %q, want %q (missing parent should treat row as root)", got, want)
	}
	if got := result.Diagnostics.WarningCount; got < 1 {
		t.Fatalf("warning_count = %d, want >=1", got)
	}
	found := false
	for _, w := range result.Diagnostics.Warnings {
		if w.Reason == "MISSING_PARENT" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_PARENT warning; got %+v", result.Diagnostics.Warnings)
	}
}

func TestJenniferCSVBreaksParentCycles(t *testing.T) {
	path := writeTempCSV(t, "key,parent_key,method_name,ratio,sample_count\n"+
		"a,b,A,50,5\n"+
		"b,a,B,50,5\n")
	result, err := ParseJenniferFlamegraphCSV(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if got, want := result.Root.Name, "All"; got != want {
		t.Fatalf("root name = %q, want %q (cycle should produce virtual root)", got, want)
	}
	names := map[string]bool{}
	for _, child := range result.Root.Children {
		names[child.Name] = true
	}
	if !names["A"] || !names["B"] {
		t.Fatalf("expected A and B as roots after cycle break; got %v", names)
	}
	if len(result.Diagnostics.Errors) == 0 || result.Diagnostics.Errors[0].Reason != "PARENT_CYCLE" {
		t.Fatalf("expected first error to be PARENT_CYCLE; got %+v", result.Diagnostics.Errors)
	}
}

func TestJenniferCSVMissingRequiredColumns(t *testing.T) {
	path := writeTempCSV(t, "key,parent_key,method_name\nroot,,Root\n")
	result, err := ParseJenniferFlamegraphCSV(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if got := result.Root.Samples; got != 0 {
		t.Fatalf("root samples = %d, want 0 when required columns are missing", got)
	}
	if len(result.Diagnostics.Errors) == 0 || result.Diagnostics.Errors[0].Reason != "MISSING_REQUIRED_COLUMNS" {
		t.Fatalf("expected MISSING_REQUIRED_COLUMNS error; got %+v", result.Diagnostics.Errors)
	}
}

func TestJenniferCSVCp949Fallback(t *testing.T) {
	encoder := korean.EUCKR.NewEncoder()
	encoded, err := encoder.Bytes([]byte("key,parent_key,method_name,ratio,sample_count\nroot,,서비스,100,7\n"))
	if err != nil {
		t.Fatalf("encode cp949: %v", err)
	}
	path := filepath.Join(t.TempDir(), "jennifer-cp949.csv")
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := ParseJenniferFlamegraphCSV(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if got, want := result.Root.Name, "서비스"; got != want {
		t.Fatalf("root name = %q, want %q", got, want)
	}
	if got, want := result.Root.Samples, 7; got != want {
		t.Fatalf("root samples = %d, want %d", got, want)
	}
	if result.Diagnostics.WarningCount < 1 {
		t.Fatalf("expected ENCODING_FALLBACK warning, got 0")
	}
}

func TestJenniferCSVUTF8WithBOM(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	body := append(append([]byte(nil), bom...),
		[]byte("key,parent_key,method_name,ratio,sample_count\nroot,,Root,100,3\n")...)
	path := filepath.Join(t.TempDir(), "jennifer-bom.csv")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := ParseJenniferFlamegraphCSV(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if got, want := result.Root.Samples, 3; got != want {
		t.Fatalf("root samples = %d, want %d", got, want)
	}
	for _, w := range result.Diagnostics.Warnings {
		if w.Reason == "ENCODING_FALLBACK" {
			t.Fatalf("BOM should be detected as utf-8-sig with no fallback warning; got %+v", result.Diagnostics.Warnings)
		}
	}
}

func TestJenniferCSVRatioPercentSuffix(t *testing.T) {
	path := writeTempCSV(t, "key,parent_key,method_name,ratio,sample_count\n"+
		"root,,Root,100%,12\n"+
		"child,root,Child,75 %,9\n")
	result, err := ParseJenniferFlamegraphCSV(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if got, want := result.Root.Samples, 12; got != want {
		t.Fatalf("root samples = %d, want %d", got, want)
	}
	if got := result.Diagnostics.ParsedRecords; got != 2 {
		t.Fatalf("parsed_records = %d, want 2", got)
	}
}

func TestJenniferCSVEmptyFile(t *testing.T) {
	path := writeTempCSV(t, "")
	result, err := ParseJenniferFlamegraphCSV(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	found := false
	for _, w := range result.Diagnostics.Warnings {
		if w.Reason == "EMPTY_FILE" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected EMPTY_FILE warning; got %+v", result.Diagnostics.Warnings)
	}
}

func TestJenniferCSVColumnAliasResolution(t *testing.T) {
	path := writeTempCSV(t, "id,parent,method,percent,samples,category\n"+
		"root,,Root,100,8,application\n")
	result, err := ParseJenniferFlamegraphCSV(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if got, want := result.Root.Name, "Root"; got != want {
		t.Fatalf("root name = %q, want %q", got, want)
	}
	if got, want := result.Root.Samples, 8; got != want {
		t.Fatalf("root samples = %d, want %d", got, want)
	}
	if result.Root.Category == nil || *result.Root.Category != "application" {
		t.Fatalf("expected category=application; got %v", result.Root.Category)
	}
}

func TestJenniferCSVChildSamplesExceedParentWarning(t *testing.T) {
	path := writeTempCSV(t, "key,parent_key,method_name,ratio,sample_count\n"+
		"a,,Parent,100,5\n"+
		"b,a,ChildOne,50,4\n"+
		"c,a,ChildTwo,50,4\n")
	result, err := ParseJenniferFlamegraphCSV(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	found := false
	for _, w := range result.Diagnostics.Warnings {
		if w.Reason == "CHILD_SAMPLES_EXCEED_PARENT" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected CHILD_SAMPLES_EXCEED_PARENT warning; got %+v", result.Diagnostics.Warnings)
	}
}

func TestAnalyzeJenniferFileProducesAnalysisResult(t *testing.T) {
	sample := filepath.Join("..", "..", "..", "..", "examples", "profiler", "sample-jennifer-flame.csv")
	elapsed := 11.5
	result, err := AnalyzeJenniferFile(sample, Options{
		IntervalMS:  100,
		TopN:        20,
		ProfileKind: "wall",
		ElapsedSec:  &elapsed,
	})
	if err != nil {
		t.Fatalf("AnalyzeJenniferFile: %v", err)
	}
	if got, want := result.Type, "profiler_jennifer"; got != want {
		t.Fatalf("type = %q, want %q", got, want)
	}
	if got, want := result.Metadata.Parser, "jennifer_flamegraph_csv"; got != want {
		t.Fatalf("parser = %q, want %q", got, want)
	}
	if got, want := result.Summary.TotalSamples, 115; got != want {
		t.Fatalf("total_samples = %d, want %d", got, want)
	}
	if len(result.Series.TopStacks) == 0 {
		t.Fatalf("expected non-empty top_stacks")
	}
	topStack := result.Series.TopStacks[0]
	if !strings.Contains(topStack.Stack, "OraclePreparedStatement.executeQuery") {
		t.Fatalf("expected SQL executeQuery to be top stack; got %q", topStack.Stack)
	}
	if topStack.Samples != 45 {
		t.Fatalf("top stack samples = %d, want 45", topStack.Samples)
	}
}

func writeTempCSV(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jennifer.csv")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	return path
}
