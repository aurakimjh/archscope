package comautomation

import (
	"context"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestEventPrimeUpdateDiscardAndSessionReset(t *testing.T) {
	rows := []models.CaptureTransaction{{ID: "one", Method: "GET", StatusCode: 200}}
	version := uint64(1)
	generation := uint64(0)
	c := New(func(_ context.Context, call automation.Call) (any, error) {
		switch call.Command {
		case "status":
			return capture.Session{ID: "session", State: capture.StateRunning, Generation: generation}, nil
		case "stats":
			return capture.Stats{SnapshotVersion: version}, nil
		case "legacy-log":
			return rows, nil
		}
		return nil, nil
	})
	ctx := context.Background()
	cursor := &eventCursor{}
	if events, e := c.pollEvents(ctx, cursor, true); e != nil || len(events) != 0 {
		t.Fatal(events, e)
	}
	rows = append(rows, models.CaptureTransaction{ID: "two", Method: "POST", StatusCode: 201})
	version++
	events, e := c.pollEvents(ctx, cursor, false)
	if e != nil || len(events) != 1 || !events[0].New || events[0].ID != "two" {
		t.Fatal(events, e)
	}
	rows[1].StatusCode = 202
	version++
	events, e = c.pollEvents(ctx, cursor, false)
	if e != nil || len(events) != 1 || events[0].New {
		t.Fatal(events, e)
	}
	c.discardEntry("two", cursor.Source)
	if events, e = c.pollEvents(ctx, cursor, false); e != nil || len(events) != 0 {
		t.Fatal(events, e)
	}
	log, e := c.Invoke(ctx, "Log", nil, false)
	if e != nil {
		t.Fatal(e)
	}
	if count, _ := log.(Object).Invoke(ctx, "Count", nil, false); count != 1 {
		t.Fatal("discard did not update live log", count)
	}
	rows = []models.CaptureTransaction{{ID: "three"}}
	generation++
	version++
	events, e = c.pollEvents(ctx, cursor, false)
	if e != nil || len(events) != 1 || !events[0].New {
		t.Fatal(events, e)
	}
	c.discardEntry("three", "obsolete-source")
	if c.omittedIDs["three"] {
		t.Fatal("stale callback discarded replacement entry")
	}
	if len(cursor.Seen) != 1 {
		t.Fatal("stale subscription state retained")
	}
}
