package dumppolicy

import (
	"runtime"
	"strings"
	"testing"
)

// The preflight must never report verified on a platform where it did not
// actually check anything. Live capture is Windows only, so every other
// platform has to say so rather than defaulting to permissive.
func TestCurrentIsUnavailableOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this asserts the non-Windows contract")
	}
	result := Current()
	if result.Verified() {
		t.Fatalf("Current() verified on %s; the preflight must fail closed where it cannot check", runtime.GOOS)
	}
	if result.State != StateUnavailable {
		t.Fatalf("State=%q, want %q", result.State, StateUnavailable)
	}
	if result.Reason != ReasonUnsupportedPlatform {
		t.Fatalf("Reason=%q, want %q", result.Reason, ReasonUnsupportedPlatform)
	}
}

// Verified must be the single predicate every call site branches on. A state
// value that is neither verified nor a recognized unavailable token must not
// read as permission.
func TestVerifiedOnlyForVerifiedState(t *testing.T) {
	for _, state := range []State{StateUnavailable, State(""), State("partial"), State("VERIFIED")} {
		if (Result{State: state}).Verified() {
			t.Fatalf("Result{State:%q}.Verified() = true, want false", state)
		}
	}
	if !(Result{State: StateVerified}).Verified() {
		t.Fatal("Result{State: StateVerified}.Verified() = false")
	}
}

// Detail reaches the store manifest and the renderer, so it must stay free of
// paths and identifiers. This pins the constructors rather than every possible
// string, which is what the callers in this package actually produce.
func TestResultDetailCarriesNoPathOrIdentifier(t *testing.T) {
	results := []Result{
		Current(),
		unavailable(ReasonPolicyUnreadable, "the Windows Error Reporting LocalDumps policy could not be read"),
		verified("no LocalDumps policy collects a dump for this application"),
	}
	for _, result := range results {
		for _, fragment := range []string{"/", `\`, "C:", "session", ".exe"} {
			if strings.Contains(result.Detail, fragment) {
				t.Fatalf("Detail %q contains %q; the manifest and renderer must not receive paths or identifiers", result.Detail, fragment)
			}
		}
	}
}

// An unavailable result must always name a reason: the renderer maps the token
// to localized prose, and an empty token would render as an unexplained
// refusal.
func TestUnavailableAlwaysCarriesAReason(t *testing.T) {
	result := Current()
	if !result.Verified() && result.Reason == ReasonNone {
		t.Fatal("unavailable result carries no reason token")
	}
}
