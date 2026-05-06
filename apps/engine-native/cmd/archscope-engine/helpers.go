// Shared helpers for the archscope-engine Cobra CLI. Each subcommand
// file (cmd_accesslog.go, cmd_gclog.go, …) keeps its own flag-binding
// and Run* function, but the JSON write path, time-flag parsing, and
// repeatable input-flag plumbing are common across them and live here.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// writeJSONResult marshals the AnalysisResult envelope to indented
// JSON and writes it to `out` (`-` for stdout). Mirrors the original
// flag-based emitResult.
func writeJSONResult(result models.AnalysisResult, out string) error {
	return writeJSONAny(result, out)
}

// writeJSONAny writes any JSON-marshalable payload (the `to-collapsed`
// command emits `map[string]int`; the `report json` command emits a
// generic `map[string]any`).
func writeJSONAny(payload any, out string) error {
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if out == "-" || out == "" {
		_, err := os.Stdout.Write(body)
		return err
	}
	return os.WriteFile(out, body, 0o644)
}

// parseTimeFlag converts a user-supplied --start-time / --end-time
// value to *time.Time. Returns (nil, nil) when `value` is empty so
// callers can wire the flag straight into Options without checking.
func parseTimeFlag(name, value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("--%s: %w", name, err)
	}
	return &t, nil
}

// readJSONFile loads `path` and decodes it as a generic map. The
// `report` exporters' toMap helpers accept arbitrary JSON-marshalable
// input, so feeding them this map gives the user the round-trip they
// expect (load → render).
func readJSONFile(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return payload, nil
}

// splitCommaSeparated normalises a slice of CLI values that may be a
// mix of repeated --in invocations and comma-separated strings. It
// preserves order and drops empty fragments.
func splitCommaSeparated(values []string) []string {
	out := make([]string, 0, len(values))
	for _, raw := range values {
		for _, item := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}
