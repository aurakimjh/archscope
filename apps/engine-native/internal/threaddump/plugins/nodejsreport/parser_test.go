package nodejsreport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// canonicalReport mirrors `_REPORT` in the Python test suite. Keys and
// values are byte-identical so the assertion suite below stays a
// 1-for-1 port.
func canonicalReport() map[string]any {
	return map[string]any{
		"header": map[string]any{
			"event":         "JavaScript API",
			"trigger":       "writeReport",
			"filename":      "report.json",
			"nodejsVersion": "v20.10.0",
		},
		"javascriptStack": map[string]any{
			"message": "Diagnostic report on demand",
			"stack": []any{
				"    at handler (/app/server.js:42:5)",
				"    at Layer.handle [as handle_request] (/app/node_modules/express/lib/router/layer.js:95:5)",
				"    at next (/app/node_modules/express/lib/router/route.js:144:13)",
				"    at /app/server.js:88:3",
			},
		},
		"nativeStack": []any{
			map[string]any{"pc": "0xdead", "symbol": "uv_run"},
			map[string]any{"pc": "0xbeef", "symbol": "node::SpinEventLoop"},
		},
		"libuv": []any{
			map[string]any{"type": "tcp", "is_active": true},
			map[string]any{"type": "timer", "is_active": false},
		},
	}
}

func writeReport(t *testing.T, name string, payload any) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestCanParseRecognizesDiagnosticReportJSON(t *testing.T) {
	body, err := json.Marshal(canonicalReport())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	head := string(body)
	if len(head) > 4000 {
		head = head[:4000]
	}
	if !(DiagnosticReportPlugin{}).CanParse(head) {
		t.Fatalf("CanParse should accept canonical report head")
	}
}

func TestCanParseRejectsUnrelatedJSON(t *testing.T) {
	if (DiagnosticReportPlugin{}).CanParse(`{"foo": 1, "bar": 2}`) {
		t.Fatalf("CanParse must reject unrelated JSON")
	}
}

func TestParseExtractsJSMainAndNativePool(t *testing.T) {
	path := writeReport(t, "report.json", canonicalReport())
	bundle, err := (DiagnosticReportPlugin{}).Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bundle.Language != Language {
		t.Fatalf("Language = %q, want %q", bundle.Language, Language)
	}
	if bundle.SourceFormat != FormatIDDiagnosticReport {
		t.Fatalf("SourceFormat = %q", bundle.SourceFormat)
	}
	names := []string{}
	for _, snap := range bundle.Snapshots {
		names = append(names, snap.ThreadName)
	}
	if len(names) != 2 || names[0] != "main" || names[1] != "native" {
		t.Fatalf("thread names = %+v", names)
	}
	js := bundle.Snapshots[0]
	if got := js.StackFrames[0].Function; got != "handler" {
		t.Errorf("frame[0].function = %q", got)
	}
	if js.StackFrames[0].File == nil || *js.StackFrames[0].File != "/app/server.js" {
		t.Errorf("frame[0].file mismatch: %+v", js.StackFrames[0].File)
	}
	if js.StackFrames[0].Line == nil || *js.StackFrames[0].Line != 42 {
		t.Errorf("frame[0].line mismatch: %+v", js.StackFrames[0].Line)
	}
	if got := js.StackFrames[1].Function; got != "Layer.handle" {
		t.Errorf("alias not stripped: %q", got)
	}
	if js.State != models.ThreadStateNetworkWait {
		t.Errorf("state = %q, want NETWORK_WAIT", js.State)
	}
}

func TestNativeThreadMarkedAsNativeLanguage(t *testing.T) {
	path := writeReport(t, "report.json", canonicalReport())
	bundle, err := (DiagnosticReportPlugin{}).Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	native := bundle.Snapshots[1]
	if native.Language == nil || *native.Language != NativeLanguage {
		t.Fatalf("native.language = %+v", native.Language)
	}
	if native.StackFrames[0].Language == nil || *native.StackFrames[0].Language != NativeLanguage {
		t.Fatalf("native frame language mismatch: %+v", native.StackFrames[0].Language)
	}
	if native.StackFrames[0].Function != "uv_run" {
		t.Fatalf("native frame func = %q", native.StackFrames[0].Function)
	}
}

func TestLibuvHandlesRecordedInMetadata(t *testing.T) {
	report := canonicalReport()
	path := writeReport(t, "report.json", report)
	bundle, err := (DiagnosticReportPlugin{}).Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	libuv, ok := bundle.Metadata["libuv_handles"].([]any)
	if !ok {
		t.Fatalf("libuv_handles missing or wrong type: %T", bundle.Metadata["libuv_handles"])
	}
	if len(libuv) != 2 {
		t.Fatalf("libuv_handles len = %d", len(libuv))
	}
	first, ok := libuv[0].(map[string]any)
	if !ok {
		t.Fatalf("libuv_handles[0] type = %T", libuv[0])
	}
	if first["type"] != "tcp" {
		t.Fatalf("libuv_handles[0].type = %v", first["type"])
	}
	header, ok := bundle.Metadata["header"].(map[string]any)
	if !ok {
		t.Fatalf("header metadata wrong type: %T", bundle.Metadata["header"])
	}
	if header["nodejsVersion"] != "v20.10.0" {
		t.Fatalf("header.nodejsVersion = %v", header["nodejsVersion"])
	}
}

func TestNormalizeStripsExpressAlias(t *testing.T) {
	lang := Language
	file := "/x.js"
	line := 1
	frame := models.StackFrame{
		Function: "Layer.handle [as handle_request]",
		File:     &file,
		Line:     &line,
		Language: &lang,
	}
	cleaned := normalizeJSFrame(frame)
	if cleaned.Function != "Layer.handle" {
		t.Fatalf("Function = %q", cleaned.Function)
	}
}

func TestInvalidJSONReturnsEmptyBundle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	bundle, err := (DiagnosticReportPlugin{}).Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(bundle.Snapshots) != 0 {
		t.Fatalf("expected empty snapshots, got %d", len(bundle.Snapshots))
	}
	if bundle.Metadata["parse_error"] != "INVALID_NODEJS_REPORT_JSON" {
		t.Fatalf("parse_error = %v", bundle.Metadata["parse_error"])
	}
}

func TestAnonymousFrameLocationOnlyFallback(t *testing.T) {
	report := canonicalReport()
	path := writeReport(t, "report.json", report)
	bundle, err := (DiagnosticReportPlugin{}).Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	js := bundle.Snapshots[0]
	// Fourth frame is `at /app/server.js:88:3` — anonymous fallback.
	if len(js.StackFrames) < 4 {
		t.Fatalf("expected >=4 frames, got %d", len(js.StackFrames))
	}
	frame := js.StackFrames[3]
	if frame.Function != "<anonymous>" {
		t.Errorf("anon frame function = %q", frame.Function)
	}
	if frame.File == nil || *frame.File != "/app/server.js" {
		t.Errorf("anon frame file = %+v", frame.File)
	}
	if frame.Line == nil || *frame.Line != 88 {
		t.Errorf("anon frame line = %+v", frame.Line)
	}
}

func TestInferJSStateFileWaitWithoutNetwork(t *testing.T) {
	payload := map[string]any{
		"libuv": []any{
			map[string]any{"type": "timer", "is_active": true},
			map[string]any{"type": "tcp", "is_active": false},
		},
	}
	got := InferJSState(models.ThreadStateRunnable, payload)
	if got != models.ThreadStateIOWait {
		t.Fatalf("state = %q, want IO_WAIT", got)
	}
}

func TestInferJSStateNoLibuvKeepsInputState(t *testing.T) {
	if got := InferJSState(models.ThreadStateRunnable, map[string]any{}); got != models.ThreadStateRunnable {
		t.Fatalf("state = %q, want RUNNABLE", got)
	}
}

func TestInferJSStateAllInactiveKeepsInputState(t *testing.T) {
	payload := map[string]any{
		"libuv": []any{
			map[string]any{"type": "tcp", "is_active": false},
		},
	}
	if got := InferJSState(models.ThreadStateRunnable, payload); got != models.ThreadStateRunnable {
		t.Fatalf("state = %q, want RUNNABLE", got)
	}
}

func TestNativeStackMissingProducesNoSecondSnapshot(t *testing.T) {
	report := canonicalReport()
	delete(report, "nativeStack")
	path := writeReport(t, "report.json", report)
	bundle, err := (DiagnosticReportPlugin{}).Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(bundle.Snapshots) != 1 {
		t.Fatalf("expected only JS snapshot, got %d", len(bundle.Snapshots))
	}
	if bundle.Snapshots[0].ThreadName != "main" {
		t.Fatalf("thread = %q", bundle.Snapshots[0].ThreadName)
	}
}

func TestLibuvTruncatedToFifty(t *testing.T) {
	report := canonicalReport()
	libuv := make([]any, 0, 75)
	for i := 0; i < 75; i++ {
		libuv = append(libuv, map[string]any{"type": "tcp", "is_active": false})
	}
	report["libuv"] = libuv
	path := writeReport(t, "report.json", report)
	bundle, err := (DiagnosticReportPlugin{}).Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got, ok := bundle.Metadata["libuv_handles"].([]any)
	if !ok {
		t.Fatalf("libuv_handles type = %T", bundle.Metadata["libuv_handles"])
	}
	if len(got) != 50 {
		t.Fatalf("libuv_handles len = %d, want 50", len(got))
	}
}

func TestSampleTraceParsesMultipleSamples(t *testing.T) {
	body := `Sample #1
Error
    at outer (/app/a.js:10:5)
    at inner (/app/b.js:20:6)
Sample #2
Error
    at handler (/app/c.js:33:9)
`
	dir := t.TempDir()
	path := filepath.Join(dir, "samples.txt")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	bundle, err := (SampleTracePlugin{}).Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bundle.SourceFormat != FormatIDSampleTrace {
		t.Fatalf("SourceFormat = %q", bundle.SourceFormat)
	}
	if len(bundle.Snapshots) != 2 {
		t.Fatalf("snapshots = %d, want 2", len(bundle.Snapshots))
	}
	first := bundle.Snapshots[0]
	if first.ThreadName != "sample-1" {
		t.Errorf("thread name = %q", first.ThreadName)
	}
	if first.SnapshotID != "nodejs::0::sample-1" {
		t.Errorf("snapshot id = %q", first.SnapshotID)
	}
	if first.ThreadID == nil || *first.ThreadID != "1" {
		t.Errorf("thread id = %+v", first.ThreadID)
	}
	if first.State != models.ThreadStateRunnable {
		t.Errorf("state = %q", first.State)
	}
	if len(first.StackFrames) != 2 {
		t.Fatalf("frames = %d, want 2", len(first.StackFrames))
	}
	if first.StackFrames[0].Function != "outer" {
		t.Errorf("frame[0].func = %q", first.StackFrames[0].Function)
	}
	if first.Metadata["sample_index"] != 1 {
		t.Errorf("sample_index = %v", first.Metadata["sample_index"])
	}
	second := bundle.Snapshots[1]
	if second.SnapshotID != "nodejs::1::sample-2" {
		t.Errorf("snapshot id = %q", second.SnapshotID)
	}
	if len(second.StackFrames) != 1 {
		t.Errorf("frames = %d, want 1", len(second.StackFrames))
	}
}

func TestSampleTraceCanParse(t *testing.T) {
	if !(SampleTracePlugin{}).CanParse("Sample #1\nError\n  at foo (/x.js:1:2)\n") {
		t.Fatal("CanParse should accept Sample header")
	}
	if (SampleTracePlugin{}).CanParse(`{"header":{"event":"x"},"javascriptStack":{}}`) {
		t.Fatal("CanParse should reject diagnostic-report JSON")
	}
}

func TestSampleTraceFormatID(t *testing.T) {
	if (SampleTracePlugin{}).FormatID() != FormatIDSampleTrace {
		t.Fatalf("format id mismatch")
	}
	if (SampleTracePlugin{}).Language() != Language {
		t.Fatalf("language mismatch")
	}
}

func TestDiagnosticReportFormatID(t *testing.T) {
	if (DiagnosticReportPlugin{}).FormatID() != FormatIDDiagnosticReport {
		t.Fatalf("format id mismatch")
	}
	if (DiagnosticReportPlugin{}).Language() != Language {
		t.Fatalf("language mismatch")
	}
}

func TestNormalizeLeavesNonJSFramesAlone(t *testing.T) {
	other := "native"
	frame := models.StackFrame{
		Function: "uv_run [as native]",
		Language: &other,
	}
	cleaned := normalizeJSFrame(frame)
	if cleaned.Function != "uv_run [as native]" {
		t.Fatalf("non-JS frame should be untouched, got %q", cleaned.Function)
	}
}

func TestParseJSFrameDegradesToRawText(t *testing.T) {
	frame := parseJSFrame("anonymous junk that matches nothing")
	if frame.Function != "anonymous junk that matches nothing" {
		t.Fatalf("Function = %q", frame.Function)
	}
}
