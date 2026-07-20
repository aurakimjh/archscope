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
func normalizedSnapshot(a *Aggregator, entries []parser.Entry) Snapshot {
	a.ApplyBatch(entries)
	s := a.Snapshot()
	s.SessionID = ""
	s.GeneratedAt = time.Time{}
	return s
}
