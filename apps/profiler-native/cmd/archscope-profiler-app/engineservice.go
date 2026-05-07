package main

import (
	"fmt"
	"strings"
	"time"

	engineapi "github.com/aurakimjh/archscope/apps/engine-native/api"
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

// JenniferProfileRequest accepts a single Path or repeatable Paths
// for multi-file batches. FallbackCorrelationToTxid mirrors the
// CLI flag of the same name.
type JenniferProfileRequest struct {
	Path                      string   `json:"path,omitempty"`
	Paths                     []string `json:"paths,omitempty"`
	FallbackCorrelationToTxid bool     `json:"fallbackCorrelationToTxid,omitempty"`
	HeaderBodyToleranceMs     int      `json:"headerBodyToleranceMs,omitempty"`
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

// ──────────────────────────────────────────────────────────────────
// Async analyzers — multi-file inputs that walk N bundles. We keep
// the renderer responsive by returning a TaskID and emitting events.
// Reuses the package-level taskRegistry from profilerservice.go.
// ──────────────────────────────────────────────────────────────────

// AnalyzeMultiThread parses every path in `req.Paths` through the
// engine plugin registry and runs the multithread correlator. Async
// because plugin sniffing + N-way correlation can run for hundreds of
// ms on real-world dumps.
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
