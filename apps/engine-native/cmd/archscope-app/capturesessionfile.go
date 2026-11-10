package main

import (
	"context"
	"fmt"

	engineapi "github.com/aurakimjh/archscope/apps/engine-native/api"
	httpanalyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/httpcapture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/halog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type CaptureFileSource struct {
	SessionID string `json:"sessionId,omitempty"`
	Path      string `json:"path,omitempty"`
}
type CaptureExportRequest struct {
	Source      CaptureFileSource                `json:"source"`
	Destination string                           `json:"destination"`
	Format      string                           `json:"format"`
	Scope       string                           `json:"scope"`
	IDs         []string                         `json:"ids"`
	SelectedID  string                           `json:"selectedId,omitempty"`
	Annotations map[string]models.HTTPAnnotation `json:"annotations,omitempty"`
}
type CaptureOpenResult struct {
	Result     engineapi.AnalysisResult `json:"result"`
	SelectedID string                   `json:"selectedId"`
}

func (s *CaptureService) OpenCaptureFile(path string) (CaptureOpenResult, error) {
	if halog.IsPath(path) {
		result, err := httpanalyzer.Analyze(path, httpanalyzer.Options{TransactionRows: 100000})
		return CaptureOpenResult{Result: result}, err
	}
	doc, summary, err := sessionfile.Read(path)
	if err != nil {
		return CaptureOpenResult{}, err
	}
	result := httpanalyzer.BuildLive(doc.Transactions, path, "portable-session-v1", summary, httpanalyzer.Options{TransactionRows: 100000})
	return CaptureOpenResult{Result: result, SelectedID: doc.SelectedID}, nil
}
func (s *CaptureService) ExportCaptureFile(request CaptureExportRequest) (int, error) {
	if request.Destination == "" {
		return 0, context.Canceled
	}
	if (request.Source.SessionID == "") == (request.Source.Path == "") {
		return 0, fmt.Errorf("choose exactly one source")
	}
	if request.Scope != "all" && request.Scope != "filtered" {
		return 0, fmt.Errorf("choose all or filtered export scope")
	}
	doc := sessionfile.Document{Type: sessionfile.Kind, Version: 1, Scope: request.Scope, SelectedID: request.SelectedID}
	if request.Source.Path != "" {
		loaded, _, err := sessionfile.Read(request.Source.Path)
		if err != nil {
			return 0, err
		}
		doc.Transactions = loaded.Transactions
	} else {
		cursor := ""
		for {
			page, err := s.manager.Fetch(capture.SessionID(request.Source.SessionID), store.Filter{}, cursor, store.MaxFetchLimit)
			if err != nil {
				return 0, err
			}
			for _, item := range page.Items {
				doc.Transactions = append(doc.Transactions, item.Transaction)
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
	}
	if request.Source.SessionID != "" {
		values, err := s.GetCaptureAnnotations(request.Source.SessionID)
		if err != nil {
			return 0, err
		}
		for i := range doc.Transactions {
			v := values[doc.Transactions[i].ID]
			doc.Transactions[i].Starred = v.Starred
			doc.Transactions[i].Comment = v.Comment
		}
	}
	for i := range doc.Transactions {
		if value, ok := request.Annotations[doc.Transactions[i].ID]; ok {
			value, err := cleanAnnotation(value)
			if err != nil {
				return 0, err
			}
			doc.Transactions[i].Starred = value.Starred
			doc.Transactions[i].Comment = value.Comment
		}
	}
	if request.Scope == "filtered" {
		byID := map[string]models.CaptureTransaction{}
		for _, tx := range doc.Transactions {
			byID[tx.ID] = tx
		}
		doc.Transactions = nil
		seen := map[string]bool{}
		for _, id := range request.IDs {
			tx, ok := byID[id]
			if !ok {
				return 0, fmt.Errorf("filtered exchange no longer available; refresh the grid")
			}
			if !seen[id] {
				doc.Transactions = append(doc.Transactions, tx)
				seen[id] = true
			}
		}
	}
	if err := sessionfile.Write(context.Background(), request.Destination, doc, request.Format); err != nil {
		return 0, err
	}
	return len(doc.Transactions), nil
}

func (s *CaptureService) ProbeLegacyCapture(path string) (halog.Fingerprint, error) {
	return halog.Probe(path)
}
