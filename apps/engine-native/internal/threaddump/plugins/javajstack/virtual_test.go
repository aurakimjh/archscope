package javajstack

import (
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// T-227 — virtual-thread carrier pinning detection.
// T-229 — native-method extraction.

func TestIsVirtualThreadDetectsName(t *testing.T) {
	rec := threadDumpRecord{ThreadName: "VirtualThread #42"}
	if !isVirtualThread(rec) {
		t.Fatal("expected virtual thread by name")
	}
}

func TestIsVirtualThreadDetectsRawBlock(t *testing.T) {
	rec := threadDumpRecord{
		ThreadName: "ForkJoinPool-1-worker-1",
		RawBlock:   "Carrying virtual thread #123",
	}
	if !isVirtualThread(rec) {
		t.Fatal("expected virtual thread by raw block content")
	}
}

func TestIsVirtualThreadFalseForPlain(t *testing.T) {
	rec := threadDumpRecord{ThreadName: "main", RawBlock: "Just a normal thread"}
	if isVirtualThread(rec) {
		t.Fatal("plain thread mis-tagged as virtual")
	}
}

func TestNativeMethodReturnsFirstMatch(t *testing.T) {
	rec := threadDumpRecord{
		Stack: []string{
			"sun.misc.Unsafe.park(Native Method)",
			"com.acme.Foo.run(Foo.java:10)",
		},
	}
	if got := nativeMethod(rec); got != "sun.misc.Unsafe.park(Native Method)" {
		t.Fatalf("got %q", got)
	}
}

func TestNativeMethodEmptyForUserStack(t *testing.T) {
	rec := threadDumpRecord{
		Stack: []string{"com.acme.Foo.run(Foo.java:10)"},
	}
	if got := nativeMethod(rec); got != "" {
		t.Fatalf("got %q, expected empty", got)
	}
}

func TestCarrierPinningSurfacesNonJDKCandidate(t *testing.T) {
	frames := []models.StackFrame{
		javaFrame("java.lang.VirtualThread", "parkOnCarrierThread"),
		javaFrame("com.acme.BlockingService", "call"),
	}
	rawBlock := `"ForkJoinPool-1-worker-1" #20
   java.lang.Thread.State: RUNNABLE
   Carrying virtual thread #123
	at java.lang.VirtualThread.parkOnCarrierThread(VirtualThread.java:680)
	at com.acme.BlockingService.call(BlockingService.java:77)`
	got := carrierPinning(rawBlock, frames)
	if got == nil {
		t.Fatal("carrier pinning should be detected")
	}
	if got["candidate_method"] != "com.acme.BlockingService.call" {
		t.Fatalf("candidate = %v", got["candidate_method"])
	}
	if got["reason"] != "virtual_thread_carrier_or_pinning_marker" {
		t.Fatalf("reason = %v", got["reason"])
	}
}

func TestCarrierPinningRequiresVirtualMarker(t *testing.T) {
	frames := []models.StackFrame{javaFrame("com.acme.Foo", "run")}
	if got := carrierPinning("normal thread", frames); got != nil {
		t.Fatalf("expected nil for plain thread, got %+v", got)
	}
}

func TestCarrierPinningRequiresCarrierOrPinnToken(t *testing.T) {
	frames := []models.StackFrame{javaFrame("com.acme.Foo", "run")}
	if got := carrierPinning("virtual thread alone is not enough", frames); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestFirstNonJDKFrameSkipsJDK(t *testing.T) {
	frames := []models.StackFrame{
		javaFrame("java.lang.Thread", "sleep"),
		javaFrame("sun.nio.ch.EPoll", "wait"),
		javaFrame("com.acme.Worker", "run"),
	}
	if got := firstNonJDKFrame(frames); got != "com.acme.Worker.run" {
		t.Fatalf("got %q", got)
	}
}

func TestFirstNonJDKFrameAllJDKReturnsEmpty(t *testing.T) {
	frames := []models.StackFrame{
		javaFrame("java.lang.Thread", "sleep"),
		javaFrame("sun.nio.ch.EPoll", "wait"),
	}
	if got := firstNonJDKFrame(frames); got != "" {
		t.Fatalf("got %q, expected empty", got)
	}
}
