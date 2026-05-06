// Package json ports archscope_engine.exporters.json_exporter — a
// tiny round-trip-stable AnalysisResult writer. The exported payload
// is `json.MarshalIndent` output with a trailing newline so diffs and
// editors play nicely with it; UTF-8 native (no escaping non-ASCII).
//
// Foundation for the other Tier-4 exporters (HTML / PPTX / CSV /
// report-diff) that all need to load & re-emit AnalysisResult JSON.
package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Write marshals `result` (typically `models.AnalysisResult` or any
// JSON-serializable payload), creating any missing parent directories,
// and writes it with 2-space indent + trailing newline. Mirrors Python
// `write_json_result(payload, path)` byte-for-byte.
//
// `ensure_ascii=False` parity is automatic: Go's `encoding/json`
// emits non-ASCII as raw UTF-8 by default, matching Python's
// `ensure_ascii=False`.
func Write(path string, result any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	body, err := Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// Marshal returns the byte slice that `Write` would persist — useful
// for callers that want to ship JSON over stdout / HTTP / a buffer
// without touching the filesystem.
//
// Uses an explicit Encoder with SetIndent + SetEscapeHTML(false) so
// `<`, `>`, `&` round-trip verbatim (Python's default).
func Marshal(result any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(result); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	// json.Encoder.Encode appends '\n' automatically — matches
	// Python's `json.dumps(...) + "\n"`.
	return buf.Bytes(), nil
}
