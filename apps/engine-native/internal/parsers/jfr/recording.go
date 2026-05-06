// Recording bridge: ports archscope_engine.parsers.jfr_recording.
//
// The Python module shells out to the JDK's `jfr` CLI via
// `subprocess.run([resolved, "print", "--json", path], capture_output=True)`.
// The Go port uses os/exec.Command with the same argv. The Python
// version does not pass an explicit timeout, so the Go port matches
// that — the operation is single-shot, intended to produce a JSON
// blob the in-process parser will then read. When a timeout is
// desirable (CI / API tier) callers may wrap with a context-bearing
// variant later; for now we keep the Python behaviour verbatim so the
// migration is observable.
package jfr

import (
	"errors"
	"fmt"
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
	SourceFormat string  `json:"source_format"`
	JFRCli       *string `json:"jfr_cli,omitempty"`
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

// ConvertJFRToJSON shells out to `<cli> print --json <jfrPath>` and
// writes the stdout to a temp file. Returns the path to that temp
// file; callers own its lifecycle (Python does the same — the FastAPI
// layer keeps the file around for inspection on failure).
//
// `cli` may be empty, in which case DiscoverCLI is invoked. A missing
// CLI returns *CLIMissingError; a non-zero exit returns a plain error
// carrying the stderr tail (matching Python's behaviour: the Go
// runtime quirk where `exec.Cmd.Run` returns *exec.ExitError is
// flattened into the same message shape Python emits).
func ConvertJFRToJSON(jfrPath, cli string) (string, error) {
	resolved := cli
	if resolved == "" {
		resolved = DiscoverCLI()
	}
	if resolved == "" {
		return "", &CLIMissingError{Message: "No `jfr` CLI is available on PATH or under JAVA_HOME. " +
			"Install a JDK 11+ (or set ARCHSCOPE_JFR_CLI) so binary .jfr recordings can " +
			"be converted to JSON."}
	}

	tmp, err := os.CreateTemp("", "archscope_jfr_*.json")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	cmd := exec.Command(resolved, "print", "--json", jfrPath)
	stdout, err := cmd.Output()
	if err != nil {
		os.Remove(tmpPath)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr == "" {
				stderr = "no stderr output"
			}
			return "", fmt.Errorf("jfr print --json failed (exit %d): %s", exitErr.ExitCode(), stderr)
		}
		// Couldn't even start the process — bubble up as CLI missing
		// so the API layer can render a helpful hint.
		return "", &CLIMissingError{Message: fmt.Sprintf("Failed to invoke jfr CLI: %s", err.Error())}
	}

	if err := os.WriteFile(tmpPath, stdout, 0o600); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

// ParseRecording mirrors `parse_jfr_recording`: dispatches on the
// magic-byte sniff, optionally invokes `ConvertJFRToJSON`, then runs
// `ParseJSONFile`. Returns the events slice and a SourceInfo
// describing whether the input was binary or JSON.
//
// When the input is binary and no CLI is available, returns
// *CLIMissingError unchanged.
func ParseRecording(path string, diags *diagnostics.ParserDiagnostics) ([]Event, SourceInfo, error) {
	info := SourceInfo{SourceFormat: "json"}

	if IsBinaryJFR(path) {
		info.SourceFormat = "binary_jfr"
		cli := DiscoverCLI()
		if cli != "" {
			c := cli
			info.JFRCli = &c
		}
		if cli == "" {
			return nil, info, &CLIMissingError{Message: "Binary .jfr files require a JDK `jfr` CLI to convert to JSON. " +
				"Install JDK 11+, set ARCHSCOPE_JFR_CLI, or pre-convert with " +
				"`jfr print --json recording.jfr > recording.json`."}
		}
		jsonPath, err := ConvertJFRToJSON(path, cli)
		if err != nil {
			return nil, info, err
		}
		defer os.Remove(jsonPath)
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
