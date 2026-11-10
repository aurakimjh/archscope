package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	engineapi "github.com/aurakimjh/archscope/apps/engine-native/api"
	httpanalyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/httpcapture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/acceptance"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/aggregate"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/backend"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/dumppolicy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/processlaunch"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/procmap"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/requestbuilder"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/session"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type CaptureService struct {
	lifecycleMu    sync.Mutex
	closing        bool
	controlMu      sync.Mutex
	controlTargets []models.ProcessKey
	launchOptions  automation.Options
	launchState    DesktopLaunchState
	launchOnce     sync.Once
	launchCancel   context.CancelFunc
	journal        *automation.Journal
	annotationMu   sync.Mutex
	builder        *requestbuilder.Client
	manager        *session.Manager
	trust          *certstore.Lifecycle
}

type CaptureFetchRequest struct {
	SessionID string       `json:"sessionId"`
	Filter    store.Filter `json:"filter"`
	Cursor    string       `json:"cursor,omitempty"`
	Limit     int          `json:"limit,omitempty"`
}

type CaptureAnalyzeRequest struct {
	SessionID string `json:"sessionId"`
	TopN      int    `json:"topN,omitempty"`
	// TransactionRows bounds the full transaction list independently of TopN.
	// Zero selects the analyzer default, which is sized to hold a whole
	// session rather than a top-N ranking.
	TransactionRows int `json:"transactionRows,omitempty"`
}

type CaptureImportRequest struct {
	Path            string `json:"path"`
	TopN            int    `json:"topN,omitempty"`
	TransactionRows int    `json:"transactionRows,omitempty"`
}

type CaptureTransactionsEvent struct {
	SessionID       string                      `json:"sessionId"`
	Sequence        uint64                      `json:"sequence"`
	SnapshotVersion uint64                      `json:"snapshotVersion"`
	Items           []models.CaptureTransaction `json:"items"`
}

type CaptureProgressEvent struct {
	SessionID string                      `json:"sessionId"`
	Items     []models.CaptureTransaction `json:"items"`
}

type CaptureAggregateEvent struct {
	SessionID       string             `json:"sessionId"`
	Sequence        uint64             `json:"sequence"`
	SnapshotVersion uint64             `json:"snapshotVersion"`
	Snapshot        aggregate.Snapshot `json:"snapshot"`
}

type CaptureErrorEvent struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

type captureEventSink struct{ journal *automation.Journal }

func (s captureEventSink) record(kind string, id capture.SessionID, value any) {
	if s.journal != nil {
		s.journal.Append(kind, string(id), value)
	}
}
func controlRows(items []models.CaptureTransaction) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, tx := range items {
		row := map[string]any{"id": tx.ID, "sequence": tx.Sequence, "connectionId": tx.ConnectionID, "method": tx.Method, "url": tx.URL, "state": tx.State, "statusCode": tx.StatusCode}
		if tx.Process != nil {
			row["process"] = map[string]any{"key": tx.Process.Key, "name": tx.Process.Name, "attribution": tx.Process.Attribution}
		}
		out = append(out, row)
	}
	return out
}

func (sink captureEventSink) Started(value capture.Session) {
	sink.record("started", value.ID, value)
	emitEvent("capture:started", value)
}

func (sink captureEventSink) Progress(id capture.SessionID, items []models.CaptureTransaction) {
	sink.record("updated", id, controlRows(items))
	emitEvent("capture:progress", CaptureProgressEvent{
		SessionID: string(id), Items: items,
	})
}

func (sink captureEventSink) Transactions(id capture.SessionID, sequence, version uint64, items []models.CaptureTransaction) {
	sink.record("new", id, controlRows(items))
	emitEvent("capture:transactions", CaptureTransactionsEvent{
		SessionID: string(id), Sequence: sequence, SnapshotVersion: version, Items: items,
	})
}

func (sink captureEventSink) Aggregate(id capture.SessionID, sequence, version uint64, value any) {
	snapshot, ok := value.(aggregate.Snapshot)
	if !ok {
		return
	}
	emitEvent("capture:aggregate", CaptureAggregateEvent{
		SessionID: string(id), Sequence: sequence, SnapshotVersion: version, Snapshot: snapshot,
	})
}

func (sink captureEventSink) Stats(value capture.Stats) {
	emitEvent("capture:stats", value)
}

func (sink captureEventSink) Stopped(value capture.Session) {
	sink.record("stopped", value.ID, value)
	emitEvent("capture:stopped", value)
}

func (sink captureEventSink) Error(id capture.SessionID, message string) {
	sink.record("error", id, map[string]string{"message": message})
	emitEvent("capture:error", CaptureErrorEvent{SessionID: string(id), Message: message})
}

func NewCaptureService() *CaptureService {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	root := filepath.Join(configDir, "ArchScope", "captures")
	return newCaptureService(root, certstore.NewSystem(filepath.Join(root, "ca-trust.json")))
}

func newCaptureService(root string, trust *certstore.Lifecycle) *CaptureService {
	return newCaptureServiceForPlatform(root, trust, runtime.GOOS)
}

func newCaptureServiceForPlatform(root string, trust *certstore.Lifecycle, platform string) *CaptureService {
	journal := &automation.Journal{}
	return &CaptureService{
		journal: journal, builder: requestbuilder.NewClient(),
		manager: session.NewManagerForPlatform(root, captureEventSink{journal: journal}, platform),
		trust:   trust,
	}
}

func (s *CaptureService) ListCaptureBackends() []backend.Capability { return s.manager.Backends() }

func (s *CaptureService) ListCaptureModes() []capture.Mode {
	return s.manager.Modes()
}

// GetLiveCaptureContract runs the SEC-10 preflight rather than reporting a
// fixed capability. The renderer asks at startup and on page re-entry, so a
// machine whose dump policy changed between sessions is described by the
// policy in force now, not by the one that applied when the app launched.
func (s *CaptureService) GetLiveCaptureContract() capture.LiveCaptureContract {
	return capture.LiveCaptureContractFor(dumppolicy.Current())
}

func (s *CaptureService) ListCaptureProcesses() ([]models.ProcessInstance, error) {
	return procmap.List()
}

func (s *CaptureService) StartCapture(config capture.Config) (capture.Session, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.closing {
		return capture.Session{}, fmt.Errorf("desktop capture service is shutting down")
	}
	// The renderer cannot choose an arbitrary filesystem destination. Capture
	// data always stays under the application-owned root selected at startup.
	config.StoreRoot = ""
	selectedBackend, backendErr := s.manager.ValidateBackend(config.Backend)
	if backendErr != nil {
		return capture.Session{}, backendErr
	}
	config.Backend = selectedBackend
	if config.ProcessScope == "current_session" {
		id, err := procmap.CurrentSession()
		if err != nil {
			return capture.Session{}, err
		}
		config.ScopeSessionID = &id
	}
	var launchTarget processlaunch.Target
	if config.ProcessScope == "launch" {
		var err error
		launchTarget, err = processlaunch.Resolve(config.LaunchPath, config.LaunchArgs)
		if err != nil {
			return capture.Session{}, err
		}
	}
	started, err := s.manager.Start(context.Background(), config)
	if err != nil || config.ProcessScope != "launch" {
		return started, err
	}
	child, err := processlaunch.Prepare(launchTarget, started.ListenAddress)
	if err != nil {
		_, _ = s.StopCapture(string(started.ID))
		return capture.Session{}, err
	}
	selected, err := s.manager.SelectLaunchedProcess(started.ID, child.Key)
	if err == nil {
		err = child.Resume()
	}
	if err != nil {
		child.Abort()
		_, _ = s.StopCapture(string(started.ID))
		return capture.Session{}, err
	}
	return selected, nil
}

func (s *CaptureService) SetCapturePaused(sessionID string, paused bool) (capture.Session, error) {
	return s.manager.SetPaused(capture.SessionID(sessionID), paused)
}

func (s *CaptureService) ClearCapture(sessionID string) (capture.Session, error) {
	s.annotationMu.Lock()
	defer s.annotationMu.Unlock()
	result, err := s.manager.Clear(capture.SessionID(sessionID))
	if err != nil {
		return result, err
	}
	dir, err := s.manager.SessionPath(capture.SessionID(sessionID))
	if err == nil {
		err = store.WriteAnnotations(dir, map[string]models.HTTPAnnotation{})
	}
	return result, err
}

func (s *CaptureService) StopCapture(sessionID string) (capture.Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stopped, stopErr := s.manager.Stop(ctx, capture.SessionID(sessionID))
	_, trustErr := s.trust.Remove()
	s.manager.ReleaseAuthority()
	return stopped, errors.Join(stopErr, trustErr)
}

func (s *CaptureService) GetCaptureStats(sessionID string) (capture.Stats, error) {
	return s.manager.Stats(capture.SessionID(sessionID))
}

func (s *CaptureService) GetCurrentCaptureSession() capture.Session {
	return s.manager.Current()
}

func (s *CaptureService) GetCaptureLiveWindow(sessionID string) ([]models.CaptureTransaction, error) {
	return s.manager.LiveWindow(capture.SessionID(sessionID))
}

func (s *CaptureService) GetCaptureAcceptanceEvidence(sessionID string) (acceptance.Evidence, error) {
	path, err := s.manager.SessionPath(capture.SessionID(sessionID))
	if err != nil {
		return acceptance.Evidence{}, err
	}
	return acceptance.Build(path)
}

func (s *CaptureService) FetchCaptureTransactions(request CaptureFetchRequest) (store.Page, error) {
	return s.manager.Fetch(capture.SessionID(request.SessionID), request.Filter, request.Cursor, request.Limit)
}

// SearchCaptureTransactions searches the authoritative redacted store, including
// exchanges outside the live ring. Result rows exclude payloads until selected.
func (s *CaptureService) SearchCaptureTransactions(request CaptureFetchRequest) (store.Page, error) {
	if err := store.ValidateSearch(request.Filter.Query, request.Filter.SearchFields); err != nil {
		return store.Page{}, err
	}
	if request.Limit <= 0 || request.Limit > 100 {
		request.Limit = 25
	}
	page, err := s.manager.Fetch(capture.SessionID(request.SessionID), request.Filter, request.Cursor, request.Limit)
	if err != nil {
		return store.Page{}, err
	}
	for index := range page.Items {
		page.Items[index].Transaction.Request.LegacyPreview = nil
		page.Items[index].Transaction.Response.LegacyPreview = nil
		page.Items[index].Transaction.Request.ImagePreview = nil
		page.Items[index].Transaction.Response.ImagePreview = nil
		page.Items[index].Transaction.Request.BodyPreview = ""
		page.Items[index].Transaction.Response.BodyPreview = ""
	}
	return page, nil
}

// GetCaptureTransaction reads one redacted, persisted exchange while recording
// continues. Payloads remain outside the live ring and event stream.
func (s *CaptureService) GetCaptureTransaction(sessionID, transactionID string) (*models.CaptureTransaction, error) {
	if sessionID == "" || transactionID == "" {
		return nil, fmt.Errorf("session and transaction IDs are required")
	}
	page, err := s.manager.Fetch(capture.SessionID(sessionID), store.Filter{TransactionID: transactionID}, "", 1)
	if err != nil {
		return nil, err
	}
	if len(page.Items) == 0 {
		return nil, nil
	}
	return &page.Items[0].Transaction, nil
}

func (s *CaptureService) GetCaptureSnapshot(sessionID string) (aggregate.Snapshot, error) {
	return s.manager.Snapshot(capture.SessionID(sessionID))
}

func (s *CaptureService) AnalyzeCaptureSession(request CaptureAnalyzeRequest) (engineapi.AnalysisResult, error) {
	if request.SessionID == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("sessionId is required")
	}
	page, err := s.manager.Fetch(capture.SessionID(request.SessionID), store.Filter{}, "", store.MaxFetchLimit)
	if err != nil {
		return engineapi.AnalysisResult{}, err
	}
	transactions := make([]models.CaptureTransaction, 0, page.Total)
	for {
		for _, item := range page.Items {
			transactions = append(transactions, item.Transaction)
		}
		if page.NextCursor == "" {
			break
		}
		page, err = s.manager.Fetch(capture.SessionID(request.SessionID), store.Filter{}, page.NextCursor, store.MaxFetchLimit)
		if err != nil {
			return engineapi.AnalysisResult{}, err
		}
	}
	annotations, annotationErr := s.GetCaptureAnnotations(request.SessionID)
	if annotationErr != nil {
		return engineapi.AnalysisResult{}, annotationErr
	}
	for i := range transactions {
		value := annotations[transactions[i].ID]
		transactions[i].Starred = value.Starred
		transactions[i].Comment = value.Comment
	}
	source := "capture://" + request.SessionID
	manifest, err := s.manager.Manifest(capture.SessionID(request.SessionID))
	if err != nil {
		return engineapi.AnalysisResult{}, err
	}
	redactionSummary := redact.Summary{Known: false, Version: redact.PolicyVersion, Rules: []string{}, Counts: map[string]int{}}
	if manifest.Redaction != nil {
		redactionSummary = *manifest.Redaction
	}
	result := httpanalyzer.BuildLive(transactions, source, "canonical-v1", redactionSummary, httpanalyzer.Options{TopN: request.TopN, TransactionRows: request.TransactionRows})
	httpanalyzer.SetDiffCaptureSessionRef(&result, request.SessionID, page.SnapshotVersion, len(transactions))
	result.Metadata.Extra["capture_session_ref"] = map[string]any{
		"session_id": request.SessionID, "snapshot_version": page.SnapshotVersion,
		"transactions": len(transactions),
	}
	return result, nil
}

func (s *CaptureService) ImportHar(request CaptureImportRequest) (engineapi.AnalysisResult, error) {
	if request.Path == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeHttpCapture(request.Path, engineapi.HttpCaptureOptions{TopN: request.TopN, TransactionRows: request.TransactionRows})
}

func (s *CaptureService) InstallCaptureCA() (certstore.Status, error) {
	der, expiresAt, err := s.manager.PrepareAuthority()
	if err != nil {
		return certstore.Status{}, err
	}
	return s.trust.Install(der, expiresAt)
}

func (s *CaptureService) RemoveCaptureCA() (certstore.Status, error) {
	status, err := s.trust.Remove()
	s.manager.ReleaseAuthority()
	return status, err
}

func (s *CaptureService) GetCaptureCAStatus() certstore.Status {
	return s.trust.Status()
}

func (s *CaptureService) RecoverCaptureSessions() ([]store.RecoveryReport, error) {
	return s.manager.RecoverSessions()
}

// SendBuilderRequest performs a user-triggered request outside the capture store.
func (s *CaptureService) SendBuilderRequest(request requestbuilder.Request) (models.CaptureTransaction, error) {
	return s.builder.Send(request)
}
func (s *CaptureService) CancelBuilderRequest(id string) bool { return s.builder.Cancel(id) }
