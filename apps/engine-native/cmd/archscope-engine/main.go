// archscope-engine is the Go counterpart of `python -m
// archscope_engine.cli`. The full Cobra surface lands under T-360;
// this stub stays flag-based and grows one subcommand per analyzer to
// power the parity gate (T-390). Every subcommand emits the same
// `models.AnalysisResult` envelope as JSON via `json.MarshalIndent`,
// matching the exporter Python ships through `write_json_result`.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/accesslog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/exception"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gclog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/jfr"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/lockcontention"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/multithread"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/otel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/runtime"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/threaddump"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/threaddumpcollapsed"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	td "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump"
	// Side-effect imports register thread-dump plugins on the default
	// registry. javajstack + gogoroutine self-register; the rest expose
	// New() factories we wire into the registry below.
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/gogoroutine"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/javajstack"
)

const usage = `archscope-engine — Go engine CLI (T-390 parity stub)

Usage:
  archscope-engine accesslog       --in <path> [--format nginx] [--max-lines N]
                                    [--start-time RFC3339] [--end-time RFC3339] [--out <path>]
  archscope-engine gclog           --in <path> [--top-n N] [--max-lines N] [--strict]
                                    [--out <path>]
  archscope-engine jfr             --in <path> [--top-n N] [--mode all|cpu|wall|...]
                                    [--from <ts>] [--to <ts>] [--state <STATE>]
                                    [--min-duration-ms F] [--out <path>]
  archscope-engine exception       --in <path> [--top-n N] [--max-lines N] [--strict]
                                    [--out <path>]
  archscope-engine runtime         --variant nodejs|python|go|dotnet --in <path>
                                    [--top-n N] [--out <path>]
  archscope-engine otel            --in <path> [--top-n N] [--out <path>]
  archscope-engine threaddump      --in <path> [--top-n N] [--out <path>]
  archscope-engine multithread     --in <path>[,<path>...] [--in <path>...] [--top-n N]
                                    [--threshold N] [--format <id>] [--out <path>]
  archscope-engine lockcontention  --in <path>[,<path>...] [--in <path>...] [--top-n N]
                                    [--format <id>] [--out <path>]
  archscope-engine collapsed       --in <path>[,<path>...] [--in <path>...] [--no-thread-name]
                                    [--format <id>] [--out <path>]

The flag-based shape is intentionally narrow: T-360 will replace it
with a Cobra surface. Until then this CLI exists mainly so the parity
job at .github/workflows/profiler-native.yml can run "Go vs Python"
on every analyzer's reference fixture.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "accesslog":
		exitOnErr(runAccessLog(os.Args[2:]))
	case "gclog":
		exitOnErr(runGCLog(os.Args[2:]))
	case "jfr":
		exitOnErr(runJFR(os.Args[2:]))
	case "exception":
		exitOnErr(runException(os.Args[2:]))
	case "runtime":
		exitOnErr(runRuntime(os.Args[2:]))
	case "otel":
		exitOnErr(runOTel(os.Args[2:]))
	case "threaddump":
		exitOnErr(runThreadDump(os.Args[2:]))
	case "multithread":
		exitOnErr(runMultiThread(os.Args[2:]))
	case "lockcontention":
		exitOnErr(runLockContention(os.Args[2:]))
	case "collapsed":
		exitOnErr(runCollapsed(os.Args[2:]))
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

func exitOnErr(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "archscope-engine: %v\n", err)
	os.Exit(1)
}

// repeatableStringSlice collects --in <path> values and also accepts
// comma-separated lists per flag invocation. Mirrors Python's
// `typer.Option(..., multiple=True)` plumbing for the multi-file
// analyzers (multithread / lockcontention / collapsed).
type repeatableStringSlice struct{ values *[]string }

func (r repeatableStringSlice) String() string {
	if r.values == nil {
		return ""
	}
	return strings.Join(*r.values, ",")
}

func (r repeatableStringSlice) Set(v string) error {
	for _, item := range strings.Split(v, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			*r.values = append(*r.values, trimmed)
		}
	}
	return nil
}

// emitResult marshals the AnalysisResult to indented JSON and writes
// it to `out` (`-` for stdout). Same body shape as the original
// accesslog handler.
func emitResult(result models.AnalysisResult, out string) error {
	body, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if out == "-" {
		_, err := os.Stdout.Write(body)
		return err
	}
	return os.WriteFile(out, body, 0o644)
}

// emitJSON is the same as emitResult but for arbitrary payloads (the
// `collapsed` subcommand emits map[string]int).
func emitJSON(payload any, out string) error {
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if out == "-" {
		_, err := os.Stdout.Write(body)
		return err
	}
	return os.WriteFile(out, body, 0o644)
}

// ── accesslog (the original subcommand, unchanged in shape) ─────────

func runAccessLog(args []string) error {
	fs := flag.NewFlagSet("accesslog", flag.ExitOnError)
	in := fs.String("in", "", "path to access log (required)")
	format := fs.String("format", "nginx", "log format label")
	maxLines := fs.Int("max-lines", 0, "stop after N lines (0 = unlimited)")
	startStr := fs.String("start-time", "", "RFC3339 lower bound (inclusive)")
	endStr := fs.String("end-time", "", "RFC3339 upper bound (inclusive)")
	out := fs.String("out", "-", "output path; `-` for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" {
		return fmt.Errorf("--in is required")
	}

	opts := accesslog.Options{MaxLines: *maxLines}
	if *startStr != "" {
		t, err := time.Parse(time.RFC3339, *startStr)
		if err != nil {
			return fmt.Errorf("--start-time: %w", err)
		}
		opts.StartTime = &t
	}
	if *endStr != "" {
		t, err := time.Parse(time.RFC3339, *endStr)
		if err != nil {
			return fmt.Errorf("--end-time: %w", err)
		}
		opts.EndTime = &t
	}

	result, err := accesslog.Analyze(*in, *format, opts)
	if err != nil {
		return err
	}
	return emitResult(result, *out)
}

// ── gclog ───────────────────────────────────────────────────────────

func runGCLog(args []string) error {
	fs := flag.NewFlagSet("gclog", flag.ExitOnError)
	in := fs.String("in", "", "path to GC log (required)")
	topN := fs.Int("top-n", 0, "top-N rows in tables.events (0 = analyzer default)")
	maxLines := fs.Int("max-lines", 0, "stop after N lines (0 = unlimited)")
	strict := fs.Bool("strict", false, "surface parser skips as fatal errors")
	out := fs.String("out", "-", "output path; `-` for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" {
		return fmt.Errorf("--in is required")
	}
	result, err := gclog.Analyze(*in, gclog.Options{
		TopN:     *topN,
		MaxLines: *maxLines,
		Strict:   *strict,
	})
	if err != nil {
		return err
	}
	return emitResult(result, *out)
}

// ── jfr ─────────────────────────────────────────────────────────────

func runJFR(args []string) error {
	fs := flag.NewFlagSet("jfr", flag.ExitOnError)
	in := fs.String("in", "", "path to JFR recording or `jfr print --json` output (required)")
	topN := fs.Int("top-n", 0, "top-N notable events (0 = analyzer default)")
	mode := fs.String("mode", "all", "filter mode (all|cpu|wall|alloc|lock|gc|exception|io|nativemem)")
	from := fs.String("from", "", "lower-bound time filter (ISO 8601, HH:MM[:SS], or relative like +30s)")
	to := fs.String("to", "", "upper-bound time filter (same syntax as --from)")
	state := fs.String("state", "", "filter to events whose .state matches (case-insensitive)")
	minDur := fs.Float64("min-duration-ms", 0, "drop events whose duration_ms < this")
	out := fs.String("out", "-", "output path; `-` for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" {
		return fmt.Errorf("--in is required")
	}
	opts := jfr.Options{
		TopN:     *topN,
		Mode:     *mode,
		FromTime: *from,
		ToTime:   *to,
		State:    *state,
	}
	if *minDur > 0 {
		v := *minDur
		opts.MinDurationMS = &v
	}
	result, err := jfr.Analyze(*in, opts)
	if err != nil {
		return err
	}
	return emitResult(result, *out)
}

// ── exception ───────────────────────────────────────────────────────

func runException(args []string) error {
	fs := flag.NewFlagSet("exception", flag.ExitOnError)
	in := fs.String("in", "", "path to exception/stack file (required)")
	topN := fs.Int("top-n", 0, "top-N rows (0 = analyzer default)")
	maxLines := fs.Int("max-lines", 0, "stop after N lines (0 = unlimited)")
	strict := fs.Bool("strict", false, "surface parser skips as fatal errors")
	out := fs.String("out", "-", "output path; `-` for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" {
		return fmt.Errorf("--in is required")
	}
	result, err := exception.Analyze(*in, exception.Options{
		TopN:     *topN,
		MaxLines: *maxLines,
		Strict:   *strict,
	})
	if err != nil {
		return err
	}
	return emitResult(result, *out)
}

// ── runtime (variant: nodejs / python / go / dotnet) ────────────────

func runRuntime(args []string) error {
	fs := flag.NewFlagSet("runtime", flag.ExitOnError)
	variant := fs.String("variant", "", "one of: nodejs, python, go, dotnet (required)")
	in := fs.String("in", "", "path to runtime stack/log file (required)")
	topN := fs.Int("top-n", 0, "top-N rows (0 = analyzer default)")
	maxLines := fs.Int("max-lines", 0, "stop after N lines (0 = unlimited)")
	strict := fs.Bool("strict", false, "surface parser skips as fatal errors")
	out := fs.String("out", "-", "output path; `-` for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" {
		return fmt.Errorf("--in is required")
	}
	opts := runtime.Options{TopN: *topN, MaxLines: *maxLines, Strict: *strict}
	var (
		result models.AnalysisResult
		err    error
	)
	switch *variant {
	case "nodejs":
		result, err = runtime.AnalyzeNodejsStack(*in, opts)
	case "python":
		result, err = runtime.AnalyzePythonTraceback(*in, opts)
	case "go":
		result, err = runtime.AnalyzeGoPanic(*in, opts)
	case "dotnet":
		result, err = runtime.AnalyzeDotnetExceptionIIS(*in, opts)
	default:
		return fmt.Errorf("--variant must be one of: nodejs, python, go, dotnet (got %q)", *variant)
	}
	if err != nil {
		return err
	}
	return emitResult(result, *out)
}

// ── otel ────────────────────────────────────────────────────────────

func runOTel(args []string) error {
	fs := flag.NewFlagSet("otel", flag.ExitOnError)
	in := fs.String("in", "", "path to OTel JSONL logs (required)")
	topN := fs.Int("top-n", 0, "top-N rows (0 = analyzer default)")
	out := fs.String("out", "-", "output path; `-` for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" {
		return fmt.Errorf("--in is required")
	}
	result, err := otel.Analyze(*in, otel.Options{TopN: *topN})
	if err != nil {
		return err
	}
	return emitResult(result, *out)
}

// ── threaddump (single jstack) ──────────────────────────────────────

func runThreadDump(args []string) error {
	fs := flag.NewFlagSet("threaddump", flag.ExitOnError)
	in := fs.String("in", "", "path to a Java jstack thread dump (required)")
	topN := fs.Int("top-n", 0, "top-N stack signatures (0 = analyzer default)")
	out := fs.String("out", "-", "output path; `-` for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" {
		return fmt.Errorf("--in is required")
	}
	result, err := threaddump.Analyze(*in, threaddump.Options{TopN: *topN})
	if err != nil {
		return err
	}
	return emitResult(result, *out)
}

// ── multithread ─────────────────────────────────────────────────────

func runMultiThread(args []string) error {
	fs := flag.NewFlagSet("multithread", flag.ExitOnError)
	var paths []string
	fs.Var(repeatableStringSlice{values: &paths}, "in", "path to a thread-dump file; repeat or pass comma-separated")
	topN := fs.Int("top-n", 0, "top-N table rows (0 = analyzer default)")
	threshold := fs.Int("threshold", 0, "consecutive-dump threshold for persistence findings")
	format := fs.String("format", "", "force a thread-dump plugin format-id (skips header sniff)")
	out := fs.String("out", "-", "output path; `-` for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("--in is required (repeat or comma-separated)")
	}
	bundles, err := td.DefaultRegistry.ParseMany(paths, td.ParseOptions{FormatOverride: *format})
	if err != nil {
		return err
	}
	result, err := multithread.Analyze(bundles, multithread.Options{
		TopN:      *topN,
		Threshold: *threshold,
	})
	if err != nil {
		return err
	}
	return emitResult(result, *out)
}

// ── lockcontention ──────────────────────────────────────────────────

func runLockContention(args []string) error {
	fs := flag.NewFlagSet("lockcontention", flag.ExitOnError)
	var paths []string
	fs.Var(repeatableStringSlice{values: &paths}, "in", "path to a thread-dump file; repeat or pass comma-separated")
	topN := fs.Int("top-n", 0, "top-N rows (0 = analyzer default)")
	format := fs.String("format", "", "force a thread-dump plugin format-id (skips header sniff)")
	out := fs.String("out", "-", "output path; `-` for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("--in is required (repeat or comma-separated)")
	}
	bundles, err := td.DefaultRegistry.ParseMany(paths, td.ParseOptions{FormatOverride: *format})
	if err != nil {
		return err
	}
	result, err := lockcontention.Analyze(bundles, lockcontention.Options{TopN: *topN})
	if err != nil {
		return err
	}
	return emitResult(result, *out)
}

// ── collapsed ───────────────────────────────────────────────────────
//
// `collapsed` returns a `map[string]int` (stack -> count) — there's no
// AnalysisResult envelope here. The Python CLI has no JSON sibling for
// this either; it emits a flamegraph.pl-style text file via
// `write_collapsed_file`. The parity step uses our JSON form on the
// Go side and reads the text on the Python side, then compares the
// (stack, count) sets.

func runCollapsed(args []string) error {
	fs := flag.NewFlagSet("collapsed", flag.ExitOnError)
	var paths []string
	fs.Var(repeatableStringSlice{values: &paths}, "in", "path to a thread-dump file; repeat or pass comma-separated")
	noThreadName := fs.Bool("no-thread-name", false, "do not prepend the thread name as the synthetic root frame")
	format := fs.String("format", "", "force a thread-dump plugin format-id (skips header sniff)")
	out := fs.String("out", "-", "output path; `-` for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("--in is required (repeat or comma-separated)")
	}
	bundles, err := td.DefaultRegistry.ParseMany(paths, td.ParseOptions{FormatOverride: *format})
	if err != nil {
		return err
	}
	counts := threaddumpcollapsed.Convert(bundles, threaddumpcollapsed.Options{
		IncludeThreadName: !*noThreadName,
	})
	return emitJSON(counts, *out)
}
