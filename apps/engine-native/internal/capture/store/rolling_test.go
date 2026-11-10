package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRollingRetentionDeletesRowsAndUnsharedBodiesAndRecoversCounters(t *testing.T) {
	s := newTestStore(t, Config{MaxRecords: 2})
	ref, e := s.PutBody([]byte("sanitized synthetic body"))
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 4; i++ {
		tx := testTx(i)
		if i < 2 {
			tx.Response.BodyRef = ref
		}
		if _, e = s.Append(tx); e != nil {
			t.Fatal(e)
		}
	}
	m := s.Meta()
	if m.Counters.Persisted != 4 || m.Counters.Rotated != 2 || m.RetainedRecords != 2 {
		t.Fatal(m)
	}
	if _, e = os.Stat(filepath.Join(s.Path(), blobsDir, ref)); !os.IsNotExist(e) {
		t.Fatal("evicted body still exists", e)
	}
	entries, e := os.ReadDir(filepath.Join(s.Path(), rollingDirectory))
	if e != nil || len(entries) != 2 {
		t.Fatal(len(entries), e)
	}
	page, e := s.Fetch(Filter{}, "", 10)
	if e != nil || page.Total != 2 || page.Items[0].Transaction.ID != testTx(2).ID {
		t.Fatal(page, e)
	}
	var actual int64
	for _, entry := range entries {
		info, _ := entry.Info()
		actual += info.Size()
	}
	if m.StoreBytes != actual {
		t.Fatal(m.StoreBytes, actual)
	}
	path := s.Path()
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	reopened, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if reopened.Meta().Counters.Persisted != 4 || reopened.Meta().Counters.Rotated != 2 {
		t.Fatal("lifetime counters became retained counts")
	}
	if _, e = reopened.Append(testTx(4)); e != nil {
		t.Fatal(e)
	}
	if reopened.Meta().Counters.Persisted != 5 || reopened.Meta().RetainedRecords != 2 {
		t.Fatal(reopened.Meta())
	}
}
func TestRollingSharedBodyAndSnapshotInvalidation(t *testing.T) {
	s := newTestStore(t, Config{MaxRecords: 2})
	ref, e := s.PutBody([]byte("shared synthetic"))
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		tx := testTx(i)
		tx.Request.BodyRef = ref
		tx.Response.BodyRef = ref
		if _, e = s.Append(tx); e != nil {
			t.Fatal(e)
		}
	}
	page, e := s.Fetch(Filter{}, "", 1)
	if e != nil || page.NextCursor == "" {
		t.Fatal(page, e)
	}
	if _, e = s.Append(testTx(2)); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(s.Path(), blobsDir, ref)); e != nil {
		t.Fatal("shared retained body deleted", e)
	}
	if _, e = s.Fetch(Filter{}, page.NextCursor, 1); !errors.Is(e, ErrStaleCursor) {
		t.Fatal("mixed pre/post-eviction snapshot", e)
	}
	if _, e = s.Append(testTx(3)); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(s.Path(), blobsDir, ref)); !os.IsNotExist(e) {
		t.Fatal("last reference not reclaimed", e)
	}
	next, e := s.PutBody([]byte("new"))
	if e != nil || next == ref {
		t.Fatal("body name reused after reclamation", next, e)
	}
	if e = s.Clear(); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Append(testTx(4)); e != nil {
		t.Fatal(e)
	}
	if s.Meta().RetainedRecords != 1 || s.Meta().Counters.Persisted != 1 {
		t.Fatal(s.Meta())
	}
}
func TestRollingRecoveryIgnoresUncommittedFilesButRejectsMissingCommittedRows(t *testing.T) {
	s := newTestStore(t, Config{MaxRecords: 2})
	s.Append(testTx(0))
	s.Append(testTx(1))
	path := s.Path()
	s.Close()
	b, _ := json.Marshal(StoredTransaction{Seq: 3, Transaction: testTx(2)})
	os.WriteFile(filepath.Join(path, rollingDirectory, recordName(3)), b, 0600)
	os.WriteFile(filepath.Join(path, rollingDirectory, "pending-interrupted"), []byte("partial"), 0600)
	recovered, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	page, e := recovered.Fetch(Filter{}, "", 10)
	if e != nil || page.Total != 2 {
		t.Fatal(page, e)
	}
	if _, e = os.Stat(filepath.Join(path, rollingDirectory, recordName(3))); !os.IsNotExist(e) {
		t.Fatal("uncommitted row survived")
	}
	recovered.Close()
	os.Remove(filepath.Join(path, rollingDirectory, recordName(1)))
	if broken, e := Open(path); e == nil {
		broken.Close()
		t.Fatal("missing committed row silently accepted")
	}
}
