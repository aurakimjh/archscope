package automation

import (
	"encoding/json"
	"sync"
)

type Event struct {
	ResyncRequired bool            `json:"resyncRequired,omitempty"`
	Sequence       uint64          `json:"sequence"`
	Kind           string          `json:"kind"`
	SessionID      string          `json:"sessionId"`
	Data           json.RawMessage `json:"data"`
}
type Events struct {
	Items          []Event `json:"items"`
	Latest         uint64  `json:"latest"`
	ResyncRequired bool    `json:"resyncRequired"`
}
type Journal struct {
	mu    sync.Mutex
	next  uint64
	size  int
	items []Event
}

func (j *Journal) Append(kind, session string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	oversized := len(raw) > 256<<10
	if oversized {
		raw = []byte(`{"resyncRequired":true}`)
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.next++
	j.items = append(j.items, Event{ResyncRequired: oversized, Sequence: j.next, Kind: kind, SessionID: session, Data: raw})
	j.size += len(raw)
	for len(j.items) > 512 || j.size > 1<<20 {
		j.size -= len(j.items[0].Data)
		j.items = j.items[1:]
	}
}
func (j *Journal) Read(after uint64, limit int) Events {
	j.mu.Lock()
	defer j.mu.Unlock()
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	out := Events{Items: []Event{}, Latest: j.next}
	if after > j.next {
		out.ResyncRequired = true
		return out
	}
	if len(j.items) > 0 && after < j.items[0].Sequence-1 {
		out.ResyncRequired = true
	}
	for _, item := range j.items {
		if item.Sequence > after && len(out.Items) < limit {
			if item.ResyncRequired {
				out.ResyncRequired = true
			}
			item.Data = append(json.RawMessage(nil), item.Data...)
			out.Items = append(out.Items, item)
		}
	}
	return out
}
