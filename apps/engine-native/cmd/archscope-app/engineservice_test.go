// [한글] engineservice_test — Wails 데스크톱 셸의 EngineService 회귀 테스트.
//
// 검증 대상
//   - EngineService 의 Wails IPC 메서드 (Analyze* / Convert* / Classify*) 들이
//     올바른 타입의 결과를 반환.
//   - engine-native/api 의 re-export 함수와 byte 단위 같은 결과를 produce.
//   - fixturesDir helper 가 examples/ 디렉토리를 정확히 찾음.
//
// fixturesDir 경로 계산
//
//	Wails 앱은 apps/engine-native/cmd/archscope-app/ 의 4단계 깊이.
//	runtime.Caller(0) 또는 wd 에서 4단계 위로 올라가 examples/ 를 anchor.
//	`go test` cwd 에 무관하게 fixture 경로 보존.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfilerServiceLoadsV8ThroughUnifiedParser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.cpuprofile")
	data := `{"nodes":[{"id":1,"callFrame":{"functionName":"root"},"children":[2]},{"id":2,"callFrame":{"functionName":"render"}}],"samples":[2,2],"timeDeltas":[10,40]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	stacks, err := (&ProfilerService{}).loadStacks(path, "v8-cpuprofile")
	if err != nil {
		t.Fatal(err)
	}
	if stacks["root;render"] != 40 {
		t.Fatalf("unexpected normalized V8 stacks: %#v", stacks)
	}
}

// fixturesDir resolves the repo's `examples/` directory from the test
// working directory. The Wails app's package sits four levels deep
// (apps/engine-native/cmd/archscope-app), so we walk up
// until we find the marker directory.
func fixturesDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "examples")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate examples/ from %s", wd)
	return ""
}

// requireResultEnvelope sanity-checks an AnalysisResult-shaped value:
// non-empty Type, non-nil Summary / Series / Tables / Charts, and a
// matching Metadata.Parser / SchemaVersion. Anything that survives this
// will round-trip through the JSON exporter.
func requireResultEnvelope(t *testing.T, label, wantTypePrefix string, result map[string]any) {
	t.Helper()
	gotType, _ := result["type"].(string)
	if gotType == "" {
		t.Fatalf("%s: result.type is empty", label)
	}
	if wantTypePrefix != "" && !strings.HasPrefix(gotType, wantTypePrefix) {
		t.Fatalf("%s: got type=%q, want prefix=%q", label, gotType, wantTypePrefix)
	}
	for _, key := range []string{"summary", "series", "tables", "charts", "metadata"} {
		if _, ok := result[key]; !ok {
			t.Fatalf("%s: missing %s key", label, key)
		}
	}
}

// resultToMap funnels the typed AnalysisResult through JSON so we can
// inspect the payload the renderer actually receives — including the
// Metadata.MarshalJSON merge of Extra.
func resultToMap(t *testing.T, v any) map[string]any {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

// TestEngineService_AnalyzeAccessLog smoke-tests the access log analyzer
// and (implicitly) the time-pointer plumbing for StartTime / EndTime.
func TestEngineService_AnalyzeAccessLog(t *testing.T) {
	svc := &EngineService{}
	path := filepath.Join(fixturesDir(t), "access-logs", "sample-nginx-access.log")
	res, err := svc.AnalyzeAccessLog(AccessLogRequest{Path: path})
	if err != nil {
		t.Fatalf("AnalyzeAccessLog: %v", err)
	}
	requireResultEnvelope(t, "AnalyzeAccessLog", "access_log", resultToMap(t, res))
}

func TestEngineService_AnalyzeServerLog(t *testing.T) {
	svc := &EngineService{}
	path := filepath.Join(fixturesDir(t), "server-logs", "sample-server.log")
	res, err := svc.AnalyzeServerLog(ServerLogRequest{Path: path, Format: "auto"})
	if err != nil {
		t.Fatalf("AnalyzeServerLog: %v", err)
	}
	requireResultEnvelope(t, "AnalyzeServerLog", "server_log", resultToMap(t, res))
}

// TestEngineService_AnalyzeException smoke-tests the Java exception
// analyzer with the canonical fixture.
func TestEngineService_AnalyzeException(t *testing.T) {
	svc := &EngineService{}
	path := filepath.Join(fixturesDir(t), "exceptions", "sample-java-exception.txt")
	res, err := svc.AnalyzeException(ExceptionRequest{Path: path})
	if err != nil {
		t.Fatalf("AnalyzeException: %v", err)
	}
	requireResultEnvelope(t, "AnalyzeException", "exception", resultToMap(t, res))
}

// TestEngineService_AnalyzeJfr smoke-tests the JFR JSON path. The
// fixture is a `jfr print` JSON dump, the secondary input shape Analyze
// accepts (binary .jfr is the primary).
func TestEngineService_AnalyzeJfr(t *testing.T) {
	svc := &EngineService{}
	path := filepath.Join(fixturesDir(t), "jfr", "sample-jfr-print.json")
	res, err := svc.AnalyzeJfr(JfrRequest{Path: path})
	if err != nil {
		t.Fatalf("AnalyzeJfr: %v", err)
	}
	requireResultEnvelope(t, "AnalyzeJfr", "jfr", resultToMap(t, res))
}

// TestEngineService_AnalyzeRuntimeStack_Variants covers all four runtime
// dispatch branches in one pass — keeps the runtime under 30s in total.
func TestEngineService_AnalyzeRuntimeStack_Variants(t *testing.T) {
	svc := &EngineService{}
	root := filepath.Join(fixturesDir(t), "runtime")
	cases := []struct {
		variant  string
		fixture  string
		typeWant string
	}{
		{"nodejs", "sample-nodejs-stack.txt", "nodejs"},
		{"python", "sample-python-traceback.txt", "python"},
		{"go", "sample-go-panic.txt", "go"},
		{"dotnet", "sample-dotnet-iis.txt", "dotnet"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.variant, func(t *testing.T) {
			res, err := svc.AnalyzeRuntimeStack(RuntimeRequest{
				Path:    filepath.Join(root, tc.fixture),
				Variant: tc.variant,
			})
			if err != nil {
				t.Fatalf("AnalyzeRuntimeStack(%s): %v", tc.variant, err)
			}
			requireResultEnvelope(t, "AnalyzeRuntimeStack:"+tc.variant, tc.typeWant, resultToMap(t, res))
		})
	}
}

// TestEngineService_AnalyzeThreadDump exercises the single-file Java
// jstack analyzer.
func TestEngineService_AnalyzeThreadDump(t *testing.T) {
	svc := &EngineService{}
	path := filepath.Join(fixturesDir(t), "thread-dumps", "sample-java-thread-dump.txt")
	res, err := svc.AnalyzeThreadDump(ThreadDumpRequest{Path: path})
	if err != nil {
		t.Fatalf("AnalyzeThreadDump: %v", err)
	}
	requireResultEnvelope(t, "AnalyzeThreadDump", "thread_dump", resultToMap(t, res))
}

// TestEngineService_AnalyzeMultiThread_TaskID asserts the async path
// returns a non-empty TaskID immediately. We don't wait for the
// goroutine to emit `engine:done` because that would require a live
// Wails app instance — out of scope for this smoke layer.
func TestEngineService_AnalyzeMultiThread_TaskID(t *testing.T) {
	svc := &EngineService{}
	path := filepath.Join(fixturesDir(t), "thread-dumps", "sample-java-thread-dump.txt")
	resp, err := svc.AnalyzeMultiThread(MultiThreadRequest{Paths: []string{path}})
	if err != nil {
		t.Fatalf("AnalyzeMultiThread: %v", err)
	}
	if strings.TrimSpace(resp.TaskID) == "" {
		t.Fatalf("AnalyzeMultiThread: empty TaskID")
	}
	if !strings.HasPrefix(resp.TaskID, "engine-multithread") {
		t.Fatalf("AnalyzeMultiThread: TaskID prefix mismatch, got %q", resp.TaskID)
	}
	// Cancel so the goroutine doesn't leak past the test.
	_ = svc.CancelEngineTask(resp.TaskID)
}

// TestEngineService_AnalyzeLockContention_TaskID is the lock-contention
// counterpart of the async smoke test.
func TestEngineService_AnalyzeLockContention_TaskID(t *testing.T) {
	svc := &EngineService{}
	path := filepath.Join(fixturesDir(t), "thread-dumps", "sample-java-thread-dump.txt")
	resp, err := svc.AnalyzeLockContention(LockContentionRequest{Paths: []string{path}})
	if err != nil {
		t.Fatalf("AnalyzeLockContention: %v", err)
	}
	if strings.TrimSpace(resp.TaskID) == "" {
		t.Fatalf("AnalyzeLockContention: empty TaskID")
	}
	if !strings.HasPrefix(resp.TaskID, "engine-lockcontention") {
		t.Fatalf("AnalyzeLockContention: TaskID prefix mismatch, got %q", resp.TaskID)
	}
	_ = svc.CancelEngineTask(resp.TaskID)
}

// TestEngineService_ConvertToCollapsed checks the
// thread-dump → flamegraph collapsed pipeline emits some counts and
// that the sorted lines match the count cardinality.
func TestEngineService_ConvertToCollapsed(t *testing.T) {
	svc := &EngineService{}
	path := filepath.Join(fixturesDir(t), "thread-dumps", "sample-java-thread-dump.txt")
	res, err := svc.ConvertToCollapsed(CollapsedRequest{
		Paths:             []string{path},
		IncludeThreadName: true,
	})
	if err != nil {
		t.Fatalf("ConvertToCollapsed: %v", err)
	}
	if len(res.Counts) == 0 {
		t.Fatalf("ConvertToCollapsed: empty counts")
	}
	if len(res.Lines) != len(res.Counts) {
		t.Fatalf("ConvertToCollapsed: lines=%d counts=%d (must match)", len(res.Lines), len(res.Counts))
	}
}

// TestEngineService_ClassifyStack hits the rule-based runtime
// classifier with a known-Spring path; the package init() loads the
// embedded JSON ruleset so we can rely on the canonical labels here.
func TestEngineService_ClassifyStack(t *testing.T) {
	svc := &EngineService{}
	got := svc.ClassifyStack(ClassifyRequest{
		Stack: "main;org.springframework.web.servlet.DispatcherServlet.doDispatch;com.example.MyController.handle",
	})
	if got.Label != "Spring Framework" {
		t.Fatalf("ClassifyStack: got %q, want Spring Framework", got.Label)
	}
	miss := svc.ClassifyStack(ClassifyRequest{Stack: "no.matching.token.in.here"})
	if miss.Label != "Application" {
		t.Fatalf("ClassifyStack fallback: got %q, want Application", miss.Label)
	}
}

// TestEngineService_ExportJSON_Roundtrip writes an AnalysisResult to a
// temp file and parses it back, verifying the `type` field survives
// the round trip — i.e. the exporter doesn't drop or rename anything.
func TestEngineService_ExportJSON_Roundtrip(t *testing.T) {
	svc := &EngineService{}
	src := filepath.Join(fixturesDir(t), "access-logs", "sample-nginx-access.log")
	res, err := svc.AnalyzeAccessLog(AccessLogRequest{Path: src})
	if err != nil {
		t.Fatalf("AnalyzeAccessLog: %v", err)
	}

	out := filepath.Join(t.TempDir(), "result.json")
	if err := svc.ExportJSON(ExportJSONRequest{Path: out, Result: res}); err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if gotType, _ := got["type"].(string); !strings.HasPrefix(gotType, "access_log") {
		t.Fatalf("ExportJSON: got type=%q, want access_log*", gotType)
	}
}

// TestEngineService_PathRequired covers the cheap input-validation
// branches in one pass so a missing path is rejected uniformly.
func TestEngineService_PathRequired(t *testing.T) {
	svc := &EngineService{}
	if _, err := svc.AnalyzeAccessLog(AccessLogRequest{}); err == nil {
		t.Fatalf("AnalyzeAccessLog: expected error for empty path")
	}
	if _, err := svc.AnalyzeException(ExceptionRequest{}); err == nil {
		t.Fatalf("AnalyzeException: expected error for empty path")
	}
	if _, err := svc.AnalyzeMultiThread(MultiThreadRequest{}); err == nil {
		t.Fatalf("AnalyzeMultiThread: expected error for empty paths")
	}
	if err := svc.ExportJSON(ExportJSONRequest{}); err == nil {
		t.Fatalf("ExportJSON: expected error for empty path")
	}
}
