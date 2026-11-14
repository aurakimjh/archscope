package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"golang.org/x/sys/windows"
)

func TestWindowsAnnotationsWaitForTransientReader(t *testing.T) {
	dir := t.TempDir()
	if err := WriteAnnotations(dir, map[string]models.HTTPAnnotation{"request": {Comment: "old"}}); err != nil {
		t.Fatal(err)
	}
	handle := holdWindowsReader(t, filepath.Join(dir, annotationFile))
	defer func() {
		if handle != windows.InvalidHandle {
			windows.CloseHandle(handle)
		}
	}()
	done := make(chan error, 1)
	go func() {
		done <- WriteAnnotations(dir, map[string]models.HTTPAnnotation{"request": {Starred: true, Comment: "라이브 주석"}})
	}()
	select {
	case err := <-done:
		t.Fatalf("annotation update did not wait for transient reader: %v", err)
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
		t.Fatal("annotation update stayed blocked after reader closed")
	}
	values, err := ReadAnnotations(dir)
	if err != nil || !values["request"].Starred || values["request"].Comment != "라이브 주석" {
		t.Fatal("lost annotation", values, err)
	}
	assertNoAnnotationStaging(t, dir)
}

func TestWindowsAnnotationsPermanentReaderPreservesCommittedBytes(t *testing.T) {
	dir := t.TempDir()
	if err := WriteAnnotations(dir, map[string]models.HTTPAnnotation{"request": {Comment: "old"}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, annotationFile)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	handle := holdWindowsReader(t, path)
	defer windows.CloseHandle(handle)
	started := time.Now()
	if err := WriteAnnotations(dir, map[string]models.HTTPAnnotation{"request": {Comment: "replacement"}}); err == nil {
		t.Fatal("replaced annotations while reader denied deletion")
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("annotation retry exceeded bounded window")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("failed replacement changed committed annotations", err)
	}
	assertNoAnnotationStaging(t, dir)
}

func assertNoAnnotationStaging(t *testing.T, dir string) {
	t.Helper()
	staged, err := filepath.Glob(filepath.Join(dir, ".annotations-*"))
	if err != nil || len(staged) != 0 {
		t.Fatal("annotation staging files remain", staged, err)
	}
}
