package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aurakimjh/archscope/apps/profiler-native/internal/profiler"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// taskRegistry tracks in-flight analyze/diff/export tasks so the renderer
// can cancel a long-running call. The collapsed/Jennifer/SVG/HTML analyzers
// are CPU-bound and synchronous internally — cancellation is honored as
// "discard the result and emit cancelled" rather than tearing down the
// goroutine mid-flight, which is acceptable here because the work is in
// the µs–ms range and the goroutine exits naturally a moment later.
type taskRegistry struct {
	mu    sync.Mutex
	tasks map[string]chan struct{}
}

func newTaskRegistry() *taskRegistry {
	return &taskRegistry{tasks: map[string]chan struct{}{}}
}

func (r *taskRegistry) add(taskID string) chan struct{} {
	ch := make(chan struct{})
	r.mu.Lock()
	r.tasks[taskID] = ch
	r.mu.Unlock()
	return ch
}

func (r *taskRegistry) remove(taskID string) {
	r.mu.Lock()
	delete(r.tasks, taskID)
	r.mu.Unlock()
}

func (r *taskRegistry) cancel(taskID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok := r.tasks[taskID]
	if !ok {
		return false
	}
	select {
	case <-ch:
		// already cancelled
	default:
		close(ch)
	}
	return true
}

var globalTasks = newTaskRegistry()
var taskCounter atomic.Uint64

func newTaskID(prefix string) string {
	n := taskCounter.Add(1)
	return prefix + "-" + strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatUint(n, 36)
}

type ProfilerService struct{}

// AnalyzeAsyncResponse is returned to the renderer immediately. The actual
// AnalysisResult arrives later via the `analyze:done` event keyed by taskId.
type AnalyzeAsyncResponse struct {
	TaskID string `json:"taskId"`
}

// AnalyzeDoneEvent / AnalyzeErrorEvent / AnalyzeCancelledEvent are emitted
// over Wails Event so the renderer doesn't have to poll.
type AnalyzeDoneEvent struct {
	TaskID string                   `json:"taskId"`
	Result profiler.AnalysisResult  `json:"result"`
}

type AnalyzeErrorEvent struct {
	TaskID  string `json:"taskId"`
	Message string `json:"message"`
}

type AnalyzeCancelledEvent struct {
	TaskID string `json:"taskId"`
}

type AnalyzeRequest struct {
	Path               string  `json:"path"`
	Format             string  `json:"format"`
	IntervalMS         float64 `json:"intervalMs"`
	ElapsedSec         float64 `json:"elapsedSec"`
	TopN               int     `json:"topN"`
	ProfileKind        string  `json:"profileKind"`
	TimelineBaseMethod string  `json:"timelineBaseMethod"`
	// DebugLog opts the renderer into the portable parser debug log
	// (T-239g). When true and the parse encounters issues, a JSON file is
	// written under DebugLogDir (default: <cwd>/archscope-debug/) and its
	// path is returned in the result metadata.
	DebugLog    bool   `json:"debugLog,omitempty"`
	DebugLogDir string `json:"debugLogDir,omitempty"`
	// Memory guards for very large collapsed/wall inputs. Empty/zero
	// triggers analyzer defaults (250k unique stacks, depth 256).
	MaxUniqueStacks int `json:"maxUniqueStacks,omitempty"`
	MaxStackDepth   int `json:"maxStackDepth,omitempty"`
	// TimelineCategories carries user-supplied additional method
	// patterns per timeline segment. Keys are segment IDs (e.g.
	// "EXTERNAL_CALL", "SQL_EXECUTION", "INTERNAL_METHOD"); values
	// are case-insensitive substrings matched against frame text.
	TimelineCategories map[string][]string `json:"timelineCategories,omitempty"`
}

// DebugLogResult is the response shape for ProfilerService.WriteDebugLog.
type DebugLogResult struct {
	OutputPath string `json:"outputPath"`
	Verdict    string `json:"verdict"`
	Verdicts   string `json:"verdicts,omitempty"`
}

func (s *ProfilerService) Analyze(req AnalyzeRequest) (profiler.AnalysisResult, error) {
	options, err := s.optionsFromRequest(req)
	if err != nil {
		return profiler.AnalysisResult{}, err
	}
	return s.runAnalyze(req, options)
}

// AnalyzeAsync starts a non-blocking analysis. Returns a taskId immediately;
// the renderer listens for `analyze:done` / `analyze:error` /
// `analyze:cancelled` events keyed by that taskId.
func (s *ProfilerService) AnalyzeAsync(req AnalyzeRequest) (AnalyzeAsyncResponse, error) {
	options, err := s.optionsFromRequest(req)
	if err != nil {
		return AnalyzeAsyncResponse{}, err
	}
	taskID := newTaskID("analyze")
	cancelCh := globalTasks.add(taskID)

	go func() {
		defer globalTasks.remove(taskID)
		result, runErr := s.runAnalyze(req, options)

		select {
		case <-cancelCh:
			emitEvent("analyze:cancelled", AnalyzeCancelledEvent{TaskID: taskID})
		default:
			if runErr != nil {
				emitEvent("analyze:error", AnalyzeErrorEvent{TaskID: taskID, Message: runErr.Error()})
				return
			}
			emitEvent("analyze:done", AnalyzeDoneEvent{TaskID: taskID, Result: result})
		}
	}()

	return AnalyzeAsyncResponse{TaskID: taskID}, nil
}

// Cancel signals a previously started AnalyzeAsync to discard its result.
// Returns true if the task existed; false if it had already finished.
func (s *ProfilerService) Cancel(taskID string) bool {
	return globalTasks.cancel(taskID)
}

func emitEvent(name string, data any) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(name, data)
}

type DrilldownRequest struct {
	AnalyzeRequest
	Filters []profiler.DrilldownFilter `json:"filters"`
}

type DiffRequest struct {
	BaselinePath   string  `json:"baselinePath"`
	TargetPath     string  `json:"targetPath"`
	BaselineFormat string  `json:"baselineFormat"`
	TargetFormat   string  `json:"targetFormat"`
	IntervalMS     float64 `json:"intervalMs"`
	TopN           int     `json:"topN"`
	Normalize      bool    `json:"normalize"`
}

type ExportPprofRequest struct {
	AnalyzeRequest
	OutputPath string `json:"outputPath"`
}

type ExportPprofResponse struct {
	OutputPath string `json:"outputPath"`
	Bytes      int    `json:"bytes"`
}

func (s *ProfilerService) Diff(req DiffRequest) (profiler.AnalysisResult, error) {
	if strings.TrimSpace(req.BaselinePath) == "" || strings.TrimSpace(req.TargetPath) == "" {
		return profiler.AnalysisResult{}, fmt.Errorf("both baselinePath and targetPath are required")
	}
	baselineStacks, err := s.loadStacks(req.BaselinePath, req.BaselineFormat)
	if err != nil {
		return profiler.AnalysisResult{}, fmt.Errorf("baseline: %w", err)
	}
	targetStacks, err := s.loadStacks(req.TargetPath, req.TargetFormat)
	if err != nil {
		return profiler.AnalysisResult{}, fmt.Errorf("target: %w", err)
	}
	return profiler.AnalyzeProfilerDiff(baselineStacks, targetStacks, req.BaselinePath, req.TargetPath, profiler.DiffOptions{
		Normalize: req.Normalize,
	}), nil
}

func (s *ProfilerService) loadStacks(path, format string) (map[string]int, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jennifer", "jennifer_csv", "jennifer-csv":
		result, err := profiler.ParseJenniferFlamegraphCSV(path, nil)
		if err != nil {
			return nil, err
		}
		return stacksFromFlameNode(result.Root), nil
	case "flamegraph_svg", "svg":
		result, err := profiler.ParseSvgFlamegraphFile(path, nil)
		if err != nil {
			return nil, err
		}
		return result.Stacks, nil
	case "flamegraph_html", "html":
		result, err := profiler.ParseHtmlProfilerFile(path, nil)
		if err != nil {
			return nil, err
		}
		return result.Stacks, nil
	case "", "collapsed":
		stacks, _, err := profiler.ParseCollapsedFile(path)
		if err != nil {
			return nil, err
		}
		return stacks, nil
	default:
		return nil, fmt.Errorf("unsupported format: %q", format)
	}
}

func stacksFromFlameNode(root profiler.FlameNode) map[string]int {
	// Walk leaf paths with exclusive samples — the flamegraph leaves carry
	// the diff-meaningful counts.
	stacks := map[string]int{}
	var walk func(node profiler.FlameNode, path []string)
	walk = func(node profiler.FlameNode, path []string) {
		next := path
		if len(node.Path) > 0 || node.Name != "All" {
			next = append(append([]string{}, path...), node.Name)
		}
		if len(node.Children) == 0 {
			if node.Samples > 0 && len(next) > 0 {
				stacks[strings.Join(next, ";")] += node.Samples
			}
			return
		}
		// Inclusive node — record self-time as exclusive samples.
		var childTotal int
		for _, child := range node.Children {
			childTotal += child.Samples
		}
		exclusive := node.Samples - childTotal
		if exclusive > 0 && len(next) > 0 {
			stacks[strings.Join(next, ";")] += exclusive
		}
		for _, child := range node.Children {
			walk(child, next)
		}
	}
	walk(root, nil)
	return stacks
}

func (s *ProfilerService) Drilldown(req DrilldownRequest) ([]profiler.DrilldownStage, error) {
	options, err := s.optionsFromRequest(req.AnalyzeRequest)
	if err != nil {
		return nil, err
	}
	result, err := s.runAnalyze(req.AnalyzeRequest, options)
	if err != nil {
		return nil, err
	}
	root := result.Charts.Flamegraph
	stages := profiler.BuildDrilldownStages(root, req.Filters, options.IntervalMS, options.ElapsedSec, options.TopN)
	return stages, nil
}

func (s *ProfilerService) optionsFromRequest(req AnalyzeRequest) (profiler.Options, error) {
	if strings.TrimSpace(req.Path) == "" {
		return profiler.Options{}, fmt.Errorf("path is required")
	}
	options := profiler.Options{
		IntervalMS:         req.IntervalMS,
		TopN:               req.TopN,
		ProfileKind:        req.ProfileKind,
		TimelineBaseMethod: req.TimelineBaseMethod,
		DebugLogDir:        req.DebugLogDir,
		MaxUniqueStacks:    req.MaxUniqueStacks,
		MaxStackDepth:      req.MaxStackDepth,
		TimelineCategories: req.TimelineCategories,
	}
	if req.ElapsedSec >= 0 {
		elapsed := req.ElapsedSec
		options.ElapsedSec = &elapsed
	}
	if req.DebugLog {
		analyzerType, parserName := s.debugLogContext(req)
		options.DebugLog = profiler.NewDebugLog(analyzerType, parserName, req.Path)
	}
	return options, nil
}

func (s *ProfilerService) debugLogContext(req AnalyzeRequest) (string, string) {
	switch strings.ToLower(strings.TrimSpace(req.Format)) {
	case "jennifer", "jennifer_csv", "jennifer-csv":
		return "profiler_jennifer", "jennifer_flamegraph_csv"
	case "flamegraph_svg", "svg":
		return "profiler_collapsed", "flamegraph_svg"
	case "flamegraph_html", "html":
		return "profiler_collapsed", "flamegraph_html"
	default:
		return "profiler_collapsed", "async_profiler_collapsed"
	}
}

// WriteDebugLog runs an analysis with the debug log enabled and returns the
// saved JSON path. Renderer uses this from the Diagnostics tab.
func (s *ProfilerService) WriteDebugLog(req AnalyzeRequest) (DebugLogResult, error) {
	req.DebugLog = true
	options, err := s.optionsFromRequest(req)
	if err != nil {
		return DebugLogResult{}, err
	}
	if _, runErr := s.runAnalyze(req, options); runErr != nil {
		// Capture the error in the debug log even when the analyzer fails
		// outright so the user has something to ship.
		if options.DebugLog != nil {
			options.DebugLog.AddException("analyze", runErr.Error(), "")
		} else {
			return DebugLogResult{}, runErr
		}
	}
	if options.DebugLog == nil {
		return DebugLogResult{}, fmt.Errorf("debug log was not initialized")
	}
	if !options.DebugLog.HasContent() {
		return DebugLogResult{Verdict: options.DebugLog.Verdict()}, nil
	}
	path, err := options.DebugLog.WriteJSON(options.DebugLogDir)
	if err != nil {
		return DebugLogResult{}, err
	}
	return DebugLogResult{OutputPath: path, Verdict: options.DebugLog.Verdict()}, nil
}

func (s *ProfilerService) runAnalyze(req AnalyzeRequest, options profiler.Options) (profiler.AnalysisResult, error) {
	switch strings.ToLower(strings.TrimSpace(req.Format)) {
	case "jennifer", "jennifer_csv", "jennifer-csv":
		return profiler.AnalyzeJenniferFile(req.Path, options)
	case "flamegraph_svg", "svg":
		return profiler.AnalyzeFlamegraphSVGFile(req.Path, options)
	case "flamegraph_html", "html":
		return profiler.AnalyzeFlamegraphHTMLFile(req.Path, options)
	case "", "collapsed":
		return profiler.AnalyzeCollapsedFile(req.Path, options)
	default:
		return profiler.AnalysisResult{}, fmt.Errorf("unsupported format: %q (expected 'collapsed' / 'jennifer' / 'flamegraph_svg' / 'flamegraph_html')", req.Format)
	}
}

func (s *ProfilerService) ExportPprof(req ExportPprofRequest) (ExportPprofResponse, error) {
	if strings.TrimSpace(req.Path) == "" {
		return ExportPprofResponse{}, fmt.Errorf("path is required")
	}
	out := strings.TrimSpace(req.OutputPath)
	if out == "" {
		picked, err := s.PickPprofOutput()
		if err != nil {
			return ExportPprofResponse{}, err
		}
		if picked == "" {
			return ExportPprofResponse{}, fmt.Errorf("export cancelled")
		}
		out = picked
	}
	stacks, err := s.loadStacks(req.Path, req.Format)
	if err != nil {
		return ExportPprofResponse{}, err
	}
	intervalMs := req.IntervalMS
	if intervalMs <= 0 {
		intervalMs = 100
	}
	if err := profiler.ExportToPprof(stacks, out, "samples", "count", intervalMs); err != nil {
		return ExportPprofResponse{}, err
	}
	info, err := os.Stat(out)
	if err != nil {
		return ExportPprofResponse{}, err
	}
	return ExportPprofResponse{OutputPath: out, Bytes: int(info.Size())}, nil
}

func (s *ProfilerService) PickPprofOutput() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("application not initialized")
	}
	dialog := app.Dialog.SaveFile().
		SetMessage("Save pprof profile").
		SetFilename("profile.pb.gz").
		AddFilter("pprof profile (.pb.gz)", "*.pb.gz").
		AddFilter("All files", "*.*")
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}

func (s *ProfilerService) PickProfileFile() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("application not initialized")
	}
	dialog := app.Dialog.OpenFile().
		SetTitle("Select profiler input").
		AddFilter("All profiler inputs", "*.collapsed;*.txt;*.csv;*.svg;*.html;*.htm").
		AddFilter("Collapsed stacks", "*.collapsed;*.txt").
		AddFilter("Jennifer flamegraph CSV", "*.csv").
		AddFilter("FlameGraph SVG", "*.svg").
		AddFilter("FlameGraph HTML", "*.html;*.htm").
		AddFilter("All files", "*.*")
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}
