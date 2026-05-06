package javajstack

import "testing"

// T-231 — monitor handle extraction.

func TestExtractLockedHandle(t *testing.T) {
	block := `"worker" #1
   java.lang.Thread.State: RUNNABLE
	at com.acme.Foo.run(Foo.java:10)
	- locked <0x000000076ab62208> (a com.acme.Resource)
`
	holds, waiting := extractLockHandles(block)
	if waiting != nil {
		t.Fatalf("waiting should be nil, got %+v", waiting)
	}
	if len(holds) != 1 {
		t.Fatalf("expected 1 hold, got %d", len(holds))
	}
	if holds[0].LockID != "0x000000076ab62208" {
		t.Fatalf("lock id = %q", holds[0].LockID)
	}
	if holds[0].LockClass == nil || *holds[0].LockClass != "com.acme.Resource" {
		t.Fatalf("lock class = %v", holds[0].LockClass)
	}
	if holds[0].WaitMode != waitModeLockedOwner {
		t.Fatalf("wait mode = %q", holds[0].WaitMode)
	}
}

func TestExtractWaitingToLockEmitsLockEntryWait(t *testing.T) {
	block := `"contender" #2
   java.lang.Thread.State: BLOCKED
	at com.acme.Foo.acquire(Foo.java:11)
	- waiting to lock <0x000000076ab62208> (a com.acme.Resource)
`
	holds, waiting := extractLockHandles(block)
	if len(holds) != 0 {
		t.Fatalf("expected 0 holds, got %d", len(holds))
	}
	if waiting == nil {
		t.Fatal("waiting handle should not be nil")
	}
	if waiting.WaitMode != waitModeLockEntryWait {
		t.Fatalf("wait mode = %q, want lock_entry_wait", waiting.WaitMode)
	}
	if waiting.LockID != "0x000000076ab62208" {
		t.Fatalf("lock id = %q", waiting.LockID)
	}
}

func TestExtractWaitingOnEmitsObjectWait(t *testing.T) {
	block := `"obj-waiter" #3
   java.lang.Thread.State: TIMED_WAITING (on object monitor)
	at java.lang.Object.wait(Native Method)
	- waiting on <0x00000007dddddddd> (a java.lang.Object)
`
	holds, waiting := extractLockHandles(block)
	if len(holds) != 0 {
		t.Fatalf("expected 0 holds, got %d", len(holds))
	}
	if waiting == nil || waiting.WaitMode != waitModeObjectWait {
		t.Fatalf("wait mode = %v", waiting)
	}
}

func TestExtractParkingEmitsParkingConditionWait(t *testing.T) {
	block := `"parker" #4
   java.lang.Thread.State: WAITING (parking)
	at sun.misc.Unsafe.park(Native Method)
	- parking to wait for <0x000000076ab63333> (a java.util.concurrent.locks.AbstractQueuedSynchronizer$ConditionObject)
`
	_, waiting := extractLockHandles(block)
	if waiting == nil || waiting.WaitMode != waitModeParkingWait {
		t.Fatalf("wait mode = %v", waiting)
	}
}

func TestReentrantLockIsNotReportedAsContention(t *testing.T) {
	// Same lock id appears as both locked and waiting-to-lock; treated
	// as re-entrant (waiting dropped).
	block := `"reentrant" #5
   java.lang.Thread.State: RUNNABLE
	at com.acme.Foo.deeper(Foo.java:30)
	- locked <0x0000000700000001> (a com.acme.Lock)
	at com.acme.Foo.outer(Foo.java:20)
	- waiting to lock <0x0000000700000001> (a com.acme.Lock)
`
	holds, waiting := extractLockHandles(block)
	if len(holds) != 1 {
		t.Fatalf("expected 1 hold, got %d", len(holds))
	}
	if waiting != nil {
		t.Fatalf("waiting should be cleared on re-entrant lock, got %+v", waiting)
	}
}

func TestDuplicateLockedDeduped(t *testing.T) {
	block := `"dup" #6
	- locked <0x0000000700000010> (a com.acme.Lock)
	- locked <0x0000000700000010> (a com.acme.Lock)
`
	holds, _ := extractLockHandles(block)
	if len(holds) != 1 {
		t.Fatalf("expected 1 hold (dedup), got %d", len(holds))
	}
}

func TestFirstWaitWinsAcrossPatterns(t *testing.T) {
	// Both `waiting to lock` and `parking to wait for` appear; the
	// first wins (lock_entry_wait per Python's tuple ordering).
	block := `"mixed" #7
	- waiting to lock <0x0000000700000020> (a A)
	- parking to wait for <0x0000000700000021> (a B)
`
	_, waiting := extractLockHandles(block)
	if waiting == nil || waiting.WaitMode != waitModeLockEntryWait {
		t.Fatalf("expected lock_entry_wait first, got %+v", waiting)
	}
}
