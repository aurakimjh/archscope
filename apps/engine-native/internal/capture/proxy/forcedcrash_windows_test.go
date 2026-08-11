//go:build windows

package proxy

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/dumppolicy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"golang.org/x/sys/windows"
)

// These markers are what the PowerShell harness searches for in OS crash
// artifacts and the Application event log. They are fixed so both halves can
// agree without exporting the randomly generated CA private key.
const (
	ForcedCrashBodyCanary  = "ARCHSCOPE-SEC10-BODY-CANARY-9f2c41d8"
	ForcedCrashCAKeyCanary = "ARCHSCOPE-SEC10-CA-KEY-CANARY-70b13e64"
)

const (
	crashChildEnv = "ARCHSCOPE_SEC10_FORCED_CRASH_CHILD"
	crashOptInEnv = "ARCHSCOPE_SEC10_FORCED_CRASH"
)

// Keeping the transaction, authority, and a key-adjacent marker in one global
// object makes the protected state unambiguously live when the fault occurs.
// The actual Authority contains freshly generated CA and leaf private keys;
// the fixed marker lets the external harness detect memory disclosure without
// ever exporting those keys.
var forcedCrashLiveState struct {
	transaction models.CaptureTransaction
	authority   *Authority
	keyMarker   []byte
}

var procRaiseFailFastException = windows.NewLazySystemDLL("kernel32.dll").NewProc("RaiseFailFastException")

// TestSEC10ForcedCrashProbe is the evidence half of SEC-10. The child moves a
// canary through the real request and response body recorder, creates the real
// in-memory MITM authority, and then faults while all three remain reachable.
func TestSEC10ForcedCrashProbe(t *testing.T) {
	if os.Getenv(crashChildEnv) == "1" {
		crashHoldingCaptureState()
		return
	}
	if os.Getenv(crashOptInEnv) != "1" {
		t.Skip("set ARCHSCOPE_SEC10_FORCED_CRASH=1 to run the forced-crash probe")
	}

	if result := dumppolicy.Current(); !result.Verified() {
		t.Fatalf("preflight did not verify, so a forced crash would not exercise the protected state: %s (%s)",
			result.Reason, result.Detail)
	}

	command := exec.Command(os.Args[0], "-test.run=TestSEC10ForcedCrashProbe")
	command.Env = append(os.Environ(), crashChildEnv+"=1")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("the probe child exited cleanly; no crash was produced. output=%s", output)
	}
	// The readiness line deliberately contains no canary or key material; it
	// only proves the child reached the protected state before it faulted.
	if !strings.Contains(string(output), "holding-request-response-bodies-and-ca") {
		t.Fatalf("the probe child crashed before it held capture state. output=%s", output)
	}
	t.Logf("probe child crashed as required (%v) while holding request/response bodies and CA keys", err)
}

func crashHoldingCaptureState() {
	now := time.Now().UTC()
	authority, err := NewAuthority(now)
	if err != nil {
		os.Stdout.WriteString("probe-setup-failed: authority\n")
		return
	}
	if _, err := authority.LeafFor("sec10.invalid", now); err != nil {
		os.Stdout.WriteString("probe-setup-failed: leaf\n")
		return
	}

	request := captureCrashBody("request-" + ForcedCrashBodyCanary)
	response := captureCrashBody("response-" + ForcedCrashBodyCanary)
	forcedCrashLiveState = struct {
		transaction models.CaptureTransaction
		authority   *Authority
		keyMarker   []byte
	}{
		transaction: models.CaptureTransaction{Request: request, Response: response},
		authority:   authority,
		keyMarker:   []byte(strings.Repeat(ForcedCrashCAKeyCanary+" ", 64)),
	}

	os.Stdout.WriteString("holding-request-response-bodies-and-ca\n")
	_ = os.Stdout.Sync()

	// RaiseFailFastException bypasses Go's panic/unwind path and presents an
	// unhandled fail-fast exception to Windows Error Reporting and LocalDumps.
	// A nil-pointer fault in Go would only become a runtime panic and would not
	// establish the OS crash behavior SEC-10 bounds.
	if err := procRaiseFailFastException.Find(); err != nil {
		os.Stdout.WriteString("probe-setup-failed: RaiseFailFastException\n")
		return
	}
	procRaiseFailFastException.Call(0, 0, 0)
	os.Exit(127) // The API is non-returning; reaching this line is a failed probe.
}

func captureCrashBody(value string) models.HTTPMessage {
	recorder := newBodyRecorder(io.NopCloser(bytes.NewReader([]byte(value))), true, DefaultMaxBodyBytes)
	_, _ = io.Copy(io.Discard, recorder)
	message := models.HTTPMessage{}
	recorder.apply(&message, "identity")
	return message
}
