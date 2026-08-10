// Package dumppolicy implements the SEC-10 crash-dump exclusion preflight.
//
// Retaining plaintext HTTP request and response bodies inside a MITM proxy is
// only a bounded exposure if the operating system will not write this
// process's memory to disk when it crashes. This package answers exactly one
// question — "is that established for this process right now?" — and it
// answers with state re-read from the OS rather than with the fact that a
// suppression call returned success.
//
// The mechanism is process-wide suppression rather than per-region exclusion.
// Windows offers WerRegisterExcludedMemoryBlock, which excludes a fixed
// address range, but body buffers are ordinary Go heap allocations that the
// collector may move and reuse, so no stable range can be registered and any
// registration would silently cover the wrong memory. See RD-135.
//
// The consequence is deliberate: on a machine whose policy collects a dump for
// this executable and cannot be overridden without administrator rights, the
// preflight reports unavailable. Body capture then stays off rather than
// proceeding on a claim that cannot be checked.
//
// Live capture is Windows only (SYSTEM_HTTP_CAPTURE 9.4), so every other
// platform reports unavailable without pretending to have checked anything.
package dumppolicy

// State is the preflight outcome. There is no third value: a check that did
// not conclusively establish suppression is unavailable, never "probably fine".
type State string

const (
	StateVerified    State = "verified"
	StateUnavailable State = "unavailable"
)

// Reason tokens form a closed set. The renderer maps them through EN/KO label
// maps and must never display a raw token, so adding a value here requires
// adding it on both sides.
const (
	ReasonNone                 = ""
	ReasonUnsupportedPlatform  = "unsupported_platform"
	ReasonPolicyUnreadable     = "policy_unreadable"
	ReasonLocalDumpsCollecting = "local_dumps_collecting"
	ReasonExclusionFailed      = "wer_exclusion_failed"
	ReasonExclusionUnconfirmed = "wer_exclusion_unconfirmed"
)

// Result is bound to the moment it was produced. Callers must not cache it
// across a session boundary: policy is machine state and can change.
//
// Detail is operator-facing prose. It must never carry a filesystem path, a
// session identifier, or any captured data — this value reaches the manifest
// and the renderer.
type Result struct {
	State  State  `json:"state"`
	Reason string `json:"reason,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// Verified reports whether plaintext body retention is permitted. Every call
// site must branch on this rather than on State equality, so that a future
// state cannot silently read as permission.
func (r Result) Verified() bool { return r.State == StateVerified }

func unavailable(reason, detail string) Result {
	return Result{State: StateUnavailable, Reason: reason, Detail: detail}
}

func verified(detail string) Result {
	return Result{State: StateVerified, Reason: ReasonNone, Detail: detail}
}

// Current runs the preflight against live OS state.
//
// It is intentionally not memoized. Both engine boundaries call it
// independently, and each call re-reads the policy, so a stale or custom
// caller cannot hand a forged pass to either one. The cost is a few registry
// reads at session start, which is not on any per-transaction path.
func Current() Result { return current() }
