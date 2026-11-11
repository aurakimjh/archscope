//go:build !windows

package distribution

// A Windows desktop bundle can only be built/preflighted on its native OS.
// Other hosts exercise structural negative gates, never native version evidence.
func checkRuntimeVersion(string, string) error { return nil }
