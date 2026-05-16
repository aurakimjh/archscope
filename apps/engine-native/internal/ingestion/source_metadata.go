package ingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type SourceMetadataOptions struct {
	SourceKind     string
	SourceFormat   string
	Product        string
	ProductVersion string
	Host           string
	Service        string
	Environment    string
}

type SourceMetadata struct {
	SourceKind     string       `json:"source_kind"`
	SourceFormat   string       `json:"source_format,omitempty"`
	Product        string       `json:"product,omitempty"`
	ProductVersion string       `json:"product_version,omitempty"`
	Host           string       `json:"host,omitempty"`
	Service        string       `json:"service,omitempty"`
	Environment    string       `json:"environment,omitempty"`
	File           FileIdentity `json:"file"`
}

type FileIdentity struct {
	BaseName           string `json:"base_name,omitempty"`
	Extension          string `json:"extension,omitempty"`
	SizeBytes          int64  `json:"size_bytes,omitempty"`
	SanitizedID        string `json:"sanitized_id,omitempty"`
	SanitizedIDVersion string `json:"sanitized_id_version,omitempty"`
}

func NewSourceMetadata(path string, opts SourceMetadataOptions) SourceMetadata {
	return SourceMetadata{
		SourceKind:     strings.TrimSpace(opts.SourceKind),
		SourceFormat:   strings.TrimSpace(opts.SourceFormat),
		Product:        strings.TrimSpace(opts.Product),
		ProductVersion: strings.TrimSpace(opts.ProductVersion),
		Host:           strings.TrimSpace(opts.Host),
		Service:        strings.TrimSpace(opts.Service),
		Environment:    strings.TrimSpace(opts.Environment),
		File:           NewFileIdentity(path, opts.SourceKind, opts.SourceFormat),
	}
}

func NewFileIdentity(path, sourceKind, sourceFormat string) FileIdentity {
	base := filepath.Base(path)
	if path == "" || base == "." || base == string(filepath.Separator) {
		base = ""
	}
	ext := strings.ToLower(filepath.Ext(base))
	var size int64
	if path != "" {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			size = info.Size()
		}
	}
	seed := strings.Join([]string{
		"archscope-source-identity-v1",
		sourceKind,
		sourceFormat,
		base,
		ext,
		strconv.FormatInt(size, 10),
	}, "\x00")
	sum := sha256.Sum256([]byte(seed))
	return FileIdentity{
		BaseName:           base,
		Extension:          ext,
		SizeBytes:          size,
		SanitizedID:        hex.EncodeToString(sum[:])[:24],
		SanitizedIDVersion: "basename-size-v1",
	}
}

func AttachSourceMetadata(result *models.AnalysisResult, metadata ...SourceMetadata) {
	if result == nil || len(metadata) == 0 {
		return
	}
	if result.Metadata.Extra == nil {
		result.Metadata.Extra = map[string]any{}
	}
	result.Metadata.Extra["source_metadata"] = metadata
}
