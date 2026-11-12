package localconvert

import (
	"golang.org/x/sys/windows"
	"os"
	"strconv"
	"strings"
	"testing"
)

func assertChildStopped(t *testing.T, path string) {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	pid, e := strconv.Atoi(strings.TrimSpace(string(b)))
	if e != nil {
		t.Fatal(e)
	}
	p, e := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if e != nil {
		return
	}
	defer windows.CloseHandle(p)
	status, e := windows.WaitForSingleObject(p, 3000)
	if e != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatal("converter child survived cancellation", pid, e)
	}
}
