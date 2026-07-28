//go:build t581e2e

package main

import (
	"os"
	"strconv"
	"strings"
)

// windowsAdditionalBrowserArgs exposes WebView2 automation only in the
// dedicated T-581 acceptance build and only when the operator chooses an
// explicit loopback debugging port. Normal production builds compile the
// no-op implementation in webview_e2e_default.go.
func windowsAdditionalBrowserArgs() []string {
	value := strings.TrimSpace(os.Getenv("ARCHSCOPE_E2E_CDP_PORT"))
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return nil
	}
	return []string{
		"--remote-debugging-address=127.0.0.1",
		"--remote-debugging-port=" + strconv.Itoa(port),
	}
}
