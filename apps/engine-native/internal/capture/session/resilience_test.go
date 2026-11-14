package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/proxy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestRecoverSessionsReportsActualCleanupAndRetainedRows(t *testing.T) {
	root := t.TempDir()
	st, err := store.New(store.Config{Root: root, SessionID: "cap-recovery-report", MaxRecords: 2, FreeBytes: func(string) (uint64, error) { return ^uint64(0), nil }})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_, err = st.Append(models.CaptureTransaction{ID: fmt.Sprintf("tx-%d", i), Sequence: i, Method: "GET", URL: "http://example.test/", StatusCode: 200, State: models.TxComplete})
		if err != nil {
			t.Fatal(err)
		}
	}
	dir := st.Path()
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	staging, blob := []byte(`{"unfinished":`), []byte("orphan blob")
	for name, data := range map[string][]byte{filepath.Join("rolling", "pending-test"): staging, filepath.Join("blobs", "orphan-test"): blob} {
		if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	manager := NewManagerForPlatform(root, nil, "windows")
	reports, err := manager.RecoverSessions()
	if err != nil || len(reports) != 1 {
		t.Fatalf("reports=%+v err=%v", reports, err)
	}
	wantBytes := int64(len(staging) + len(blob))
	if reports[0].Records != 2 || reports[0].DiscardedBytes != wantBytes {
		t.Fatalf("report=%+v want 2 retained rows/%d discarded bytes", reports[0], wantBytes)
	}
	// A later scan must not invent repeated discarded bytes or cumulative rows.
	reports, err = manager.RecoverSessions()
	if err != nil || len(reports) != 1 || reports[0].Records != 2 || reports[0].DiscardedBytes != 0 {
		t.Fatalf("second reports=%+v err=%v", reports, err)
	}
	page, err := manager.Fetch("cap-recovery-report", store.Filter{}, "", 10)
	if err != nil || page.Total != 2 || page.Items[0].Transaction.ID != "tx-1" || page.Items[1].Transaction.ID != "tx-2" {
		t.Fatalf("retained page=%+v err=%v", page, err)
	}
}

type idleCapturer struct {
	failedPartialCapturer
	cancel context.CancelFunc
}

func (c *idleCapturer) Start(context.Context, capture.Config, func(models.CaptureTransaction) error) (string, error) {
	if c.cancel != nil {
		c.cancel()
	}
	return "127.0.0.1:12345", nil
}
func TestCanceledStartAllocatesNothing(t *testing.T) {
	root := t.TempDir()
	manager := NewManagerForPlatform(root, nil, "windows")
	constructed := false
	manager.newCapturer = func(proxy.Config) (Capturer, error) { constructed = true; return &idleCapturer{}, nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := manager.Start(ctx, capture.Config{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled start: %v", err)
	}
	files, _ := os.ReadDir(root)
	if constructed || manager.authority != nil || len(files) != 0 {
		t.Fatal("canceled start allocated resources")
	}
}
func TestCancellationDuringStartCleansPartialResources(t *testing.T) {
	manager := NewManagerForPlatform(t.TempDir(), nil, "windows")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	capturer := &idleCapturer{cancel: cancel}
	manager.newCapturer = func(proxy.Config) (Capturer, error) { return capturer, nil }
	if _, err := manager.Start(ctx, capture.Config{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled partial start: %v", err)
	}
	if capturer.stops != 1 || manager.authority != nil || manager.Current().ID != "" {
		t.Fatal("partial canceled start leaked resources")
	}
}
func TestRunningContextCancellationFinalizesStore(t *testing.T) {
	manager := NewManagerForPlatform(t.TempDir(), nil, "windows")
	manager.newCapturer = func(proxy.Config) (Capturer, error) { return &idleCapturer{}, nil }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	current, err := manager.Start(ctx, capture.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop(context.Background(), current.ID)
	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if manager.Current().State == StateFinalized {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("canceled capture remained active instead of finalizing its store")
}

type failedDrainCapturer struct{ drainOnStopCapturer }

func (c *failedDrainCapturer) Stop(ctx context.Context) error {
	return errors.Join(c.drainOnStopCapturer.Stop(ctx), context.DeadlineExceeded)
}
func TestInterruptedStopPreservesRecordsAndAllowsRestart(t *testing.T) {
	manager := NewManagerForPlatform(t.TempDir(), nil, "windows")
	failed := &failedDrainCapturer{}
	manager.newCapturer = func(proxy.Config) (Capturer, error) { return failed, nil }
	first, err := manager.Start(context.Background(), capture.Config{})
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := manager.Stop(context.Background(), first.ID)
	if !errors.Is(err, context.DeadlineExceeded) || stopped.State != StateRecoverable {
		t.Fatalf("stop=%+v err=%v", stopped, err)
	}
	page, err := manager.Fetch(first.ID, store.Filter{}, "", 10)
	if err != nil || page.Total != 1 {
		t.Fatal("interrupted stop lost its drained record", err)
	}
	if reports, err := manager.RecoverSessions(); err != nil || len(reports) != 1 {
		t.Fatal("recoverable session not discoverable", err)
	}
	manager.newCapturer = func(proxy.Config) (Capturer, error) { return &idleCapturer{}, nil }
	second, err := manager.Start(context.Background(), capture.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Stop(context.Background(), second.ID); err != nil {
		t.Fatal(err)
	}
	page, err = manager.Fetch(first.ID, store.Filter{}, "", 10)
	if err != nil || page.Total != 1 {
		t.Fatal("restart altered prior recoverable evidence", err)
	}
}
