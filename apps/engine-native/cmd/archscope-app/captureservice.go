package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	engineapi "github.com/aurakimjh/archscope/apps/engine-native/api"
	httpanalyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/httpcapture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/aggregate"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/session"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type CaptureService struct {
	manager *session.Manager
	trust   *certstore.Lifecycle
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
}

type CaptureImportRequest struct {
	Path string `json:"path"`
	TopN int    `json:"topN,omitempty"`
}

type CaptureTransactionsEvent struct {
	SessionID       string                      `json:"sessionId"`
	Sequence        uint64                      `json:"sequence"`
	SnapshotVersion uint64                      `json:"snapshotVersion"`
	Items           []models.CaptureTransaction `json:"items"`
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
	return &CaptureService{
		manager: session.NewManager(root, captureEventSink{}),
		trust:   trust,
	}
}

func (s *CaptureService) ListCaptureModes() []capture.Mode {
	return s.manager.Modes()
}

func (s *CaptureService) StartCapture(config capture.Config) (capture.Session, error) {
	// The renderer cannot choose an arbitrary filesystem destination. Capture
	// data always stays under the application-owned root selected at startup.
	config.StoreRoot = ""
	return s.manager.Start(context.Background(), config)
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

func (s *CaptureService) FetchCaptureTransactions(request CaptureFetchRequest) (store.Page, error) {
	return s.manager.Fetch(capture.SessionID(request.SessionID), request.Filter, request.Cursor, request.Limit)
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
	source := "capture://" + request.SessionID
	result := httpanalyzer.Build(transactions, source, "archscope-live", "canonical-v1", httpanalyzer.Options{TopN: request.TopN})
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
	return engineapi.AnalyzeHttpCapture(request.Path, engineapi.HttpCaptureOptions{TopN: request.TopN})
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
