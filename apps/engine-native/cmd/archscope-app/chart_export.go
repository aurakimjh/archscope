package main

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ChartArtifactRequest carries an already-rendered chart, not an AnalysisResult.
type ChartArtifactRequest struct {
	Path   string `json:"path"`
	Format string `json:"format"`
	Data   string `json:"data"`
}

// SaveChartArtifact persists output selected through the native Save dialog.
// WebView anchor downloads do not provide a portable desktop save contract.
func (s *EngineService) SaveChartArtifact(req ChartArtifactRequest) error {
	if strings.TrimSpace(req.Path) == "" {
		return fmt.Errorf("path is required")
	}
	if len(req.Data) > 32*1024*1024 {
		return fmt.Errorf("chart output exceeds 32 MiB")
	}
	if strings.ToLower(filepath.Ext(req.Path)) != "."+req.Format {
		return fmt.Errorf("chart extension must match %s", req.Format)
	}
	var data []byte
	switch req.Format {
	case "png":
		const prefix = "data:image/png;base64,"
		if !strings.HasPrefix(req.Data, prefix) {
			return fmt.Errorf("expected PNG data URL")
		}
		var err error
		data, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(req.Data, prefix))
		if err != nil {
			return fmt.Errorf("invalid PNG: %w", err)
		}
		if _, err = png.DecodeConfig(bytes.NewReader(data)); err != nil {
			return fmt.Errorf("invalid PNG: %w", err)
		}
	case "svg":
		header, body, ok := strings.Cut(req.Data, ",")
		if !ok || (header != "data:image/svg+xml" && header != "data:image/svg+xml;charset=UTF-8" && header != "data:image/svg+xml;charset=utf-8") {
			return fmt.Errorf("expected SVG data URL")
		}
		decoded, err := url.PathUnescape(body)
		if err != nil {
			return fmt.Errorf("invalid SVG encoding: %w", err)
		}
		data = []byte(decoded)
		var root struct{ XMLName xml.Name }
		if err = xml.Unmarshal(data, &root); err != nil || root.XMLName.Local != "svg" {
			return fmt.Errorf("invalid SVG document")
		}
	case "csv":
		data = []byte(req.Data)
	default:
		return fmt.Errorf("unsupported chart format: %s", req.Format)
	}
	return os.WriteFile(req.Path, data, 0600)
}
