//go:build !windows

package control

import (
	"fmt"
	"runtime"
	"time"
)

// The spike only runs meaningfully on Windows. These stubs exist so the whole
// module compiles and `go vet` passes on Linux/macOS (used for the cloud
// cross-compile gate), and so a developer who runs a probe on the wrong OS
// gets an honest error instead of a silent bogus number.

// IsElevated always returns false off Windows.
func IsElevated() bool { return false }

// OSVersion returns the non-Windows GOOS so misuse is obvious in the output.
func OSVersion() string { return runtime.GOOS + "-not-windows" }

// SampleProcessorTime is unavailable off Windows.
func SampleProcessorTime(time.Duration) (float64, error) {
	return 0, fmt.Errorf("SampleProcessorTime requires windows (GOOS=%s)", runtime.GOOS)
}
