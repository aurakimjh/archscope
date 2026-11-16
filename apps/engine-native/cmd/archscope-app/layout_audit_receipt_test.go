//go:build layoutaudit

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLayoutAuditReceiptCompletePublication(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "result.json")
	body := []byte(`{"report":{"passed":true},"synthetic":"` + strings.Repeat("x", 1<<18) + `"}`)
	done := make(chan error, 1)
	go func() { done <- publishLayoutAuditReceipt(path, body) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			if !bytes.Equal(data, body) || !json.Valid(data) {
				t.Fatal("partial final receipt visible")
			}
			break
		}
		// MoveFileEx may briefly expose a sharing violation during publication.
		// Do not ignore permission failures or accept incomplete final bytes.
		sharing := runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(32))
		if !os.IsNotExist(err) && !sharing {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("complete receipt unavailable before deadline", err)
		}
		time.Sleep(time.Millisecond)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatal("receipt permissions")
	}
	files, err := os.ReadDir(root)
	if err != nil || len(files) != 1 {
		t.Fatal("staging file leaked", err)
	}
}
func TestLayoutAuditReceiptReplacementAndFailure(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "result.json")
	if err := publishLayoutAuditReceipt(path, []byte(`{"report":{"passed":false}}`)); err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"report":{"passed":true}}`)
	if err := publishLayoutAuditReceipt(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("replacement", err)
	}
	if err := publishLayoutAuditReceipt(filepath.Join(root, "absent", "result.json"), want); err == nil {
		t.Fatal("missing parent accepted")
	}
	if err := publishLayoutAuditReceipt(root, want); err == nil {
		t.Fatal("directory target accepted")
	}
	files, err := os.ReadDir(root)
	if err != nil || len(files) != 1 {
		t.Fatal("failed staging not cleaned", err)
	}
}
