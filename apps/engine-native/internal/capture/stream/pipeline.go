// Package stream implements the live capture three-tier pipeline:
// durable append, a bounded metadata-only live ring, and constant-size
// aggregate snapshots. Transaction events may be skipped under renderer
// backpressure; persistence and aggregate/stats events are never skipped.
package stream

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/aggregate"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const (
	DefaultHighWaterBytes = int64(64 << 20)
	DefaultHardLimitBytes = int64(96 << 20)
	DefaultLiveWindow     = 5000
	DefaultQueueRecords   = 1024
	DefaultBatchRecords   = 100
)

type Config struct {
	SessionID     capture.SessionID
	Store         *store.Store
	EventSink     capture.EventSink
	HighWater     int64
	HardLimit     int64
	LiveWindow    int
	QueueRecords  int
	BatchInterval time.Duration
	StatsInterval time.Duration
	Redaction     *redact.Policy
}

type request struct {
	tx   models.CaptureTransaction
	size int64
	ack  chan error
}

type Pipeline struct {
	cfg    Config
	queue  chan request
	stop   chan struct{}
	done   chan struct{}
	space  chan struct{}
	batch  chan models.CaptureTransaction
	agg    *aggregate.Aggregator
	sendMu sync.RWMutex

	mu          sync.RWMutex
	ring        []models.CaptureTransaction
	queuedBytes int64
	closed      bool

	captured      atomic.Uint64
	persisted     atomic.Uint64
	eventSkipped  atomic.Uint64
	parseFailed   atomic.Uint64
	kernelDropped atomic.Uint64
	unsupported   atomic.Uint64
	passthrough   atomic.Uint64
	unattributed  atomic.Uint64
	backpressured atomic.Bool
}

func New(cfg Config) (*Pipeline, error) {
	if cfg.Store == nil {
		return nil, errors.New("capture stream requires a store")
	}
	if cfg.EventSink == nil {
		cfg.EventSink = capture.NopEventSink{}
	}
	if cfg.HighWater <= 0 {
		cfg.HighWater = DefaultHighWaterBytes
	}
	if cfg.HardLimit <= cfg.HighWater {
		cfg.HardLimit = DefaultHardLimitBytes
	}
	if cfg.LiveWindow <= 0 {
		cfg.LiveWindow = DefaultLiveWindow
	}
	if cfg.QueueRecords <= 0 {
		cfg.QueueRecords = DefaultQueueRecords
	}
	if cfg.BatchInterval <= 0 {
		cfg.BatchInterval = 100 * time.Millisecond
	}
	if cfg.StatsInterval <= 0 {
		cfg.StatsInterval = time.Second
	}
	if cfg.Redaction == nil {
		cfg.Redaction = redact.NewPolicy(redact.Options{})
	}
	p := &Pipeline{
		cfg: cfg, queue: make(chan request, cfg.QueueRecords), stop: make(chan struct{}),
		done: make(chan struct{}), space: make(chan struct{}, 1),
		batch: make(chan models.CaptureTransaction, DefaultBatchRecords),
		agg:   aggregate.New(string(cfg.SessionID), aggregate.DefaultTopK),
		ring:  make([]models.CaptureTransaction, 0, cfg.LiveWindow),
	}
	go p.writer()
	go p.publisher()
	return p, nil
}

func (p *Pipeline) Submit(ctx context.Context, tx models.CaptureTransaction) error {
	p.captured.Add(1)
	tx = p.redact(tx)
	data, err := json.Marshal(tx)
	if err != nil {
		p.parseFailed.Add(1)
		return err
	}
	size := int64(len(data) + 32)
	if err := p.reserve(ctx, size); err != nil {
		return err
	}
	ack := make(chan error, 1)
	req := request{tx: tx, size: size, ack: ack}
	p.sendMu.RLock()
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()
	if closed {
		p.sendMu.RUnlock()
		p.release(size)
		return errors.New("capture stream is closed")
	}
	select {
	case p.queue <- req:
		p.sendMu.RUnlock()
	case <-ctx.Done():
		p.sendMu.RUnlock()
		p.release(size)
		return ctx.Err()
	case <-p.stop:
		p.sendMu.RUnlock()
		p.release(size)
		return errors.New("capture stream is closed")
	}
	select {
	case err := <-ack:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Pipeline) reserve(ctx context.Context, size int64) error {
	if size > p.cfg.HardLimit {
		return capture.ErrBackpressureHard
	}
	for {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return errors.New("capture stream is closed")
		}
		next := p.queuedBytes + size
		if next <= p.cfg.HardLimit {
			p.queuedBytes = next
			p.backpressured.Store(next > p.cfg.HighWater)
			p.mu.Unlock()
			return nil
		}
		p.backpressured.Store(true)
		p.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.space:
		case <-p.stop:
			return errors.New("capture stream is closed")
		}
	}
}

func (p *Pipeline) release(size int64) {
	p.mu.Lock()
	p.queuedBytes -= size
	if p.queuedBytes < 0 {
		p.queuedBytes = 0
	}
	p.backpressured.Store(p.queuedBytes > p.cfg.HighWater)
	p.mu.Unlock()
	select {
	case p.space <- struct{}{}:
	default:
	}
}

func (p *Pipeline) writer() {
	defer close(p.done)
	for req := range p.queue {
		item, err := p.cfg.Store.Append(req.tx)
		if err == nil {
			p.persisted.Add(1)
			p.agg.ApplyBatch([]models.CaptureTransaction{item.Transaction})
			p.addLive(item.Transaction)
			select {
			case p.batch <- item.Transaction:
			default:
				p.eventSkipped.Add(1)
			}
		}
		p.release(req.size)
		req.ack <- err
	}
	close(p.batch)
}

func (p *Pipeline) publisher() {
	batchTicker := time.NewTicker(p.cfg.BatchInterval)
	statsTicker := time.NewTicker(p.cfg.StatsInterval)
	defer batchTicker.Stop()
	defer statsTicker.Stop()
	pending := make([]models.CaptureTransaction, 0, DefaultBatchRecords)
	flushBatch := func() {
		if len(pending) == 0 {
			return
		}
		snapshot := p.agg.Snapshot()
		items := append([]models.CaptureTransaction(nil), pending...)
		p.cfg.EventSink.Transactions(p.cfg.SessionID, snapshot.Sequence, snapshot.SnapshotVersion, items)
		pending = pending[:0]
	}
	for {
		select {
		case tx, ok := <-p.batch:
			if !ok {
				flushBatch()
				return
			}
			pending = append(pending, metadataOnly(tx))
			if len(pending) >= DefaultBatchRecords {
				flushBatch()
			}
		case <-batchTicker.C:
			flushBatch()
		case <-statsTicker.C:
			snapshot := p.agg.Snapshot()
			p.cfg.EventSink.Aggregate(p.cfg.SessionID, snapshot.Sequence, snapshot.SnapshotVersion, snapshot)
			p.cfg.EventSink.Stats(p.Stats(capture.StateRunning))
		}
	}
}

func (p *Pipeline) addLive(tx models.CaptureTransaction) {
	p.mu.Lock()
	defer p.mu.Unlock()
	tx = metadataOnly(tx)
	if len(p.ring) == p.cfg.LiveWindow {
		copy(p.ring, p.ring[1:])
		p.ring[len(p.ring)-1] = tx
		return
	}
	p.ring = append(p.ring, tx)
}

func (p *Pipeline) LiveWindow() []models.CaptureTransaction {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]models.CaptureTransaction(nil), p.ring...)
}

func (p *Pipeline) Snapshot() aggregate.Snapshot { return p.agg.Snapshot() }

func (p *Pipeline) Stats(state capture.SessionState) capture.Stats {
	meta := p.cfg.Store.Meta()
	return capture.Stats{
		SessionID: p.cfg.SessionID, State: state,
		Captured: p.captured.Load(), Persisted: p.persisted.Load(),
		BodyOmitted: meta.Counters.BodyOmitted, EventSkipped: p.eventSkipped.Load(),
		KernelDropped: p.kernelDropped.Load(), ParseFailed: p.parseFailed.Load(),
		Unsupported: p.unsupported.Load(), Passthrough: p.passthrough.Load(),
		Unattributed: p.unattributed.Load(), Backpressured: p.backpressured.Load(),
		SnapshotVersion: meta.SnapshotVersion, Sequence: p.agg.Snapshot().Sequence,
		StoreBytes: meta.StoreBytes,
	}
}

func (p *Pipeline) MarkUnsupported()           { p.unsupported.Add(1) }
func (p *Pipeline) MarkPassthrough()           { p.passthrough.Add(1) }
func (p *Pipeline) MarkKernelDropped(n uint64) { p.kernelDropped.Add(n) }

func (p *Pipeline) Close() error {
	p.sendMu.Lock()
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		p.sendMu.Unlock()
		return nil
	}
	p.closed = true
	close(p.stop)
	close(p.queue)
	p.mu.Unlock()
	p.sendMu.Unlock()
	<-p.done
	return p.cfg.Store.Flush()
}

func (p *Pipeline) redact(tx models.CaptureTransaction) models.CaptureTransaction {
	tx.URL = p.cfg.Redaction.RedactURL(tx.URL)
	tx.Query = ""
	tx.Request.Headers = p.cfg.Redaction.RedactHeaders(tx.Request.Headers)
	tx.Response.Headers = p.cfg.Redaction.RedactHeaders(tx.Response.Headers)
	tx.Request.BodyPreview, tx.Request.Redacted = p.cfg.Redaction.RedactBody(tx.Request.ContentType, tx.Request.BodyPreview)
	tx.Response.BodyPreview, tx.Response.Redacted = p.cfg.Redaction.RedactBody(tx.Response.ContentType, tx.Response.BodyPreview)
	tx.Process = p.cfg.Redaction.RedactProcess(tx.Process)
	if tx.Process == nil {
		p.unattributed.Add(1)
	}
	return tx
}

func metadataOnly(tx models.CaptureTransaction) models.CaptureTransaction {
	tx.Request.BodyPreview = ""
	tx.Response.BodyPreview = ""
	tx.Request.BodyRef = ""
	tx.Response.BodyRef = ""
	return tx
}
