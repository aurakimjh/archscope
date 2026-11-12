//go:build !windows

package localconvert

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func assertChildStopped(t *testing.T, path string) {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal("child never started", e)
	}
	pid, e := strconv.Atoi(strings.TrimSpace(string(b)))
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 30; i++ {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("converter child survived cancellation", pid)
}
