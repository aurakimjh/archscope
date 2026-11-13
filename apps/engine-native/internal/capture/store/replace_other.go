//go:build !windows

package store

import "os"

func renameCaptureFile(source, destination string) error {
	return os.Rename(source, destination)
}
