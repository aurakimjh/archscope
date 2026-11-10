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
	annotationMu sync.Mutex
	builder      *requestbuilder.Client
	manager      *session.Manager
	trust        *certstore.Lifecycle
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

type captureEventSink struct{}

func (captureEventSink) Started(value capture.Session) {
	emitEvent("capture:started", value)
}

func (captureEventSink) Progress(id capture.SessionID, items []models.CaptureTransaction) {
	emitEvent("capture:progress", CaptureProgressEvent{
		SessionID: string(id), Items: items,
	})
}

func (captureEventSink) Transactions(id capture.SessionID, sequence, version uint64, items []models.CaptureTransaction) {
	emitEvent("capture:transactions", CaptureTransactionsEvent{
		SessionID: string(id), Sequence: sequence, SnapshotVersion: version, Items: items,
	})
}

func (captureEventSink) Aggregate(id capture.SessionID, sequence, version uint64, value any) {
	snapshot, ok := value.(aggregate.Snapshot)
	if !ok {
		return
	}
	emitEvent("capture:aggregate", CaptureAggregateEvent{
		SessionID: string(id), Sequence: sequence, SnapshotVersion: version, Snapshot: snapshot,
	})
}

func (captureEventSink) Stats(value capture.Stats) {
	emitEvent("capture:stats", value)
}

func (captureEventSink) Stopped(value capture.Session) {
	emitEvent("capture:stopped", value)
}

func (captureEventSink) Error(id capture.SessionID, message string) {
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
	return &CaptureService{
		builder: requestbuilder.NewClient(),
		manager: session.NewManagerForPlatform(root, captureEventSink{}, platform),
		trust:   trust,
	}
}

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
	// The renderer cannot choose an arbitrary filesystem destination. Capture
	// data always stays under the application-owned root selected at startup.
	config.StoreRoot = ""
	if config.ProcessScope == "current_session" {
		id, err := procmap.CurrentSession()
		if err != nil {
			return capture.Session{}, err
		}
		config.ScopeSessionID = &id
	}
	if config.ProcessScope == "launch" {
		if !filepath.IsAbs(config.LaunchPath) {
			return capture.Session{}, fmt.Errorf("choose an absolute executable path")
		}
		info, err := os.Stat(config.LaunchPath)
		if err != nil || info.IsDir() {
			return capture.Session{}, fmt.Errorf("executable is unavailable")
		}
		if len(config.LaunchArgs) > 128 {
			return capture.Session{}, fmt.Errorf("too many launch arguments")
		}
	}
	started, err := s.manager.Start(context.Background(), config)
	if err != nil || config.ProcessScope != "launch" {
		return started, err
	}
	child, err := processlaunch.Prepare(config.LaunchPath, config.LaunchArgs, started.ListenAddress)
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
