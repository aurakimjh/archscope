//go:build !windows

package dumppolicy

// Live capture does not run outside Windows, so there is no supported
// configuration in which plaintext bodies are retained here. Reporting
// unavailable is the honest answer: nothing was checked, and nothing needed to
// be, because the session that would have retained the plaintext cannot start
// on this platform either.
func current() Result {
	return unavailable(ReasonUnsupportedPlatform,
		"live capture and its crash-dump preflight are supported on Windows only")
}
