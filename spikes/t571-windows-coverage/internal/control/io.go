// Package control holds cross-platform helpers shared by the spike tools:
// JSON IO, environment stamping, and CPU sampling. Windows-specific bits
// (elevation, build number, processor time) live in the build-tagged files.
package control

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WriteJSON writes v as indented JSON, creating parent dirs as needed.
func WriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// ReadJSON reads JSON from path into v.
func ReadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Hostname returns the machine name or "unknown".
func Hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}
