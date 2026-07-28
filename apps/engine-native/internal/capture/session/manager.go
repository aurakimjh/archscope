package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/aggregate"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/procmap"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/proxy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/stream"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type EventSink = capture.EventSink
type NopEventSink = capture.NopEventSink
type Session = capture.Session
type SessionID = capture.SessionID
type SessionState = capture.SessionState
type Config = capture.Config
type Mode = capture.Mode
type Capturer = capture.Capturer
type Stats = capture.Stats

const (
	StateStarting    = capture.StateStarting
	StateRunning     = capture.StateRunning
	StateStopping    = capture.StateStopping
	StateFinalized   = capture.StateFinalized
	StateFailed      = capture.StateFailed
	StateRecoverable = capture.StateRecoverable
	PrivilegeNone    = capture.PrivilegeNone
)

var (
	ErrSessionActive   = capture.ErrSessionActive
	ErrSessionNotFound = capture.ErrSessionNotFound
)

type Manager struct {
	mu          sync.RWMutex
	root        string
	sink        EventSink
	session     Session
	store       *store.Store
	pipeline    *stream.Pipeline
	capturer    Capturer
	authority   *proxy.Authority
	cancel      context.CancelFunc
	platform    string
	newCapturer func(proxy.Config) (Capturer, error)
}

func NewManager(root string, sink EventSink) *Manager {
	return NewManagerForPlatform(root, sink, runtime.GOOS)
}

// NewManagerForPlatform creates a manager with a deterministic capability
// platform. Production callers use NewManager; this constructor exists for
// cross-platform contract tests and Windows cross-compilation acceptance.
func NewManagerForPlatform(root string, sink EventSink, platform string) *Manager {
	if strings.TrimSpace(root) == "" {
		root = filepath.Join(os.TempDir(), "archscope-captures")
	}
	if sink == nil {
		sink = NopEventSink{}
	}
	return &Manager{
		root: root, sink: sink, platform: strings.ToLower(strings.TrimSpace(platform)),
		newCapturer: func(config proxy.Config) (Capturer, error) {
			return proxy.New(config)
		},
	}
}

func (m *Manager) Modes() []Mode {
	available := m.platform == "windows"
	reason := ""
	if !available {
		reason = "supported live capture requires Windows TCP-owner process attribution"
	}
	return []Mode{
		{
			Name: "proxy_h1_mitm", Available: available, Reason: reason,
			RequiredPrivilege: PrivilegeNone,
			Fidelity:          "HTTP/1.0 and HTTP/1.1 semantic interception; h2-only ALPN is explicit passthrough",
		},
	}
}

func (m *Manager) PrepareAuthority() ([]byte, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.authority == nil {
		authority, err := proxy.NewAuthority(time.Now().UTC())
		if err != nil {
			return nil, time.Time{}, err
		}
		m.authority = authority
	}
	return m.authority.CertificateDER(), m.authority.ExpiresAt(), nil
}

func (m *Manager) ReleaseAuthority() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.authority != nil {
		m.authority.Close()
		m.authority = nil
	}
}

func (m *Manager) Start(ctx context.Context, cfg Config) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if activeState(m.session.State) {
		return Session{}, ErrSessionActive
	}
	if m.platform != "windows" {
		return Session{}, fmt.Errorf("%w: supported live capture requires Windows TCP-owner process attribution", capture.ErrModeUnavailable)
	}
	id, err := newSessionID()
	if err != nil {
		return Session{}, err
	}
	root := cfg.StoreRoot
	if strings.TrimSpace(root) == "" {
		root = m.root
	}
	if m.authority == nil {
		m.authority, err = proxy.NewAuthority(time.Now().UTC())
		if err != nil {
			return Session{}, err
		}
	}
	st, err := store.New(store.Config{
		Root: root, SessionID: string(id), MaxBytes: cfg.MaxSessionBytes,
		ReserveBytes: cfg.ReserveBytes, BodyFraction: cfg.BodyFraction,
		OverflowPolicy: cfg.OverflowPolicy,
	})
	if err != nil {
		return Session{}, err
	}
	started := time.Now().UTC()
	session := Session{
		ID: id, State: StateStarting, StorePath: st.Path(), StartedAt: started,
		RetainUnattributedMetadata: cfg.RetainUnattributedMetadata,
	}
	if err := st.SetState(StateStarting); err != nil {
		_ = st.Finalize(StateFailed)
		return Session{}, err
	}
	pipeline, err := stream.New(stream.Config{
		SessionID: id, Store: st, EventSink: m.sink,
		HighWater: cfg.WriteHighWaterBytes, HardLimit: cfg.WriteHardLimitBytes,
		LiveWindow:                 cfg.LiveWindow,
		RetainUnattributedMetadata: cfg.RetainUnattributedMetadata,
	})
	if err != nil {
		_ = st.Finalize(StateFailed)
		return Session{}, err
	}
	allowUntil := map[string]time.Time{}
	ttl := time.Duration(cfg.PassthroughTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if ttl > 24*time.Hour {
		ttl = 24 * time.Hour
	}
	for _, host := range cfg.AllowPassthrough {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			allowUntil[host] = started.Add(ttl)
		}
	}
	capturer, err := m.newCapturer(proxy.Config{
		ListenAddress: cfg.ListenAddress, Authority: m.authority,
		Resolver: procmap.Resolver{}, AllowPassthrough: allowUntil,
		Progress: func(tx models.CaptureTransaction) {
			if !retainLiveMetadata(tx, cfg.RetainUnattributedMetadata) {
				return
			}
			_ = pipeline.TrackProgress(tx)
		},
	})
	if err != nil {
		_ = pipeline.Close()
		_ = st.Finalize(StateFailed)
		return Session{}, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	fatal := make(chan error, 1)
	address, err := capturer.Start(runCtx, cfg, func(tx models.CaptureTransaction) error {
		if tx.Fidelity == proxy.FidelityUnsupported {
			pipeline.MarkUnsupported()
		}
		if tx.CaptureMode == proxy.CaptureModePassthrough {
			pipeline.MarkPassthrough()
		}
		err := pipeline.Submit(runCtx, tx)
		if err != nil && runCtx.Err() == nil {
			select {
			case fatal <- err:
			default:
			}
		}
		return err
	})
	if err != nil {
		cancel()
		_ = pipeline.Close()
		_ = st.Finalize(StateFailed)
		return Session{}, err
	}
	session.State = StateRunning
	session.ListenAddress = address
	if err := st.SetState(StateRunning); err != nil {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = capturer.Stop(stopCtx)
		stopCancel()
		_ = pipeline.Close()
		_ = st.Finalize(StateFailed)
		return Session{}, err
	}
	m.session = session
	m.store = st
	m.pipeline = pipeline
	m.capturer = capturer
	m.cancel = cancel
	m.sink.Started(session)
	go func() {
		select {
		case fatalErr := <-fatal:
			m.sink.Error(id, fatalErr.Error())
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer stopCancel()
			_, _ = m.Stop(stopCtx, id)
		case <-runCtx.Done():
		}
	}()
	return session, nil
}

func (m *Manager) Stop(ctx context.Context, id SessionID) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session.ID == "" || (id != "" && id != m.session.ID) {
		return Session{}, ErrSessionNotFound
	}
	if m.session.State == StateFinalized || m.session.State == StateRecoverable || m.session.State == StateFailed {
		return m.session, nil
	}
	m.session.State = StateStopping
	_ = m.store.SetState(StateStopping)
	var stopErr error
	if m.capturer != nil {
		stopErr = m.capturer.Stop(ctx)
	}
	// Drain the capture source before cancelling the pipeline submission
	// context. A transaction may complete while the proxy is shutting down;
	// cancelling first can count that transaction as captured but prevent it
	// from entering the durable queue.
	if m.cancel != nil {
		m.cancel()
	}
	if m.pipeline != nil {
		stopErr = errors.Join(stopErr, m.pipeline.Close())
	}
	finalState := StateFinalized
	stats := Stats{}
	if m.pipeline != nil {
		stats = m.pipeline.Stats(StateStopping)
	}
	if stopErr != nil || stats.Captured != stats.Persisted {
		finalState = StateRecoverable
		_ = m.store.AddFinding(store.Finding{
			Code:    "CAPTURE_STOP_INCOMPLETE",
			Message: "capture stopped with an error or a captured/persisted mismatch",
			Evidence: map[string]any{
				"error": errorString(stopErr), "captured": stats.Captured, "persisted": stats.Persisted,
			},
		})
	}
	stats = m.pipeline.Stats(finalState)
	if err := m.store.SetCaptureStats(stats); err != nil {
		stopErr = errors.Join(stopErr, err)
		finalState = StateRecoverable
	}
	if err := m.store.Finalize(finalState); err != nil {
		stopErr = errors.Join(stopErr, err)
		finalState = StateRecoverable
	}
	ended := time.Now().UTC()
	m.session.State = finalState
	m.session.EndedAt = &ended
	m.session.Error = errorString(stopErr)
	m.sink.Stats(m.pipeline.Stats(finalState))
	m.sink.Stopped(m.session)
	return m.session, stopErr
}

func (m *Manager) Current() Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session
}

func (m *Manager) LiveWindow(id SessionID) ([]models.CaptureTransaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id == "" || id != m.session.ID || m.pipeline == nil {
		return nil, ErrSessionNotFound
	}
	return m.pipeline.LiveWindow(), nil
}

func (m *Manager) Stats(id SessionID) (Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id == "" || id == m.session.ID {
		if m.pipeline == nil {
			return Stats{}, ErrSessionNotFound
		}
		return m.pipeline.Stats(m.session.State), nil
	}
	st, err := m.openSession(id)
	if err != nil {
		return Stats{}, err
	}
	defer st.Close()
	meta := st.Meta()
	return Stats{
		SessionID: id, State: meta.State, Persisted: meta.Counters.Persisted,
		BodyOmitted: meta.Counters.BodyOmitted, SnapshotVersion: meta.SnapshotVersion,
		StoreBytes: meta.StoreBytes,
	}, nil
}

func (m *Manager) Fetch(id SessionID, filter store.Filter, cursor string, limit int) (store.Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id == m.session.ID && m.store != nil && activeState(m.session.State) {
		return m.store.Fetch(filter, cursor, limit)
	}
	st, err := m.openSession(id)
	if err != nil {
		return store.Page{}, err
	}
	defer st.Close()
	return st.Fetch(filter, cursor, limit)
}

func (m *Manager) Snapshot(id SessionID) (aggregate.Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id != m.session.ID || m.pipeline == nil {
		return aggregate.Snapshot{}, ErrSessionNotFound
	}
	return m.pipeline.Snapshot(), nil
}

func (m *Manager) RecoverSessions() ([]store.RecoveryReport, error) {
	m.mu.RLock()
	activePath := ""
	if activeState(m.session.State) {
		activePath = m.session.StorePath
	}
	m.mu.RUnlock()
	dirs, err := store.SortedSessionDirs(m.root)
	if err != nil {
		return nil, err
	}
	reports := make([]store.RecoveryReport, 0, len(dirs))
	for _, dir := range dirs {
		if activePath != "" && filepath.Clean(dir) == filepath.Clean(activePath) {
			continue
		}
		st, openErr := store.Open(dir)
		if openErr != nil {
			return reports, openErr
		}
		meta := st.Meta()
		closeErr := st.Close()
		if closeErr != nil {
			return reports, closeErr
		}
		if meta.State == StateRecoverable {
			reports = append(reports, store.RecoveryReport{
				Recovered: true, Records: int(meta.Counters.Persisted),
			})
		}
	}
	return reports, nil
}

func (m *Manager) SessionPath(id SessionID) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !validSessionID(id) {
		return "", ErrSessionNotFound
	}
	if id == m.session.ID && m.session.StorePath != "" {
		return m.session.StorePath, nil
	}
	path := filepath.Join(m.root, string(id))
	if _, err := os.Stat(filepath.Join(path, "manifest.json")); err != nil {
		return "", ErrSessionNotFound
	}
	return path, nil
}

func (m *Manager) openSession(id SessionID) (*store.Store, error) {
	if !validSessionID(id) {
		return nil, ErrSessionNotFound
	}
	path := filepath.Join(m.root, string(id))
	if id == m.session.ID && m.session.StorePath != "" {
		path = m.session.StorePath
	}
	st, err := store.Open(path)
	if os.IsNotExist(err) {
		return nil, ErrSessionNotFound
	}
	return st, err
}

func validSessionID(id SessionID) bool {
	value := string(id)
	return strings.HasPrefix(value, "cap-") &&
		filepath.Base(value) == value &&
		!strings.Contains(value, "..")
}

func activeState(state SessionState) bool {
	return state == StateStarting || state == StateRunning || state == StateStopping
}

func retainLiveMetadata(tx models.CaptureTransaction, retainUnattributed bool) bool {
	return retainUnattributed ||
		(tx.Process != nil && tx.Process.Attribution == "confirmed")
}

func newSessionID() (SessionID, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate capture session ID: %w", err)
	}
	return SessionID("cap-" + time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(random[:])), nil
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
