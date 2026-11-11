// Package netpolicy denies app-owned networking and unqualified subprocesses in
// explicit offline sessions. It records attempted operations before allocation.
// OS/WebView traffic still requires independent system-level observation.
package netpolicy

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

var state struct {
	sync.Mutex
	enabled  bool
	attempts map[string]uint64
}

type Report struct {
	Version  string            `json:"version"`
	Offline  bool              `json:"offline"`
	Attempts map[string]uint64 `json:"denied_attempts"`
	Scope    string            `json:"scope"`
}

func Enable() {
	state.Lock()
	defer state.Unlock()
	state.enabled = true
	state.attempts = map[string]uint64{}
}
func Enabled() bool { state.Lock(); defer state.Unlock(); return state.enabled }
func RequireOnline(operation string) error {
	state.Lock()
	defer state.Unlock()
	if !state.enabled {
		return nil
	}
	state.attempts[operation]++
	return fmt.Errorf("OFFLINE_OPERATION_DENIED: %s; use local files/manual collection", operation)
}
func Snapshot() Report {
	state.Lock()
	defer state.Unlock()
	r := Report{Version: "1.0.0", Offline: state.enabled, Attempts: map[string]uint64{}, Scope: "app-owned guarded operations; OS/WebView requires syscall/packet observation"}
	for k, v := range state.attempts {
		r.Attempts[k] = v
	}
	return r
}
func WriteReport(path string) error {
	if path == "" {
		return nil
	}
	b, e := json.MarshalIndent(Snapshot(), "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
