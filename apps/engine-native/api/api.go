// Package api is a thin re-export facade over the engine-native
// analyzers and exporters. It exists solely to make the internal
// packages reachable from external Go modules (notably the Wails
// desktop shell at apps/profiler-native), without having to modify
// any of the analyzer / exporter packages themselves.
//
// Why this exists
//
// Every analyzer and exporter lives under apps/engine-native/internal/...
// because those packages are the engine's implementation surface, not
// its public API. Go's `internal/` rule blocks imports from outside the
// tree rooted at apps/engine-native/, even when the importer is wired
// in through a top-level go.work file. The Wails service binding in
// apps/profiler-native is just such an outside importer — it needs to
// call Analyze*, Convert, ClassifyStack, Write, etc. from its
// EngineService methods.
//
// Rather than relax the internal/ rule (which would expose every helper
// in those packages to anyone in the workspace) or duplicate the
// analyzer code, this package re-exports just the symbols the desktop
// shell needs as type aliases + variable assignments. The cost is a
// handful of identifiers — no logic, no dependencies pulled in beyond
// the analyzer packages themselves.
//
// Naming convention: each analyzer's Options is re-exported with a
// type prefix (`AccessLogOptions`, `JfrOptions`, ...) to disambiguate
// at the call site. AnalysisResult and the bundle / model types are
// re-exported under their original names because they're the lingua
// franca everyone refers to.
package api

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/accesslog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/exception"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gclog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/jfr"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/lockcontention"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/multithread"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/otel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profileclassification"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/runtime"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/threaddump"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/threaddumpcollapsed"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/csv"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	enginejson "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/pptx"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/reportdiff"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	enginethreaddump "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump"

	// Side-effect imports — every threaddump plugin registers itself
	// into enginethreaddump.DefaultRegistry via init(). Without these,
	// ParseMany / ParseOne will only resolve Java jstack and Go
	// goroutine inputs (those two register here too via direct import).
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/dotnetclrstack"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/gogoroutine"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/javajcmdjson"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/javajstack"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/nodejsreport"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/pythondump"
)

// ── Shared types ────────────────────────────────────────────────────

// AnalysisResult is the universal envelope every analyzer emits.
type AnalysisResult = models.AnalysisResult

// ThreadDumpBundle is the (file, snapshots) pair the thread-dump
// pipeline operates on.
type ThreadDumpBundle = models.ThreadDumpBundle

// ParseOptions controls thread-dump registry dispatch (FormatOverride
// bypasses head sniffing). Used by the multithread / lockcontention
// async paths and by ConvertToCollapsed.
type ParseOptions = enginethreaddump.ParseOptions

// DefaultRegistry is the package-level registry every plugin auto-registers
// into via init(). Callers normally just call DefaultRegistry.ParseMany.
var DefaultRegistry = enginethreaddump.DefaultRegistry

// ── Access log ──────────────────────────────────────────────────────

// AccessLogOptions mirrors accesslog.Options.
type AccessLogOptions = accesslog.Options

// AnalyzeAccessLog wraps accesslog.Analyze.
var AnalyzeAccessLog = accesslog.Analyze

// ── Exception ───────────────────────────────────────────────────────

// ExceptionOptions mirrors exception.Options.
type ExceptionOptions = exception.Options

// AnalyzeException wraps exception.Analyze.
var AnalyzeException = exception.Analyze

// ── GC log ──────────────────────────────────────────────────────────

// GcLogOptions mirrors gclog.Options.
type GcLogOptions = gclog.Options

// AnalyzeGcLog wraps gclog.Analyze.
var AnalyzeGcLog = gclog.Analyze

// ── JFR ─────────────────────────────────────────────────────────────

// JfrOptions mirrors jfr.Options.
type JfrOptions = jfr.Options

// JfrNativeMemoryOptions mirrors jfr.NativeMemoryOptions.
type JfrNativeMemoryOptions = jfr.NativeMemoryOptions

// AnalyzeJfr wraps jfr.Analyze.
var AnalyzeJfr = jfr.Analyze

// AnalyzeJfrNativeMemory wraps jfr.AnalyzeNativeMemory.
var AnalyzeJfrNativeMemory = jfr.AnalyzeNativeMemory

// NewJfrNativeMemoryOptions returns the Python-default native memory
// options (LeakOnly=true, TailRatio=0.10, with the user-set flags so
// the analyzer can distinguish "leave default" from "explicitly false").
var NewJfrNativeMemoryOptions = jfr.NewNativeMemoryOptions

// ── Runtime stacks (Node.js / Python / Go panic / .NET IIS) ─────────

// RuntimeOptions mirrors runtime.Options.
type RuntimeOptions = runtime.Options

// AnalyzeNodejsStack wraps runtime.AnalyzeNodejsStack.
var AnalyzeNodejsStack = runtime.AnalyzeNodejsStack

// AnalyzePythonTraceback wraps runtime.AnalyzePythonTraceback.
var AnalyzePythonTraceback = runtime.AnalyzePythonTraceback

// AnalyzeGoPanic wraps runtime.AnalyzeGoPanic.
var AnalyzeGoPanic = runtime.AnalyzeGoPanic

// AnalyzeDotnetExceptionIIS wraps runtime.AnalyzeDotnetExceptionIIS.
var AnalyzeDotnetExceptionIIS = runtime.AnalyzeDotnetExceptionIIS

// ── OpenTelemetry logs ──────────────────────────────────────────────

// OtelOptions mirrors otel.Options.
type OtelOptions = otel.Options

// AnalyzeOtel wraps otel.Analyze.
var AnalyzeOtel = otel.Analyze

// ── Thread-dump (single file, Java jstack only) ─────────────────────

// ThreadDumpOptions mirrors threaddump.Options.
type ThreadDumpOptions = threaddump.Options

// AnalyzeThreadDump wraps threaddump.Analyze.
var AnalyzeThreadDump = threaddump.Analyze

// ── Multi-thread correlator ─────────────────────────────────────────

// MultiThreadOptions mirrors multithread.Options.
type MultiThreadOptions = multithread.Options

// AnalyzeMultiThread wraps multithread.Analyze.
var AnalyzeMultiThread = multithread.Analyze

// ── Lock contention analyzer ────────────────────────────────────────

// LockContentionOptions mirrors lockcontention.Options.
type LockContentionOptions = lockcontention.Options

// AnalyzeLockContention wraps lockcontention.Analyze.
var AnalyzeLockContention = lockcontention.Analyze

// ── Thread-dump → collapsed conversion ──────────────────────────────

// CollapsedOptions mirrors threaddumpcollapsed.Options.
type CollapsedOptions = threaddumpcollapsed.Options

// ConvertCollapsed wraps threaddumpcollapsed.Convert.
var ConvertCollapsed = threaddumpcollapsed.Convert

// SortedCollapsedLines wraps threaddumpcollapsed.SortedLines.
var SortedCollapsedLines = threaddumpcollapsed.SortedLines

// ── Stack classification (rule-based runtime label) ─────────────────

// ClassifyStack wraps profileclassification.ClassifyStack. Pass nil
// for `rules` to use the package-level DefaultRules (matches Python's
// `rules=None`).
var ClassifyStack = profileclassification.ClassifyStack

// ── Exporters ───────────────────────────────────────────────────────

// WriteJSON marshals an AnalysisResult-shaped value to a JSON file.
var WriteJSON = enginejson.Write

// WriteHTML renders an AnalysisResult-shaped value as a static HTML
// page on disk.
var WriteHTML = html.Write

// WritePPTX renders an AnalysisResult-shaped value as a slide deck
// on disk.
var WritePPTX = pptx.Write

// WriteCSV writes a single CSV with section headers.
var WriteCSV = csv.Write

// WriteCSVAll writes one CSV per logical table under a directory.
var WriteCSVAll = csv.WriteAll

// BuildComparisonReport diffs two AnalysisResult JSON files and
// returns the comparison_report payload as a generic map.
var BuildComparisonReport = reportdiff.BuildComparisonReport
