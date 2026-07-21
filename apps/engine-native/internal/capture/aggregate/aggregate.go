// Package aggregate provides the bounded, deterministic aggregation core shared
// by offline HAR import and future live capture. It never owns raw bodies or a
// Wails runtime; callers publish its immutable snapshots through their own
// transport.
package aggregate

import (
	"sort"
	"sync"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const DefaultTopK = 50

// Event is the recoverable renderer protocol. Sequence is monotonic for one
// capture session, and SnapshotVersion changes only after ApplyBatch commits.
// A client that observes a gap must request Snapshot rather than guessing.
type Event struct {
	SessionID       string `json:"session_id"`
	Sequence        uint64 `json:"sequence"`
	SnapshotVersion uint64 `json:"snapshot_version"`
	Kind            string `json:"kind"` // transaction_batch | snapshot_required
	Dropped         uint64 `json:"dropped,omitempty"`
}

type Row struct {
	Key             string  `json:"key"`
	Count           int     `json:"count"`
	Errors          int     `json:"errors"`
	TotalDurationMS float64 `json:"total_duration_ms"`
	ResponseBytes   int64   `json:"response_bytes"`
}

type Snapshot struct {
	SessionID       string    `json:"session_id"`
	SnapshotVersion uint64    `json:"snapshot_version"`
	Sequence        uint64    `json:"sequence"`
	GeneratedAt     time.Time `json:"generated_at"`
	Total           int       `json:"total"`
	Errors          int       `json:"errors"`
	TopEndpoints    []Row     `json:"top_endpoints"`
	TopHosts        []Row     `json:"top_hosts"`
}

type Aggregator struct {
	mu                        sync.Mutex
	sessionID                 string
	topK                      int
	sequence, snapshotVersion uint64
	total, errors             int
	endpoints, hosts          map[string]*Row
}

func New(sessionID string, topK int) *Aggregator {
	if topK <= 0 {
		topK = DefaultTopK
	}
	return &Aggregator{sessionID: sessionID, topK: topK, endpoints: map[string]*Row{}, hosts: map[string]*Row{}}
}

// ApplyBatch is order-independent: every input applies only associative sums.
// It returns one event after a successful atomic batch commit.
func (a *Aggregator) ApplyBatch(entries []models.CaptureTransaction) Event {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, entry := range entries {
		a.total++
		if transactionError(entry) {
			a.errors++
		}
		add(a.endpoints, entry.Method+" "+entry.Path, entry)
		add(a.hosts, entry.Host, entry)
	}
	a.sequence++
	a.snapshotVersion++
	return Event{SessionID: a.sessionID, Sequence: a.sequence, SnapshotVersion: a.snapshotVersion, Kind: "transaction_batch"}
}

func (a *Aggregator) Snapshot() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return Snapshot{SessionID: a.sessionID, SnapshotVersion: a.snapshotVersion, Sequence: a.sequence, GeneratedAt: time.Now().UTC(), Total: a.total, Errors: a.errors, TopEndpoints: topRows(a.endpoints, a.topK), TopHosts: topRows(a.hosts, a.topK)}
}

func add(target map[string]*Row, key string, entry models.CaptureTransaction) {
	if key == "" {
		key = "(unknown)"
	}
	row := target[key]
	if row == nil {
		row = &Row{Key: key}
		target[key] = row
	}
	row.Count++
	if transactionError(entry) {
		row.Errors++
	}
	row.TotalDurationMS += entry.TotalMS
	if entry.Response.TransferSize >= 0 {
		row.ResponseBytes += entry.Response.TransferSize
	} else if entry.Response.BodySize >= 0 {
		row.ResponseBytes += entry.Response.BodySize
	}
}

func transactionError(entry models.CaptureTransaction) bool {
	return entry.StatusCode >= 400 || entry.State == models.TxFailed
}
func topRows(source map[string]*Row, limit int) []Row {
	out := make([]Row, 0, len(source))
	for _, row := range source {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalDurationMS != out[j].TotalDurationMS {
			return out[i].TotalDurationMS > out[j].TotalDurationMS
		}
		return out[i].Key < out[j].Key
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
