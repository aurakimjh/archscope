//go:build !windows

package cases

import (
	"golang.org/x/sys/unix"
	"os"
)

// Nonblocking open prevents a concurrently swapped FIFO from hanging intake.
func openSource(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW, 0)
}
