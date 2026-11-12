// Package localconvert owns explicitly requested installed-tool execution.
// Parsers never choose arbitrary commands, shell expressions or downloads.
package localconvert

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/netpolicy"
)

const MaxOutput = int64(64 << 20)
const MaxDuration = 2 * time.Minute

var slots = make(chan struct{}, 2)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }
func fail(code string) error {
	return &Error{code, "Use a compatible installed JDK tool or preconvert in the collection environment; no converted output was published."}
}

type Provenance struct {
	Tool             string   `json:"tool"`
	Version          string   `json:"version"`
	ExecutableSHA256 string   `json:"executable_sha256"`
	Arguments        []string `json:"arguments"`
	InputSHA256      string   `json:"input_sha256"`
	OutputSHA256     string   `json:"output_sha256"`
	OutputBytes      int64    `json:"output_bytes"`
	Location         string   `json:"location"`
}
type Result struct {
	Path       string     `json:"-"`
	Provenance Provenance `json:"provenance"`
}

func hashFile(ctx context.Context, path string, limit int64) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return "", fail("CONVERTER_FILE_INVALID")
	}
	h := sha256.New()
	b := make([]byte, 32<<10)
	var total int64
	for {
		if e = ctx.Err(); e != nil {
			return "", e
		}
		n, err := f.Read(b)
		total += int64(n)
		if total > limit {
			return "", fail("CONVERTER_BUDGET_EXCEEDED")
		}
		h.Write(b[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type boundedWriter struct {
	out       io.Writer
	remaining int64
	overflow  bool
	cancel    context.CancelFunc
}

func (w *boundedWriter) Write(b []byte) (int, error) {
	if int64(len(b)) > w.remaining {
		w.overflow = true
		w.cancel()
		return 0, fail("CONVERTER_OUTPUT_LIMIT")
	}
	n, e := w.out.Write(b)
	w.remaining -= int64(n)
	return n, e
}
func toolEnv() []string {
	out := []string{}
	for _, name := range []string{"SystemRoot", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL"} {
		if v := os.Getenv(name); v != "" {
			out = append(out, name+"="+v)
		}
	}
	return out
}
func run(ctx context.Context, tool string, args []string, out io.Writer, limit int64, dir string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stdout := &boundedWriter{out: out, remaining: limit, cancel: cancel}
	stderr := &boundedWriter{out: io.Discard, remaining: 16 << 10, cancel: cancel}
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Env = toolEnv()
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = 500 * time.Millisecond
	started, cleanup, e := prepareProcess(ctx, cmd)
	if e != nil {
		return fail("CONVERTER_PROCESS_UNAVAILABLE")
	}
	defer cleanup()
	if e = cmd.Start(); e != nil {
		return fail("CONVERTER_START_FAILED")
	}
	if e = started(); e != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fail("CONVERTER_PROCESS_UNAVAILABLE")
	}
	e = cmd.Wait()
	if stdout.overflow || stderr.overflow {
		return fail("CONVERTER_OUTPUT_LIMIT")
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if e != nil {
		return fail("CONVERTER_EXIT_FAILED")
	}
	return nil
}

// JFR invokes only fixed --version and print --json argv. Source bytes are owned
// before execution; failures/cancellation remove all scratch and output files.
// A successful caller owns Result.Path and must remove it after saving/parsing.
func JFR(ctx context.Context, input, executable string) (result Result, err error) {
	if err = netpolicy.RequireOnline("converter.jfr"); err != nil {
		return result, err
	}
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	default:
		return result, fail("CONVERTER_BUSY")
	}
	ctx, cancel := context.WithTimeout(ctx, MaxDuration)
	defer cancel()
	if !filepath.IsAbs(executable) {
		return result, fail("CONVERTER_EXPLICIT_TOOL_REQUIRED")
	}
	tool, e := filepath.EvalSymlinks(executable)
	if e != nil {
		return result, fail("CONVERTER_TOOL_UNAVAILABLE")
	}
	before, e := hashFile(ctx, tool, 128<<20)
	if e != nil {
		return result, fail("CONVERTER_TOOL_UNAVAILABLE")
	}
	dir, e := os.MkdirTemp("", "archscope-convert-")
	if e != nil {
		return result, e
	}
	defer os.RemoveAll(dir)
	lease, e := ingestion.Acquire(ctx, input, dir, "conversion-source", ingestion.DefaultBudgets())
	if e != nil {
		return result, e
	}
	defer lease.Close()
	source, e := lease.Open(ctx)
	if e != nil {
		return result, e
	}
	magic := make([]byte, 4)
	_, e = io.ReadFull(source, magic)
	source.Close()
	if e != nil || string(magic) != "FLR\x00" {
		return result, fail("CONVERTER_INPUT_INVALID")
	}
	var version strings.Builder
	versionCtx, versionCancel := context.WithTimeout(ctx, 5*time.Second)
	e = run(versionCtx, tool, []string{"--version"}, &version, 4096, dir)
	versionCancel()
	if e != nil {
		return result, e
	}
	v := strings.TrimSpace(version.String())
	if !regexp.MustCompile(`^[0-9][0-9A-Za-z.+_-]{0,127}$`).MatchString(v) {
		return result, fail("CONVERTER_VERSION_INVALID")
	}
	tmp, e := os.CreateTemp("", "archscope_jfr_*.json")
	if e != nil {
		return result, e
	}
	path := tmp.Name()
	keep := false
	defer func() {
		tmp.Close()
		if !keep {
			os.Remove(path)
		}
	}()
	h := sha256.New()
	// JDK jfr enforces the .jfr suffix even when the magic bytes are valid.
	ownedInput := filepath.Join(dir, "input.jfr")
	if e = os.Link(filepath.Join(dir, "source.snapshot"), ownedInput); e != nil {
		return result, e
	}
	e = run(ctx, tool, []string{"print", "--json", ownedInput}, io.MultiWriter(tmp, h), MaxOutput, dir)
	if e != nil {
		return result, e
	}
	if e = tmp.Sync(); e != nil {
		return result, e
	}
	info, e := tmp.Stat()
	if e != nil {
		return result, e
	}
	if e = tmp.Close(); e != nil {
		return result, e
	}
	// Structural validation only; event interpretation belongs to the JFR parser.
	body, e := os.ReadFile(path)
	if e != nil {
		return result, e
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(body, &root) != nil {
		return result, fail("CONVERTER_OUTPUT_INVALID")
	}
	if recording, ok := root["recording"]; ok {
		if json.Unmarshal(recording, &root) != nil {
			return result, fail("CONVERTER_OUTPUT_INVALID")
		}
	}
	events := strings.TrimSpace(string(root["events"]))
	if len(events) < 2 || events[0] != '[' {
		return result, fail("CONVERTER_OUTPUT_INVALID")
	}
	after, e := hashFile(ctx, tool, 128<<20)
	if e != nil || before != after {
		return result, fail("CONVERTER_TOOL_CHANGED")
	}
	if e = lease.Verify(ctx); e != nil {
		return result, e
	}
	if e = ctx.Err(); e != nil {
		return result, e
	}
	result = Result{Path: path, Provenance: Provenance{Tool: "jdk.jfr", Version: v, ExecutableSHA256: before, Arguments: []string{"print", "--json", "<owned-input>"}, InputSHA256: lease.Info().SHA256, OutputSHA256: hex.EncodeToString(h.Sum(nil)), OutputBytes: info.Size(), Location: "local"}}
	keep = true
	return result, nil
}

// Code hides external stderr, environment and user paths from diagnostics.
func Code(e error) string {
	if e != nil && strings.HasPrefix(e.Error(), "OFFLINE_OPERATION_DENIED:") {
		return "OFFLINE_OPERATION_DENIED"
	}
	var importIssue *ingestion.ImportError
	if errors.As(e, &importIssue) {
		return importIssue.Code
	}
	var issue *Error
	if errors.As(e, &issue) {
		return issue.Code
	}
	if errors.Is(e, context.Canceled) {
		return "CONVERTER_CANCELLED"
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return "CONVERTER_TIMEOUT"
	}
	return "CONVERTER_FAILED"
}
