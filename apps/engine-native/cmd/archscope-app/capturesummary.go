package main

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/summary"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func (s *CaptureService) GetCaptureSummary(sessionID string, process *models.ProcessKey) (summary.Snapshot, error) {
	rows := []models.CaptureTransaction{}
	cursor := ""
	for {
		page, err := s.manager.Fetch(capture.SessionID(sessionID), store.Filter{MetadataOnly: true}, cursor, store.MaxFetchLimit)
		if err != nil {
			return summary.Snapshot{}, err
		}
		for _, item := range page.Items {
			tx := item.Transaction
			if process == nil || (tx.Process != nil && tx.Process.Key == *process) {
				rows = append(rows, tx)
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return summary.Build(rows), nil
}
