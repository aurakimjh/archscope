package localfiles

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestT633NewPublicationProtectsOriginalsAndManagedPaths(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	managed := filepath.Join(root, "state")
	os.Mkdir(managed, 0700)
	alias := filepath.Join(root, "alias")
	if e := os.Symlink(managed, alias); e == nil {
		if e = Publish(ctx, filepath.Join(alias, "new.json"), []byte("bad"), managed); e == nil {
			t.Fatal("alias bypass")
		}
	}
	target := filepath.Join(root, "output.json")
	if e := Publish(ctx, target, []byte("original"), managed); e != nil {
		t.Fatal(e)
	}
	if e := Publish(ctx, target, []byte("changed"), managed); e == nil {
		t.Fatal("overwrote original")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "original" {
		t.Fatal(string(b))
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	target = filepath.Join(root, "cancel.json")
	if e := Publish(cancelCtx, target, []byte("bad"), managed); e == nil {
		t.Fatal("ignored cancellation")
	}
	if _, e := os.Stat(target); !os.IsNotExist(e) {
		t.Fatal("published canceled artifact")
	}
}
