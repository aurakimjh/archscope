//go:build !windows

package browsercache

import "fmt"

type systemBackend struct{}

func (systemBackend) Available() bool { return false }
func (systemBackend) Entries() ([]Entry, error) {
	return nil, fmt.Errorf("WinINet cleanup requires Windows")
}
func (systemBackend) Delete(Entry) error { return fmt.Errorf("WinINet cleanup requires Windows") }
