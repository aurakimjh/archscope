package store

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestAppendFetchUsesStableSnapshotCursor(t *testing.T) {
	s := newTestStore(t, Config{})
	for i := 0; i < 3; i++ {
		if _, err := s.Append(testTx(i)); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.Fetch(Filter{}, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 3 || len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("first page=%+v", first)
	}
	if _, err := s.Append(testTx(4)); err != nil {
		t.Fatal(err)
	}
	second, err := s.Fetch(Filter{}, first.NextCursor, 2)
	if err != nil {
		t.Fatal(err)
	}
	if second.Total != 3 || len(second.Items) != 1 || second.Items[0].Seq != 3 {
		t.Fatalf("snapshot shifted after append: %+v", second)
	}
	fresh, err := s.Fetch(Filter{}, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Total != 4 {
		t.Fatalf("fresh snapshot total=%d", fresh.Total)
	}
}

func TestFetchFiltersInStoreAndCapsLimit(t *testing.T) {
	s := newTestStore(t, Config{})
	for i := 0; i < MaxFetchLimit+10; i++ {
		tx := testTx(i)
		if i%2 == 0 {
			tx.Host = "api.example"
		}
		if _, err := s.Append(tx); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.Fetch(Filter{Host: "api.example"}, "", MaxFetchLimit+100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != (MaxFetchLimit+10)/2 {
		t.Fatalf("filtered rows=%d", len(page.Items))
	}
}

func TestFetchRejectsFutureOrInconsistentCursor(t *testing.T) {
	s := newTestStore(t, Config{})
	if _, err := s.Append(testTx(1)); err != nil {
		t.Fatal(err)
	}
	future := encodeCursor(cursor{SessionID: "session-test", Snapshot: 2})
	if _, err := s.Fetch(Filter{}, future, 10); err == nil {
		t.Fatal("future cursor was accepted")
	}
	inconsistent := encodeCursor(cursor{SessionID: "session-test", Snapshot: 1, LastSeq: 2})
	if _, err := s.Fetch(Filter{}, inconsistent, 10); err == nil {
		t.Fatal("inconsistent cursor was accepted")
	}
}

func TestRecoverDiscardsOnlyIncompleteTail(t *testing.T) {
	s := newTestStore(t, Config{})
	if _, err := s.Append(testTx(1)); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	path := s.Path()
	f, err := os.OpenFile(filepath.Join(path, transactionsFile), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"seq":2,"transaction":`); err != nil {
		t.Fatal(err)
	}
	f.Close()
	s.file.Close()
	s.index.Close()
	s.closed = true

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Finalize(capture.StateFinalized)
	report, err := reopened.Recover()
	if err != nil {
		t.Fatal(err)
	}
	if report.Records != 1 {
		t.Fatalf("recovery=%+v", report)
	}
	indexInfo, err := os.Stat(filepath.Join(path, offsetIndexFile))
	if err != nil {
		t.Fatal(err)
	}
	if indexInfo.Size() != 8 {
		t.Fatalf("rebuilt index size=%d", indexInfo.Size())
	}
	page, err := reopened.Fetch(Filter{}, "", 10)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}

func TestStopPolicyFailsClosedAtSessionLimit(t *testing.T) {
	s := newTestStore(t, Config{MaxBytes: 1})
	if _, err := s.Append(testTx(1)); !errors.Is(err, ErrSessionLimit) {
		t.Fatalf("err=%v, want session limit", err)
	}
	if got := s.Meta().Counters.Persisted; got != 0 {
		t.Fatalf("persisted=%d", got)
	}
}

func TestStoreStopsBeforeDiskReserveIsExhausted(t *testing.T) {
	s, err := New(Config{
		Root: t.TempDir(), SessionID: "disk-reserve", ReserveBytes: 1024,
		FreeBytes: func(string) (uint64, error) { return 512, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Finalize(capture.StateFinalized)
	if _, err := s.Append(testTx(1)); !errors.Is(err, ErrDiskReserve) {
		t.Fatalf("err=%v, want disk reserve", err)
	}
}

func TestBodyBlobCountsTowardSessionLimitAndSurvivesReopen(t *testing.T) {
	root := t.TempDir()
	s, err := New(Config{
		Root: root, SessionID: "body-session", MaxBytes: 4096,
		BodyFraction: 0.7, ReserveBytes: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := s.PutBody([]byte("body"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta().StoreBytes != 4 {
		t.Fatalf("store bytes=%d", s.Meta().StoreBytes)
	}
	if err := s.Finalize(capture.StateFinalized); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(filepath.Join(root, "body-session"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.Meta().StoreBytes != 4 {
		t.Fatalf("reopened store bytes=%d", reopened.Meta().StoreBytes)
	}
	body, err := reopened.Body(ref)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil || string(data) != "body" {
		t.Fatalf("body=%q err=%v", data, err)
	}
}

func TestBodyOnlyPolicyIsExplicitAndRecorded(t *testing.T) {
	tx := testTx(1)
	tx.Request.BodyPreview = "secret body that should be omitted"
	tx.Request.BodyStorage = "inline"
	raw, _ := json.Marshal(StoredTransaction{Seq: 1, Transaction: tx})
	s := newTestStore(t, Config{MaxBytes: int64(len(raw) - 5), OverflowPolicy: capture.OverflowBodyOnly})
	item, err := s.Append(tx)
	if err != nil {
		t.Fatal(err)
	}
	if item.Transaction.Request.BodyStorage != "omitted" || item.Transaction.Request.BodyPreview != "" {
		t.Fatalf("body was not omitted: %+v", item.Transaction.Request)
	}
	if s.Meta().Counters.BodyOmitted != 1 {
		t.Fatalf("manifest=%+v", s.Meta())
	}
}

func newTestStore(t *testing.T, cfg Config) *Store {
	t.Helper()
	cfg.Root = t.TempDir()
	cfg.SessionID = "session-test"
	cfg.ReserveBytes = 0
	cfg.FreeBytes = func(string) (uint64, error) { return ^uint64(0), nil }
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !s.closed {
			_ = s.Finalize(capture.StateFinalized)
		}
	})
	return s
}

func testTx(i int) models.CaptureTransaction {
	return models.CaptureTransaction{
		ID: "tx", Method: "GET", URL: "https://example.test/a", Scheme: "https",
		Host: "example.test", Path: "/a", HTTPVersion: "HTTP/1.1",
		StatusCode: 200, State: models.TxComplete, Sequence: i,
		Request:  models.HTTPMessage{BodyStorage: "omitted"},
		Response: models.HTTPMessage{BodyStorage: "omitted"},
	}
}
