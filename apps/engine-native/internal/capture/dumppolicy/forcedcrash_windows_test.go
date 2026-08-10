//go:build windows

package dumppolicy

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"unsafe"
)

// The canary is what the harness searches for in OS crash artifacts. It is a
// fixed literal rather than a random value so the PowerShell side can grep for
// it without the two agreeing on a channel.
const ForcedCrashCanary = "ARCHSCOPE-SEC10-CANARY-9f2c41d8"

const (
	crashChildEnv = "ARCHSCOPE_SEC10_FORCED_CRASH_CHILD"
	crashOptInEnv = "ARCHSCOPE_SEC10_FORCED_CRASH"
)

// TestSEC10ForcedCrashProbe is the evidence half of SEC-10. A policy read that
// returns the expected value proves only that the registry says what we
// expect; this crashes a process that is genuinely holding plaintext and lets
// the harness look at what the OS actually wrote.
//
// It is opt-in because it deliberately terminates a process abnormally, which
// is not something an ordinary `go test ./...` run should do. The harness in
// scripts/verify-sec10-dump-exclusion.ps1 sets ARCHSCOPE_SEC10_FORCED_CRASH
// and inspects the artifacts afterwards.
func TestSEC10ForcedCrashProbe(t *testing.T) {
	if os.Getenv(crashChildEnv) == "1" {
		crashHoldingPlaintext()
		return
	}
	if os.Getenv(crashOptInEnv) != "1" {
		t.Skip("set ARCHSCOPE_SEC10_FORCED_CRASH=1 to run the forced-crash probe")
	}

	// The preflight must verify here, or the crash proves nothing about the
	// state body capture would actually run in.
	if result := Current(); !result.Verified() {
		t.Fatalf("preflight did not verify, so a forced crash would not exercise the protected state: %s (%s)",
			result.Reason, result.Detail)
	}

	command := exec.Command(os.Args[0], "-test.run=TestSEC10ForcedCrashProbe")
	command.Env = append(os.Environ(), crashChildEnv+"=1")
	output, err := command.CombinedOutput()

	// The child must die abnormally. A clean exit would mean the probe never
	// produced the condition the harness is about to look for.
	if err == nil {
		t.Fatalf("the probe child exited cleanly; no crash was produced. output=%s", output)
	}
	// It must also have reached the point where it held the canary, otherwise
	// an absent artifact says nothing about plaintext.
	if !strings.Contains(string(output), "holding-canary") {
		t.Fatalf("the probe child crashed before it held the canary; artifact absence would not be evidence. output=%s", output)
	}
	t.Logf("probe child crashed as required (%v) while holding the canary", err)
}

// crashHoldingPlaintext puts the canary in live heap memory, makes sure the
// compiler cannot discard it, and then takes an access violation. A Go panic
// would unwind and exit through the runtime, which is not the fault path
// Windows Error Reporting and LocalDumps respond to.
func crashHoldingPlaintext() {
	buffer := []byte(strings.Repeat(ForcedCrashCanary+" ", 64))
	// Announce readiness on stdout before crashing so the parent can tell
	// "crashed while holding plaintext" from "crashed on the way there".
	os.Stdout.WriteString("holding-canary " + string(buffer[:len(ForcedCrashCanary)]) + "\n")
	_ = os.Stdout.Sync()

	// Keep the buffer reachable across the fault.
	keepAlive(buffer)
	var pointer *int
	*pointer = len(buffer)
}

//go:noinline
func keepAlive(buffer []byte) {
	// Touching the memory through an escaping pointer stops the collector and
	// the optimizer from treating the allocation as dead before the fault.
	if len(buffer) > 0 {
		_ = *(*byte)(unsafe.Pointer(&buffer[0]))
	}
}
