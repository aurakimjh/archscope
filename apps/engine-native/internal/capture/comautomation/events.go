package comautomation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// A Log reference remains live across SetFilter and new capture snapshots.
// Enumeration and individual entries remain immutable, bounded snapshots.
type liveLog struct{ controller *Controller }

func (liveLog) Members() []string {
	return []string{"Count", "Item", "_NewEnum", "FilteredCount", "FilteredItem"}
}
func (l liveLog) Invoke(ctx context.Context, name string, args []any, put bool) (any, error) {
	value, err := l.controller.Invoke(ctx, "__snapshot", nil, false)
	if err != nil {
		return nil, err
	}
	return value.(Object).Invoke(ctx, name, args, put)
}

type logEvent struct {
	ID    string
	New   bool
	Entry Object
}
type eventCursor struct {
	Source string
	Seen   map[string][32]byte
}

// Poll events describe the persisted view. The admission adapter sends new-entry
// callbacks before storage; already-admitted rows produce only update events.
// Prime an Advise subscription without replaying pre-existing rows.
func (c *Controller) pollEvents(ctx context.Context, cursor *eventCursor, prime bool) ([]logEvent, error) {
	if _, err := c.Invoke(ctx, "__snapshot", nil, false); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	source := fmt.Sprintf("%s:%d", c.snapshotSource, c.eventEpoch)
	if cursor.Source != source {
		cursor.Seen = nil
		cursor.Source = source
	}
	next := make(map[string][32]byte, len(c.lastRows))
	out := []logEvent{}
	for _, row := range c.lastRows {
		data, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		fingerprint := sha256.Sum256(data)
		next[row.ID] = fingerprint
		before, exists := cursor.Seen[row.ID]
		if prime || (exists && before == fingerprint) {
			continue
		}
		entry := entryObject(row).(Typed)
		entry.Object.(Properties)["IsFiltered"] = !c.filter.matches(row)
		key := fmt.Sprintf("%s:%d:%s", c.sessionID, row.Generation, row.ID)
		priorAdmission := c.preNotified[key]

		out = append(out, logEvent{row.ID, !exists && !priorAdmission, entry})
	}
	cursor.Seen = next
	return out, nil
}
func (c *Controller) discardEntry(id, source string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if source != fmt.Sprintf("%s:%d", c.snapshotSource, c.eventEpoch) {
		return
	}
	for _, row := range c.lastRows {
		if row.ID == id {
			if c.omittedIDs == nil {
				c.omittedIDs = map[string]bool{}
			}
			c.omittedIDs[id] = true
			c.cachedLog = nil
			return
		}
	}
}
