package lockcontention

import (
	"errors"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fixture helpers — kept compact so individual tests stay readable.
// ─────────────────────────────────────────────────────────────────────────────

func javaSnapshot(name string, state models.ThreadState, frames ...string) models.ThreadSnapshot {
	stack := make([]models.StackFrame, 0, len(frames))
	for _, fn := range frames {
		stack = append(stack, models.StackFrame{Function: fn})
	}
	lang := "java"
	srcFmt := "java_jstack"
	cat := string(state)
	return models.ThreadSnapshot{
		SnapshotID:   "java::" + name,
		ThreadName:   name,
		State:        state,
		Category:     &cat,
		StackFrames:  stack,
		Metadata:     map[string]any{},
		LockHolds:    []models.LockHandle{},
		Language:     &lang,
		SourceFormat: &srcFmt,
	}
}

func makeBundle(dumpIndex int, snapshots []models.ThreadSnapshot) models.ThreadDumpBundle {
	label := "d-" + itoa(dumpIndex)
	return models.ThreadDumpBundle{
		Snapshots:    snapshots,
		SourceFile:   "d-" + itoa(dumpIndex) + ".txt",
		SourceFormat: "java_jstack",
		Language:     "java",
		DumpIndex:    dumpIndex,
		DumpLabel:    &label,
		Metadata:     map[string]any{},
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+(i%10))) + digits
		i /= 10
	}
	if negative {
		return "-" + digits
	}
	return digits
}

// classPtr returns a pointer to a string literal — common pattern in
// every test fixture below.
func classPtr(s string) *string { return &s }

// holdLock builds a LockHandle for `LockHolds`.
func holdLock(id, cls string) models.LockHandle {
	return models.LockHandle{LockID: id, LockClass: classPtr(cls)}
}

// waitLock builds a LockHandle for `LockWaiting`. WaitMode `"to_lock"`
// matches the Java jstack convention for blocked threads.
func waitLock(id, cls string) *models.LockHandle {
	h := models.LockHandle{LockID: id, LockClass: classPtr(cls), WaitMode: "to_lock"}
	return &h
}

func findingByCode(findings []map[string]any, code string) map[string]any {
	for _, f := range findings {
		if f["code"] == code {
			return f
		}
	}
	return nil
}

func countFindingsWithCode(findings []map[string]any, code string) int {
	n := 0
	for _, f := range findings {
		if f["code"] == code {
			n++
		}
	}
	return n
}

// ─────────────────────────────────────────────────────────────────────────────
// Orchestration / envelope
// ─────────────────────────────────────────────────────────────────────────────

func TestAnalyze_RejectsEmptyBundles(t *testing.T) {
	t.Parallel()
	_, err := Analyze(nil, Options{})
	if !errors.Is(err, ErrNoBundles) {
		t.Fatalf("expected ErrNoBundles, got %v", err)
	}
}

func TestAnalyze_EnvelopeFields(t *testing.T) {
	t.Parallel()
	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{javaSnapshot("solo", models.ThreadStateRunnable)}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if result.Type != ResultType {
		t.Fatalf("type = %q, want %q", result.Type, ResultType)
	}
	if result.Metadata.Parser != ParserName {
		t.Fatalf("parser = %q, want %q", result.Metadata.Parser, ParserName)
	}
	if result.Metadata.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q, want %q", result.Metadata.SchemaVersion, SchemaVersion)
	}
	if got := result.Summary["total_dumps"]; got != 1 {
		t.Fatalf("total_dumps = %v, want 1", got)
	}
	if got := result.Summary["total_thread_snapshots"]; got != 1 {
		t.Fatalf("total_thread_snapshots = %v, want 1", got)
	}
	if got := result.Summary["unique_locks"]; got != 0 {
		t.Fatalf("unique_locks = %v, want 0", got)
	}
	if result.Metadata.Diagnostics == nil || result.Metadata.Diagnostics.ParsedRecords != 1 {
		t.Fatalf("diagnostics not populated as expected: %+v", result.Metadata.Diagnostics)
	}
	if len(result.SourceFiles) != 1 || result.SourceFiles[0] != "d-0.txt" {
		t.Fatalf("source_files = %v", result.SourceFiles)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// LOCK_CONTENTION_HOTSPOT
// ─────────────────────────────────────────────────────────────────────────────

func TestLockContention_HotspotRanksByWaiterCount(t *testing.T) {
	t.Parallel()
	pool := holdLock("0xPOOL", "com.example.Pool")
	other := holdLock("0xOTHER", "com.example.Other")

	owner := javaSnapshot("owner", models.ThreadStateRunnable)
	owner.LockHolds = []models.LockHandle{pool, other}

	w1 := javaSnapshot("w1", models.ThreadStateBlocked)
	w1.LockWaiting = waitLock("0xPOOL", "com.example.Pool")
	w2 := javaSnapshot("w2", models.ThreadStateBlocked)
	w2.LockWaiting = waitLock("0xPOOL", "com.example.Pool")
	w3 := javaSnapshot("w3", models.ThreadStateBlocked)
	w3.LockWaiting = waitLock("0xPOOL", "com.example.Pool")

	bundle := makeBundle(0, []models.ThreadSnapshot{owner, w1, w2, w3})
	result, err := Analyze([]models.ThreadDumpBundle{bundle}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if got := result.Summary["contended_locks"]; got != 1 {
		t.Fatalf("contended_locks = %v, want 1", got)
	}
	hotspots := []map[string]any{}
	for _, f := range result.Metadata.Findings {
		if f["code"] == "LOCK_CONTENTION_HOTSPOT" {
			hotspots = append(hotspots, f)
		}
	}
	if len(hotspots) != 1 {
		t.Fatalf("hotspot count = %d, want 1", len(hotspots))
	}
	evidence := hotspots[0]["evidence"].(map[string]any)
	if evidence["lock_id"] != "0xPOOL" {
		t.Fatalf("lock_id = %v", evidence["lock_id"])
	}
	if evidence["waiter_count"] != 3 {
		t.Fatalf("waiter_count = %v, want 3", evidence["waiter_count"])
	}
	if evidence["owner_thread"] != "owner" {
		t.Fatalf("owner_thread = %v, want owner", evidence["owner_thread"])
	}
}

func TestLockContention_NoFindingsWhenLocksUncontended(t *testing.T) {
	t.Parallel()
	solo := javaSnapshot("solo", models.ThreadStateRunnable)
	solo.LockHolds = []models.LockHandle{holdLock("0xA", "com.example.A")}
	bundle := makeBundle(0, []models.ThreadSnapshot{solo})
	result, err := Analyze([]models.ThreadDumpBundle{bundle}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if got := result.Summary["contended_locks"]; got != 0 {
		t.Fatalf("contended_locks = %v, want 0", got)
	}
	if got := result.Summary["deadlocks_detected"]; got != 0 {
		t.Fatalf("deadlocks_detected = %v, want 0", got)
	}
	if findingByCode(result.Metadata.Findings, "LOCK_CONTENTION_HOTSPOT") != nil {
		t.Fatal("uncontended lock must not produce hotspot")
	}
	if findingByCode(result.Metadata.Findings, "DEADLOCK_DETECTED") != nil {
		t.Fatal("uncontended lock must not produce deadlock")
	}
}

func TestLockContention_ObjectWaitIsNotContention(t *testing.T) {
	t.Parallel()
	// Cooperative wait modes (Object.wait, Condition.await) must not
	// flip `contention_candidate=true`. Mirrors the Python set
	// {"object_wait", "parking_condition_wait"}.
	owner := javaSnapshot("owner", models.ThreadStateRunnable)
	owner.LockHolds = []models.LockHandle{holdLock("0xCOND", "com.example.Cond")}

	objWaiter := javaSnapshot("objw", models.ThreadStateWaiting)
	objWaiter.LockWaiting = &models.LockHandle{
		LockID: "0xCOND", LockClass: classPtr("com.example.Cond"), WaitMode: "object_wait",
	}
	parkWaiter := javaSnapshot("park", models.ThreadStateWaiting)
	parkWaiter.LockWaiting = &models.LockHandle{
		LockID: "0xCOND", LockClass: classPtr("com.example.Cond"), WaitMode: "parking_condition_wait",
	}

	bundle := makeBundle(0, []models.ThreadSnapshot{owner, objWaiter, parkWaiter})
	result, err := Analyze([]models.ThreadDumpBundle{bundle}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if got := result.Summary["contended_locks"]; got != 0 {
		t.Fatalf("contended_locks = %v, want 0", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// DEADLOCK_DETECTED
// ─────────────────────────────────────────────────────────────────────────────

func TestLockContention_DetectsTwoThreadDeadlock(t *testing.T) {
	t.Parallel()
	t1 := javaSnapshot("T1", models.ThreadStateBlocked)
	t1.LockHolds = []models.LockHandle{holdLock("0xA", "com.example.A")}
	t1.LockWaiting = waitLock("0xB", "com.example.B")

	t2 := javaSnapshot("T2", models.ThreadStateBlocked)
	t2.LockHolds = []models.LockHandle{holdLock("0xB", "com.example.B")}
	t2.LockWaiting = waitLock("0xA", "com.example.A")

	bundle := makeBundle(0, []models.ThreadSnapshot{t1, t2})
	result, err := Analyze([]models.ThreadDumpBundle{bundle}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if got := countFindingsWithCode(result.Metadata.Findings, "DEADLOCK_DETECTED"); got != 1 {
		t.Fatalf("DEADLOCK_DETECTED count = %d, want 1", got)
	}
	finding := findingByCode(result.Metadata.Findings, "DEADLOCK_DETECTED")
	if finding["severity"] != "critical" {
		t.Fatalf("severity = %v, want critical", finding["severity"])
	}
	chain := finding["evidence"].(map[string]any)
	threads := chain["threads"].([]string)
	if len(threads) != 2 {
		t.Fatalf("threads = %v, want 2", threads)
	}
	want := map[string]bool{"T1": true, "T2": true}
	for _, name := range threads {
		if !want[name] {
			t.Fatalf("threads contained unexpected %q (full=%v)", name, threads)
		}
	}
	if result.Summary["deadlocks_detected"] != 1 {
		t.Fatalf("deadlocks_detected = %v", result.Summary["deadlocks_detected"])
	}
	// Verify the cycle is rendered with arrow + closing thread.
	msg := finding["message"].(string)
	if msg == "" {
		t.Fatal("empty message")
	}
}

func TestLockContention_HandlesThreeThreadCycle(t *testing.T) {
	t.Parallel()
	t1 := javaSnapshot("T1", models.ThreadStateBlocked)
	t1.LockHolds = []models.LockHandle{holdLock("0xA", "")}
	t1.LockWaiting = waitLock("0xB", "")

	t2 := javaSnapshot("T2", models.ThreadStateBlocked)
	t2.LockHolds = []models.LockHandle{holdLock("0xB", "")}
	t2.LockWaiting = waitLock("0xC", "")

	t3 := javaSnapshot("T3", models.ThreadStateBlocked)
	t3.LockHolds = []models.LockHandle{holdLock("0xC", "")}
	t3.LockWaiting = waitLock("0xA", "")

	bundle := makeBundle(0, []models.ThreadSnapshot{t1, t2, t3})
	result, err := Analyze([]models.ThreadDumpBundle{bundle}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if got := countFindingsWithCode(result.Metadata.Findings, "DEADLOCK_DETECTED"); got != 1 {
		t.Fatalf("DEADLOCK_DETECTED count = %d, want 1 (must dedupe by canonical rotation)", got)
	}
	finding := findingByCode(result.Metadata.Findings, "DEADLOCK_DETECTED")
	chain := finding["evidence"].(map[string]any)
	threads := chain["threads"].([]string)
	if len(threads) != 3 {
		t.Fatalf("threads len = %d, want 3 (got %v)", len(threads), threads)
	}
	want := map[string]bool{"T1": true, "T2": true, "T3": true}
	for _, name := range threads {
		if !want[name] {
			t.Fatalf("unexpected thread %q in cycle %v", name, threads)
		}
	}
	edges := chain["edges"].([]map[string]any)
	if len(edges) != 3 {
		t.Fatalf("edges len = %d, want 3", len(edges))
	}
}

func TestLockContention_DeadlockExcludesCooperativeWait(t *testing.T) {
	t.Parallel()
	// T1 holds A, "waits" on B in object_wait mode (Object.wait) → not
	// a real deadlock. T2 holds B, waits on A. The cooperative-wait
	// edge breaks the cycle → no DEADLOCK_DETECTED.
	t1 := javaSnapshot("T1", models.ThreadStateWaiting)
	t1.LockHolds = []models.LockHandle{holdLock("0xA", "com.example.A")}
	t1.LockWaiting = &models.LockHandle{
		LockID: "0xB", LockClass: classPtr("com.example.B"), WaitMode: "object_wait",
	}

	t2 := javaSnapshot("T2", models.ThreadStateBlocked)
	t2.LockHolds = []models.LockHandle{holdLock("0xB", "com.example.B")}
	t2.LockWaiting = waitLock("0xA", "com.example.A")

	bundle := makeBundle(0, []models.ThreadSnapshot{t1, t2})
	result, err := Analyze([]models.ThreadDumpBundle{bundle}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if findingByCode(result.Metadata.Findings, "DEADLOCK_DETECTED") != nil {
		t.Fatal("object_wait edge must not produce a deadlock")
	}
}

func TestLockContention_NoDeadlockWhenOwnerMissing(t *testing.T) {
	t.Parallel()
	// Waiter waits on a lock with no recorded owner → no edge → no
	// cycle. Mirrors the `if owner` guard in the Python builder.
	w := javaSnapshot("w", models.ThreadStateBlocked)
	w.LockWaiting = waitLock("0xORPHAN", "com.example.Orphan")
	bundle := makeBundle(0, []models.ThreadSnapshot{w})
	result, err := Analyze([]models.ThreadDumpBundle{bundle}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if findingByCode(result.Metadata.Findings, "DEADLOCK_DETECTED") != nil {
		t.Fatal("orphan waiter must not produce a deadlock")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Multi-bundle union
// ─────────────────────────────────────────────────────────────────────────────

func TestLockContention_UnionsMultipleBundles(t *testing.T) {
	t.Parallel()
	owner := javaSnapshot("owner", models.ThreadStateRunnable)
	owner.LockHolds = []models.LockHandle{holdLock("0xA", "com.example.A")}
	w1 := javaSnapshot("w1", models.ThreadStateBlocked)
	w1.LockWaiting = waitLock("0xA", "com.example.A")
	w2 := javaSnapshot("w2", models.ThreadStateBlocked)
	w2.LockWaiting = waitLock("0xA", "com.example.A")

	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{owner, w1}),
		makeBundle(1, []models.ThreadSnapshot{w2}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if got := result.Summary["total_dumps"]; got != 2 {
		t.Fatalf("total_dumps = %v, want 2", got)
	}
	rows := result.Tables["locks"].([]map[string]any)
	if len(rows) == 0 {
		t.Fatal("no lock rows")
	}
	if rows[0]["lock_id"] != "0xA" {
		t.Fatalf("first lock_id = %v, want 0xA", rows[0]["lock_id"])
	}
	if rows[0]["waiter_count"] != 2 {
		t.Fatalf("waiter_count = %v, want 2 (union of w1 + w2)", rows[0]["waiter_count"])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Contention table sorting + projection
// ─────────────────────────────────────────────────────────────────────────────

func TestContentionTable_SortsByWaiterCountDesc(t *testing.T) {
	t.Parallel()
	owner := javaSnapshot("owner", models.ThreadStateRunnable)
	owner.LockHolds = []models.LockHandle{
		holdLock("0xA", "com.example.A"),
		holdLock("0xB", "com.example.B"),
	}
	// 0xB has 2 waiters; 0xA has 1 → 0xB should rank first.
	wA := javaSnapshot("wA", models.ThreadStateBlocked)
	wA.LockWaiting = waitLock("0xA", "com.example.A")
	wB1 := javaSnapshot("wB1", models.ThreadStateBlocked)
	wB1.LockWaiting = waitLock("0xB", "com.example.B")
	wB2 := javaSnapshot("wB2", models.ThreadStateBlocked)
	wB2.LockWaiting = waitLock("0xB", "com.example.B")

	bundle := makeBundle(0, []models.ThreadSnapshot{owner, wA, wB1, wB2})
	result, err := Analyze([]models.ThreadDumpBundle{bundle}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	rows := result.Tables["locks"].([]map[string]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0]["lock_id"] != "0xB" {
		t.Fatalf("first lock_id = %v, want 0xB", rows[0]["lock_id"])
	}
	if rows[0]["waiter_count"] != 2 {
		t.Fatalf("first waiter_count = %v, want 2", rows[0]["waiter_count"])
	}
}

func TestContentionRanking_RespectsTopN(t *testing.T) {
	t.Parallel()
	// 3 contended locks, ask for top_n = 2.
	owner := javaSnapshot("owner", models.ThreadStateRunnable)
	owner.LockHolds = []models.LockHandle{
		holdLock("0xA", "com.example.A"),
		holdLock("0xB", "com.example.B"),
		holdLock("0xC", "com.example.C"),
	}
	wA := javaSnapshot("wA", models.ThreadStateBlocked)
	wA.LockWaiting = waitLock("0xA", "com.example.A")
	wB := javaSnapshot("wB", models.ThreadStateBlocked)
	wB.LockWaiting = waitLock("0xB", "com.example.B")
	wC := javaSnapshot("wC", models.ThreadStateBlocked)
	wC.LockWaiting = waitLock("0xC", "com.example.C")

	bundle := makeBundle(0, []models.ThreadSnapshot{owner, wA, wB, wC})
	result, err := Analyze([]models.ThreadDumpBundle{bundle}, Options{TopN: 2})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	ranking := result.Series["contention_ranking"].([]map[string]any)
	if len(ranking) != 2 {
		t.Fatalf("ranking length = %d, want 2 (top_n cap)", len(ranking))
	}
	hotspots := 0
	for _, f := range result.Metadata.Findings {
		if f["code"] == "LOCK_CONTENTION_HOTSPOT" {
			hotspots++
		}
	}
	if hotspots != 2 {
		t.Fatalf("hotspot findings = %d, want 2 (top_n cap)", hotspots)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Languages / formats summary
// ─────────────────────────────────────────────────────────────────────────────

func TestSummary_LanguagesAndFormatsAreSorted(t *testing.T) {
	t.Parallel()
	pyBundle := makeBundle(0, []models.ThreadSnapshot{javaSnapshot("a", models.ThreadStateRunnable)})
	pyBundle.Language = "python"
	pyBundle.SourceFormat = "python_pyspy"
	javaBundle := makeBundle(1, []models.ThreadSnapshot{javaSnapshot("b", models.ThreadStateRunnable)})

	result, err := Analyze([]models.ThreadDumpBundle{pyBundle, javaBundle}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	langs := result.Summary["languages_detected"].([]string)
	if len(langs) != 2 || langs[0] != "java" || langs[1] != "python" {
		t.Fatalf("languages_detected = %v, want [java python]", langs)
	}
	formats := result.Summary["source_formats"].([]string)
	if len(formats) != 2 || formats[0] != "java_jstack" || formats[1] != "python_pyspy" {
		t.Fatalf("source_formats = %v", formats)
	}
}
