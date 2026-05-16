// Package api is a thin re-export facade over the engine-native
// analyzers and exporters. It keeps the Wails service layer on a
// small stable facade instead of importing every analyzer package
// directly.
//
// # Why this exists
//
// Every analyzer and exporter lives under apps/engine-native/internal/...
// because those packages are the engine's implementation surface, not
// its public API. The Wails service binding calls Analyze*, Convert,
// ClassifyStack, Write, etc. from EngineService methods; this facade
// keeps that boundary narrow.
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
//
// ─────────────────────────────────────────────────────────────────────
// [한글] api 패키지 — engine-native 의 분석기/익스포터를 외부 Go 모듈
// 에 노출하기 위한 얇은 re-export 파사드.
//
// 존재 이유: Wails 서비스 boundary 안정화
//
//	Wails 서비스가 analyzer/exporter 패키지 전체를 직접 import 하면
//	UI 바인딩 계층이 내부 패키지 구조 변화에 민감해집니다. 이 facade가
//	작은 공개 surface를 제공해 EngineService의 의존성을 좁힙니다.
//
//	해결 후보 비교
//	  A) internal 규칙을 풀어 모든 helper 까지 외부에 노출
//	     → 워크스페이스 내 어떤 모듈이든 helper 까지 잡아 쓸 수 있게
//	       되어 캡슐화가 무너짐. 거부.
//	  B) 분석기 코드를 데스크톱 모듈에 복사
//	     → 두 곳을 동기화해야 하는 회귀 폭탄. 거부.
//	  C) (채택) 데스크톱 셸이 실제로 부르는 심볼만 type alias /
//	     var 할당 으로 다시 노출하는 얇은 파사드.
//	     → 로직 0줄, 추가 의존성 0개, 파일 1개로 표면 통제 가능.
//
// 노출 규칙
//   - 모델 타입(AnalysisResult, ThreadDumpBundle, ParseOptions 등)은
//     원래 이름 유지 — 이미 "공용어" 이므로 prefix 가 오히려 혼란.
//   - Options 구조체는 prefix 부여 (AccessLogOptions, JfrOptions, ...)
//     — 호출 지점에서 어떤 분석기의 옵션인지 즉시 구별 가능.
//   - 함수는 var alias 로 노출 (var AnalyzeAccessLog = accesslog.Analyze).
//     이렇게 하면 시그니처가 변경되었을 때 컴파일 에러로 즉시 표면화.
//
// 부수효과 import (`_` blank import)
//
//	threaddump 플러그인은 자기 자신을 enginethreaddump.DefaultRegistry
//	에 등록하는 init() 만 가지고 있습니다. 데스크톱 셸이 ParseMany /
//	ParseOne 을 호출할 때 모든 포맷을 인식할 수 있으려면 이 init()
//	들이 실행되어야 합니다. 따라서 6개 플러그인을 모두 _ import 해서
//	"api 만 import 해도 모든 플러그인이 자동 등록" 되도록 만듭니다.
//
// 카테고리별 표면 (아래 섹션 헤더 참조)
//   - Shared types
//   - Access log / Exception / GC log / JFR / Runtime / OTel
//   - Jennifer Profile (MSA)
//   - Thread-dump (single + multi + lock + collapsed 변환)
//   - Stack classification
//   - Exporters (JSON / HTML / PPTX / CSV / Comparison)
package api

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/accesslog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/brokerlog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/databaselog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/exception"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gclog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/jenniferprofile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/jfr"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/lockcontention"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/multithread"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/observability"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/otel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/platform"
	profileevidence "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profileclassification"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/runtime"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/serverlog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/stitching"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/threaddump"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/threaddumpcollapsed"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/traceimport"
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

// ── Server logs ─────────────────────────────────────────────────────

// ServerLogOptions mirrors serverlog.Options.
type ServerLogOptions = serverlog.Options

// AnalyzeServerLog wraps serverlog.Analyze.
var AnalyzeServerLog = serverlog.Analyze

// ── Metrics and observability exports ───────────────────────────────

type MetricsOptions = metrics.Options

var AnalyzeMetrics = metrics.Analyze

type ObservabilityOptions = observability.Options

var AnalyzeObservability = observability.Analyze

// ── Database logs and plans ─────────────────────────────────────────

type DatabaseLogOptions = databaselog.Options

var AnalyzeDatabaseLog = databaselog.Analyze

// ── Broker logs ─────────────────────────────────────────────────────

type BrokerLogOptions = brokerlog.Options

var AnalyzeBrokerLog = brokerlog.Analyze

// ── Platform and cloud audit evidence ───────────────────────────────

type PlatformOptions = platform.Options

var AnalyzePlatform = platform.Analyze

// ── Unified runtime profile evidence ────────────────────────────────

type ProfileEvidenceOptions = profileevidence.Options

var AnalyzeProfileEvidence = profileevidence.Analyze

// ── Cross-source evidence stitching ─────────────────────────────────

type StitchingOptions = stitching.Options

var AnalyzeStitchedEvidence = stitching.AnalyzeFiles

// ── External trace import ───────────────────────────────────────────

// TraceImportOptions mirrors traceimport.Options.
type TraceImportOptions = traceimport.Options

// AnalyzeTraceImport wraps traceimport.Analyze.
var AnalyzeTraceImport = traceimport.Analyze

// ── Jennifer Profile (MSA timeline) ─────────────────────────────────

// JenniferProfileOptions mirrors jenniferprofile.Options.
type JenniferProfileOptions = jenniferprofile.Options

// AnalyzeJenniferProfile wraps jenniferprofile.AnalyzeFile (single).
var AnalyzeJenniferProfile = jenniferprofile.AnalyzeFile

// AnalyzeJenniferProfiles wraps jenniferprofile.AnalyzeFiles (multi).
var AnalyzeJenniferProfiles = jenniferprofile.AnalyzeFiles

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
