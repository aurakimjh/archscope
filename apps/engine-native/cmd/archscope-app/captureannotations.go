package main

import (
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func cleanAnnotation(value models.HTTPAnnotation) (models.HTTPAnnotation, error) {
	if len(value.Comment) > 4096 {
		return value, fmt.Errorf("comment exceeds 4096 UTF-8 bytes")
	}
	value.Comment, _ = redact.NewPolicy(redact.Options{}).RedactBody("text/plain", value.Comment)
	if len(value.Comment) > 4096 {
		return value, fmt.Errorf("redacted comment exceeds 4096 UTF-8 bytes")
	}
	return value, nil
}
func (s *CaptureService) GetCaptureAnnotations(sessionID string) (map[string]models.HTTPAnnotation, error) {
	s.annotationMu.Lock()
	defer s.annotationMu.Unlock()
	return s.captureAnnotations(sessionID)
}
func (s *CaptureService) captureAnnotations(sessionID string) (map[string]models.HTTPAnnotation, error) {
	dir, err := s.manager.SessionPath(capture.SessionID(sessionID))
	if err != nil {
		return nil, err
	}
	values, err := store.ReadAnnotations(dir)
	if err != nil {
		return nil, err
	}
	for id, value := range values {
		value, err = cleanAnnotation(value)
		if err != nil {
			return nil, err
		}
		values[id] = value
	}
	return values, nil
}
func (s *CaptureService) SetCaptureAnnotation(sessionID, id string, value models.HTTPAnnotation) (models.HTTPAnnotation, error) {
	value, err := cleanAnnotation(value)
	if err != nil {
		return value, err
	}
	s.annotationMu.Lock()
	defer s.annotationMu.Unlock()
	page, err := s.manager.Fetch(capture.SessionID(sessionID), store.Filter{TransactionID: id, MetadataOnly: true}, "", 1)
	if err != nil {
		return value, err
	}
	if len(page.Items) != 1 {
		return value, fmt.Errorf("exchange is no longer available")
	}
	dir, err := s.manager.SessionPath(capture.SessionID(sessionID))
	if err != nil {
		return value, err
	}
	values, err := store.ReadAnnotations(dir)
	if err != nil {
		return value, err
	}
	values[id] = value
	return value, store.WriteAnnotations(dir, values)
}
