package capture

import (
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/dumppolicy"
)

// The published contract must be a function of the preflight result. A build
// that reports the tier available without a verified check is the failure
// RD-134 was raised against, in the direction that matters.
func TestLiveCaptureContractFollowsThePreflightResult(t *testing.T) {
	verified := LiveCaptureContractFor(dumppolicy.Result{State: dumppolicy.StateVerified})
	if !verified.BodyCaptureAvailable {
		t.Fatal("a verified preflight did not make the tier available")
	}
	if verified.BodyCaptureGate != BodyGateVerified {
		t.Fatalf("gate=%q, want %q", verified.BodyCaptureGate, BodyGateVerified)
	}
	if verified.BodyCaptureGateReason != "" {
		t.Fatalf("a verified gate carried reason %q, want empty", verified.BodyCaptureGateReason)
	}

	blocked := LiveCaptureContractFor(dumppolicy.Result{
		State:  dumppolicy.StateUnavailable,
		Reason: dumppolicy.ReasonLocalDumpsCollecting,
	})
	if blocked.BodyCaptureAvailable {
		t.Fatal("an unavailable preflight still published the tier as available")
	}
	if blocked.BodyCaptureGate != BodyGateUnavailable {
		t.Fatalf("gate=%q, want %q", blocked.BodyCaptureGate, BodyGateUnavailable)
	}
	if blocked.BodyCaptureGateReason != dumppolicy.ReasonLocalDumpsCollecting {
		t.Fatalf("reason=%q, want the preflight's own token", blocked.BodyCaptureGateReason)
	}
}

// A zero-valued result must not read as permission. This covers the case where
// a future caller forgets to run the preflight and passes an empty struct.
func TestZeroPreflightResultDoesNotEnableTheTier(t *testing.T) {
	contract := LiveCaptureContractFor(dumppolicy.Result{})
	if contract.BodyCaptureAvailable {
		t.Fatal("a zero-valued preflight result enabled body capture")
	}
	if contract.BodyCaptureGate != BodyGateUnavailable {
		t.Fatalf("gate=%q, want %q", contract.BodyCaptureGate, BodyGateUnavailable)
	}
}

// The fallback the renderer uses when it cannot read a contract must never
// report the tier as available, whatever the host policy happens to be.
func TestDefaultContractIsFailClosed(t *testing.T) {
	contract := DefaultLiveCaptureContract()
	if contract.BodyCaptureAvailable {
		t.Fatal("the fail-closed default reported body capture as available")
	}
	if contract.SchemaVersion != LiveCaptureContractVersion {
		t.Fatalf("schemaVersion=%d, want %d", contract.SchemaVersion, LiveCaptureContractVersion)
	}
}
