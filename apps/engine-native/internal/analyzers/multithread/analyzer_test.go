package multithread

import (
	"errors"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fixture helpers — kept compact to keep individual tests readable.
// ─────────────────────────────────────────────────────────────────────────────

func javaSnapshot(name string, state models.ThreadState, frames ...string) models.ThreadSnapshot {
	stack := make([]models.StackFrame, 0, len(frames))
	for _, fn := range frames {
		stack = append(stack, models.StackFrame{Function: fn})
	}
	lang := "java"
	srcFmt := "java_jstack"
	return models.ThreadSnapshot{
		SnapshotID:   "java::" + name,
		ThreadName:   name,
		State:        state,
		StackFrames:  stack,
		Metadata:     map[string]any{},
		LockHolds:    []models.LockHandle{},
		Language:     &lang,
		SourceFormat: &srcFmt,
	}
}

func makeBundle(dumpIndex int, snapshots []models.ThreadSnapshot) models.ThreadDumpBundle {
	label := "dump-" + itoa(dumpIndex)
	return models.ThreadDumpBundle{
		Snapshots:    snapshots,
		SourceFile:   "dump-" + itoa(dumpIndex) + ".txt",
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
		makeBundle(0, []models.ThreadSnapshot{javaSnapshot("a", models.ThreadStateRunnable, "x")}),
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
	if got := result.Summary["consecutive_dump_threshold"]; got != ConsecutiveDumpsThreshold {
		t.Fatalf("threshold default = %v, want %d", got, ConsecutiveDumpsThreshold)
	}
	if got := result.Summary["total_dumps"]; got != 1 {
		t.Fatalf("total_dumps = %v, want 1", got)
	}
	langs, ok := result.Summary["languages_detected"].([]string)
	if !ok || len(langs) != 1 || langs[0] != "java" {
		t.Fatalf("languages_detected = %v", result.Summary["languages_detected"])
	}
	if result.Metadata.Diagnostics == nil {
		t.Fatal("diagnostics should be populated")
	}
	if result.Metadata.Diagnostics.ParsedRecords != 1 {
		t.Fatalf("parsed_records = %d, want 1", result.Metadata.Diagnostics.ParsedRecords)
	}
}

func TestAnalyze_ThresholdOverride(t *testing.T) {
	t.Parallel()
	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{javaSnapshot("w", models.ThreadStateRunnable, "loop")}),
		makeBundle(1, []models.ThreadSnapshot{javaSnapshot("w", models.ThreadStateRunnable, "loop")}),
	}
	// Default threshold (3) → no LONG_RUNNING.
	resDefault, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if findingByCode(resDefault.Metadata.Findings, "LONG_RUNNING_THREAD") != nil {
		t.Fatalf("threshold=3 should not fire; findings=%v", resDefault.Metadata.Findings)
	}
	// Threshold 2 → LONG_RUNNING fires.
	resTwo, err := Analyze(bundles, Options{Threshold: 2})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if findingByCode(resTwo.Metadata.Findings, "LONG_RUNNING_THREAD") == nil {
		t.Fatalf("threshold=2 should fire LONG_RUNNING_THREAD")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// LONG_RUNNING_THREAD
// ─────────────────────────────────────────────────────────────────────────────

func TestLongRunning_ThreeConsecutiveRunnable(t *testing.T) {
	t.Parallel()
	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{javaSnapshot("worker-1", models.ThreadStateRunnable, "compute", "loop")}),
		makeBundle(1, []models.ThreadSnapshot{javaSnapshot("worker-1", models.ThreadStateRunnable, "compute", "loop")}),
		makeBundle(2, []models.ThreadSnapshot{javaSnapshot("worker-1", models.ThreadStateRunnable, "compute", "loop")}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if got := result.Summary["long_running_threads"]; got != 1 {
		t.Fatalf("long_running_threads = %v, want 1", got)
	}
	finding := findingByCode(result.Metadata.Findings, "LONG_RUNNING_THREAD")
	if finding == nil {
		t.Fatal("LONG_RUNNING_THREAD finding missing")
	}
	if finding["severity"] != "warning" {
		t.Fatalf("severity = %v", finding["severity"])
	}
	evidence := finding["evidence"].(map[string]any)
	if evidence["thread_name"] != "worker-1" {
		t.Fatalf("thread_name = %v", evidence["thread_name"])
	}
	if evidence["dumps"] != 3 {
		t.Fatalf("dumps = %v", evidence["dumps"])
	}
	if evidence["first_dump_index"] != 0 {
		t.Fatalf("first_dump_index = %v", evidence["first_dump_index"])
	}
}

func TestLongRunning_BrokenRunSkips(t *testing.T) {
	t.Parallel()
	// RUNNABLE → WAITING → RUNNABLE breaks the run.
	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{javaSnapshot("w", models.ThreadStateRunnable, "compute")}),
		makeBundle(1, []models.ThreadSnapshot{javaSnapshot("w", models.ThreadStateWaiting, "park")}),
		makeBundle(2, []models.ThreadSnapshot{javaSnapshot("w", models.ThreadStateRunnable, "compute")}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if findingByCode(result.Metadata.Findings, "LONG_RUNNING_THREAD") != nil {
		t.Fatalf("expected no LONG_RUNNING_THREAD finding")
	}
	if got := result.Summary["long_running_threads"]; got != 0 {
		t.Fatalf("long_running_threads = %v, want 0", got)
	}
}

func TestLongRunning_StackChangeBreaksRun(t *testing.T) {
	t.Parallel()
	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{javaSnapshot("w", models.ThreadStateRunnable, "stackA")}),
		makeBundle(1, []models.ThreadSnapshot{javaSnapshot("w", models.ThreadStateRunnable, "stackB")}),
		makeBundle(2, []models.ThreadSnapshot{javaSnapshot("w", models.ThreadStateRunnable, "stackA")}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if findingByCode(result.Metadata.Findings, "LONG_RUNNING_THREAD") != nil {
		t.Fatalf("stack-change should break run")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PERSISTENT_BLOCKED_THREAD
// ─────────────────────────────────────────────────────────────────────────────

func TestPersistentBlocked_ThreeConsecutiveBlocked(t *testing.T) {
	t.Parallel()
	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{javaSnapshot("blocker", models.ThreadStateBlocked, "lock")}),
		makeBundle(1, []models.ThreadSnapshot{javaSnapshot("blocker", models.ThreadStateBlocked, "lock")}),
		makeBundle(2, []models.ThreadSnapshot{javaSnapshot("blocker", models.ThreadStateBlocked, "lock")}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	finding := findingByCode(result.Metadata.Findings, "PERSISTENT_BLOCKED_THREAD")
	if finding == nil {
		t.Fatal("PERSISTENT_BLOCKED_THREAD finding missing")
	}
	if finding["severity"] != "critical" {
		t.Fatalf("severity = %v, want critical", finding["severity"])
	}
	if got := result.Summary["persistent_blocked_threads"]; got != 1 {
		t.Fatalf("persistent_blocked_threads = %v", got)
	}
}

func TestPersistentBlocked_LockWaitCounts(t *testing.T) {
	t.Parallel()
	// LOCK_WAIT is treated as blocked (Python's _BLOCKED_STATES).
	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{javaSnapshot("t", models.ThreadStateLockWait, "wait")}),
		makeBundle(1, []models.ThreadSnapshot{javaSnapshot("t", models.ThreadStateBlocked, "wait")}),
		makeBundle(2, []models.ThreadSnapshot{javaSnapshot("t", models.ThreadStateLockWait, "wait")}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if findingByCode(result.Metadata.Findings, "PERSISTENT_BLOCKED_THREAD") == nil {
		t.Fatal("LOCK_WAIT should count as blocked")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// LATENCY_SECTION_DETECTED
// ─────────────────────────────────────────────────────────────────────────────

func TestLatencySection_NetworkWaitFires(t *testing.T) {
	t.Parallel()
	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{javaSnapshot("io-1", models.ThreadStateNetworkWait, "recv")}),
		makeBundle(1, []models.ThreadSnapshot{javaSnapshot("io-1", models.ThreadStateNetworkWait, "recv")}),
		makeBundle(2, []models.ThreadSnapshot{javaSnapshot("io-1", models.ThreadStateNetworkWait, "recv")}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	finding := findingByCode(result.Metadata.Findings, "LATENCY_SECTION_DETECTED")
	if finding == nil {
		t.Fatal("LATENCY_SECTION_DETECTED missing")
	}
	evidence := finding["evidence"].(map[string]any)
	if evidence["wait_category"] != "NETWORK_WAIT" {
		t.Fatalf("wait_category = %v, want NETWORK_WAIT", evidence["wait_category"])
	}
	if got := result.Summary["latency_sections"]; got != 1 {
		t.Fatalf("latency_sections = %v", got)
	}
}

func TestLatencySection_DoesNotFireForLockWait(t *testing.T) {
	t.Parallel()
	// LOCK_WAIT is intentionally excluded from latency categories so
	// PERSISTENT_BLOCKED is the canonical signal for lock waits.
	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{javaSnapshot("t", models.ThreadStateLockWait, "wait")}),
		makeBundle(1, []models.ThreadSnapshot{javaSnapshot("t", models.ThreadStateLockWait, "wait")}),
		makeBundle(2, []models.ThreadSnapshot{javaSnapshot("t", models.ThreadStateLockWait, "wait")}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if findingByCode(result.Metadata.Findings, "LATENCY_SECTION_DETECTED") != nil {
		t.Fatal("LOCK_WAIT must not produce LATENCY_SECTION_DETECTED")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GROWING_LOCK_CONTENTION
// ─────────────────────────────────────────────────────────────────────────────

func javaWaiterSnapshot(name string, lockID string) models.ThreadSnapshot {
	s := javaSnapshot(name, models.ThreadStateBlocked, "wait")
	cls := "com.example.Pool"
	s.LockWaiting = &models.LockHandle{
		LockID:    lockID,
		LockClass: &cls,
		WaitMode:  "to_lock",
	}
	return s
}

func TestGrowingLock_StrictlyIncreasingFires(t *testing.T) {
	t.Parallel()
	lockID := "0xCAFEBABE"
	bundles := []models.ThreadDumpBundle{
		// Dump 0 — 1 waiter.
		makeBundle(0, []models.ThreadSnapshot{javaWaiterSnapshot("w0", lockID)}),
		// Dump 1 — 2 waiters.
		makeBundle(1, []models.ThreadSnapshot{
			javaWaiterSnapshot("w0", lockID),
			javaWaiterSnapshot("w1", lockID),
		}),
		// Dump 2 — 3 waiters.
		makeBundle(2, []models.ThreadSnapshot{
			javaWaiterSnapshot("w0", lockID),
			javaWaiterSnapshot("w1", lockID),
			javaWaiterSnapshot("w2", lockID),
		}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	finding := findingByCode(result.Metadata.Findings, "GROWING_LOCK_CONTENTION")
	if finding == nil {
		t.Fatal("GROWING_LOCK_CONTENTION finding missing")
	}
	evidence := finding["evidence"].(map[string]any)
	if evidence["lock_id"] != lockID {
		t.Fatalf("lock_id = %v, want %s", evidence["lock_id"], lockID)
	}
	if evidence["consecutive_dumps"] != 3 {
		t.Fatalf("consecutive_dumps = %v, want 3", evidence["consecutive_dumps"])
	}
	if evidence["max_waiters"] != 3 {
		t.Fatalf("max_waiters = %v, want 3", evidence["max_waiters"])
	}
}

func TestGrowingLock_SteadyDoesNotFire(t *testing.T) {
	t.Parallel()
	lockID := "0xDEADBEEF"
	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{javaWaiterSnapshot("w0", lockID), javaWaiterSnapshot("w1", lockID)}),
		makeBundle(1, []models.ThreadSnapshot{javaWaiterSnapshot("w0", lockID), javaWaiterSnapshot("w1", lockID)}),
		makeBundle(2, []models.ThreadSnapshot{javaWaiterSnapshot("w0", lockID), javaWaiterSnapshot("w1", lockID)}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if findingByCode(result.Metadata.Findings, "GROWING_LOCK_CONTENTION") != nil {
		t.Fatal("steady waiter count must not fire growing-lock")
	}
}

func TestGrowingLock_ObjectWaitIsExcluded(t *testing.T) {
	t.Parallel()
	lockID := "0xBEEFFACE"
	makeWait := func(name string) models.ThreadSnapshot {
		s := javaSnapshot(name, models.ThreadStateWaiting, "wait")
		s.LockWaiting = &models.LockHandle{LockID: lockID, WaitMode: "object_wait"}
		return s
	}
	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{makeWait("a")}),
		makeBundle(1, []models.ThreadSnapshot{makeWait("a"), makeWait("b")}),
		makeBundle(2, []models.ThreadSnapshot{makeWait("a"), makeWait("b"), makeWait("c")}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if findingByCode(result.Metadata.Findings, "GROWING_LOCK_CONTENTION") != nil {
		t.Fatal("object_wait should be excluded from growing-lock")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// THREAD_CONGESTION_DETECTED  / EXTERNAL_RESOURCE_WAIT_HIGH /
// LIKELY_GC_PAUSE_DETECTED — heuristic ratio findings
// ─────────────────────────────────────────────────────────────────────────────

func TestCongestion_FiresWhenWaitingExceeds10Percent(t *testing.T) {
	t.Parallel()
	// 20% waiting (1 of 5) > 10% threshold.
	snaps := []models.ThreadSnapshot{
		javaSnapshot("a", models.ThreadStateRunnable, "x"),
		javaSnapshot("b", models.ThreadStateRunnable, "x"),
		javaSnapshot("c", models.ThreadStateRunnable, "x"),
		javaSnapshot("d", models.ThreadStateRunnable, "x"),
		javaSnapshot("e", models.ThreadStateWaiting, "x"),
	}
	result, err := Analyze([]models.ThreadDumpBundle{makeBundle(0, snaps)}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	finding := findingByCode(result.Metadata.Findings, "THREAD_CONGESTION_DETECTED")
	if finding == nil {
		t.Fatal("THREAD_CONGESTION_DETECTED missing")
	}
	evidence := finding["evidence"].(map[string]any)
	if evidence["waiting_threads"] != 1 {
		t.Fatalf("waiting_threads = %v, want 1", evidence["waiting_threads"])
	}
	if evidence["total_threads"] != 5 {
		t.Fatalf("total_threads = %v, want 5", evidence["total_threads"])
	}
	if rate, _ := evidence["waiting_ratio"].(float64); rate != 0.2 {
		t.Fatalf("waiting_ratio = %v, want 0.2", evidence["waiting_ratio"])
	}
}

func TestCongestion_IgnoresEmptyBundles(t *testing.T) {
	t.Parallel()
	bundles := []models.ThreadDumpBundle{makeBundle(0, []models.ThreadSnapshot{})}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if got := countFindingsWithCode(result.Metadata.Findings, "THREAD_CONGESTION_DETECTED"); got != 0 {
		t.Fatalf("empty bundle should not produce congestion finding; got %d", got)
	}
}

func TestExternalResource_FiresWhenSleepingExceeds25Percent(t *testing.T) {
	t.Parallel()
	// 50% timed-waiting (2 of 4).
	snaps := []models.ThreadSnapshot{
		javaSnapshot("a", models.ThreadStateRunnable, "x"),
		javaSnapshot("b", models.ThreadStateRunnable, "x"),
		javaSnapshot("c", models.ThreadStateTimedWaiting, "sleep"),
		javaSnapshot("d", models.ThreadStateTimedWaiting, "sleep"),
	}
	result, err := Analyze([]models.ThreadDumpBundle{makeBundle(0, snaps)}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	finding := findingByCode(result.Metadata.Findings, "EXTERNAL_RESOURCE_WAIT_HIGH")
	if finding == nil {
		t.Fatal("EXTERNAL_RESOURCE_WAIT_HIGH missing")
	}
	if finding["severity"] != "info" {
		t.Fatalf("severity = %v, want info", finding["severity"])
	}
	evidence := finding["evidence"].(map[string]any)
	if evidence["sleeping_threads"] != 2 {
		t.Fatalf("sleeping_threads = %v", evidence["sleeping_threads"])
	}
}

func TestLikelyGCPause_FiresWhenUnownedBlockedExceeds50Percent(t *testing.T) {
	t.Parallel()
	lockID := "0xUNOWNED"
	mkBlocked := func(name string) models.ThreadSnapshot {
		s := javaSnapshot(name, models.ThreadStateBlocked, "wait")
		s.LockWaiting = &models.LockHandle{LockID: lockID, WaitMode: "to_lock"}
		return s
	}
	// 75% blocked on lock with no holder (3 of 4).
	snaps := []models.ThreadSnapshot{
		mkBlocked("a"),
		mkBlocked("b"),
		mkBlocked("c"),
		javaSnapshot("d", models.ThreadStateRunnable, "x"),
	}
	result, err := Analyze([]models.ThreadDumpBundle{makeBundle(0, snaps)}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	finding := findingByCode(result.Metadata.Findings, "LIKELY_GC_PAUSE_DETECTED")
	if finding == nil {
		t.Fatal("LIKELY_GC_PAUSE_DETECTED missing")
	}
	evidence := finding["evidence"].(map[string]any)
	if evidence["unowned_blocked_threads"] != 3 {
		t.Fatalf("unowned_blocked_threads = %v", evidence["unowned_blocked_threads"])
	}
}

func TestLikelyGCPause_DoesNotFireWhenLockIsHeld(t *testing.T) {
	t.Parallel()
	lockID := "0xHELD"
	cls := "com.example.Pool"
	owner := javaSnapshot("owner", models.ThreadStateRunnable, "work")
	owner.LockHolds = []models.LockHandle{{LockID: lockID, LockClass: &cls}}
	mkBlocked := func(name string) models.ThreadSnapshot {
		s := javaSnapshot(name, models.ThreadStateBlocked, "wait")
		s.LockWaiting = &models.LockHandle{LockID: lockID, WaitMode: "to_lock"}
		return s
	}
	snaps := []models.ThreadSnapshot{owner, mkBlocked("a"), mkBlocked("b"), mkBlocked("c")}
	result, err := Analyze([]models.ThreadDumpBundle{makeBundle(0, snaps)}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if findingByCode(result.Metadata.Findings, "LIKELY_GC_PAUSE_DETECTED") != nil {
		t.Fatal("held lock must suppress GC-pause finding")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// VIRTUAL_THREAD_CARRIER_PINNING / SMR_UNRESOLVED_THREAD /
// INCOMPLETE_HISTOGRAM — JVM metadata findings
// ─────────────────────────────────────────────────────────────────────────────

func TestCarrierPinning_FindingFromMetadata(t *testing.T) {
	t.Parallel()
	snap := javaSnapshot("vt-carrier", models.ThreadStateRunnable, "park")
	snap.Metadata["carrier_pinning"] = map[string]any{
		"candidate_method": "java.lang.VirtualThread.park",
		"top_frame":        "Park.java:42",
		"reason":           "synchronized block holds carrier",
	}
	bundles := []models.ThreadDumpBundle{makeBundle(0, []models.ThreadSnapshot{snap})}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	finding := findingByCode(result.Metadata.Findings, "VIRTUAL_THREAD_CARRIER_PINNING")
	if finding == nil {
		t.Fatal("VIRTUAL_THREAD_CARRIER_PINNING missing")
	}
	evidence := finding["evidence"].(map[string]any)
	if evidence["candidate_method"] != "java.lang.VirtualThread.park" {
		t.Fatalf("candidate_method = %v", evidence["candidate_method"])
	}
}

func TestSMRUnresolved_TaggedAndAddressBoth(t *testing.T) {
	t.Parallel()
	bundle := makeBundle(0, []models.ThreadSnapshot{javaSnapshot("a", models.ThreadStateRunnable, "x")})
	bundle.Metadata["smr"] = map[string]any{
		"unresolved": []any{
			map[string]any{
				"section_line": 12,
				"line":         "0xdead-zombie",
			},
		},
		"addresses_unresolved": []any{
			map[string]any{
				"section_line":      14,
				"address":           "0xfeedface",
				"tagged_unresolved": false,
			},
			// This one is already tagged in `unresolved` and should be
			// skipped to avoid duplicates.
			map[string]any{
				"section_line":      15,
				"address":           "0xdeadbeef",
				"tagged_unresolved": true,
			},
		},
	}
	result, err := Analyze([]models.ThreadDumpBundle{bundle}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	if got := countFindingsWithCode(result.Metadata.Findings, "SMR_UNRESOLVED_THREAD"); got != 2 {
		t.Fatalf("SMR_UNRESOLVED_THREAD count = %d, want 2", got)
	}
	smrTable, ok := result.Tables["smr_unresolved_threads"].([]map[string]any)
	if !ok {
		t.Fatalf("smr table missing or wrong type: %T", result.Tables["smr_unresolved_threads"])
	}
	kinds := map[string]int{}
	for _, row := range smrTable {
		kinds[row["kind"].(string)]++
	}
	if kinds["tagged"] != 1 || kinds["address_unresolved"] != 1 {
		t.Fatalf("kinds = %v, want one each", kinds)
	}
}

func TestIncompleteHistogram_FiresWhenIncompleteFlagSet(t *testing.T) {
	t.Parallel()
	bundle := makeBundle(0, []models.ThreadSnapshot{javaSnapshot("a", models.ThreadStateRunnable, "x")})
	bundle.Metadata["class_histogram"] = map[string]any{
		"classes": []any{
			map[string]any{
				"rank":       1,
				"class_name": "[B",
				"instances":  1234,
				"bytes":      456789,
			},
		},
		"incomplete":        true,
		"incomplete_reason": "histogram cut off mid-row",
		"partial_tail_line": "  9000:  some.class",
		"last_rank":         8999,
		"total_rows":        9001,
	}
	result, err := Analyze([]models.ThreadDumpBundle{bundle}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	finding := findingByCode(result.Metadata.Findings, "INCOMPLETE_HISTOGRAM")
	if finding == nil {
		t.Fatal("INCOMPLETE_HISTOGRAM missing")
	}
	if finding["message"] != "histogram cut off mid-row" {
		t.Fatalf("message = %v, want histogram cut off mid-row", finding["message"])
	}
	rows, ok := result.Tables["class_histogram_top_classes"].([]map[string]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("histogram rows missing")
	}
	if rows[0]["bytes"] != 456789 {
		t.Fatalf("bytes = %v", rows[0]["bytes"])
	}
}

func TestIncompleteHistogram_FallbackMessage(t *testing.T) {
	t.Parallel()
	bundle := makeBundle(0, []models.ThreadSnapshot{javaSnapshot("a", models.ThreadStateRunnable, "x")})
	bundle.Metadata["class_histogram"] = map[string]any{
		"classes":    []any{},
		"incomplete": true,
	}
	result, err := Analyze([]models.ThreadDumpBundle{bundle}, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	finding := findingByCode(result.Metadata.Findings, "INCOMPLETE_HISTOGRAM")
	if finding == nil {
		t.Fatal("INCOMPLETE_HISTOGRAM missing")
	}
	if finding["message"] != "Class histogram block is incomplete (likely truncated source)." {
		t.Fatalf("fallback message wrong: %v", finding["message"])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Series + tables + findings ordering
// ─────────────────────────────────────────────────────────────────────────────

func TestStateDistribution_PerDump(t *testing.T) {
	t.Parallel()
	bundles := []models.ThreadDumpBundle{
		makeBundle(0, []models.ThreadSnapshot{
			javaSnapshot("a", models.ThreadStateRunnable, "x"),
			javaSnapshot("b", models.ThreadStateBlocked, "y"),
		}),
		makeBundle(1, []models.ThreadSnapshot{
			javaSnapshot("a", models.ThreadStateRunnable, "x"),
		}),
	}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	rows, ok := result.Series["state_distribution_per_dump"].([]map[string]any)
	if !ok {
		t.Fatalf("state_distribution_per_dump missing")
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	counts0 := rows[0]["counts"].(map[string]int)
	if counts0["RUNNABLE"] != 1 || counts0["BLOCKED"] != 1 {
		t.Fatalf("dump 0 counts = %v", counts0)
	}
	counts1 := rows[1]["counts"].(map[string]int)
	if counts1["RUNNABLE"] != 1 {
		t.Fatalf("dump 1 counts = %v", counts1)
	}
}

func TestThreadPersistenceSeries_RespectsTopN(t *testing.T) {
	t.Parallel()
	// 5 threads, ask for top 2.
	snaps := []models.ThreadSnapshot{}
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		snaps = append(snaps, javaSnapshot(name, models.ThreadStateRunnable, "x"))
	}
	bundles := []models.ThreadDumpBundle{makeBundle(0, snaps)}
	result, err := Analyze(bundles, Options{TopN: 2})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	rows := result.Series["thread_persistence"].([]map[string]any)
	if len(rows) != 2 {
		t.Fatalf("thread_persistence rows = %d, want 2", len(rows))
	}
}

func TestSummary_LanguagesAndFormatsAreSorted(t *testing.T) {
	t.Parallel()
	mk := func(idx int, lang, fmtID string) models.ThreadDumpBundle {
		b := makeBundle(idx, []models.ThreadSnapshot{javaSnapshot("a", models.ThreadStateRunnable, "x")})
		b.Language = lang
		b.SourceFormat = fmtID
		return b
	}
	bundles := []models.ThreadDumpBundle{
		mk(0, "python", "python_pyspy"),
		mk(1, "java", "java_jstack"),
	}
	// Use FormatOverride-style flow by skipping mixed-format detection
	// — analyzer doesn't enforce mixed-format; that's the registry's
	// job. Here we just check sort order.
	result, err := Analyze(bundles, Options{})
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

func TestFindings_OrderingIsHeuristicsThenPersistenceThenJVM(t *testing.T) {
	t.Parallel()
	// Build a fixture that activates THREAD_CONGESTION + LONG_RUNNING +
	// VIRTUAL_THREAD_CARRIER_PINNING simultaneously, then check the
	// canonical ordering required by the renderer.
	mkBundle := func(idx int) models.ThreadDumpBundle {
		blocker := javaSnapshot("blocker-"+itoa(idx), models.ThreadStateWaiting, "wait")
		runner := javaSnapshot("worker-1", models.ThreadStateRunnable, "compute", "loop")
		runner.Metadata["carrier_pinning"] = map[string]any{
			"candidate_method": "park",
			"top_frame":        "f",
			"reason":           "r",
		}
		// Add 8 runnable padding threads so 1 waiter still trips the
		// 10% congestion threshold (1 of 10 = 10% — strict > so add
		// another waiter too, total 2 of 10 = 20%).
		snaps := []models.ThreadSnapshot{blocker, runner}
		for i := 0; i < 8; i++ {
			snaps = append(snaps, javaSnapshot("pad-"+itoa(i), models.ThreadStateRunnable, "x"))
		}
		// One more waiter so > 10%.
		snaps = append(snaps, javaSnapshot("blocker2-"+itoa(idx), models.ThreadStateWaiting, "wait"))
		return makeBundle(idx, snaps)
	}
	bundles := []models.ThreadDumpBundle{mkBundle(0), mkBundle(1), mkBundle(2)}
	result, err := Analyze(bundles, Options{})
	if err != nil {
		t.Fatalf("Analyze err: %v", err)
	}
	codes := []string{}
	for _, f := range result.Metadata.Findings {
		codes = append(codes, f["code"].(string))
	}
	// Heuristic findings must come before LONG_RUNNING; LONG_RUNNING
	// must come before VIRTUAL_THREAD_CARRIER_PINNING.
	idxOf := func(code string) int {
		for i, c := range codes {
			if c == code {
				return i
			}
		}
		return -1
	}
	if idxOf("THREAD_CONGESTION_DETECTED") < 0 {
		t.Fatalf("missing THREAD_CONGESTION_DETECTED; codes=%v", codes)
	}
	if idxOf("LONG_RUNNING_THREAD") < 0 {
		t.Fatalf("missing LONG_RUNNING_THREAD; codes=%v", codes)
	}
	if idxOf("VIRTUAL_THREAD_CARRIER_PINNING") < 0 {
		t.Fatalf("missing VIRTUAL_THREAD_CARRIER_PINNING; codes=%v", codes)
	}
	if !(idxOf("THREAD_CONGESTION_DETECTED") < idxOf("LONG_RUNNING_THREAD")) {
		t.Fatalf("heuristic must come before persistence; codes=%v", codes)
	}
	if !(idxOf("LONG_RUNNING_THREAD") < idxOf("VIRTUAL_THREAD_CARRIER_PINNING")) {
		t.Fatalf("persistence must come before JVM metadata; codes=%v", codes)
	}
}

func TestRoundToEvenBankerRounding(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in       float64
		decimals int
		want     float64
	}{
		{2.5, 0, 2}, // banker's rounding: 2.5 → 2 (even)
		{3.5, 0, 4}, // 3.5 → 4 (even)
		{1.25, 1, 1.2},
		{1.35, 1, 1.4},
		{0.0, 4, 0.0},
		{0.20004, 4, 0.2},
	}
	for _, c := range cases {
		got := roundToEven(c.in, c.decimals)
		if got != c.want {
			t.Fatalf("roundToEven(%v, %d) = %v, want %v", c.in, c.decimals, got, c.want)
		}
	}
}
