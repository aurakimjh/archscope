// ─────────────────────────────────────────────────────────────────────
// [한글] engineservice — Wails 데스크톱 앱의 EngineService 바인딩.
//
// 책임/목적
//   apps/engine-native 패키지의 분석기/exporter 들을 Wails renderer 에
//   노출. 통합 데스크톱 앱은 profiler 분석뿐 아니라 access log,
//   GC log, JFR, exception, runtime stack, jennifer profile, OTel,
//   threaddump 같은 engine-native 분석도 같은 UI 에서 제공한다.
//
// Sync vs Async 정책
//   - 단일 파일 + 단순 분석(accesslog, gclog, exception 등): 동기 호출.
//     호출 즉시 AnalysisResult 반환. ms 단위라 round-trip 오버헤드보다 빠름.
//   - 다중 파일 / N-way correlation(multithread, lockcontention): 비동기.
//     EngineAsyncResponse{TaskID} 즉시 반환 → goroutine 에서 분석 후
//     engine:done / engine:error / engine:cancelled 이벤트 emit.
//
// 등록 메서드 (예시)
//   AnalyzeAccessLog / AnalyzeGcLog / AnalyzeException / AnalyzeRuntimeStack /
//   AnalyzeJfr / AnalyzeNativeMemory / AnalyzeJennifer / AnalyzeOtel /
//   AnalyzeMultithreadAsync / AnalyzeLockContentionAsync / Cancel
//   (실제 메서드 목록은 본 파일 아래에서 직접 확인).
//
// Request 타입
//   - AccessLogRequest / GcLogRequest / ExceptionRequest / JfrRequest /
//     JenniferRequest / OtelRequest / RuntimeStackRequest /
//     MultithreadRequest / LockContentionRequest
//   - 각 Request 는 engine-native 의 Options 와 1:1 매핑되며, RFC3339
//     문자열로 들어온 시간 필드는 time.Parse 로 변환.
//
// 트리키한 부분
//   • RFC3339 빈 문자열은 zero time 으로 매핑되어 옵션에서 nil 처리됨.
//   • 비동기 분석은 같은 globalTasks 레지스트리를 공유 (ProfilerService 와
//     공동) — taskID prefix 로 어떤 종류의 작업인지 구분.
//   • engineapi.AnalysisResult 는 profiler 와는 별개의 envelope 이지만 JSON
//     shape 컨트랙트는 동일 (Type / Summary / Series / Tables / Charts / Metadata).
// ─────────────────────────────────────────────────────────────────────

package main

import (
	"fmt"
	"strings"
	"time"

	engineapi "github.com/aurakimjh/archscope/apps/engine-native/api"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/aiinterpretation"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// EngineService exposes the Tier-3/4 analyzers and exporters from
// apps/engine-native to the Wails renderer. The engine packages are
// stateless per-call (they take a path / bundle slice and return an
// AnalysisResult), so the service holds no fields. The Wails binding
// generator picks up exported methods on this type and emits matching
// TypeScript signatures under
// `frontend/bindings/.../engineservice.ts`.
//
// Sync vs async split: any analyzer that walks multiple files or builds
// N-way correlations (multithread, lockcontention) returns immediately
// with an EngineAsyncResponse{TaskID} and reports completion via the
// engine:done / engine:error / engine:cancelled events. Single-file
// analyzers are synchronous because the µs–ms cost wouldn't justify the
// round-trip ceremony.
type EngineService struct{}

// EngineAsyncResponse mirrors AnalyzeAsyncResponse on the profiler side.
type EngineAsyncResponse struct {
	TaskID string `json:"taskId"`
}

// EngineDoneEvent carries an AnalysisResult back to the renderer.
type EngineDoneEvent struct {
	TaskID string                   `json:"taskId"`
	Result engineapi.AnalysisResult `json:"result"`
}

// EngineErrorEvent carries an analyzer / exporter failure.
type EngineErrorEvent struct {
	TaskID  string `json:"taskId"`
	Message string `json:"message"`
}

// EngineCancelledEvent fires when the renderer cancels a task before
// it produced a result.
type EngineCancelledEvent struct {
	TaskID string `json:"taskId"`
}

// ──────────────────────────────────────────────────────────────────
// Request shapes — kept minimal; the Wails binding generator emits
// matching TS interfaces from these.
// ──────────────────────────────────────────────────────────────────

// AccessLogRequest mirrors accesslog.Options + a Path.
type AccessLogRequest struct {
	Path      string `json:"path"`
	Format    string `json:"format,omitempty"`
	MaxLines  int    `json:"maxLines,omitempty"`
	StartTime string `json:"startTime,omitempty"` // RFC3339; empty = unset
	EndTime   string `json:"endTime,omitempty"`
}

// GcLogRequest mirrors gclog.Options + a Path.
type GcLogRequest struct {
	Path     string `json:"path"`
	TopN     int    `json:"topN,omitempty"`
	MaxLines int    `json:"maxLines,omitempty"`
	Strict   bool   `json:"strict,omitempty"`
}

// JfrRequest mirrors jfr.Options + a Path. Used for both Analyze and
// AnalyzeNativeMemory — the latter only honors Path / LeakOnly /
// TailRatio.
type JfrRequest struct {
	Path     string `json:"path"`
	TopN     int    `json:"topN,omitempty"`
	Mode     string `json:"mode,omitempty"`
	FromTime string `json:"fromTime,omitempty"`
	ToTime   string `json:"toTime,omitempty"`
	State    string `json:"state,omitempty"`
	// Native-memory knobs.
	LeakOnly  bool    `json:"leakOnly,omitempty"`
	TailRatio float64 `json:"tailRatio,omitempty"`
}

// ExceptionRequest mirrors exception.Options + a Path.
type ExceptionRequest struct {
	Path     string `json:"path"`
	TopN     int    `json:"topN,omitempty"`
	MaxLines int    `json:"maxLines,omitempty"`
	Strict   bool   `json:"strict,omitempty"`
}

// RuntimeRequest dispatches to one of the four runtime.Analyze*
// entry points based on Variant. Variant must be one of:
// "nodejs", "python", "go", "dotnet".
type RuntimeRequest struct {
	Path     string `json:"path"`
	Variant  string `json:"variant"`
	TopN     int    `json:"topN,omitempty"`
	MaxLines int    `json:"maxLines,omitempty"`
	Strict   bool   `json:"strict,omitempty"`
}

// OtelRequest mirrors otel.Options + a Path.
type OtelRequest struct {
	Path string `json:"path"`
	TopN int    `json:"topN,omitempty"`
}

// ServerLogRequest mirrors serverlog.Options + a Path.
type ServerLogRequest struct {
	Path     string `json:"path"`
	Format   string `json:"format,omitempty"`
	TopN     int    `json:"topN,omitempty"`
	MaxLines int    `json:"maxLines,omitempty"`
	Strict   bool   `json:"strict,omitempty"`
}

type MetricsRequest struct {
	Path     string `json:"path"`
	TopN     int    `json:"topN,omitempty"`
	MaxLines int    `json:"maxLines,omitempty"`
	Strict   bool   `json:"strict,omitempty"`
}

type ObservabilityRequest struct {
	Path   string `json:"path"`
	Format string `json:"format,omitempty"`
	TopN   int    `json:"topN,omitempty"`
}

type DatabaseLogRequest struct {
	Path   string `json:"path"`
	Format string `json:"format,omitempty"`
	TopN   int    `json:"topN,omitempty"`
	Strict bool   `json:"strict,omitempty"`
}

type BrokerLogRequest struct {
	Path   string `json:"path"`
	Format string `json:"format,omitempty"`
	TopN   int    `json:"topN,omitempty"`
	Strict bool   `json:"strict,omitempty"`
}

type PlatformRequest struct {
	Path   string `json:"path"`
	Format string `json:"format,omitempty"`
	TopN   int    `json:"topN,omitempty"`
}

type ProfileEvidenceRequest struct {
	Path        string  `json:"path"`
	Format      string  `json:"format,omitempty"`
	TopN        int     `json:"topN,omitempty"`
	IntervalMS  float64 `json:"intervalMs,omitempty"`
	ProfileKind string  `json:"profileKind,omitempty"`
	MaxBytes    int64   `json:"maxBytes,omitempty"`
	MaxSamples  int     `json:"maxSamples,omitempty"`
}

type StitchedEvidenceRequest struct {
	Paths             []string `json:"paths"`
	TopN              int      `json:"topN,omitempty"`
	TimeWindowSeconds int      `json:"timeWindowSeconds,omitempty"`
}

type ApiContractRequest struct {
	OpenAPIPath        string  `json:"openapiPath,omitempty"`
	AccessResultPath   string  `json:"accessResultPath,omitempty"`
	AsyncAPIPath       string  `json:"asyncapiPath,omitempty"`
	BrokerResultPath   string  `json:"brokerResultPath,omitempty"`
	TopN               int     `json:"topN,omitempty"`
	SlowThresholdMS    float64 `json:"slowThresholdMs,omitempty"`
	ErrorRateThreshold float64 `json:"errorRateThreshold,omitempty"`
}

type ArchitectureDocsRequest struct {
	Paths []string `json:"paths"`
	TopN  int      `json:"topN,omitempty"`
}

// TraceImportRequest mirrors traceimport.Options + a Path.
type TraceImportRequest struct {
	Path   string `json:"path"`
	Format string `json:"format,omitempty"`
	TopN   int    `json:"topN,omitempty"`
}

// HttpCaptureRequest mirrors the file-first HAR analyzer. Live capture is not
// exposed here: the desktop boundary intentionally begins with safe imports.
type HttpCaptureRequest struct {
	Path       string `json:"path"`
	Format     string `json:"format,omitempty"`
	TopN       int    `json:"topN,omitempty"`
	MaxEntries int    `json:"maxEntries,omitempty"`
}

// JenniferProfileRequest accepts a single Path or repeatable Paths
// for multi-file batches. FallbackCorrelationToTxid mirrors the
// CLI flag of the same name.
type JenniferProfileRequest struct {
	Path                      string   `json:"path,omitempty"`
	Paths                     []string `json:"paths,omitempty"`
	FallbackCorrelationToTxid bool     `json:"fallbackCorrelationToTxid,omitempty"`
	HeaderBodyToleranceMs     int      `json:"headerBodyToleranceMs,omitempty"`
	// NetworkPrepPatterns are additional case-insensitive substrings
	// that mark an event as a "network prep" wrapper method. Built-in
	// defaults such as IntegrationUtil.sendToService always remain
	// active.
	NetworkPrepPatterns []string `json:"networkPrepPatterns,omitempty"`
	// ServletDispatchPatterns mark the callee servlet entry-point whose
	// self-time is dispatch overhead. Built-in defaults always remain active.
	ServletDispatchPatterns []string `json:"servletDispatchPatterns,omitempty"`
	// EventCategoryPatterns extends the event classifier. Keys are
	// JenniferEventType values; values are case-insensitive substrings.
	// User patterns only apply to METHOD/UNKNOWN events so they can't
	// override well-known matches.
	EventCategoryPatterns map[string][]string `json:"eventCategoryPatterns,omitempty"`
	// CustomAnalysisRules produces user-defined MSA roll-up cards.
	CustomAnalysisRules []models.JenniferCustomAnalysisRule `json:"customAnalysisRules,omitempty"`
}

// ThreadDumpRequest mirrors threaddump.Options + a Path.
type ThreadDumpRequest struct {
	Path string `json:"path"`
	TopN int    `json:"topN,omitempty"`
}

// MultiThreadRequest takes paths and lets the engine's plugin registry
// detect format from header bytes. FormatOverride bypasses sniffing
// when the renderer already knows the format.
type MultiThreadRequest struct {
	Paths               []string `json:"paths"`
	FormatOverride      string   `json:"formatOverride,omitempty"`
	Threshold           int      `json:"threshold,omitempty"`
	TopN                int      `json:"topN,omitempty"`
	IncludeRawSnapshots bool     `json:"includeRawSnapshots,omitempty"`
}

// LockContentionRequest is the lock-contention analog of MultiThreadRequest.
type LockContentionRequest struct {
	Paths          []string `json:"paths"`
	FormatOverride string   `json:"formatOverride,omitempty"`
	TopN           int      `json:"topN,omitempty"`
}

// CollapsedRequest converts a set of thread-dump paths into the
// FlameGraph collapsed format. Returns the `stack -> count` map
// directly (Counter equivalent).
type CollapsedRequest struct {
	Paths             []string `json:"paths"`
	FormatOverride    string   `json:"formatOverride,omitempty"`
	IncludeThreadName bool     `json:"includeThreadName,omitempty"`
}

// CollapsedResult carries both the raw counter and the sorted line
// projection so the renderer can pick whichever shape suits.
type CollapsedResult struct {
	Counts map[string]int `json:"counts"`
	Lines  []string       `json:"lines"`
}

// ClassifyRequest classifies one collapsed-stack string. Returns the
// runtime label (e.g. "JVM", "Node.js", "Application").
type ClassifyRequest struct {
	Stack string `json:"stack"`
}

// ClassifyResult is a single-field wrapper so the TS binding has a
// readable type name on the renderer side.
type ClassifyResult struct {
	Label string `json:"label"`
}

// ExportJSONRequest writes an AnalysisResult-shaped value to disk as
// JSON. `Result` is `any` — the renderer typically forwards the same
// shape it received from a previous Analyze* call.
type ExportJSONRequest struct {
	Path   string `json:"path"`
	Result any    `json:"result"`
}

// ExportHTMLRequest is the HTML analog.
type ExportHTMLRequest struct {
	Path   string `json:"path"`
	Result any    `json:"result"`
}

// ExportPPTXRequest is the PPTX analog.
type ExportPPTXRequest struct {
	Path   string `json:"path"`
	Result any    `json:"result"`
}

// ExportCSVRequest is the single-file CSV analog. For directory mode
// (one CSV per logical table) call ExportCSVDir.
type ExportCSVRequest struct {
	Path   string `json:"path"`
	Result any    `json:"result"`
}

// EngineDiffRequest carries two AnalysisResult JSON paths and an
// optional label. Returns the full reportdiff payload as a generic
// map so the renderer can render it like any other AnalysisResult.
type EngineDiffRequest struct {
	BeforePath string `json:"beforePath"`
	AfterPath  string `json:"afterPath"`
	Label      string `json:"label,omitempty"`
}

// AiInterpretationGateRequest evaluates an AI interpretation payload against
// the same Go evidence registry and validator used before LLM output is stored.
type AiInterpretationGateRequest struct {
	Result                map[string]any `json:"result"`
	Interpretation        map[string]any `json:"interpretation"`
	MinConfidence         float64        `json:"minConfidence,omitempty"`
	RequireEvidenceQuotes bool           `json:"requireEvidenceQuotes,omitempty"`
}

// ──────────────────────────────────────────────────────────────────
// Sync analyzers — single-file inputs, microsecond-to-millisecond
// runtimes. We don't bother with the task-registry round-trip.
// ──────────────────────────────────────────────────────────────────

// AnalyzeAccessLog wraps engineapi.AnalyzeAccessLog. RFC3339 strings
// on the request are parsed once and forwarded as *time.Time.
func (s *EngineService) AnalyzeAccessLog(req AccessLogRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	opts := engineapi.AccessLogOptions{MaxLines: req.MaxLines}
	if strings.TrimSpace(req.StartTime) != "" {
		t, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			return engineapi.AnalysisResult{}, fmt.Errorf("startTime: %w", err)
		}
		opts.StartTime = &t
	}
	if strings.TrimSpace(req.EndTime) != "" {
		t, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			return engineapi.AnalysisResult{}, fmt.Errorf("endTime: %w", err)
		}
		opts.EndTime = &t
	}
	return engineapi.AnalyzeAccessLog(req.Path, req.Format, opts)
}

// AnalyzeGcLog wraps engineapi.AnalyzeGcLog.
func (s *EngineService) AnalyzeGcLog(req GcLogRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeGcLog(req.Path, engineapi.GcLogOptions{
		TopN:     req.TopN,
		MaxLines: req.MaxLines,
		Strict:   req.Strict,
	})
}

// AnalyzeJfr wraps engineapi.AnalyzeJfr.
func (s *EngineService) AnalyzeJfr(req JfrRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeJfr(req.Path, engineapi.JfrOptions{
		TopN:     req.TopN,
		Mode:     req.Mode,
		FromTime: req.FromTime,
		ToTime:   req.ToTime,
		State:    req.State,
	})
}

// AnalyzeNativeMemory wraps engineapi.AnalyzeJfrNativeMemory.
func (s *EngineService) AnalyzeNativeMemory(req JfrRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	opts := engineapi.NewJfrNativeMemoryOptions()
	// LeakOnly defaults to true in the constructor; honour the
	// renderer's explicit value because the request always carries
	// one branch positively.
	opts.LeakOnly = req.LeakOnly
	if req.TailRatio > 0 {
		opts.TailRatio = req.TailRatio
	}
	return engineapi.AnalyzeJfrNativeMemory(req.Path, opts)
}

// AnalyzeException wraps engineapi.AnalyzeException.
func (s *EngineService) AnalyzeException(req ExceptionRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeException(req.Path, engineapi.ExceptionOptions{
		TopN:     req.TopN,
		MaxLines: req.MaxLines,
		Strict:   req.Strict,
	})
}

// AnalyzeRuntimeStack dispatches to one of the four runtime variants
// based on Variant. Empty Variant falls back to "nodejs".
func (s *EngineService) AnalyzeRuntimeStack(req RuntimeRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	opts := engineapi.RuntimeOptions{
		TopN:     req.TopN,
		MaxLines: req.MaxLines,
		Strict:   req.Strict,
	}
	switch strings.ToLower(strings.TrimSpace(req.Variant)) {
	case "", "nodejs", "node":
		return engineapi.AnalyzeNodejsStack(req.Path, opts)
	case "python":
		return engineapi.AnalyzePythonTraceback(req.Path, opts)
	case "go", "golang":
		return engineapi.AnalyzeGoPanic(req.Path, opts)
	case "dotnet", ".net", "iis":
		return engineapi.AnalyzeDotnetExceptionIIS(req.Path, opts)
	default:
		return engineapi.AnalysisResult{}, fmt.Errorf("unsupported runtime variant: %q (expected nodejs/python/go/dotnet)", req.Variant)
	}
}

// AnalyzeOtel wraps engineapi.AnalyzeOtel.
func (s *EngineService) AnalyzeOtel(req OtelRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeOtel(req.Path, engineapi.OtelOptions{TopN: req.TopN})
}

// AnalyzeServerLog wraps engineapi.AnalyzeServerLog.
func (s *EngineService) AnalyzeServerLog(req ServerLogRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeServerLog(req.Path, req.Format, engineapi.ServerLogOptions{
		MaxLines: req.MaxLines,
		TopN:     req.TopN,
		Strict:   req.Strict,
	})
}

func (s *EngineService) AnalyzeMetrics(req MetricsRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeMetrics(req.Path, engineapi.MetricsOptions{
		MaxLines: req.MaxLines,
		TopN:     req.TopN,
		Strict:   req.Strict,
	})
}

func (s *EngineService) AnalyzeObservability(req ObservabilityRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeObservability(req.Path, engineapi.ObservabilityOptions{Format: req.Format, TopN: req.TopN})
}

func (s *EngineService) AnalyzeDatabaseLog(req DatabaseLogRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeDatabaseLog(req.Path, engineapi.DatabaseLogOptions{Format: req.Format, TopN: req.TopN, Strict: req.Strict})
}

func (s *EngineService) AnalyzeBrokerLog(req BrokerLogRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeBrokerLog(req.Path, engineapi.BrokerLogOptions{Format: req.Format, TopN: req.TopN, Strict: req.Strict})
}

func (s *EngineService) AnalyzePlatform(req PlatformRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzePlatform(req.Path, engineapi.PlatformOptions{Format: req.Format, TopN: req.TopN})
}

func (s *EngineService) AnalyzeProfileEvidence(req ProfileEvidenceRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeProfileEvidence(req.Path, engineapi.ProfileEvidenceOptions{
		Format:      req.Format,
		TopN:        req.TopN,
		IntervalMS:  req.IntervalMS,
		ProfileKind: req.ProfileKind,
		MaxBytes:    req.MaxBytes,
		MaxSamples:  req.MaxSamples,
	})
}

func (s *EngineService) AnalyzeHttpCapture(req HttpCaptureRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeHttpCapture(req.Path, engineapi.HttpCaptureOptions{Format: req.Format, TopN: req.TopN, MaxEntries: req.MaxEntries})
}

func (s *EngineService) AnalyzeStitchedEvidence(req StitchedEvidenceRequest) (engineapi.AnalysisResult, error) {
	if len(req.Paths) == 0 {
		return engineapi.AnalysisResult{}, fmt.Errorf("paths are required")
	}
	return engineapi.AnalyzeStitchedEvidence(req.Paths, engineapi.StitchingOptions{TopN: req.TopN, TimeWindowSeconds: req.TimeWindowSeconds})
}

func (s *EngineService) AnalyzeApiContract(req ApiContractRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.OpenAPIPath) == "" && strings.TrimSpace(req.AsyncAPIPath) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("openapiPath or asyncapiPath is required")
	}
	return engineapi.AnalyzeApiContract(engineapi.ApiContractOptions{
		OpenAPIPath:        req.OpenAPIPath,
		AccessResultPath:   req.AccessResultPath,
		AsyncAPIPath:       req.AsyncAPIPath,
		BrokerResultPath:   req.BrokerResultPath,
		TopN:               req.TopN,
		SlowThresholdMS:    req.SlowThresholdMS,
		ErrorRateThreshold: req.ErrorRateThreshold,
	})
}

func (s *EngineService) AnalyzeArchitectureDocs(req ArchitectureDocsRequest) (engineapi.AnalysisResult, error) {
	if len(req.Paths) == 0 {
		return engineapi.AnalysisResult{}, fmt.Errorf("paths are required")
	}
	return engineapi.AnalyzeArchitectureDocs(req.Paths, engineapi.ArchitectureDocsOptions{TopN: req.TopN})
}

// AnalyzeTraceImport wraps engineapi.AnalyzeTraceImport.
func (s *EngineService) AnalyzeTraceImport(req TraceImportRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeTraceImport(req.Path, engineapi.TraceImportOptions{
		Format: req.Format,
		TopN:   req.TopN,
	})
}

// AnalyzeJenniferProfile wraps engineapi.AnalyzeJenniferProfile/s.
// Either Path (single) or Paths (batch) must be provided; if both
// are set, Paths wins.
func (s *EngineService) AnalyzeJenniferProfile(req JenniferProfileRequest) (engineapi.AnalysisResult, error) {
	paths := req.Paths
	if len(paths) == 0 && strings.TrimSpace(req.Path) != "" {
		paths = []string{req.Path}
	}
	if len(paths) == 0 {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	opts := engineapi.JenniferProfileOptions{
		FallbackCorrelationToTxid: req.FallbackCorrelationToTxid,
		HeaderBodyToleranceMs:     req.HeaderBodyToleranceMs,
		NetworkPrepPatterns:       req.NetworkPrepPatterns,
		ServletDispatchPatterns:   req.ServletDispatchPatterns,
		EventCategoryPatterns:     req.EventCategoryPatterns,
		CustomAnalysisRules:       req.CustomAnalysisRules,
	}
	if len(paths) == 1 {
		return engineapi.AnalyzeJenniferProfile(paths[0], opts)
	}
	return engineapi.AnalyzeJenniferProfiles(paths, opts)
}

// AnalyzeThreadDump wraps engineapi.AnalyzeThreadDump (single Java
// jstack file).
func (s *EngineService) AnalyzeThreadDump(req ThreadDumpRequest) (engineapi.AnalysisResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return engineapi.AnalysisResult{}, fmt.Errorf("path is required")
	}
	return engineapi.AnalyzeThreadDump(req.Path, engineapi.ThreadDumpOptions{TopN: req.TopN})
}

// ClassifyStack wraps engineapi.ClassifyStack. The package-level
// default rules are used (matches Python's `rules=None`).
func (s *EngineService) ClassifyStack(req ClassifyRequest) ClassifyResult {
	return ClassifyResult{Label: engineapi.ClassifyStack(req.Stack, nil)}
}

// EvaluateAiInterpretation exposes the authoritative Go AI evidence gate to the
// renderer so report surfaces can display validation status without reimplementing
// analyzer evidence lookup semantics.
func (s *EngineService) EvaluateAiInterpretation(req AiInterpretationGateRequest) (map[string]any, error) {
	if req.Interpretation == nil {
		return map[string]any{
			"total_findings":           0,
			"valid_findings":           0,
			"rejected_findings":        0,
			"evidence_integrity_ratio": 0.0,
			"valid":                    false,
			"gate_status":              "not_present",
			"issue_codes":              []string{},
			"issues":                   []aiinterpretation.ValidationIssue{},
		}, nil
	}
	minConfidence := req.MinConfidence
	if minConfidence <= 0 {
		minConfidence = 0.3
	}
	registry := aiinterpretation.BuildEvidenceRegistry(req.Result)
	validator := aiinterpretation.AiFindingValidator{
		Registry:              registry,
		MinConfidence:         minConfidence,
		RequireEvidenceQuotes: req.RequireEvidenceQuotes,
	}
	evaluation := aiinterpretation.EvaluateInterpretation(req.Interpretation, validator)
	evaluation["min_confidence"] = minConfidence
	evaluation["require_evidence_quotes"] = req.RequireEvidenceQuotes
	return evaluation, nil
}

// ──────────────────────────────────────────────────────────────────
// Async analyzers — multi-file inputs that walk N bundles. We keep
// the renderer responsive by returning a TaskID and emitting events.
// Reuses the package-level taskRegistry from profilerservice.go.
// ──────────────────────────────────────────────────────────────────

// AnalyzeMultiThread parses every path in `req.Paths` through the
// engine plugin registry and runs the multithread correlator. Async
// because plugin sniffing + N-way correlation can run for hundreds of
// ms on real-world dumps.
//
// [한글] AnalyzeMultiThread — 비동기 multithread 분석.
// 1) plugin registry 로 형식 자동 인식 + ParseMany 로 ThreadDumpBundle 추출
// 2) AnalyzeMultiThread (장시간 N-way correlation) 실행
// 3) cancellation 신호 우선, 정상이면 engine:done, 에러면 engine:error.
func (s *EngineService) AnalyzeMultiThread(req MultiThreadRequest) (EngineAsyncResponse, error) {
	if len(req.Paths) == 0 {
		return EngineAsyncResponse{}, fmt.Errorf("paths is required")
	}
	taskID := newTaskID("engine-multithread")
	cancelCh := globalTasks.add(taskID)

	go func() {
		defer globalTasks.remove(taskID)
		bundles, err := engineapi.DefaultRegistry.ParseMany(req.Paths, engineapi.ParseOptions{
			FormatOverride: req.FormatOverride,
		})
		if err != nil {
			emitOrCancel(taskID, cancelCh, engineapi.AnalysisResult{}, err)
			return
		}
		result, runErr := engineapi.AnalyzeMultiThread(bundles, engineapi.MultiThreadOptions{
			Threshold:           req.Threshold,
			TopN:                req.TopN,
			IncludeRawSnapshots: req.IncludeRawSnapshots,
		})
		emitOrCancel(taskID, cancelCh, result, runErr)
	}()

	return EngineAsyncResponse{TaskID: taskID}, nil
}

// AnalyzeLockContention is the lock-contention analog of AnalyzeMultiThread.
func (s *EngineService) AnalyzeLockContention(req LockContentionRequest) (EngineAsyncResponse, error) {
	if len(req.Paths) == 0 {
		return EngineAsyncResponse{}, fmt.Errorf("paths is required")
	}
	taskID := newTaskID("engine-lockcontention")
	cancelCh := globalTasks.add(taskID)

	go func() {
		defer globalTasks.remove(taskID)
		bundles, err := engineapi.DefaultRegistry.ParseMany(req.Paths, engineapi.ParseOptions{
			FormatOverride: req.FormatOverride,
		})
		if err != nil {
			emitOrCancel(taskID, cancelCh, engineapi.AnalysisResult{}, err)
			return
		}
		result, runErr := engineapi.AnalyzeLockContention(bundles, engineapi.LockContentionOptions{TopN: req.TopN})
		emitOrCancel(taskID, cancelCh, result, runErr)
	}()

	return EngineAsyncResponse{TaskID: taskID}, nil
}

// CancelEngineTask is the engine-side cancel hook. ProfilerService.Cancel
// already covers profiler tasks; the registry is shared so either name
// works, but a separate method keeps the binding name self-documenting.
func (s *EngineService) CancelEngineTask(taskID string) bool {
	return globalTasks.cancel(taskID)
}

// emitOrCancel honours a cancellation signal that arrived during the
// goroutine's runtime. Mirrors the pattern in ProfilerService.AnalyzeAsync.
//
// [한글] emitOrCancel — 분석 종료 시점에 cancel 신호 우선 검사.
// cancel 되었으면 engine:cancelled, 에러면 engine:error, 정상은 engine:done.
// channel 의 select default branch 가 race condition 없이 우선순위를 결정.
func emitOrCancel(taskID string, cancelCh chan struct{}, result engineapi.AnalysisResult, err error) {
	select {
	case <-cancelCh:
		emitEvent("engine:cancelled", EngineCancelledEvent{TaskID: taskID})
	default:
		if err != nil {
			emitEvent("engine:error", EngineErrorEvent{TaskID: taskID, Message: err.Error()})
			return
		}
		emitEvent("engine:done", EngineDoneEvent{TaskID: taskID, Result: result})
	}
}

// ──────────────────────────────────────────────────────────────────
// Conversions — sync, cheap.
// ──────────────────────────────────────────────────────────────────

// ConvertToCollapsed parses each path through the engine plugin
// registry and folds the snapshots into the FlameGraph collapsed
// format. Returns counts + sorted lines so the renderer can pick its
// preferred shape.
func (s *EngineService) ConvertToCollapsed(req CollapsedRequest) (CollapsedResult, error) {
	if len(req.Paths) == 0 {
		return CollapsedResult{}, fmt.Errorf("paths is required")
	}
	bundles, err := engineapi.DefaultRegistry.ParseMany(req.Paths, engineapi.ParseOptions{
		FormatOverride: req.FormatOverride,
	})
	if err != nil {
		return CollapsedResult{}, err
	}
	counts := engineapi.ConvertCollapsed(bundles, engineapi.CollapsedOptions{
		IncludeThreadName: req.IncludeThreadName,
	})
	return CollapsedResult{
		Counts: counts,
		Lines:  engineapi.SortedCollapsedLines(counts),
	}, nil
}

// ──────────────────────────────────────────────────────────────────
// Exporters — all sync. The exporter packages handle parent-dir
// creation + atomic write, so we just forward the call.
// ──────────────────────────────────────────────────────────────────

// ExportJSON marshals `Result` and writes it to `Path`.
func (s *EngineService) ExportJSON(req ExportJSONRequest) error {
	if strings.TrimSpace(req.Path) == "" {
		return fmt.Errorf("path is required")
	}
	return engineapi.WriteJSON(req.Path, req.Result)
}

// ExportHTML renders `Result` as a static HTML page at `Path`.
func (s *EngineService) ExportHTML(req ExportHTMLRequest) error {
	if strings.TrimSpace(req.Path) == "" {
		return fmt.Errorf("path is required")
	}
	return engineapi.WriteHTML(req.Path, req.Result)
}

// ExportPPTX renders `Result` as a slide deck at `Path`.
func (s *EngineService) ExportPPTX(req ExportPPTXRequest) error {
	if strings.TrimSpace(req.Path) == "" {
		return fmt.Errorf("path is required")
	}
	return engineapi.WritePPTX(req.Path, req.Result)
}

// ExportCSV writes a single CSV with section headers to `Path`.
func (s *EngineService) ExportCSV(req ExportCSVRequest) error {
	if strings.TrimSpace(req.Path) == "" {
		return fmt.Errorf("path is required")
	}
	return engineapi.WriteCSV(req.Path, req.Result)
}

// ExportCSVDir is the directory-mode variant — one CSV per logical
// table under the directory at `Path`.
func (s *EngineService) ExportCSVDir(req ExportCSVRequest) error {
	if strings.TrimSpace(req.Path) == "" {
		return fmt.Errorf("path is required")
	}
	return engineapi.WriteCSVAll(req.Path, req.Result)
}

// DiffReports loads two AnalysisResult JSON files from disk and
// returns the comparison_report payload. Mirrors Python's
// build_comparison_report.
func (s *EngineService) DiffReports(req EngineDiffRequest) (map[string]any, error) {
	if strings.TrimSpace(req.BeforePath) == "" || strings.TrimSpace(req.AfterPath) == "" {
		return nil, fmt.Errorf("beforePath and afterPath are required")
	}
	return engineapi.BuildComparisonReport(req.BeforePath, req.AfterPath, req.Label)
}
