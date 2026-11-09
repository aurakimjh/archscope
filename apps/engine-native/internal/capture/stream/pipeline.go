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
	ProcessScope   string
	ScopeSessionID *uint32
	// RequireGeneration expects producers to call Stamp when each request begins.
	// Legacy direct callers retain the timestamp boundary when this is false.
	RequireGeneration          bool
	AllowedProcesses           []models.ProcessKey
	SessionID                  capture.SessionID
	Store                      *store.Store
	EventSink                  capture.EventSink
	HighWater                  int64
	HardLimit                  int64
	LiveWindow                 int
	QueueRecords               int
	BatchInterval              time.Duration
	StatsInterval              time.Duration
	Redaction                  *redact.Policy
	RetainUnattributedMetadata bool
}

type request struct {
	tx   models.CaptureTransaction
	size int64
	ack  chan error
}

type Pipeline struct {
	paused     bool      // protected by sendMu
	clearedAt  time.Time // protected by sendMu
	generation atomic.Uint64
	cfg        Config
	queue      chan request
	stop       chan struct{}
	done       chan struct{}
	published  chan struct{}
	space      chan struct{}
	batch      chan models.CaptureTransaction
	progress   chan models.CaptureTransaction
	agg        *aggregate.Aggregator
	sendMu     sync.RWMutex

	mu          sync.RWMutex
	liveOrder   []string
	liveByID    map[string]models.CaptureTransaction
	queuedBytes int64
	closed      bool

	observed      atomic.Uint64
	captured      atomic.Uint64
	persisted     atomic.Uint64
	eventSkipped  atomic.Uint64
	parseFailed   atomic.Uint64
	kernelDropped atomic.Uint64
	unsupported   atomic.Uint64
	passthrough   atomic.Uint64
	unattributed  atomic.Uint64
	dropped       atomic.Uint64
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
		done: make(chan struct{}), published: make(chan struct{}),
		space:     make(chan struct{}, 1),
		batch:     make(chan models.CaptureTransaction, DefaultBatchRecords),
		progress:  make(chan models.CaptureTransaction, DefaultBatchRecords),
		agg:       aggregate.New(string(cfg.SessionID), aggregate.DefaultTopK),
		liveOrder: make([]string, 0, cfg.LiveWindow),
		liveByID:  make(map[string]models.CaptureTransaction, cfg.LiveWindow),
	}
	go p.writer()
	go p.publisher()
	return p, nil
}

func (p *Pipeline) Submit(ctx context.Context, tx models.CaptureTransaction) error {
	p.sendMu.RLock()
	defer p.sendMu.RUnlock()
	if !p.recording(tx) {
		return nil
	}
	tx.Generation = p.generation.Load()
	p.observed.Add(1)
	if tx.Fidelity == "unsupported" {
		p.unsupported.Add(1)
	}
	if tx.CaptureMode == "proxy_passthrough" {
		p.passthrough.Add(1)
	}
	if !p.allows(tx) {
		p.dropped.Add(1)
		return nil
	}
	if unattributed(tx) {
		p.unattributed.Add(1)
		if !p.cfg.RetainUnattributedMetadata {
			p.dropped.Add(1)
			return nil
		}
	}
	p.captured.Add(1)
	tx = p.redact(tx)
	data, err := json.Marshal(tx)
	if err != nil {
		p.parseFailed.Add(1)
		p.failLive(tx, err)
		return err
	}
	size := int64(len(data) + 32)
	if err := p.reserve(ctx, size); err != nil {
		p.failLive(tx, err)
		return err
	}
	ack := make(chan error, 1)
	req := request{tx: tx, size: size, ack: ack}
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()
	if closed {
		p.release(size)
		err := errors.New("capture stream is closed")
		p.failLive(tx, err)
		return err
	}
	select {
	case p.queue <- req:
	case <-ctx.Done():
		p.release(size)
		p.failLive(tx, ctx.Err())
		return ctx.Err()
	case <-p.stop:
		p.release(size)
		err := errors.New("capture stream is closed")
		p.failLive(tx, err)
		return err
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
		item, err := p.cfg.Store.AppendWithCheckpoint(req.tx, store.CaptureCheckpoint{
			Stats:     p.Stats(capture.StateRunning),
			Redaction: p.RedactionSummary(),
		})
		if err == nil {
			p.persisted.Add(1)
			p.agg.ApplyBatch([]models.CaptureTransaction{item.Transaction})
			p.upsertLive(item.Transaction)
			select {
			case p.batch <- item.Transaction:
			default:
				p.eventSkipped.Add(1)
			}
		} else {
			p.failLive(req.tx, err)
		}
		p.release(req.size)
		req.ack <- err
	}
	close(p.batch)
}

func (p *Pipeline) publisher() {
	defer close(p.published)
	batchTicker := time.NewTicker(p.cfg.BatchInterval)
	statsTicker := time.NewTicker(p.cfg.StatsInterval)
	defer batchTicker.Stop()
	defer statsTicker.Stop()
	pendingTransactions := make([]models.CaptureTransaction, 0, DefaultBatchRecords)
	pendingProgress := make([]models.CaptureTransaction, 0, DefaultBatchRecords)
	flushTransactions := func() {
		if len(pendingTransactions) == 0 {
			return
		}
		snapshot := p.agg.Snapshot()
		items := append([]models.CaptureTransaction(nil), pendingTransactions...)
		p.cfg.EventSink.Transactions(p.cfg.SessionID, snapshot.Sequence, snapshot.SnapshotVersion, items)
		pendingTransactions = pendingTransactions[:0]
	}
	flushProgress := func() {
		if len(pendingProgress) == 0 {
			return
		}
		items := append([]models.CaptureTransaction(nil), pendingProgress...)
		p.cfg.EventSink.Progress(p.cfg.SessionID, items)
		pendingProgress = pendingProgress[:0]
	}
	transactionInput := p.batch
	progressInput := p.progress
	for transactionInput != nil || progressInput != nil {
		select {
		case tx, ok := <-transactionInput:
			if !ok {
				transactionInput = nil
				flushTransactions()
				continue
			}
			pendingTransactions = append(pendingTransactions, metadataOnly(tx))
			if len(pendingTransactions) >= DefaultBatchRecords {
				flushTransactions()
			}
		case tx, ok := <-progressInput:
			if !ok {
				progressInput = nil
				flushProgress()
				continue
			}
			pendingProgress = append(pendingProgress, tx)
			if len(pendingProgress) >= DefaultBatchRecords {
				flushProgress()
			}
		case <-batchTicker.C:
			flushTransactions()
			flushProgress()
		case <-statsTicker.C:
			snapshot := p.agg.Snapshot()
			p.cfg.EventSink.Aggregate(p.cfg.SessionID, snapshot.Sequence, snapshot.SnapshotVersion, snapshot)
			p.cfg.EventSink.Stats(p.Stats(capture.StateRunning))
		}
	}
	flushTransactions()
	flushProgress()
}

func (p *Pipeline) upsertLive(tx models.CaptureTransaction) {
	p.mu.Lock()
	defer p.mu.Unlock()
	tx = metadataOnly(tx)
	if _, exists := p.liveByID[tx.ID]; exists {
		p.liveByID[tx.ID] = tx
		return
	}
	if len(p.liveOrder) == p.cfg.LiveWindow {
		delete(p.liveByID, p.liveOrder[0])
		copy(p.liveOrder, p.liveOrder[1:])
		p.liveOrder[len(p.liveOrder)-1] = tx.ID
	} else {
		p.liveOrder = append(p.liveOrder, tx.ID)
	}
	p.liveByID[tx.ID] = tx
}

func (p *Pipeline) LiveWindow() []models.CaptureTransaction {
	p.mu.RLock()
	defer p.mu.RUnlock()
	window := make([]models.CaptureTransaction, 0, len(p.liveOrder))
	for _, id := range p.liveOrder {
		if tx, ok := p.liveByID[id]; ok {
			window = append(window, tx)
		}
	}
	return window
}

func (p *Pipeline) LiveMetadata(tx models.CaptureTransaction) models.CaptureTransaction {
	return metadataOnly(p.redact(tx))
}

func (p *Pipeline) TrackProgress(tx models.CaptureTransaction) error {
	p.sendMu.RLock()
	defer p.sendMu.RUnlock()
	if !p.recording(tx) {
		return nil
	}
	tx.Generation = p.generation.Load()
	if !p.allows(tx) {
		return nil
	}
	tx = p.LiveMetadata(tx)
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()
	if closed {
		return errors.New("capture stream is closed")
	}
	p.upsertLive(tx)
	select {
	case p.progress <- tx:
	default:
		p.eventSkipped.Add(1)
	}
	return nil
}

func (p *Pipeline) Snapshot() aggregate.Snapshot { return p.agg.Snapshot() }

func (p *Pipeline) RedactionSummary() redact.Summary { return p.cfg.Redaction.Summary() }

func (p *Pipeline) Stats(state capture.SessionState) capture.Stats {
	meta := p.cfg.Store.Meta()
	return capture.Stats{
		SessionID: p.cfg.SessionID, State: state,
		Generation: p.generation.Load(), Observed: p.observed.Load(), Captured: p.captured.Load(), Persisted: p.persisted.Load(),
		BodyOmitted: meta.Counters.BodyOmitted, EventSkipped: p.eventSkipped.Load(),
		KernelDropped: p.kernelDropped.Load(), ParseFailed: p.parseFailed.Load(),
		Unsupported: p.unsupported.Load(), Passthrough: p.passthrough.Load(),
		Unattributed: p.unattributed.Load(), Dropped: p.dropped.Load(),
		Backpressured:   p.backpressured.Load(),
		SnapshotVersion: meta.SnapshotVersion, Sequence: p.agg.Snapshot().Sequence,
		StoreBytes: meta.StoreBytes,
	}
}

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
	<-p.done
	abortErr := p.abortInflight(time.Now().UTC())
	close(p.progress)
	p.sendMu.Unlock()
	<-p.published
	return errors.Join(abortErr, p.cfg.Store.Flush())
}

func (p *Pipeline) failLive(tx models.CaptureTransaction, err error) {
	if tx.ID == "" || err == nil {
		return
	}
	tx.State = models.TxFailed
	tx.Error = "transaction was not persisted: " + err.Error()
	tx.EndedAt = time.Now().UTC().Format(time.RFC3339Nano)
	tx = metadataOnly(tx)
	p.upsertLive(tx)
	p.publishProgress(tx)
}

func (p *Pipeline) abortInflight(ended time.Time) error {
	p.mu.Lock()
	aborted := make([]models.CaptureTransaction, 0)
	for _, id := range p.liveOrder {
		tx, ok := p.liveByID[id]
		if !ok || (tx.State != models.TxRequestSent && tx.State != models.TxReceiving) {
			continue
		}
		tx.State = models.TxAborted
		tx.Fidelity = "unsupported"
		tx.EndedAt = ended.Format(time.RFC3339Nano)
		tx.Error = "capture stopped before the transaction completed"
		p.liveByID[id] = tx
		aborted = append(aborted, tx)
	}
	p.mu.Unlock()
	var persistErr error
	for _, tx := range aborted {
		p.observed.Add(1)
		p.captured.Add(1)
		if tx.Fidelity == "unsupported" {
			p.unsupported.Add(1)
		}
		if tx.CaptureMode == "proxy_passthrough" {
			p.passthrough.Add(1)
		}
		item, err := p.cfg.Store.Append(tx)
		if err != nil {
			persistErr = errors.Join(persistErr, err)
		} else {
			p.persisted.Add(1)
			p.agg.ApplyBatch([]models.CaptureTransaction{item.Transaction})
		}
		select {
		case p.progress <- tx:
		default:
			p.eventSkipped.Add(1)
		}
	}
	return persistErr
}

func (p *Pipeline) publishProgress(tx models.CaptureTransaction) {
	select {
	case p.progress <- tx:
	default:
		p.eventSkipped.Add(1)
	}
}

func (p *Pipeline) redact(tx models.CaptureTransaction) models.CaptureTransaction {
	tx.URL = p.cfg.Redaction.RedactURL(tx.URL)
	tx.Query = ""
	tx.Request.Headers = p.cfg.Redaction.RedactHeaders(tx.Request.Headers)
	tx.Response.Headers = p.cfg.Redaction.RedactHeaders(tx.Response.Headers)
	p.redactMessage(&tx.Request)
	p.redactMessage(&tx.Response)
	tx.Process = p.cfg.Redaction.RedactProcess(tx.Process)
	return tx
}

// redactMessage runs the body policy and keeps BodyStorage truthful about the
// result. The policy suppresses non-text and oversized-JSON payloads outright,
// and a message left claiming `inline` with an empty preview would read as "the
// body was empty" instead of "the body was suppressed".
func (p *Pipeline) redactMessage(message *models.HTTPMessage) {
	before := message.BodyPreview
	message.BodyPreview, message.Redacted = p.cfg.Redaction.RedactBody(message.ContentType, before)
	if before != "" && message.BodyPreview == "" {
		message.BodyStorage = "redacted"
	}
}

func unattributed(tx models.CaptureTransaction) bool {
	return tx.Process == nil || tx.Process.Attribution != "confirmed"
}

func metadataOnly(tx models.CaptureTransaction) models.CaptureTransaction {
	tx.Request.BodyPreview = ""
	tx.Response.BodyPreview = ""
	tx.Request.BodyRef = ""
	tx.Response.BodyRef = ""
	return tx
}

// A selected recording scope fails closed for unknown or reused identities.
// This check precedes redaction, persistence and publication.
func (p *Pipeline) allows(tx models.CaptureTransaction) bool {
	if p.cfg.ProcessScope == "current_session" {
		return !unattributed(tx) && tx.Process.WindowsSessionID != nil && p.cfg.ScopeSessionID != nil && *tx.Process.WindowsSessionID == *p.cfg.ScopeSessionID
	}
	if len(p.cfg.AllowedProcesses) == 0 {
		return p.cfg.ProcessScope != "selected" && p.cfg.ProcessScope != "launch"
	}
	if unattributed(tx) {
		return false
	}
	for _, key := range p.cfg.AllowedProcesses {
		if key == tx.Process.Key {
			return true
		}
	}
	return false
}

func (p *Pipeline) recording(tx models.CaptureTransaction) bool {
	if p.paused {
		return false
	}
	if p.cfg.RequireGeneration {
		return tx.Generation == p.generation.Load()
	}
	if p.clearedAt.IsZero() {
		return true
	}
	started, err := time.Parse(time.RFC3339Nano, tx.StartedAt)
	return err == nil && started.After(p.clearedAt)
}

// Stamp binds admission to the log generation when the request begins. Wall
// clocks can give Clear and a later request identical timestamps on Windows.
// Requests begun while paused carry an invalid generation even after resume.
func (p *Pipeline) Stamp(tx *models.CaptureTransaction) {
	p.sendMu.RLock()
	defer p.sendMu.RUnlock()
	tx.Generation = p.generation.Load()
	if p.paused {
		tx.Generation = ^uint64(0)
	}
}

func (p *Pipeline) SetPaused(paused bool) {
	p.sendMu.Lock()
	defer p.sendMu.Unlock()
	p.paused = paused
}

// Clear waits for acknowledged writes, removes the log and resets aggregation.
// Old buffered events retain their generation and cannot repopulate the renderer.
func (p *Pipeline) Clear() (uint64, error) {
	p.sendMu.Lock()
	defer p.sendMu.Unlock()
	if p.closed {
		return p.generation.Load(), errors.New("capture stream is closed")
	}
	if err := p.cfg.Store.Clear(); err != nil {
		p.paused = true
		return p.generation.Load(), err
	}
	p.clearedAt = time.Now().UTC()
	generation := p.generation.Load() + 1
	p.agg.Clear()
	p.mu.Lock()
	p.liveOrder = nil
	p.liveByID = map[string]models.CaptureTransaction{}
	p.mu.Unlock()
	p.observed.Store(0)
	p.captured.Store(0)
	p.persisted.Store(0)
	p.eventSkipped.Store(0)
	p.parseFailed.Store(0)
	p.kernelDropped.Store(0)
	p.unsupported.Store(0)
	p.passthrough.Store(0)
	p.unattributed.Store(0)
	p.dropped.Store(0)
	p.cfg.Redaction.ClearCounts()

	p.generation.Store(generation)
	return generation, nil
}

// SetSelectedProcesses is used before a newly launched, suspended process runs.
func (p *Pipeline) SetSelectedProcesses(keys []models.ProcessKey) {
	p.sendMu.Lock()
	defer p.sendMu.Unlock()
	p.cfg.AllowedProcesses = append([]models.ProcessKey(nil), keys...)
}
