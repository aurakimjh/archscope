//go:build layoutaudit

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLayoutAuditReceiptCompletePublication(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "result.json")
	body := []byte(`{"report":{"passed":true},"synthetic":"` + strings.Repeat("x", 1<<18) + `"}`)
	writerFinished := false
	done := make(chan error, 1)
	go func() { done <- publishLayoutAuditReceipt(path, body) }()
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			if !bytes.Equal(data, body) || !json.Valid(data) {
				t.Fatal("partial final receipt visible")
			}
			break
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(path)
			if err != nil || !bytes.Equal(data, body) {
				t.Fatal("completed receipt", err)
			}
			writerFinished = true
			break
		default:
			runtime.Gosched()
		}
		if writerFinished {
			break
		}
	}
	if !writerFinished {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
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
