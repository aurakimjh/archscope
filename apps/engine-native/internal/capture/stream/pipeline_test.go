package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/aggregate"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type recordingSink struct {
	mu            sync.Mutex
	transactions  int
	progressCalls int
	progressItems int
	stats         int
	aggregates    int
}

func (*recordingSink) Started(capture.Session) {}
func (s *recordingSink) Progress(_ capture.SessionID, tx []models.CaptureTransaction) {
	s.mu.Lock()
	s.progressCalls++
	s.progressItems += len(tx)
	s.mu.Unlock()
}
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
		RetainUnattributedMetadata: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		tx := models.CaptureTransaction{
			ID: fmt.Sprintf("tx-%d", i), Method: "GET", URL: "https://example.test/?token=secret",
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
	if stats.Observed != 20 || stats.Captured != 20 || stats.Persisted != 20 {
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
		RetainUnattributedMetadata: true,
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
	p, err := New(Config{
		SessionID: "s", Store: st, HighWater: 10, HardLimit: 20,
		RetainUnattributedMetadata: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	tx := models.CaptureTransaction{URL: "https://example.test/" + string(make([]byte, 100))}
	if err := p.Submit(context.Background(), tx); err != capture.ErrBackpressureHard {
		t.Fatalf("err=%v", err)
	}
	stats := p.Stats(capture.StateRunning)
	if stats.Observed != 1 || stats.Captured != 1 || stats.Persisted != 0 {
		t.Fatalf("stats=%+v", stats)
	}
}

func TestPipelineDropsUnattributedByDefaultAndRetainsOnlyWithOptIn(t *testing.T) {
	newPipeline := func(t *testing.T, retain bool) (*Pipeline, *store.Store) {
		t.Helper()
		st := testStore(t)
		p, err := New(Config{
			SessionID: "s", Store: st,
			BatchInterval: time.Hour, StatsInterval: time.Hour,
			RetainUnattributedMetadata: retain,
		})
		if err != nil {
			t.Fatal(err)
		}
		return p, st
	}
	tx := models.CaptureTransaction{
		ID: "unknown", Method: "GET", Host: "example.test", Path: "/",
		State:    models.TxComplete,
		Request:  models.HTTPMessage{BodyStorage: "omitted"},
		Response: models.HTTPMessage{BodyStorage: "omitted"},
	}

	dropped, droppedStore := newPipeline(t, false)
	if err := dropped.Submit(context.Background(), tx); err != nil {
		t.Fatal(err)
	}
	if err := dropped.Close(); err != nil {
		t.Fatal(err)
	}
	droppedStats := dropped.Stats(capture.StateFinalized)
	if droppedStats.Observed != 1 || droppedStats.Unattributed != 1 || droppedStats.Dropped != 1 ||
		droppedStats.Captured != 0 || droppedStats.Persisted != 0 {
		t.Fatalf("default stats=%+v", droppedStats)
	}
	droppedPage, err := droppedStore.Fetch(store.Filter{}, "", 10)
	if err != nil || len(droppedPage.Items) != 0 {
		t.Fatalf("default page=%+v err=%v", droppedPage, err)
	}

	retained, retainedStore := newPipeline(t, true)
	if err := retained.Submit(context.Background(), tx); err != nil {
		t.Fatal(err)
	}
	if err := retained.Close(); err != nil {
		t.Fatal(err)
	}
	retainedStats := retained.Stats(capture.StateFinalized)
	if retainedStats.Observed != 1 || retainedStats.Unattributed != 1 || retainedStats.Dropped != 0 ||
		retainedStats.Captured != 1 || retainedStats.Persisted != 1 {
		t.Fatalf("opt-in stats=%+v", retainedStats)
	}
	retainedPage, err := retainedStore.Fetch(store.Filter{}, "", 10)
	if err != nil || len(retainedPage.Items) != 1 {
		t.Fatalf("opt-in page=%+v err=%v", retainedPage, err)
	}
}

func TestLiveMetadataIsRedactedBeforeRendererExposure(t *testing.T) {
	st := testStore(t)
	p, err := New(Config{SessionID: "s", Store: st})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	live := p.LiveMetadata(models.CaptureTransaction{
		URL:   "https://example.test/path?token=example-secret",
		Query: "token=example-secret",
		Process: &models.ProcessInstance{
			Name:        "client",
			CommandLine: "client --password example-secret",
			User:        "example-user",
			Attribution: "confirmed",
		},
		Request: models.HTTPMessage{
			Headers:     []models.HeaderField{{Name: "Authorization", Value: "Bearer example-secret"}},
			BodyPreview: "example-secret",
			BodyRef:     "blob-secret",
		},
	})
	encoded, err := json.Marshal(live)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "example-secret") ||
		strings.Contains(string(encoded), "example-user") ||
		strings.Contains(string(encoded), "blob-secret") {
		t.Fatalf("renderer metadata leaked sensitive content: %s", encoded)
	}
}

func TestProgressIsBatchedStableAndAbortedOnClose(t *testing.T) {
	st := testStore(t)
	sink := &recordingSink{}
	p, err := New(Config{
		SessionID: "s", Store: st, EventSink: sink, LiveWindow: 10,
		BatchInterval: 10 * time.Millisecond, StatsInterval: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	process := &models.ProcessInstance{Name: "client", Attribution: "confirmed"}
	first := models.CaptureTransaction{
		ID: "first", Method: "GET", Host: "example.test", Path: "/first",
		State: models.TxRequestSent, Fidelity: "pending", Process: process,
	}
	second := models.CaptureTransaction{
		ID: "second", Method: "CONNECT", Host: "example.test", Path: "",
		State: models.TxRequestSent, Fidelity: "unsupported", Process: process,
	}
	if err := p.TrackProgress(first); err != nil {
		t.Fatal(err)
	}
	if err := p.TrackProgress(second); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	sink.mu.Lock()
	progressCalls, progressItems := sink.progressCalls, sink.progressItems
	sink.mu.Unlock()
	if progressCalls != 1 || progressItems != 2 {
		t.Fatalf("progress calls=%d items=%d", progressCalls, progressItems)
	}
	first.State = models.TxComplete
	first.Fidelity = "decoded_wire"
	first.StatusCode = http.StatusOK
	if err := p.Submit(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	window := p.LiveWindow()
	if len(window) != 2 || window[0].ID != "first" ||
		window[0].State != models.TxComplete || window[1].ID != "second" {
		t.Fatalf("stable live window=%+v", window)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	window = p.LiveWindow()
	if len(window) != 2 || window[1].State != models.TxAborted ||
		window[1].Fidelity != "unsupported" {
		t.Fatalf("closed live window=%+v", window)
	}
}

func TestPipelineRedactionIsSafeAcrossConcurrentProgressAndPersistence(t *testing.T) {
	st := testStore(t)
	p, err := New(Config{
		SessionID: "s", Store: st, LiveWindow: 64,
		BatchInterval: time.Hour, StatsInterval: time.Hour,
		Redaction: redact.NewPolicy(redact.Options{
			CustomPatterns: []string{`example-secret`},
			RuleTimeLimit:  time.Nanosecond,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 32
	const transactionsPerWorker = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for index := 0; index < transactionsPerWorker; index++ {
				tx := models.CaptureTransaction{
					ID:  fmt.Sprintf("tx-%d-%d", worker, index),
					URL: "https://example.test/path?token=example-secret",
					Process: &models.ProcessInstance{
						Name:        "client",
						CommandLine: "client --password example-secret",
						Attribution: "confirmed",
					},
					Request: models.HTTPMessage{
						Headers: []models.HeaderField{
							{Name: "Authorization", Value: "Bearer example-secret"},
							{Name: "Cookie", Value: "session=example-secret"},
						},
						BodyStorage: "omitted",
					},
					Response: models.HTTPMessage{
						Headers: []models.HeaderField{
							{Name: "Set-Cookie", Value: "session=example-secret"},
						},
						BodyStorage: "omitted",
					},
				}
				live := p.LiveMetadata(tx)
				encoded, marshalErr := json.Marshal(live)
				if marshalErr != nil {
					errs <- marshalErr
					return
				}
				if strings.Contains(string(encoded), "example-secret") {
					errs <- fmt.Errorf("renderer metadata leaked a secret")
					return
				}
				if submitErr := p.Submit(context.Background(), tx); submitErr != nil {
					errs <- submitErr
					return
				}
				_ = p.cfg.Redaction.Warnings()
				_ = p.cfg.Redaction.Summary()
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	stats := p.Stats(capture.StateFinalized)
	if stats.Persisted != workers*transactionsPerWorker {
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
