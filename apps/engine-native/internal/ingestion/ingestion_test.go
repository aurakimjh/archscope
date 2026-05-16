package ingestion

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestCoreEvidenceFamilySpecsValidate(t *testing.T) {
	if err := ValidateEvidenceFamilySpecs(CoreEvidenceFamilies()); err != nil {
		t.Fatalf("ValidateEvidenceFamilySpecs: %v", err)
	}
}

func TestFormatRegistryDetectsByConfidence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.json")
	if err := os.WriteFile(path, []byte(`{"resourceSpans":[]}`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	registry, err := NewFormatRegistry(
		SourceFormat{
			ID:         "json-extension",
			SourceKind: SourceKindTrace,
			Extensions: []string{"json"},
		},
		SourceFormat{
			ID:         "otlp-json",
			SourceKind: SourceKindTrace,
			Product:    "OpenTelemetry",
			Detector:   ContainsSignature("resourceSpans", 0.95, "OTLP resourceSpans key"),
		},
	)
	if err != nil {
		t.Fatalf("NewFormatRegistry: %v", err)
	}
	detection, err := registry.DetectFile(path)
	if err != nil {
		t.Fatalf("DetectFile: %v", err)
	}
	if detection.FormatID != "otlp-json" {
		t.Fatalf("FormatID = %q", detection.FormatID)
	}
	if detection.SourceKind != SourceKindTrace || detection.Product != "OpenTelemetry" {
		t.Fatalf("detection metadata = %+v", detection)
	}
}

func TestFormatRegistryUnknownFormat(t *testing.T) {
	registry, err := NewFormatRegistry(SourceFormat{
		ID:         "known",
		SourceKind: SourceKindAccessLog,
		Detector:   ContainsSignature("known", 0.9, "known marker"),
	})
	if err != nil {
		t.Fatalf("NewFormatRegistry: %v", err)
	}
	_, err = registry.Detect(Probe{Path: "unknown.log", Head: []byte("something else")})
	if !errors.Is(err, ErrUnknownFormat) {
		t.Fatalf("Detect error = %v, want ErrUnknownFormat", err)
	}
}

func TestCheckFixtureCoversDiagnostics(t *testing.T) {
	diags := diagnostics.New("demo")
	diags.TotalLines = 3
	diags.ParsedRecords = 2
	diags.AddSkipped(3, "NO_FORMAT_MATCH", "bad line", "bad")
	errs := CheckFixture("partial-demo", FixtureObservation{
		Format:      "demo",
		Diagnostics: diags,
	}, nil, FixtureExpectation{
		Kind:                FixturePartial,
		WantFormat:          "demo",
		MinTotalLines:       3,
		MinParsedRecords:    2,
		MinSkippedLines:     1,
		MinErrorCount:       1,
		WantSkippedByReason: map[string]int{"NO_FORMAT_MATCH": 1},
		WantErrorByReason:   map[string]int{"NO_FORMAT_MATCH": 1},
	})
	if len(errs) > 0 {
		t.Fatalf("CheckFixture returned errors: %v", errs)
	}
}

func TestSourceMetadataUsesSanitizedFileIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "customer-prod-access.log")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	metadata := NewSourceMetadata(path, SourceMetadataOptions{
		SourceKind:   SourceKindAccessLog,
		SourceFormat: "nginx",
		Product:      "nginx",
		Service:      "checkout",
	})
	if metadata.File.BaseName != "customer-prod-access.log" {
		t.Fatalf("base name = %q", metadata.File.BaseName)
	}
	if metadata.File.SizeBytes != 5 {
		t.Fatalf("size = %d", metadata.File.SizeBytes)
	}
	if metadata.File.SanitizedID == "" || len(metadata.File.SanitizedID) != 24 {
		t.Fatalf("sanitized id = %q", metadata.File.SanitizedID)
	}
	result := models.New("access_log", "parser")
	AttachSourceMetadata(&result, metadata)
	if got := result.Metadata.Extra["source_metadata"]; got == nil {
		t.Fatalf("source_metadata not attached")
	}
}

func TestCorrelationKeysNormalizeAndAttach(t *testing.T) {
	ts := time.Date(2026, 5, 16, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	key := NormalizeCorrelationKeys(CorrelationKeys{
		TraceID: " ABCDEF ",
		SpanID:  " ROOT ",
		HostID:  " Web-01 ",
		PID:     4242,
		Window:  TimestampWindowFor(ts, 10*time.Second),
	})
	if key.TraceID != "abcdef" || key.SpanID != "root" || key.HostID != "web-01" {
		t.Fatalf("normalized key = %+v", key)
	}
	if key.StableID == "" {
		t.Fatalf("stable id should be populated")
	}
	result := models.New("trace_import", "trace_import")
	AttachCorrelationKeys(&result, []CorrelationKeys{key})
	if got := result.Metadata.Extra["correlation_keys"]; got == nil {
		t.Fatalf("correlation_keys not attached")
	}
}
