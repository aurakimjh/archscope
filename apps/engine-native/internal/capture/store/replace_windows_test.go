package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func holdWindowsReader(t *testing.T, path string) windows.Handle {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	// A normal reader can share reads/writes while temporarily denying delete
	// sharing, which Windows requires to replace an existing destination.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	return handle
}

func TestWindowsRollingAppendWaitsForManifestReader(t *testing.T) {
	s := newTestStore(t, Config{MaxRecords: 2})
	handle := holdWindowsReader(t, filepath.Join(s.Path(), manifestFile))
	defer func() {
		if handle != windows.InvalidHandle {
			windows.CloseHandle(handle)
		}
	}()
	done := make(chan error, 1)
	go func() { _, err := s.Append(testTx(1)); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("append did not tolerate transient manifest reader: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	handle = windows.InvalidHandle
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("append stayed blocked after reader closed")
	}
	page, err := s.Fetch(Filter{}, "", 10)
	if err != nil || page.Total != 1 || s.Meta().Counters.Persisted != 1 {
		t.Fatal("released reader lost transaction", page, err)
	}
}

func TestWindowsPermanentManifestLockKeepsOldBytesAndCleansStaging(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, manifestFile)
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	handle := holdWindowsReader(t, path)
	defer windows.CloseHandle(handle)
	started := time.Now()
	if err := durableReplace(path, []byte("replacement")); err == nil {
		t.Fatal("replaced destination while reader denied deletion")
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatal("permanent lock did not return within bounded window", elapsed)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original" {
		t.Fatal("failed replacement changed committed bytes", string(data), err)
	}
	staged, err := filepath.Glob(filepath.Join(dir, "pending-*"))
	if err != nil || len(staged) != 0 {
		t.Fatal("failed replacement left staging files", staged, err)
	}
}
