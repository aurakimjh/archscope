package runtimestack

import (
	"strings"
	"testing"
)

const goPanicFixture = `panic: runtime error: invalid memory address or nil pointer dereference

goroutine 42 [running]:
main.(*OrderService).Load(0x0, 0xc00018c000)
	/app/orders/service.go:42 +0x45
main.handleOrder(0xc00018c000)
	/app/orders/handler.go:21 +0x84
net/http.HandlerFunc.ServeHTTP(0x104ea50, 0x1072cc0, 0xc00018c000)
	/usr/local/go/src/net/http/server.go:2136 +0x2f

goroutine 18 [IO wait]:
internal/poll.runtime_pollWait(0x102a8, 0x72)
	/usr/local/go/src/runtime/netpoll.go:343 +0x85
`

func TestParseGoPanicFileFixtureHappyPath(t *testing.T) {
	path := writeTempFile(t, "panic.txt", goPanicFixture)
	records, diags, err := ParseGoPanicFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseGoPanicFile: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %d, want 3 (panic + 2 goroutines)", len(records))
	}
	if records[0].RecordType != "panic" {
		t.Errorf("records[0].RecordType = %q, want panic", records[0].RecordType)
	}
	if records[0].Message == nil || !strings.HasPrefix(*records[0].Message, "runtime error") {
		t.Errorf("records[0].Message = %v", records[0].Message)
	}
	if records[1].RecordType != "goroutine" {
		t.Errorf("records[1].RecordType = %q, want goroutine", records[1].RecordType)
	}
	if records[1].Message == nil || *records[1].Message != "running" {
		t.Errorf("records[1].Message = %v", records[1].Message)
	}
	// goroutine 42 [running] — the running block carries 3 stack
	// entries (Load, handleOrder, ServeHTTP). The trailing
	// indented `\t/app/...` lines are dropped by the `\t` guard.
	if len(records[1].Stack) != 3 {
		t.Errorf("records[1].Stack len = %d, want 3 — got %v", len(records[1].Stack), records[1].Stack)
	}
	if records[1].Stack[0] != "main.(*OrderService).Load" {
		t.Errorf("records[1].Stack[0] = %q", records[1].Stack[0])
	}
	if records[2].Message == nil || *records[2].Message != "IO wait" {
		t.Errorf("records[2].Message = %v", records[2].Message)
	}
	if records[0].Runtime != "go" {
		t.Errorf("Runtime = %q, want go", records[0].Runtime)
	}
	if diags.ParsedRecords != 3 {
		t.Errorf("ParsedRecords = %d, want 3", diags.ParsedRecords)
	}
}

func TestParseGoPanicFileFlagsOutsideBlockLine(t *testing.T) {
	body := "noise outside any panic block\npanic: oops\nmain.foo()\n"
	path := writeTempFile(t, "noise.txt", body)
	_, diags, err := ParseGoPanicFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseGoPanicFile: %v", err)
	}
	if diags.SkippedByReason[ReasonOutsideGoPanic] != 1 {
		t.Errorf("expected 1 OUTSIDE_GO_PANIC, got %v", diags.SkippedByReason)
	}
}

func TestParseGoPanicFileStrictMode(t *testing.T) {
	body := "noise\n"
	path := writeTempFile(t, "strict.txt", body)
	_, _, err := ParseGoPanicFile(path, Options{Strict: true})
	if err == nil {
		t.Fatalf("expected strict-mode error")
	}
	if !strings.Contains(err.Error(), ReasonOutsideGoPanic) {
		t.Errorf("err = %v", err)
	}
}

func TestParseGoPanicFileNoFrameSentinel(t *testing.T) {
	// goroutine header with no follow-up frame lines → "(no-frame)"
	// in the signature.
	body := "goroutine 1 [running]:\n"
	path := writeTempFile(t, "noframe.txt", body)
	records, _, err := ParseGoPanicFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseGoPanicFile: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if records[0].Signature != "goroutine|(no-frame)" {
		t.Errorf("Signature = %q", records[0].Signature)
	}
	if len(records[0].Stack) != 0 {
		t.Errorf("Stack should be empty, got %v", records[0].Stack)
	}
}
