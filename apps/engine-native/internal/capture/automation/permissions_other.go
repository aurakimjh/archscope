//go:build !windows

package automation

import "os"

func ownerOnly(path string) error { return os.Chmod(path, 0600) }
