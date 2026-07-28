package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWritePrivateJSONAnyUsesOwnerOnlyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows ACL enforcement is covered by packaging acceptance")
	}
	path := filepath.Join(t.TempDir(), "acceptance-evidence.json")
	if err := writePrivateJSONAny(map[string]any{"task": "T-581"}, path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if permissions := info.Mode().Perm(); permissions != 0o600 {
		t.Fatalf("permissions=%#o, want 0600", permissions)
	}
}
