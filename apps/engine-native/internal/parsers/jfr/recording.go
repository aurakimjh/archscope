// Binary conversion uses an explicitly configured installed JDK and the bounded
// localconvert runner. DiscoverCLI is advisory; PATH discovery never authorizes execution.
package jfr

import (
	"context"
	"errors"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localconvert"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

// jfrMagic is the four-byte preamble of any binary JFR recording
// (`b"FLR\x00"` in the Python module).
var jfrMagic = []byte{'F', 'L', 'R', 0x00}

// envOverride matches Python's `ARCHSCOPE_JFR_CLI` env var.
const envOverride = "ARCHSCOPE_JFR_CLI"

// CLIMissingError is the structured error the API layer surfaces when
// no `jfr` binary can be found. Mirrors Python's `JfrCliMissingError`.
type CLIMissingError struct {
	Message string
}

func (e *CLIMissingError) Error() string { return e.Message }

// IsCLIMissing reports whether `err` (or anything it wraps) is a
// CLIMissingError. Callers in the API layer use this to translate the
// failure into a structured user-facing message.
func IsCLIMissing(err error) bool {
	var target *CLIMissingError
	return errors.As(err, &target)
}

// SourceInfo mirrors the `dict[str, object]` Python returns alongside
// the events. Keys match the Python wire shape so analyzers and
// metadata renderers can consume them unchanged.
type SourceInfo struct {
	SourceFormat string                   `json:"source_format"`
	JFRCli       *string                  `json:"jfr_cli,omitempty"`
	Conversion   *localconvert.Provenance `json:"conversion,omitempty"`
}

// IsBinaryJFR returns true when `path` starts with the JFR magic
// bytes. Mirrors `is_binary_jfr` — a missing/unreadable file returns
// false rather than propagating the error so callers can fall through
// to the JSON parse path (which will then surface a clear ENOENT).
func IsBinaryJFR(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	buf := make([]byte, len(jfrMagic))
	n, err := io.ReadFull(file, buf)
	if err != nil || n != len(jfrMagic) {
		return false
	}
	for i := 0; i < len(jfrMagic); i++ {
		if buf[i] != jfrMagic[i] {
			return false
		}
	}
	return true
}

// DiscoverCLI mirrors `discover_jfr_cli`: env override → PATH lookup
// → `$JAVA_HOME/bin/jfr`. Returns the empty string if nothing is
// found.
func DiscoverCLI() string {
	if explicit := strings.TrimSpace(os.Getenv(envOverride)); explicit != "" {
		if info, err := os.Stat(explicit); err == nil && !info.IsDir() {
			return explicit
		}
	}
	if found, err := exec.LookPath("jfr"); err == nil {
		return found
	}
	javaHome := strings.TrimSpace(os.Getenv("JAVA_HOME"))
	if javaHome != "" {
		name := "jfr"
		if runtime.GOOS == "windows" {
			name = "jfr.exe"
		}
		candidate := filepath.Join(javaHome, "bin", name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// ConvertJFRToJSON preserves the legacy temporary-file ownership contract.
func ConvertJFRToJSON(jfrPath, cli string) (string, error) {
	result, e := convertContext(context.Background(), jfrPath, cli)
	return result.Path, e
}
func convertContext(ctx context.Context, path, cli string) (localconvert.Result, error) {
	if cli == "" {
		cli = strings.TrimSpace(os.Getenv(envOverride))
	}
	if cli == "" {
		return localconvert.Result{}, &CLIMissingError{Message: "Binary JFR needs an explicit absolute installed JDK jfr path via ARCHSCOPE_JFR_CLI, or preconvert with jfr print --json in the collection environment."}
	}
	return localconvert.JFR(ctx, path, cli)
}

// ParseRecording mirrors `parse_jfr_recording`: dispatches on the
// magic-byte sniff, optionally invokes `ConvertJFRToJSON`, then runs
// `ParseJSONFile`. Returns the events slice and a SourceInfo
// describing whether the input was binary or JSON.
//
// When the input is binary and no CLI is available, returns
// *CLIMissingError unchanged.
func ParseRecording(path string, diags *diagnostics.ParserDiagnostics) ([]Event, SourceInfo, error) {
	return ParseRecordingContext(context.Background(), path, diags)
}
func ParseRecordingContext(ctx context.Context, path string, diags *diagnostics.ParserDiagnostics) ([]Event, SourceInfo, error) {
	info := SourceInfo{SourceFormat: "json"}

	if IsBinaryJFR(path) {
		info.SourceFormat = "binary_jfr"
		conversion, err := convertContext(ctx, path, "")
		if err != nil {
			return nil, info, err
		}
		jsonPath := conversion.Path
		defer os.Remove(jsonPath)
		info.Conversion = &conversion.Provenance
		tool := "jdk.jfr"
		info.JFRCli = &tool

		events, err := ParseJSONFile(jsonPath, diags)
		if err != nil {
			return nil, info, err
		}
		return events, info, nil
	}

	events, err := ParseJSONFile(path, diags)
	if err != nil {
		return nil, info, err
	}
	return events, info, nil
}
