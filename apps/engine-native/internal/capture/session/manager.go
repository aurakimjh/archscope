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
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/admission"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/aggregate"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/backend"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/dumppolicy"
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
	admissions  *admission.Hub
	backends    *backend.Registry
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
	// dumpPolicy is the SEC-10 preflight. It is unexported and has no setter,
	// so only this package's own tests can substitute it — a caller outside
	// the package cannot hand the manager a forged pass. proxy.New performs
	// the same check independently rather than trusting this one.
	dumpPolicy func() dumppolicy.Result
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
	return &Manager{admissions: admission.New(),
		root: root, sink: sink, platform: strings.ToLower(strings.TrimSpace(platform)),
		backends:   backend.Default(),
		dumpPolicy: dumppolicy.Current,
	}
}

func (m *Manager) Backends() []backend.Capability { return m.backends.Capabilities(m.platform) }
func (m *Manager) ValidateBackend(name string) (string, error) {
	return m.backends.Validate(m.platform, name)
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
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if activeState(m.session.State) {
		return Session{}, ErrSessionActive
	}
	backendName, backendErr := m.ValidateBackend(cfg.Backend)
	if backendErr != nil {
		return Session{}, backendErr
	}
	cfg.Backend = backendName
	// H-SEC2 C1 requires a verified crash-dump exclusion preflight before any
	// plaintext body tier can start. The preflight runs here, before an
	// authority is generated or any session file is created, so a rejected
	// request leaves no artifact behind.
	switch cfg.ProcessScope {
	case "", "all_sessions", "launch":
	case "selected":
		if len(cfg.AllowedProcesses) == 0 {
			return Session{}, fmt.Errorf("select at least one process")
		}
	case "current_session":
		if cfg.ScopeSessionID == nil {
			return Session{}, fmt.Errorf("current Windows session is unavailable")
		}
	default:
		return Session{}, fmt.Errorf("unknown recording scope")
	}
	if len(cfg.AllowedProcesses) > 1024 {
		return Session{}, fmt.Errorf("too many selected processes")
	}
	for _, key := range cfg.AllowedProcesses {
		if key.PID <= 0 || key.StartTime == "" {
			return Session{}, fmt.Errorf("selected process requires PID and start time")
		}
	}
	cfg.AllowedProcesses = append([]models.ProcessKey(nil), cfg.AllowedProcesses...)
	if err := m.admissions.Validate(cfg.AdmissionOwner); err != nil {
		return Session{}, err
	}
	bodyGate := capture.BodyGateDisabled
	bodyGateReason := ""
	if cfg.CaptureBodies {
		result := m.dumpPolicy()
		if !result.Verified() {
			return Session{}, fmt.Errorf("%w: %s", capture.ErrBodyCapturePreflightPending, result.Detail)
		}
		bodyGate = capture.BodyGateVerified
		bodyGateReason = result.Reason
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
	createdAuthority := m.authority == nil
	success := false
	defer func() {
		if !success && createdAuthority && m.authority != nil {
			m.authority.Close()
			m.authority = nil
		}
	}()
	if m.authority == nil {
		m.authority, err = proxy.NewAuthority(time.Now().UTC())
		if err != nil {
			return Session{}, err
		}
	}
	st, err := store.New(store.Config{
		Root: root, SessionID: string(id), MaxBytes: cfg.MaxSessionBytes, MaxRecords: cfg.RetentionRows,
		ReserveBytes: cfg.ReserveBytes, BodyFraction: cfg.BodyFraction,
		OverflowPolicy: cfg.OverflowPolicy,
	})
	if err != nil {
		return Session{}, err
	}
	started := time.Now().UTC()
	// The body policy is fixed here and never re-read while the session runs,
	// so a mid-session policy change cannot retroactively describe what this
	// session did. That the machine policy could change under a running
	// session is a documented residual, not something this value hides.
	maxBodyBytes := normalizeMaxBodyBytes(cfg.MaxBodyBytes)
	session := Session{
		Backend:      cfg.Backend,
		ProcessScope: cfg.ProcessScope, ScopeSessionID: cfg.ScopeSessionID,
		AllowedProcesses: cfg.AllowedProcesses,
		ID:               id, State: StateStarting, StorePath: st.Path(), StartedAt: started,
		RetainUnattributedMetadata: cfg.RetainUnattributedMetadata,
		CaptureBodies:              cfg.CaptureBodies,
		BodyCaptureGate:            bodyGate,
		BodyCaptureGateReason:      bodyGateReason,
		MaxBodyBytes:               maxBodyBytes,
	}
	if err := st.SetCaptureBackend(cfg.Backend); err != nil {
		_ = st.Finalize(StateFailed)
		return Session{}, err
	}
	if err := st.SetState(StateStarting); err != nil {
		_ = st.Finalize(StateFailed)
		return Session{}, err
	}
	if err := st.SetBodyCapturePolicy(store.BodyCapturePolicy{
		Enabled: cfg.CaptureBodies, Gate: bodyGate, GateReason: bodyGateReason,
		MaxBodyBytes: maxBodyBytes,
	}); err != nil {
		_ = st.Finalize(StateFailed)
		return Session{}, err
	}
	if cfg.RetentionRows > 0 && (cfg.LiveWindow <= 0 || cfg.LiveWindow > cfg.RetentionRows) {
		cfg.LiveWindow = cfg.RetentionRows
	}
	if err = m.admissions.Begin(string(id), cfg.AdmissionOwner, 0); err != nil {
		_ = st.Finalize(StateFailed)
		return Session{}, err
	}
	defer func() {
		if !success {
			m.admissions.Forget(string(id))
		}
	}()
	pipeline, err := stream.New(stream.Config{Admit: func(ctx context.Context, tx models.CaptureTransaction) (bool, error) {
		return m.admissions.Decide(ctx, string(id), tx)
	},
		RequireGeneration: true,
		SessionID:         id, Store: st, EventSink: m.sink, AllowedProcesses: cfg.AllowedProcesses, ProcessScope: cfg.ProcessScope, ScopeSessionID: cfg.ScopeSessionID,
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
	proxyConfig := proxy.Config{
		Stamp:         pipeline.Stamp,
		ListenAddress: cfg.ListenAddress, Authority: m.authority,
		Resolver: procmap.Resolver{}, AllowPassthrough: allowUntil,
		CaptureBodies: cfg.CaptureBodies, MaxBodyBytes: maxBodyBytes,
		Progress: func(tx models.CaptureTransaction) {
			if !retainLiveMetadata(tx, cfg.RetainUnattributedMetadata) {
				return
			}
			_ = pipeline.TrackProgress(tx)
		},
	}
	var capturer Capturer
	if m.newCapturer != nil {
		capturer, err = m.newCapturer(proxyConfig)
	} else {
		capturer, err = m.backends.Create(m.platform, cfg.Backend, proxyConfig)
	}
	if err != nil {
		if capturer != nil {
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = capturer.Stop(stopCtx)
			stopCancel()
		}
		_ = pipeline.Close()
		_ = st.Finalize(StateFailed)
		return Session{}, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	fatal := make(chan error, 1)
	address, err := capturer.Start(runCtx, cfg, func(tx models.CaptureTransaction) error {
		err := pipeline.Submit(runCtx, tx)
		if err != nil && runCtx.Err() == nil {
			select {
			case fatal <- err:
			default:
			}
		}
		return err
	})
	if err == nil {
		err = runCtx.Err()
	}
	if err != nil {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		stopErr := capturer.Stop(stopCtx)
		stopCancel()
		err = errors.Join(err, stopErr)
		_ = pipeline.Close()
		_ = st.Finalize(StateFailed)
		return Session{}, err
	}
	if source, ok := capturer.(interface{ Errors() <-chan error }); ok {
		go func() {
			select {
			case err := <-source.Errors():
				if err != nil {
					select {
					case fatal <- err:
					default:
					}
				}
			case <-runCtx.Done():
			}
		}()
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
	success = true
	m.sink.Started(session)
	go func() {
		select {
		case fatalErr := <-fatal:
			m.sink.Error(id, fatalErr.Error())
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer stopCancel()
			_, _ = m.Stop(stopCtx, id)
		case <-runCtx.Done():
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer stopCancel()
			_, _ = m.Stop(stopCtx, id)
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
	m.admissions.End(string(m.session.ID))
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
		if err := m.store.SetRedactionSummary(m.pipeline.RedactionSummary()); err != nil {
			stopErr = errors.Join(stopErr, err)
		}
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
	m.admissions.Forget(string(m.session.ID))
	return m.session, stopErr
}

func (m *Manager) SelectLaunchedProcess(id SessionID, key models.ProcessKey) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id != m.session.ID || m.session.State != StateRunning || m.session.ProcessScope != "launch" {
		return Session{}, ErrSessionNotFound
	}
	if key.PID <= 0 || key.StartTime == "" {
		return Session{}, fmt.Errorf("launched process identity is unavailable")
	}
	if selector, ok := m.capturer.(interface{ SetSelectedProcesses([]models.ProcessKey) }); ok {
		selector.SetSelectedProcesses([]models.ProcessKey{key})
	}
	m.pipeline.SetSelectedProcesses([]models.ProcessKey{key})
	m.session.AllowedProcesses = []models.ProcessKey{key}
	return m.session, nil
}

func (m *Manager) SetPaused(id SessionID, paused bool) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id != m.session.ID || !activeState(m.session.State) {
		return Session{}, ErrSessionNotFound
	}
	if paused {
		m.admissions.CancelPending(string(id))
	}
	m.pipeline.SetPaused(paused)
	m.session.Paused = paused
	return m.session, nil
}

func (m *Manager) Clear(id SessionID) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id == "" || id != m.session.ID {
		return Session{}, ErrSessionNotFound
	}
	if activeState(m.session.State) {
		m.admissions.CancelPending(string(id))
		generation, err := m.pipeline.Clear()
		if err != nil {
			m.session.Paused = true
			return m.session, err
		}
		m.session.Generation = generation
		m.admissions.Advance(string(id), generation)
	} else {
		st, err := m.openSession(id)
		if err != nil {
			return m.session, err
		}
		defer st.Close()
		if err := st.Clear(); err != nil {
			return m.session, err
		}
		m.session.Generation++
		meta := st.Meta()
		if err := st.SetCaptureStats(Stats{SessionID: id, State: m.session.State, Generation: m.session.Generation, SnapshotVersion: meta.SnapshotVersion}); err != nil {
			return m.session, err
		}
		m.pipeline = nil
	}
	return m.session, nil
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
		if m.pipeline != nil {
			return m.pipeline.Stats(m.session.State), nil
		}
		id = m.session.ID
	}
	st, err := m.openSession(id)
	if err != nil {
		return Stats{}, err
	}
	defer st.Close()
	meta := st.Meta()
	return Stats{RetentionRows: meta.RetentionRows, Retained: meta.RetainedRecords, Evicted: meta.Counters.Rotated,
		Generation: m.session.Generation, SessionID: id, State: meta.State, Persisted: meta.Counters.Persisted,
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
	// store.Open may recover/truncate an unfinalized store. Serialize the entire
	// scan with Start/Stop and other scans, not just the active-path snapshot.
	m.mu.Lock()
	defer m.mu.Unlock()
	activePath := ""
	if activeState(m.session.State) {
		activePath = m.session.StorePath
	}

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

func (m *Manager) Manifest(id SessionID) (store.Manifest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !validSessionID(id) {
		return store.Manifest{}, ErrSessionNotFound
	}
	if id == m.session.ID && m.store != nil {
		return m.store.Meta(), nil
	}
	path := filepath.Join(m.root, string(id))
	st, err := store.OpenReadOnly(path)
	if os.IsNotExist(err) {
		return store.Manifest{}, ErrSessionNotFound
	}
	if err != nil {
		return store.Manifest{}, err
	}
	defer st.Close()
	return st.Meta(), nil
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

func normalizeMaxBodyBytes(value int) int {
	if value <= 0 {
		return proxy.DefaultMaxBodyBytes
	}
	if value > proxy.MaxMaxBodyBytes {
		return proxy.MaxMaxBodyBytes
	}
	return value
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

func (m *Manager) Admissions() *admission.Hub { return m.admissions }
