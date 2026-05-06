package javajstack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// CanParse — three detection paths.

func TestCanParseFullThreadHeader(t *testing.T) {
	p := New()
	head := "Full thread dump OpenJDK 64-Bit Server VM\n\"main\" #1\n"
	if !p.CanParse(head) {
		t.Fatal("Full-thread header should be recognised")
	}
}

func TestCanParseNidLine(t *testing.T) {
	p := New()
	head := `"http-nio-8080-exec-1" #25 prio=5 nid=0x00aa runnable`
	if !p.CanParse(head) {
		t.Fatal("nid line should be recognised")
	}
}

func TestCanParseLooseHeaderRequiresTwoMatches(t *testing.T) {
	p := New()
	one := `"main" #1 prio=5 tid=0x0001 RUNNABLE` + "\n"
	two := one + `"worker" #2 prio=5 tid=0x0002 BLOCKED` + "\n"
	if p.CanParse(one) {
		t.Fatal("single loose match should NOT be enough")
	}
	if !p.CanParse(two) {
		t.Fatal("two loose matches should be recognised")
	}
}

func TestCanParseRejectsUnrelated(t *testing.T) {
	if New().CanParse("goroutine 1 [running]:\nmain.main()\n") {
		t.Fatal("non-jstack input mis-detected")
	}
}

// FormatID / Language identity.

func TestFormatIDAndLanguage(t *testing.T) {
	p := New()
	if p.FormatID() != "java_jstack" {
		t.Fatalf("FormatID = %q", p.FormatID())
	}
	if p.Language() != "java" {
		t.Fatalf("Language = %q", p.Language())
	}
}

// End-to-end Parse exercising T-194 (proxy normalization) + T-195
// (state inference).

const dumpWithProxy = `2025-05-04 10:00:00
Full thread dump OpenJDK 64-Bit Server VM (17.0.1+12 mixed mode):

"http-nio-8080-exec-1" #25 prio=5 os_prio=0 tid=0x00007f001 nid=0x00aa runnable [0x00007f]
   java.lang.Thread.State: RUNNABLE
	at sun.nio.ch.EPoll.epollWait(EPoll.java:42)
	at com.example.PaymentService$$EnhancerByCGLIB$$abc123.charge(MyService.java:88)
	at com.example.PaymentController.handle(PaymentController.java:12)
`

func TestPluginAppliesProxyNormalizationAndStateInference(t *testing.T) {
	path := writeFile(t, "td.txt", dumpWithProxy)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(bundle.Snapshots) != 1 {
		t.Fatalf("snapshots = %d", len(bundle.Snapshots))
	}
	snap := bundle.Snapshots[0]

	// T-195: epollWait promoted RUNNABLE -> NETWORK_WAIT.
	if snap.State != models.ThreadStateNetworkWait {
		t.Fatalf("state = %s, want NETWORK_WAIT", snap.State)
	}
	// T-194: CGLIB hash stripped on the second frame.
	if len(snap.StackFrames) < 2 {
		t.Fatalf("frames = %+v", snap.StackFrames)
	}
	proxyFrame := snap.StackFrames[1]
	if proxyFrame.Module == nil || *proxyFrame.Module != "com.example.PaymentService" {
		t.Fatalf("proxy module = %v", proxyFrame.Module)
	}
	if proxyFrame.Function != "charge" {
		t.Fatalf("proxy function = %q", proxyFrame.Function)
	}
}

func TestPluginPopulatesBundleMetadata(t *testing.T) {
	path := writeFile(t, "td.txt", dumpWithProxy)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bundle.SourceFormat != "java_jstack" || bundle.Language != "java" {
		t.Fatalf("bundle identity = %+v", bundle)
	}
	if got, ok := bundle.Metadata["class_histogram_row_limit"]; !ok || got != 500 {
		t.Fatalf("class_histogram_row_limit metadata missing/wrong: %v", got)
	}
}

// ParseAll — multi-section file.

const multiSectionDump = `2026-05-05 10:00:00
Full thread dump OpenJDK 64-Bit Server VM (21.0.1 mixed mode):

"main" #1 prio=5 os_prio=0 tid=0x0001 nid=0x0001 runnable
   java.lang.Thread.State: RUNNABLE
	at com.acme.Main.run(Main.java:10)

2026-05-05 10:00:05
Full thread dump OpenJDK 64-Bit Server VM (21.0.1 mixed mode):

"worker" #1 prio=5 os_prio=0 tid=0x0002 nid=0x0002 runnable
   java.lang.Thread.State: RUNNABLE
	at com.acme.Main.run(Main.java:10)
`

func TestParseAllExpandsMultipleSections(t *testing.T) {
	path := writeFile(t, "multi.txt", multiSectionDump)
	bundles, err := New().ParseAll(path)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	if len(bundles) != 2 {
		t.Fatalf("bundles = %d", len(bundles))
	}
	if bundles[0].Snapshots[0].ThreadName != "main" {
		t.Fatalf("first bundle name = %q", bundles[0].Snapshots[0].ThreadName)
	}
	if bundles[1].Snapshots[0].ThreadName != "worker" {
		t.Fatalf("second bundle name = %q", bundles[1].Snapshots[0].ThreadName)
	}
	if bundles[0].Metadata["raw_timestamp"] != "2026-05-05 10:00:00" {
		t.Fatalf("first raw_timestamp = %v", bundles[0].Metadata["raw_timestamp"])
	}
	if bundles[1].Metadata["raw_timestamp"] != "2026-05-05 10:00:05" {
		t.Fatalf("second raw_timestamp = %v", bundles[1].Metadata["raw_timestamp"])
	}
}

// End-to-end SMR + native + class histogram (T-228 / T-229 / T-230).

const dumpWithMetadata = `Full thread dump OpenJDK 64-Bit Server VM (21.0.1 mixed mode):

"native-reader" #1 prio=5 os_prio=0 tid=0x0001 nid=0x0001 runnable
   java.lang.Thread.State: RUNNABLE
	at java.io.FileInputStream.readBytes(Native Method)
	at com.acme.Reader.read(Reader.java:12)

Threads class SMR info:
  unresolved zombie thread 0x000000001234abcd

 num     #instances         #bytes  class name
-------------------------------------------------------
   1:           100           2400  java.lang.String
   2:            10           1600  com.acme.Order
Total           110           4000
`

func TestParseAllSurfacesSMRNativeAndHistogramMetadata(t *testing.T) {
	path := writeFile(t, "metadata.txt", dumpWithMetadata)
	bundles, err := New().ParseAll(path)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	if len(bundles) != 1 {
		t.Fatalf("bundles = %d", len(bundles))
	}
	bundle := bundles[0]
	smr, ok := bundle.Metadata["smr"].(map[string]any)
	if !ok {
		t.Fatalf("smr metadata missing: %+v", bundle.Metadata)
	}
	if smr["unresolved_count"] != 1 {
		t.Fatalf("smr unresolved_count = %v", smr["unresolved_count"])
	}
	if hist, ok := bundle.Metadata["class_histogram"].(map[string]any); !ok {
		t.Fatal("class_histogram missing")
	} else {
		classes := hist["classes"].([]map[string]any)
		if classes[0]["class_name"] != "java.lang.String" {
			t.Fatalf("first class = %v", classes[0]["class_name"])
		}
	}

	snap := bundle.Snapshots[0]
	if got := snap.Metadata["native_method"]; got != "java.io.FileInputStream.readBytes(Native Method)" {
		t.Fatalf("native_method = %v", got)
	}
}

// End-to-end carrier pinning (T-227).

const dumpWithCarrierPinning = `Full thread dump OpenJDK 64-Bit Server VM (21.0.1 mixed mode):

"ForkJoinPool-1-worker-1" #20 prio=5 os_prio=0 tid=0x0020 nid=0x0020 runnable
   java.lang.Thread.State: RUNNABLE
   Carrying virtual thread #123
	at java.lang.VirtualThread.parkOnCarrierThread(VirtualThread.java:680)
	at com.acme.BlockingService.call(BlockingService.java:77)
`

func TestParseSurfacesCarrierPinning(t *testing.T) {
	path := writeFile(t, "carrier.txt", dumpWithCarrierPinning)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	snap := bundle.Snapshots[0]
	pinning, ok := snap.Metadata["carrier_pinning"].(map[string]any)
	if !ok {
		t.Fatalf("carrier_pinning metadata missing: %+v", snap.Metadata)
	}
	if pinning["candidate_method"] != "com.acme.BlockingService.call (BlockingService.java:77)" {
		t.Fatalf("candidate_method = %v", pinning["candidate_method"])
	}
	if !snap.Metadata["is_virtual_thread"].(bool) {
		t.Fatal("is_virtual_thread should be true")
	}
}

// End-to-end monitor extraction (T-231).

const dumpWithMonitors = `Full thread dump OpenJDK 64-Bit Server VM (21.0.1 mixed mode):

"obj-waiter" #4 prio=5 os_prio=0 tid=0x0004 nid=0x0004 in Object.wait
   java.lang.Thread.State: TIMED_WAITING (on object monitor)
	at java.lang.Object.wait(Native Method)
	- waiting on <0x00000007dddddddd> (a java.lang.Object)
`

func TestParseSurfacesObjectWaitMonitor(t *testing.T) {
	path := writeFile(t, "monitor.txt", dumpWithMonitors)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	snap := bundle.Snapshots[0]
	if snap.LockWaiting == nil {
		t.Fatal("lock_waiting should be populated")
	}
	if snap.LockWaiting.WaitMode != "object_wait" {
		t.Fatalf("wait_mode = %q", snap.LockWaiting.WaitMode)
	}
	if snap.Metadata["monitor_wait_mode"] != "object_wait" {
		t.Fatalf("monitor_wait_mode metadata = %v", snap.Metadata["monitor_wait_mode"])
	}
}

// State coercion: BLOCKED + waiting-to-lock should also flip RUNNABLE
// → LOCK_WAIT (T-231 + T-195 interaction).

const dumpWithLockEntryWait = `Full thread dump OpenJDK 64-Bit Server VM (21.0.1 mixed mode):

"contender" #2 prio=5 os_prio=0 tid=0x0002 nid=0x0002 runnable
   java.lang.Thread.State: RUNNABLE
	at com.acme.Foo.acquire(Foo.java:11)
	- waiting to lock <0x000000076ab62208> (a com.acme.Resource)
`

func TestRunnableContenderPromotedToLockWait(t *testing.T) {
	path := writeFile(t, "contender.txt", dumpWithLockEntryWait)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	snap := bundle.Snapshots[0]
	if snap.State != models.ThreadStateLockWait {
		t.Fatalf("state = %s, want LOCK_WAIT", snap.State)
	}
}

// Frame parsing — make sure native-method markers parse cleanly and
// non-`at ` lines stay out of the stack.

func TestFrameFromJstackParsesQualified(t *testing.T) {
	// Python uses rpartition('.') so module is "com.example.Foo" and
	// function is "bar".
	frame := frameFromJstack("com.example.Foo.bar(Foo.java:42)")
	if frame.Module == nil {
		t.Fatal("module should not be nil")
	}
	if *frame.Module != "com.example.Foo" {
		t.Fatalf("module = %q, want com.example.Foo (rpartition)", *frame.Module)
	}
	if frame.Function != "bar" {
		t.Fatalf("function = %q", frame.Function)
	}
	if frame.File == nil || *frame.File != "Foo.java" {
		t.Fatalf("file = %v", frame.File)
	}
	if frame.Line == nil || *frame.Line != 42 {
		t.Fatalf("line = %v", frame.Line)
	}
}

func TestFrameFromJstackHandlesNativeMethod(t *testing.T) {
	frame := frameFromJstack("sun.misc.Unsafe.park(Native Method)")
	if frame.Module == nil || *frame.Module != "sun.misc.Unsafe" {
		t.Fatalf("module = %v", frame.Module)
	}
	if frame.Function != "park" {
		t.Fatalf("function = %q", frame.Function)
	}
	if frame.File == nil || *frame.File != "Native Method" {
		t.Fatalf("file = %v", frame.File)
	}
	if frame.Line != nil {
		t.Fatalf("line should be nil for native methods, got %v", *frame.Line)
	}
}

func TestFrameFromJstackUnparseableFallsThrough(t *testing.T) {
	frame := frameFromJstack("just random text")
	if frame.Function != "just random text" {
		t.Fatalf("function = %q", frame.Function)
	}
	if frame.Module != nil {
		t.Fatalf("module should be nil, got %v", *frame.Module)
	}
}

// Hardening: a dump containing only a thread-block-style header with no
// `at` lines should still produce a snapshot with an empty stack.

func TestParseEmptyStackThreadBlock(t *testing.T) {
	body := strings.Join([]string{
		`Full thread dump OpenJDK`,
		``,
		`"empty" #1 prio=5 nid=0x0001 runnable`,
		`   java.lang.Thread.State: RUNNABLE`,
		``,
	}, "\n")
	path := writeFile(t, "empty.txt", body)
	bundle, err := New().Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(bundle.Snapshots) != 1 {
		t.Fatalf("snapshots = %d", len(bundle.Snapshots))
	}
	if len(bundle.Snapshots[0].StackFrames) != 0 {
		t.Fatalf("stack frames should be empty: %+v", bundle.Snapshots[0].StackFrames)
	}
}

// ParseAll falls back to single Parse when no Full thread header is seen.

func TestParseAllFallsBackWithoutSectionHeader(t *testing.T) {
	body := `"main" #1 prio=5 os_prio=0 tid=0x0001 nid=0x0001 runnable
   java.lang.Thread.State: RUNNABLE
	at com.acme.Main.run(Main.java:1)
`
	path := writeFile(t, "no-section.txt", body)
	bundles, err := New().ParseAll(path)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	if len(bundles) != 1 {
		t.Fatalf("bundles = %d", len(bundles))
	}
}
