//go:build !windows

package analysis

import (
	"golang.org/x/sys/unix"
	"os"
)

// A kernel-owned lock is released on crash. Never collect another service's
// staging files merely by their age or a reused process ID.
func lockStore(file *os.File) error { return unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
