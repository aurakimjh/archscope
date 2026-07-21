package aggregate

import (
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestAggregationIsOrderIndependentAndBounded(t *testing.T) {
	base := []models.CaptureTransaction{
		{Method: "GET", Path: "/a", Host: "api", TotalMS: 20, State: models.TxComplete, Response: models.HTTPMessage{TransferSize: 3}},
		{Method: "GET", Path: "/b", Host: "api", TotalMS: 40, StatusCode: 500, State: models.TxComplete, Response: models.HTTPMessage{TransferSize: -1, BodySize: -1}},
		{Method: "POST", Path: "/a", Host: "worker", TotalMS: 10, State: models.TxComplete, Response: models.HTTPMessage{TransferSize: -1, BodySize: -1}},
	}
	want := normalizedSnapshot(New("one", 1), base)
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 50; i++ {
		gotEntries := append([]models.CaptureTransaction(nil), base...)
		rng.Shuffle(len(gotEntries), func(a, b int) { gotEntries[a], gotEntries[b] = gotEntries[b], gotEntries[a] })
		got := normalizedSnapshot(New("other", 1), gotEntries)
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("order changed aggregation: want=%+v got=%+v", want, got)
		}
	}
}
func TestEventSequenceAndSnapshotRecoveryContract(t *testing.T) {
	a := New("session-1", 10)
	first := a.ApplyBatch([]models.CaptureTransaction{{Method: "GET", Path: "/", Host: "api"}})
	second := a.ApplyBatch([]models.CaptureTransaction{{Method: "GET", Path: "/", Host: "api"}})
	if first.Sequence != 1 || second.Sequence != 2 || first.SnapshotVersion != 1 || second.SnapshotVersion != 2 {
		t.Fatalf("unexpected event sequence: %+v %+v", first, second)
	}
	snapshot := a.Snapshot()
	if snapshot.Sequence != second.Sequence || snapshot.SnapshotVersion != second.SnapshotVersion || snapshot.Total != 2 {
		t.Fatalf("snapshot cannot recover event state: %+v", snapshot)
	}
}

func TestLongSessionLiveCompletionOrderMatchesOfflineStartOrder(t *testing.T) {
	entries := make([]models.CaptureTransaction, 0, 50_000)
	for i := 0; i < cap(entries); i++ {
		status := 200
		if i%29 == 0 {
			status = 500
		}
		entries = append(entries, models.CaptureTransaction{Method: "GET", Path: "/orders/" + string(rune('a'+i%23)), Host: "api", TotalMS: float64(i % 97), StatusCode: status, State: models.TxComplete, Response: models.HTTPMessage{TransferSize: int64(i % 4096)}})
	}
	offline := normalizedSnapshot(New("offline", 50), entries)
	liveEntries := append([]models.CaptureTransaction(nil), entries...)
	rng := rand.New(rand.NewSource(99))
	rng.Shuffle(len(liveEntries), func(i, j int) { liveEntries[i], liveEntries[j] = liveEntries[j], liveEntries[i] })
	live := New("live", 50)
	for start := 0; start < len(liveEntries); start += 137 {
		end := start + 137
		if end > len(liveEntries) {
			end = len(liveEntries)
		}
		live.ApplyBatch(liveEntries[start:end])
	}
	got := live.Snapshot()
	got.SessionID = ""
	got.GeneratedAt = time.Time{}
	got.Sequence = 1
	got.SnapshotVersion = 1
	if !reflect.DeepEqual(offline, got) {
		t.Fatalf("long-session live/offline parity mismatch: offline=%+v live=%+v", offline, got)
	}
}
func normalizedSnapshot(a *Aggregator, entries []models.CaptureTransaction) Snapshot {
	a.ApplyBatch(entries)
	s := a.Snapshot()
	s.SessionID = ""
	s.GeneratedAt = time.Time{}
	return s
}
