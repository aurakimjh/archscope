package main

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/content"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/legacy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func (s *CaptureService) InspectLegacyContent(format, encoded string) *models.LegacyPreview {
	result := legacy.DecodeBase64(format, encoded)
	return content.CleanLegacy(&result, redact.NewPolicy(redact.Options{}))
}
