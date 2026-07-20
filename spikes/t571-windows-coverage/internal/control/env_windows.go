//go:build windows

package control

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// IsElevated reports whether the current process runs with an elevated
// (administrator) token. Uses the well-known `net session` probe, which fails
// with access-denied for non-elevated tokens and is available on every
// Windows SKU without extra privileges of its own.
func IsElevated() bool {
	// `whoami /groups` contains the "S-1-16-12288" (High Mandatory Level)
	// integrity SID when elevated. This avoids taking a token handle.
	out, err := exec.Command("whoami", "/groups").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "S-1-16-12288")
}

// OSVersion returns a human-readable Windows build string.
func OSVersion() string {
	out, err := exec.Command("cmd", "/c", "ver").Output()
	if err != nil {
		return "windows-unknown"
	}
	return strings.TrimSpace(string(out))
}

// SampleProcessorTime returns the average "% Processor Time" for _Total over
// the given window using typeperf, which ships with Windows. Returned value is
// a percentage of total CPU capacity (0..100). This is the CAP-5 primitive:
// the orchestrator samples once at baseline and once during capture and the
// judge compares the delta.
func SampleProcessorTime(window time.Duration) (float64, error) {
	// typeperf takes one sample per -si second; ask for 3 samples and average
	// the middle-ground reading. -sc bounds the run so it terminates.
	secs := int(window.Seconds())
	if secs < 1 {
		secs = 1
	}
	cmd := exec.Command("typeperf",
		`\Processor(_Total)\% Processor Time`,
		"-si", "1", "-sc", strconv.Itoa(secs))
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("typeperf: %w", err)
	}
	var sum float64
	var n int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, `"(PDH-CSV`) || strings.Contains(line, "Processor Time") {
			continue
		}
		// Format: "MM/DD/YYYY HH:MM:SS.mmm","<value>"
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		v := strings.Trim(strings.TrimSpace(parts[len(parts)-1]), `"`)
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			continue
		}
		sum += f
		n++
	}
	if n == 0 {
		return 0, fmt.Errorf("typeperf produced no samples")
	}
	return sum / float64(n), nil
}
