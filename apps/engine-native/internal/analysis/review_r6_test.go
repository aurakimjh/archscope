package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func r6Service(t *testing.T) (*Service, string) {
	t.Helper()
	c, _ := ingestion.NewCatalog(nil, nil, nil)
	r, _ := resultcontract.New()
	root := t.TempDir()
	s, e := New(root, c, r)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	return s, root
}
func r6Body(n int) []byte {
	r := models.New("synthetic", "fixture")
	r.CreatedAt = "2026-11-11T00:00:00Z"
	r.Summary["value"] = n
	b, _ := json.Marshal(r)
	return b
}
func TestR602FullStoreReserveReopenAndExplicitOwnedCleanup(t *testing.T) {
	s, _ := r6Service(t)
	var first ResultView
	for i := 0; i < maxStoredResults; i++ {
		v, e := s.LoadSnapshot(context.Background(), r6Body(i))
		if e != nil {
			t.Fatal(i, e)
		}
		if i == 0 {
			first = v
		}
	}
	a, e := s.AcquireResult(first.Handle)
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.AcquireResult(first.Handle)
	if e != nil {
		t.Fatal(e)
	}
	raw := r6Body(2000)
	if _, e = s.LoadSnapshot(context.Background(), raw); e == nil || !strings.Contains(e.Error(), "CleanupEvidenceCache") {
		t.Fatal("ordinary import evicted data or lacked recovery guidance", e)
	}
	view, owner, e := s.LoadOwnedSnapshot(context.Background(), raw)
	if e != nil || !s.stored[view.Handle].reserved || s.ordinaryCountLocked() != maxStoredResults {
		t.Fatal(view, owner, e)
	}
	if body, _, e := s.Read(view.Handle); e != nil || !bytes.Equal(body, raw) {
		t.Fatal("saved snapshot changed", e)
	}
	_, shared, e := s.LoadOwnedSnapshot(context.Background(), raw)
	if e != nil || shared.Handle != view.Handle || shared.ID == owner.ID {
		t.Fatal(shared, e)
	}
	cleanup, e := s.CleanupEvidenceCache()
	if e != nil || cleanup.Removed != maxStoredResults-1 || cleanup.RetainedOwned != 2 {
		t.Fatal(cleanup, e)
	}
	for _, key := range []string{view.Handle, first.Handle} {
		if _, _, e = s.Read(key); e != nil {
			t.Fatal("owned result deleted", e)
		}
	}
	// Ordinary publishing must not inherit a session-reserve handle that can
	// expire as soon as another caller releases its final lease.
	ordinary, e := s.LoadSnapshot(context.Background(), raw)
	if e != nil || ordinary.Handle == view.Handle {
		t.Fatal("ordinary result inherited transient lifetime", ordinary, e)
	}
	if e = s.ReleaseResultLease(owner.ID); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Read(shared.Handle); e != nil {
		t.Fatal("other reserve owner lost snapshot", e)
	}
	if e = s.ReleaseResultLease(shared.ID); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Read(view.Handle); e == nil {
		t.Fatal("unowned reserve copy leaked")
	}
	if _, _, e = s.Read(ordinary.Handle); e != nil {
		t.Fatal("ordinary snapshot lost with reserve owner", e)
	}
	if _, e = s.LoadSnapshot(context.Background(), r6Body(3000)); e != nil {
		t.Fatal("new import did not recover after cleanup", e)
	}
	s.ReleaseResultLease(a.ID)
	if _, _, e = s.Read(b.Handle); e != nil {
		t.Fatal(e)
	}
	s.ReleaseResultLease(b.ID)
}
func TestR602ReserveBoundsRestartAndAtomicOwnership(t *testing.T) {
	s, root := r6Service(t)
	// Force the ordinary byte ceiling without allocating a 512 MiB fixture.
	s.storedBytes = maxStoredBytes
	owners := []ResultLease{}
	for i := 0; i < maxReopenedResults; i++ {
		_, owner, e := s.LoadOwnedSnapshot(context.Background(), r6Body(i))
		if e != nil {
			t.Fatal(i, e)
		}
		owners = append(owners, owner)
	}
	if _, _, e := s.LoadOwnedSnapshot(context.Background(), r6Body(100)); e == nil {
		t.Fatal("reserve count ignored")
	}
	s.Close()
	c, _ := ingestion.NewCatalog(nil, nil, nil)
	r, _ := resultcontract.New()
	reopened, e := New(root, c, r)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	entries, e := os.ReadDir(filepath.Join(root, "reopened"))
	if e != nil || len(entries) != 0 {
		t.Fatal("crash-left reserve not reclaimed", e)
	}
	stop := make(chan struct{})
	var cleanup sync.WaitGroup
	cleanup.Add(1)
	go func() {
		defer cleanup.Done()
		for {
			select {
			case <-stop:
				return
			default:
				reopened.CleanupEvidenceCache()
			}
		}
	}()
	defer func() { close(stop); cleanup.Wait() }()
	for i := 0; i < 20; i++ {
		view, owner, e := reopened.LoadOwnedSnapshot(context.Background(), r6Body(i))
		if e != nil {
			t.Fatal("ownership gap at reopen", e)
		}
		if _, _, e = reopened.Read(view.Handle); e != nil {
			t.Fatal(e)
		}
		reopened.ReleaseResultLease(owner.ID)
	}
	// Reserve byte budget independently from handle count.
	s2, _ := r6Service(t)
	s2.storedBytes = maxStoredBytes
	s2.stored["reserved-bound"] = storedRevision{reserved: true, bytes: maxReopenedBytes}
	if _, _, e = s2.LoadOwnedSnapshot(context.Background(), r6Body(1)); e == nil {
		t.Fatal("reserve byte bound ignored")
	}
}
func TestR605PartialLegacyAndActionableCorruptRecovery(t *testing.T) {
	c, _ := ingestion.NewCatalog(nil, nil, nil)
	r, _ := resultcontract.New()
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprint(partial), func(t *testing.T) {
			root := t.TempDir()
			legacy := filepath.Join(root, "legacy")
			os.Mkdir(legacy, 0700)
			if partial {
				os.Mkdir(filepath.Join(legacy, "results"), 0700)
			}
			s, e := OpenDurable(context.Background(), legacy, filepath.Join(root, "durable"), c, r)
			if e != nil {
				t.Fatal(e)
			}
			s.Close()
		})
	}
	root := t.TempDir()
	legacy := filepath.Join(root, "legacy")
	dest := filepath.Join(root, "durable")
	prior, e := New(legacy, c, r)
	if e != nil {
		t.Fatal(e)
	}
	prior.Close()
	broken := filepath.Join(legacy, "results", "result-00000000000000000000000000000001.json")
	os.WriteFile(broken, []byte("broken original"), 0600)
	_, e = OpenDurable(context.Background(), legacy, dest, c, r)
	var recovery *MigrationRecoveryError
	if !errors.As(e, &recovery) || recovery.LegacyPath != legacy || recovery.DurablePath != dest || !strings.Contains(recovery.Action, "backup") {
		t.Fatal(e)
	}
	if body, _ := os.ReadFile(broken); string(body) != "broken original" {
		t.Fatal("corrupt original removed")
	}
	if _, e = os.Stat(filepath.Join(dest, "cache-migration.json")); !os.IsNotExist(e) {
		t.Fatal("bad migration completed", e)
	}
	backup := legacy + "-preserved-backup"
	if e = os.Rename(legacy, backup); e != nil {
		t.Fatal(e)
	}
	healthy, e := OpenDurable(context.Background(), legacy, dest, c, r)
	if e != nil {
		t.Fatal("documented recovery did not reopen service", e)
	}
	healthy.Close()
	if body, _ := os.ReadFile(filepath.Join(backup, "results", filepath.Base(broken))); string(body) != "broken original" {
		t.Fatal("recovery lost backup")
	}
}
