package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
)

func TestReserveRefusalPreservesCommittedRowsAndAllowsRetry(t *testing.T) {
	free := uint64(2048)
	s, err := New(Config{Root: t.TempDir(), SessionID: "reserve-retry", MaxRecords: 2, ReserveBytes: 1024, FreeBytes: func(string) (uint64, error) { return free, nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Append(testTx(1)); err != nil {
		t.Fatal(err)
	}
	before := s.Meta()
	record := filepath.Join(s.Path(), rollingDirectory, recordName(before.SnapshotVersion))
	bytes, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	free = 512
	if _, err = s.Append(testTx(2)); !errors.Is(err, ErrDiskReserve) {
		t.Fatalf("expected reserve refusal: %v", err)
	}
	if !reflect.DeepEqual(before, s.Meta()) {
		t.Fatal("refused append changed authoritative manifest")
	}
	actual, err := os.ReadFile(record)
	if err != nil || string(actual) != string(bytes) {
		t.Fatal("refused append changed committed bytes", err)
	}
	entries, err := os.ReadDir(filepath.Join(s.Path(), rollingDirectory))
	if err != nil || len(entries) != 1 {
		t.Fatal("refused append left another record", err)
	}
	free = 2048
	if _, err = s.Append(testTx(2)); err != nil {
		t.Fatal("retry after free-space recovery", err)
	}
	if s.Meta().Counters.Persisted != 2 || s.Meta().RetainedRecords != 2 {
		t.Fatal("retry counters")
	}
	if err = s.Finalize(capture.StateFinalized); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	page, err := reopened.Fetch(Filter{}, "", 10)
	if err != nil || page.Total != 2 || page.Items[0].Transaction.ID != testTx(1).ID || page.Items[1].Transaction.ID != testTx(2).ID {
		t.Fatal("retry/reopen changed identity", err)
	}
}

// Opt-in bounded Docker tmpfs test of the real OS free-space reader. It never
// fills the host disk or lowers the production 512MiB reserve.
func TestActualDiskReserveAndRetryOnBoundedTmpfs(t *testing.T) {
	root := os.Getenv("ARCHSCOPE_DISK_FIXTURE_ROOT")
	if root == "" || runtime.GOOS != "linux" {
		t.Skip("requires explicitly mounted bounded Linux Docker tmpfs")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	owned, err := os.MkdirTemp(root, "archscope-reserve-")
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, owned)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		t.Fatal("fixture escaped owned root")
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(owned); err != nil {
			t.Error(err)
		}
	})
	s, err := New(Config{Root: owned, SessionID: "actual-reserve", MaxRecords: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	free, err := diskFreeBytes(owned)
	if err != nil || free <= uint64(DefaultReserveBytes+1<<20) || free > 640<<20 {
		t.Fatal("requires a600MiB tmpfs with default reserve headroom", free, err)
	}
	needed := int64(free) - DefaultReserveBytes + (1 << 20)
	if needed <= 0 || needed > 128<<20 {
		t.Fatal("bounded fixture allocation exceeded")
	}
	filler := filepath.Join(owned, "owned-space-fixture")
	f, err := os.Create(filler)
	if err != nil {
		t.Fatal(err)
	}
	block := make([]byte, 1<<20)
	for remaining := needed; remaining > 0; {
		n := min(int64(len(block)), remaining)
		written, e := f.Write(block[:int(n)])
		if e != nil {
			f.Close()
			t.Fatal(e)
		}
		remaining -= int64(written)
	}
	if err = f.Sync(); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	constrained, err := diskFreeBytes(owned)
	if err != nil || constrained >= uint64(DefaultReserveBytes) {
		t.Fatal("reserve boundary not reached", constrained, err)
	}
	if _, err = s.Append(testTx(1)); !errors.Is(err, ErrDiskReserve) {
		t.Fatal("real OS reserve must reject append", err)
	}
	if s.Meta().Counters.Persisted != 0 || s.Meta().RetainedRecords != 0 {
		t.Fatal("reserve refusal published a row")
	}
	if err = os.Remove(filler); err != nil {
		t.Fatal(err)
	}
	restored, err := diskFreeBytes(owned)
	if err != nil || restored < uint64(DefaultReserveBytes) {
		t.Fatal("fixture space did not recover", err)
	}
	if _, err = s.Append(testTx(1)); err != nil {
		t.Fatal("actual-space retry", err)
	}
	page, err := s.Fetch(Filter{}, "", 10)
	if err != nil || page.Total != 1 || page.Items[0].Transaction.ID != testTx(1).ID {
		t.Fatal("retry identity", err)
	}
	if destination := os.Getenv("ARCHSCOPE_CAPTURE_DISK_REPORT"); destination != "" {
		raw, _ := json.MarshalIndent(map[string]any{"passed": true, "platform": "Linux Docker bounded tmpfs", "reserveBytes": DefaultReserveBytes, "initialAvailableBytes": free, "constrainedAvailableBytes": constrained, "restoredAvailableBytes": restored, "temporaryAllocatedBytes": needed, "rejectedRowsPublished": 0, "retryRows": 1, "hostDiskFilled": false}, "", "  ")
		if err = os.WriteFile(destination, append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
