// Package acceptance builds bounded, read-only evidence from a finalized live
// capture session. It never starts capture and therefore preserves the SEC-16
// prohibition on CLI/headless capture start.
package acceptance

import (
	"fmt"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const (
	SchemaVersion          = 2
	HarnessSchemaVersion   = 3
	MaxEvidenceRows        = store.MaxFetchLimit
	MinLongSessionRequests = 1000
)

type Evidence struct {
	SchemaVersion int                         `json:"schemaVersion"`
	Task          string                      `json:"task"`
	GeneratedAt   time.Time                   `json:"generatedAt"`
	Session       SessionEvidence             `json:"session"`
	Stats         capture.Stats               `json:"stats"`
	Redaction     redact.Summary              `json:"redaction"`
	LiveContract  capture.LiveCaptureContract `json:"liveContract"`
	Counts        map[string]uint64           `json:"counts"`
	Rows          []Row                       `json:"rows"`
	RowsTruncated bool                        `json:"rowsTruncated"`
}

type SessionEvidence struct {
	SessionID       string               `json:"sessionId"`
	State           capture.SessionState `json:"state"`
	SnapshotVersion uint64               `json:"snapshotVersion"`
	StoreBytes      int64                `json:"storeBytes"`
	TotalRows       int64                `json:"totalRows"`
}

type Row struct {
	ID                  string         `json:"id"`
	Method              string         `json:"method"`
	URL                 string         `json:"url"`
	Scheme              string         `json:"scheme"`
	Host                string         `json:"host"`
	Path                string         `json:"path"`
	StatusCode          int            `json:"statusCode"`
	State               models.TxState `json:"state"`
	Error               string         `json:"error,omitempty"`
	CaptureMode         string         `json:"captureMode"`
	Fidelity            string         `json:"fidelity"`
	Coverage            string         `json:"coverage"`
	ProcessName         string         `json:"processName,omitempty"`
	ProcessAttribution  string         `json:"processAttribution,omitempty"`
	RequestBodyStorage  string         `json:"requestBodyStorage"`
	ResponseBodyStorage string         `json:"responseBodyStorage"`
}

func Build(sessionPath string) (Evidence, error) {
	if strings.TrimSpace(sessionPath) == "" {
		return Evidence{}, fmt.Errorf("session path is required")
	}
	st, err := store.OpenReadOnly(sessionPath)
	if err != nil {
		return Evidence{}, fmt.Errorf("open finalized capture session: %w", err)
	}
	defer st.Close()
	meta := st.Meta()
	if meta.State != capture.StateFinalized &&
		meta.State != capture.StateRecoverable &&
		meta.State != capture.StateFailed {
		return Evidence{}, fmt.Errorf("capture session must be stopped before evidence export")
	}
	stats := capture.Stats{SessionID: capture.SessionID(meta.SessionID), State: meta.State}
	if meta.CaptureStats != nil {
		stats = *meta.CaptureStats
		stats.State = meta.State
	}
	result := Evidence{
		SchemaVersion: SchemaVersion,
		Task:          "T-581",
		GeneratedAt:   time.Now().UTC(),
		Stats:         stats,
		LiveContract:  capture.DefaultLiveCaptureContract(),
		Counts:        map[string]uint64{},
		Rows:          make([]Row, 0, MaxEvidenceRows),
	}
	if meta.Redaction != nil {
		result.Redaction = *meta.Redaction
	} else {
		result.Redaction = redact.Summary{Version: redact.PolicyVersion, Rules: []string{}, Counts: map[string]int{}}
	}
	cursor := ""
	for {
		page, fetchErr := st.Fetch(store.Filter{}, cursor, store.MaxFetchLimit)
		if fetchErr != nil {
			return Evidence{}, fetchErr
		}
		result.Session = SessionEvidence{
			SessionID:       meta.SessionID,
			State:           meta.State,
			SnapshotVersion: page.SnapshotVersion,
			StoreBytes:      meta.StoreBytes,
			TotalRows:       page.Total,
		}
		for _, item := range page.Items {
			tx := item.Transaction
			count(result.Counts, "mode:"+valueOrUnknown(tx.CaptureMode))
			count(result.Counts, "fidelity:"+valueOrUnknown(tx.Fidelity))
			count(result.Counts, "coverage:"+valueOrUnknown(tx.Coverage))
			count(result.Counts, "state:"+valueOrUnknown(string(tx.State)))
			if len(result.Rows) < MaxEvidenceRows {
				result.Rows = append(result.Rows, evidenceRow(tx))
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	result.RowsTruncated = result.Session.TotalRows > int64(len(result.Rows))
	return result, nil
}

func evidenceRow(tx models.CaptureTransaction) Row {
	row := Row{
		ID: tx.ID, Method: tx.Method, URL: tx.URL, Scheme: tx.Scheme,
		Host: tx.Host, Path: tx.Path, StatusCode: tx.StatusCode, State: tx.State, Error: tx.Error,
		CaptureMode: tx.CaptureMode, Fidelity: tx.Fidelity, Coverage: tx.Coverage,
		RequestBodyStorage:  tx.Request.BodyStorage,
		ResponseBodyStorage: tx.Response.BodyStorage,
	}
	if tx.Process != nil {
		row.ProcessName = tx.Process.Name
		row.ProcessAttribution = tx.Process.Attribution
	}
	return row
}

func count(counts map[string]uint64, key string) {
	counts[key]++
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
