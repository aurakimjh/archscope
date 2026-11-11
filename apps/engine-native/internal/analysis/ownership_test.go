package analysis_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
)

func TestT630SharedHandleOwnershipAndSavedSnapshotRecovery(t *testing.T) {
	s, _, _ := setup(t, nil)
	job := importModel(t, s, "memory")
	raw, _, e := s.Read(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	same, e := s.LoadSnapshot(context.Background(), raw)
	if e != nil || same.Handle != job.ResultHandle {
		t.Fatal(same, e)
	}
	a, e := s.AcquireResult(same.Handle)
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.AcquireResult(same.Handle)
	if e != nil || a.ID == b.ID {
		t.Fatal(a, b, e)
	}
	if e = s.Release(same.Handle); e == nil {
		t.Fatal("administrative release deleted owned result")
	}
	if e = s.ReleaseResultLease(a.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.Release(same.Handle); e == nil {
		t.Fatal("second owner lost shared result")
	}
	if _, _, e = s.Read(b.Handle); e != nil {
		t.Fatal(e)
	}
	if e = s.ReleaseResultLease(a.ID); e == nil {
		t.Fatal("double release accepted")
	}
	if e = s.ReleaseResultLease(b.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.Release(same.Handle); e != nil {
		t.Fatal(e)
	}
	recovered, e := s.LoadSnapshot(context.Background(), raw)
	if e != nil {
		t.Fatal(e)
	}
	after, _, e := s.Read(recovered.Handle)
	if e != nil || !bytes.Equal(raw, after) {
		t.Fatal("snapshot recovery changed data", e)
	}
}
func TestT630CacheMigrationRetryMarkerAndSourcePreservation(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "old-cache")
	dest := filepath.Join(root, "app-data")
	c, r, e := catalog.Builtin()
	if e != nil {
		t.Fatal(e)
	}
	source, e := analysis.New(legacy, c, r)
	if e != nil {
		t.Fatal(e)
	}
	job := importModel(t, source, "memory")
	body, _, _ := source.Read(job.ResultHandle)
	source.Close()
	// Simulate interruption after copying bytes but before the completion marker.
	partiallyMigrated, e := analysis.New(dest, c, r)
	if e != nil {
		t.Fatal(e)
	}
	prior, e := partiallyMigrated.LoadSnapshot(context.Background(), body)
	if e != nil {
		t.Fatal(e)
	}
	partiallyMigrated.Close()
	service, e := analysis.OpenDurable(context.Background(), legacy, dest, c, r)
	if e != nil {
		t.Fatal(e)
	}
	view, e := service.LoadSnapshot(context.Background(), body)
	if e != nil || view.Handle != prior.Handle {
		t.Fatal(view, e)
	}
	service.Close()
	old, e := os.ReadFile(filepath.Join(legacy, "results", job.ResultHandle+".json"))
	if e != nil || !bytes.Equal(old, body) {
		t.Fatal("migration changed legacy source", e)
	}
	marker := filepath.Join(dest, "cache-migration.json")
	if _, e = os.Stat(marker); e != nil {
		t.Fatal(e)
	}
	// A completed migration is not an automatic undelete of a later cache cleanup.
	os.Remove(filepath.Join(dest, "results", view.Handle+".json"))
	reopened, e := analysis.OpenDurable(context.Background(), legacy, dest, c, r)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if _, _, e = reopened.Read(view.Handle); e == nil {
		t.Fatal("completed migration resurrected removed cache snapshot")
	}
}
func TestT630CacheMigrationNeverMarksInvalidSnapshotsComplete(t *testing.T) {
	c, r, _ := catalog.Builtin()
	root := t.TempDir()
	legacy := filepath.Join(root, "legacy")
	source, e := analysis.New(legacy, c, r)
	if e != nil {
		t.Fatal(e)
	}
	source.Close()
	os.WriteFile(filepath.Join(legacy, "results", "result-00000000000000000000000000000001.json"), []byte("invalid"), 0600)
	dest := filepath.Join(root, "durable")
	if s, e := analysis.OpenDurable(context.Background(), legacy, dest, c, r); e == nil {
		s.Close()
		t.Fatal("invalid cache silently migrated")
	}
	if _, e = os.Stat(filepath.Join(dest, "cache-migration.json")); !os.IsNotExist(e) {
		t.Fatal("failed migration published completion marker")
	}
	if _, e = os.Stat(filepath.Join(legacy, "results", "result-00000000000000000000000000000001.json")); e != nil {
		t.Fatal("failed migration deleted original")
	}
}
