package threaddump

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// writeFile drops `body` into a fresh temp file and returns its path.
// Mirrors the helper used by the javajstack plugin tests.
func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// Mirrors test_jvm_analyzers.test_thread_dump_parser_and_analyzer —
// the canonical Python regression test for the analyzer.
const pythonRegressionDump = `"http-nio-8080-exec-1" #31 tid=0x1 nid=0x101 waiting for monitor entry
   java.lang.Thread.State: BLOCKED (on object monitor)
        at com.example.OrderService.find(OrderService.java:42)
        - waiting to lock <0x00000001>

"HikariPool-1 housekeeper" #32 tid=0x2 nid=0x102 waiting on condition
   java.lang.Thread.State: TIMED_WAITING (parking)
        at jdk.internal.misc.Unsafe.park(Native Method)
`

func TestAnalyzeReplicatesPythonRegression(t *testing.T) {
	path := writeFile(t, "thread-dump.txt", pythonRegressionDump)

	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.Type != "thread_dump" {
		t.Fatalf("type = %q, want thread_dump", result.Type)
	}
	if got := asInt(result.Summary["blocked_threads"]); got != 1 {
		t.Fatalf("blocked_threads = %d, want 1", got)
	}
	if got := asInt(result.Summary["waiting_threads"]); got != 1 {
		t.Fatalf("waiting_threads = %d, want 1", got)
	}
	if len(result.Metadata.Findings) == 0 {
		t.Fatalf("findings should not be empty")
	}
	if result.Metadata.Findings[0]["code"] != "BLOCKED_THREADS_PRESENT" {
		t.Fatalf(
			"first finding code = %v, want BLOCKED_THREADS_PRESENT",
			result.Metadata.Findings[0]["code"],
		)
	}
}

// AnalysisResult metadata identity (parser, schema_version, source).
func TestAnalyzeMetadataIdentity(t *testing.T) {
	path := writeFile(t, "thread-dump.txt", pythonRegressionDump)
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Metadata.Parser != "java_thread_dump" {
		t.Fatalf("parser = %q", result.Metadata.Parser)
	}
	if result.Metadata.SchemaVersion != "0.1.0" {
		t.Fatalf("schema_version = %q", result.Metadata.SchemaVersion)
	}
	if len(result.SourceFiles) != 1 || result.SourceFiles[0] != path {
		t.Fatalf("source_files = %v", result.SourceFiles)
	}
}

// state_distribution / category_distribution shape and ordering.
func TestSeriesDistributionsAreSortedByCount(t *testing.T) {
	body := strings.Join([]string{
		`"a" #1 prio=5 nid=0x01 runnable`,
		`   java.lang.Thread.State: RUNNABLE`,
		`        at com.acme.A.run(A.java:1)`,
		``,
		`"b" #2 prio=5 nid=0x02 waiting on condition`,
		`   java.lang.Thread.State: TIMED_WAITING (parking)`,
		`        at jdk.internal.misc.Unsafe.park(Native Method)`,
		``,
		`"c" #3 prio=5 nid=0x03 waiting on condition`,
		`   java.lang.Thread.State: TIMED_WAITING (parking)`,
		`        at jdk.internal.misc.Unsafe.park(Native Method)`,
		``,
		`"d" #4 prio=5 nid=0x04 waiting on condition`,
		`   java.lang.Thread.State: TIMED_WAITING (parking)`,
		`        at jdk.internal.misc.Unsafe.park(Native Method)`,
		``,
	}, "\n")
	path := writeFile(t, "td.txt", body)

	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	stateDist, ok := result.Series["state_distribution"].([]map[string]any)
	if !ok {
		t.Fatalf("state_distribution missing/wrong shape: %T", result.Series["state_distribution"])
	}
	if len(stateDist) != 2 {
		t.Fatalf("state distribution rows = %d", len(stateDist))
	}
	// 3x TIMED_WAITING vs 1x RUNNABLE — TIMED_WAITING wins on count.
	if stateDist[0]["state"] != "TIMED_WAITING" || asInt(stateDist[0]["count"]) != 3 {
		t.Fatalf("first row = %v", stateDist[0])
	}
	if stateDist[1]["state"] != "RUNNABLE" || asInt(stateDist[1]["count"]) != 1 {
		t.Fatalf("second row = %v", stateDist[1])
	}

	catDist, ok := result.Series["category_distribution"].([]map[string]any)
	if !ok {
		t.Fatalf("category_distribution missing/wrong shape")
	}
	// WAITING (3) > RUNNABLE (1).
	if catDist[0]["category"] != "WAITING" || asInt(catDist[0]["count"]) != 3 {
		t.Fatalf("first cat row = %v", catDist[0])
	}
}

// stack_signature aggregation: identical top frames should collapse.
func TestStackSignatureAggregation(t *testing.T) {
	body := strings.Join([]string{
		`"a" #1 prio=5 nid=0x01 waiting on condition`,
		`   java.lang.Thread.State: TIMED_WAITING (parking)`,
		`        at jdk.internal.misc.Unsafe.park(Native Method)`,
		`        at java.util.concurrent.locks.LockSupport.parkNanos(LockSupport.java:252)`,
		``,
		`"b" #2 prio=5 nid=0x02 waiting on condition`,
		`   java.lang.Thread.State: TIMED_WAITING (parking)`,
		`        at jdk.internal.misc.Unsafe.park(Native Method)`,
		`        at java.util.concurrent.locks.LockSupport.parkNanos(LockSupport.java:252)`,
		``,
		`"c" #3 prio=5 nid=0x03 runnable`,
		`   java.lang.Thread.State: RUNNABLE`,
		`        at com.acme.Different.work(Different.java:10)`,
		``,
	}, "\n")
	path := writeFile(t, "sig.txt", body)

	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	sigs, ok := result.Series["top_stack_signatures"].([]map[string]any)
	if !ok {
		t.Fatalf("top_stack_signatures missing/wrong shape")
	}
	if len(sigs) != 2 {
		t.Fatalf("signature rows = %d, want 2", len(sigs))
	}
	if asInt(sigs[0]["count"]) != 2 {
		t.Fatalf("most common signature count = %d, want 2", asInt(sigs[0]["count"]))
	}
}

// "(no-stack)" sentinel matches Python's `_stack_signature` fallback.
func TestStackSignatureEmptyStackSentinel(t *testing.T) {
	body := strings.Join([]string{
		`"empty" #1 prio=5 nid=0x01 runnable`,
		`   java.lang.Thread.State: RUNNABLE`,
		``,
	}, "\n")
	path := writeFile(t, "empty.txt", body)

	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	sigs := result.Series["top_stack_signatures"].([]map[string]any)
	if len(sigs) != 1 || sigs[0]["signature"] != "(no-stack)" {
		t.Fatalf("expected (no-stack) sentinel, got %v", sigs)
	}
}

// findings: WAITING_THREADS_DOMINATE fires when waiting > runnable.
func TestWaitingThreadsDominateFinding(t *testing.T) {
	body := strings.Join([]string{
		`"a" #1 prio=5 nid=0x01 waiting on condition`,
		`   java.lang.Thread.State: TIMED_WAITING (parking)`,
		`        at jdk.internal.misc.Unsafe.park(Native Method)`,
		``,
		`"b" #2 prio=5 nid=0x02 waiting on condition`,
		`   java.lang.Thread.State: TIMED_WAITING (parking)`,
		`        at jdk.internal.misc.Unsafe.park(Native Method)`,
		``,
	}, "\n")
	path := writeFile(t, "td.txt", body)

	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	codes := []string{}
	for _, f := range result.Metadata.Findings {
		codes = append(codes, f["code"].(string))
	}
	found := false
	for _, c := range codes {
		if c == "WAITING_THREADS_DOMINATE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected WAITING_THREADS_DOMINATE, got %v", codes)
	}
}

// findings: only BLOCKED is set when waiting <= runnable.
func TestBlockedOnlyFinding(t *testing.T) {
	body := strings.Join([]string{
		`"r1" #1 prio=5 nid=0x01 runnable`,
		`   java.lang.Thread.State: RUNNABLE`,
		`        at com.acme.A.run(A.java:1)`,
		``,
		`"r2" #2 prio=5 nid=0x02 runnable`,
		`   java.lang.Thread.State: RUNNABLE`,
		`        at com.acme.B.run(B.java:1)`,
		``,
		`"b1" #3 prio=5 nid=0x03 waiting for monitor entry`,
		`   java.lang.Thread.State: BLOCKED (on object monitor)`,
		`        at com.acme.C.acquire(C.java:1)`,
		`        - waiting to lock <0x000000000a>`,
		``,
	}, "\n")
	path := writeFile(t, "td.txt", body)

	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	codes := []string{}
	for _, f := range result.Metadata.Findings {
		codes = append(codes, f["code"].(string))
	}
	if len(codes) != 1 || codes[0] != "BLOCKED_THREADS_PRESENT" {
		t.Fatalf("expected only BLOCKED_THREADS_PRESENT, got %v", codes)
	}
}

// threads_with_locks counts snapshots whose LockInfo is set.
func TestThreadsWithLocksCount(t *testing.T) {
	path := writeFile(t, "td.txt", pythonRegressionDump)
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	// "http-nio-8080-exec-1" has `- waiting to lock` → LockInfo populated.
	if got := asInt(result.Summary["threads_with_locks"]); got != 1 {
		t.Fatalf("threads_with_locks = %d, want 1", got)
	}
}

// tables.threads is capped to TopN snapshots.
func TestThreadsTableHonorsTopN(t *testing.T) {
	parts := []string{}
	for i := 0; i < 10; i++ {
		parts = append(parts,
			`"t`+itoa(i)+`" #`+itoa(i)+` prio=5 nid=0x0`+itoa(i)+` runnable`,
			`   java.lang.Thread.State: RUNNABLE`,
			`        at com.acme.X.run(X.java:1)`,
			``,
		)
	}
	body := strings.Join(parts, "\n")
	path := writeFile(t, "td.txt", body)

	result, err := Analyze(path, Options{TopN: 3})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	rows := result.Tables["threads"].([]map[string]any)
	if len(rows) != 3 {
		t.Fatalf("threads table rows = %d, want 3", len(rows))
	}
}

// Regression on regression dump: tables["threads"][0].lock_info points at the BLOCKED row text.
func TestThreadsTableLockInfoSurfaced(t *testing.T) {
	path := writeFile(t, "td.txt", pythonRegressionDump)
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	rows := result.Tables["threads"].([]map[string]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	lock, ok := rows[0]["lock_info"].(*string)
	if !ok || lock == nil {
		t.Fatalf("first row lock_info = %v (%T)", rows[0]["lock_info"], rows[0]["lock_info"])
	}
	if !strings.Contains(*lock, "waiting to lock") {
		t.Fatalf("lock_info should contain 'waiting to lock', got %q", *lock)
	}
	// Second thread (HikariPool) is parking — lock_info should still
	// be set (parking to wait would set it; here it's `TIMED_WAITING` with
	// a `parking to wait` Native Method line). For this fixture the
	// second thread has no `waiting on / locked / parking to wait` token
	// in stripped form, so lock_info stays nil.
	if rows[1]["lock_info"] != nil {
		switch v := rows[1]["lock_info"].(type) {
		case *string:
			if v != nil {
				t.Fatalf("second row lock_info should be nil, got %q", *v)
			}
		}
	}
}

// Build can be invoked directly with a synthesized bundle — no file IO.
func TestBuildAcceptsSynthesizedBundle(t *testing.T) {
	bundle := models.NewThreadDumpBundle("/synthetic/path", "java_jstack", "java")
	cat := "RUNNABLE"
	tid := "0x01"
	frame := models.StackFrame{Function: "run"}
	snap := models.NewThreadSnapshot("java::0::0::main", "main", models.ThreadStateRunnable)
	snap.ThreadID = &tid
	snap.Category = &cat
	snap.StackFrames = []models.StackFrame{frame}
	bundle.Snapshots = append(bundle.Snapshots, snap)

	result := Build(bundle, "/synthetic/path", nil, Options{})
	if result.Type != "thread_dump" {
		t.Fatalf("type = %q", result.Type)
	}
	if asInt(result.Summary["total_threads"]) != 1 {
		t.Fatalf("total_threads = %v", result.Summary["total_threads"])
	}
	if asInt(result.Summary["runnable_threads"]) != 1 {
		t.Fatalf("runnable_threads = %v", result.Summary["runnable_threads"])
	}
}

// Default TopN kicks in when Options.TopN is 0.
func TestDefaultTopN(t *testing.T) {
	bundle := models.NewThreadDumpBundle("p", "java_jstack", "java")
	for i := 0; i < 30; i++ {
		snap := models.NewThreadSnapshot("id-"+itoa(i), "t-"+itoa(i), models.ThreadStateRunnable)
		bundle.Snapshots = append(bundle.Snapshots, snap)
	}
	result := Build(bundle, "p", nil, Options{})
	rows := result.Tables["threads"].([]map[string]any)
	if len(rows) != DefaultTopN {
		t.Fatalf("threads rows = %d, want %d (DefaultTopN)", len(rows), DefaultTopN)
	}
}

// Unknown / nil category falls back to UNKNOWN bucket.
func TestUnknownCategoryFallback(t *testing.T) {
	bundle := models.NewThreadDumpBundle("p", "java_jstack", "java")
	snap := models.NewThreadSnapshot("id-0", "t0", models.ThreadStateUnknown)
	// Category left nil → must show up as UNKNOWN.
	bundle.Snapshots = append(bundle.Snapshots, snap)

	result := Build(bundle, "p", nil, Options{})
	cat := result.Series["category_distribution"].([]map[string]any)
	if len(cat) != 1 || cat[0]["category"] != "UNKNOWN" {
		t.Fatalf("category_distribution = %v", cat)
	}
	state := result.Series["state_distribution"].([]map[string]any)
	if len(state) != 1 || state[0]["state"] != "UNKNOWN" {
		t.Fatalf("state_distribution = %v", state)
	}
}

// Default diagnostics has parsed_records == total snapshots.
func TestDefaultDiagnostics(t *testing.T) {
	path := writeFile(t, "td.txt", pythonRegressionDump)
	result, err := Analyze(path, Options{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Metadata.Diagnostics == nil {
		t.Fatal("diagnostics should not be nil")
	}
	if result.Metadata.Diagnostics.ParsedRecords != 2 {
		t.Fatalf("parsed_records = %d, want 2", result.Metadata.Diagnostics.ParsedRecords)
	}
	if result.Metadata.Diagnostics.Format != "java_thread_dump" {
		t.Fatalf("format = %q", result.Metadata.Diagnostics.Format)
	}
}

// Sample fixture under examples/thread-dumps/ should analyze cleanly.
func TestSampleFixtureFromExamplesDir(t *testing.T) {
	// Locate examples/thread-dumps/sample-java-thread-dump.txt by
	// walking up from cwd until we find the repo root marker.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 12; i++ {
		candidate := filepath.Join(dir, "examples", "thread-dumps", "sample-java-thread-dump.txt")
		if _, err := os.Stat(candidate); err == nil {
			result, err := Analyze(candidate, Options{})
			if err != nil {
				t.Fatalf("Analyze sample: %v", err)
			}
			if result.Type != "thread_dump" {
				t.Fatalf("type = %q", result.Type)
			}
			if asInt(result.Summary["total_threads"]) != 2 {
				t.Fatalf("total_threads = %v, want 2", result.Summary["total_threads"])
			}
			if asInt(result.Summary["blocked_threads"]) != 1 {
				t.Fatalf("blocked_threads = %v, want 1", result.Summary["blocked_threads"])
			}
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("sample-java-thread-dump.txt not found relative to test cwd")
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	out := []byte{}
	for i > 0 {
		out = append([]byte{byte('0' + i%10)}, out...)
		i /= 10
	}
	if negative {
		out = append([]byte{'-'}, out...)
	}
	return string(out)
}
