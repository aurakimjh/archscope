//go:build !windows

package certstore

import "fmt"

type systemBackend struct{}

func (systemBackend) Install(string, []byte) error {
	return fmt.Errorf("system capture CA trust is only implemented on Windows")
}

func (systemBackend) Remove(string, []byte) error {
	return fmt.Errorf("system capture CA trust is only implemented on Windows")
}
