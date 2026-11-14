package store

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// Windows readers can briefly deny delete sharing while capture files are read.
// Retain atomic replacement and its old destination; never delete it first.
func renameCaptureFile(source, destination string) error {
	deadline := time.Now().Add(time.Second)
	delay := 10 * time.Millisecond
	for {
		err := os.Rename(source, destination)
		if err == nil || (!errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_LOCK_VIOLATION)) {
			return err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return err
		}
		time.Sleep(min(delay, remaining))
		delay = min(delay*2, 100*time.Millisecond)
	}
}
