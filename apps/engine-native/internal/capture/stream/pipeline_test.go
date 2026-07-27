package stream

import (
	"context"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/aggregate"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type recordingSink struct {
	mu           sync.Mutex
	transactions int
	stats        int
	aggregates   int
}

func (*recordingSink) Started(capture.Session) {}
func (s *recordingSink) Transactions(_ capture.SessionID, _, _ uint64, tx []models.CaptureTransaction) {
	s.mu.Lock()
	s.transactions += len(tx)
	s.mu.Unlock()
}
func (s *recordingSink) Aggregate(capture.SessionID, uint64, uint64, any) {
	s.mu.Lock()
	s.aggregates++
	s.mu.Unlock()
}
func (s *recordingSink) Stats(capture.Stats)           { s.mu.Lock(); s.stats++; s.mu.Unlock() }
func (*recordingSink) Stopped(capture.Session)         {}
func (*recordingSink) Error(capture.SessionID, string) {}

func TestPipelinePersistsAllWhileBoundingLiveWindow(t *testing.T) {
	st := testStore(t)
	sink := &recordingSink{}
	p, err := New(Config{
		SessionID: "s", Store: st, EventSink: sink, LiveWindow: 5,
		BatchInterval: time.Millisecond, StatsInterval: 5 * time.Millisecond,
		HighWater: 1024, HardLimit: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		tx := models.CaptureTransaction{
			ID: "tx", Method: "GET", URL: "https://example.test/?token=secret",
			Host: "example.test", Path: "/", StatusCode: 200, State: models.TxComplete,
			Request:  models.HTTPMessage{Headers: []models.HeaderField{{Name: "Authorization", Value: "Bearer secret"}}},
			Response: models.HTTPMessage{},
		}
		if err := p.Submit(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	stats := p.Stats(capture.StateFinalized)
	if stats.Captured != 20 || stats.Persisted != 20 {
		t.Fatalf("stats=%+v", stats)
	}
	if len(p.LiveWindow()) != 5 {
		t.Fatalf("live window=%d", len(p.LiveWindow()))
	}
	page, err := st.Fetch(store.Filter{}, "", 100)
	if err != nil || len(page.Items) != 20 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if page.Items[0].Transaction.Request.Headers[0].Value != "[REDACTED]" {
		t.Fatalf("secret was persisted: %+v", page.Items[0])
	}
	replayed := make([]models.CaptureTransaction, 0, len(page.Items))
	for _, item := range page.Items {
		replayed = append(replayed, item.Transaction)
	}
	offline := aggregate.New("s", aggregate.DefaultTopK)
	offline.ApplyBatch(replayed)
	liveSnapshot := p.Snapshot()
	offlineSnapshot := offline.Snapshot()
	if liveSnapshot.Total != offlineSnapshot.Total ||
		!reflect.DeepEqual(liveSnapshot.TopEndpoints, offlineSnapshot.TopEndpoints) ||
		!reflect.DeepEqual(liveSnapshot.TopHosts, offlineSnapshot.TopHosts) {
		t.Fatalf("live/offline aggregate mismatch: live=%+v offline=%+v", liveSnapshot, offlineSnapshot)
	}
}

type blockingSink struct {
	recordingSink
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockingSink) Transactions(id capture.SessionID, sequence, version uint64, tx []models.CaptureTransaction) {
	s.once.Do(func() {
		close(s.entered)
		<-s.release
	})
	s.recordingSink.Transactions(id, sequence, version, tx)
}

func TestRendererBackpressureSkipsEventsWithoutLosingPersistence(t *testing.T) {
	st := testStore(t)
	sink := &blockingSink{entered: make(chan struct{}), release: make(chan struct{})}
	p, err := New(Config{
		SessionID: "s", Store: st, EventSink: sink,
		BatchInterval: time.Hour, StatsInterval: time.Hour,
		HighWater: 1 << 20, HardLimit: 2 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300; i++ {
		tx := models.CaptureTransaction{
			ID: "tx", Method: "GET", Host: "example.test", Path: "/",
			StatusCode: http.StatusOK, State: models.TxComplete,
		}
		if err := p.Submit(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
		if i == DefaultBatchRecords-1 {
			select {
			case <-sink.entered:
			case <-time.After(time.Second):
				t.Fatal("publisher did not enter blocking sink")
			}
		}
	}
	stats := p.Stats(capture.StateRunning)
	if stats.Persisted != 300 || stats.EventSkipped == 0 {
		t.Fatalf("stats=%+v", stats)
	}
	close(sink.release)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if p.Snapshot().Total != 300 {
		t.Fatalf("aggregate total=%d", p.Snapshot().Total)
	}
}

func TestPipelineRejectsSingleRecordOverHardLimit(t *testing.T) {
	st := testStore(t)
	p, err := New(Config{SessionID: "s", Store: st, HighWater: 10, HardLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	tx := models.CaptureTransaction{URL: "https://example.test/" + string(make([]byte, 100))}
	if err := p.Submit(context.Background(), tx); err != capture.ErrBackpressureHard {
		t.Fatalf("err=%v", err)
	}
	stats := p.Stats(capture.StateRunning)
	if stats.Captured != 1 || stats.Persisted != 0 {
		t.Fatalf("stats=%+v", stats)
	}
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(store.Config{
		Root: t.TempDir(), SessionID: "s", ReserveBytes: 0,
		FreeBytes: func(string) (uint64, error) { return ^uint64(0), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Finalize(capture.StateFinalized) })
	return st
}
