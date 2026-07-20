package aggregate

import (
	"math/rand"
	"reflect"
	"testing"
	"time"

	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/httpcapture"
)

func TestAggregationIsOrderIndependentAndBounded(t *testing.T) {
	base := []parser.Entry{{Method: "GET", Path: "/a", Host: "api", DurationMS: 20, ResponseBytes: 3}, {Method: "GET", Path: "/b", Host: "api", DurationMS: 40, Error: true}, {Method: "POST", Path: "/a", Host: "worker", DurationMS: 10}}
	want := normalizedSnapshot(New("one", 1), base)
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 50; i++ {
		gotEntries := append([]parser.Entry(nil), base...)
		rng.Shuffle(len(gotEntries), func(a, b int) { gotEntries[a], gotEntries[b] = gotEntries[b], gotEntries[a] })
		got := normalizedSnapshot(New("other", 1), gotEntries)
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("order changed aggregation: want=%+v got=%+v", want, got)
		}
	}
}
func TestEventSequenceAndSnapshotRecoveryContract(t *testing.T) {
	a := New("session-1", 10)
	first := a.ApplyBatch([]parser.Entry{{Method: "GET", Path: "/", Host: "api"}})
	second := a.ApplyBatch([]parser.Entry{{Method: "GET", Path: "/", Host: "api"}})
	if first.Sequence != 1 || second.Sequence != 2 || first.SnapshotVersion != 1 || second.SnapshotVersion != 2 {
		t.Fatalf("unexpected event sequence: %+v %+v", first, second)
	}
	snapshot := a.Snapshot()
	if snapshot.Sequence != second.Sequence || snapshot.SnapshotVersion != second.SnapshotVersion || snapshot.Total != 2 {
		t.Fatalf("snapshot cannot recover event state: %+v", snapshot)
	}
}

func TestLongSessionLiveCompletionOrderMatchesOfflineStartOrder(t *testing.T) {
	entries := make([]parser.Entry, 0, 50_000)
	for i := 0; i < cap(entries); i++ {
		entries = append(entries, parser.Entry{Method: "GET", Path: "/orders/" + string(rune('a'+i%23)), Host: "api", DurationMS: float64(i % 97), Error: i%29 == 0, ResponseBytes: int64(i % 4096)})
	}
	offline := normalizedSnapshot(New("offline", 50), entries)
	liveEntries := append([]parser.Entry(nil), entries...)
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
func normalizedSnapshot(a *Aggregator, entries []parser.Entry) Snapshot {
	a.ApplyBatch(entries)
	s := a.Snapshot()
	s.SessionID = ""
	s.GeneratedAt = time.Time{}
	return s
}
