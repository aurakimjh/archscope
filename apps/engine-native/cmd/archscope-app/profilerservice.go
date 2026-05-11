// ─────────────────────────────────────────────────────────────────────
// [한글] profilerservice — Wails 데스크톱 앱의 ProfilerService 바인딩.
//
// 책임/목적
//   renderer (React) 가 호출할 수 있도록 profiler 분석 4종(Analyze /
//   AnalyzeAsync / Diff / Drilldown), 옵션 모음(WriteDebugLog,
//   ExportPprof, PickProfileFile, PickPprofOutput, Cancel) 을 노출.
//
// 주요 구성
//   - taskRegistry  : taskID → cancel channel 매핑. AnalyzeAsync 가 등록.
//                     Cancel(taskID) 가 channel close 로 신호.
//                     실제 cancellation 정책: 작업이 µs–ms 수준이라 mid-flight
//                     중단 대신 "결과 폐기 + cancelled 이벤트 발행" 방식.
//   - newTaskID     : "<prefix>-<unix nano base36>-<counter base36>"
//   - emitEvent     : Wails Event 시스템 thin wrapper
//   - AnalyzeRequest / DiffRequest / DrilldownRequest / ExportPprofRequest:
//     renderer ↔ Go 사이의 JSON 컨트랙트 (TypeScript 측 타입과 1:1)
//
// Async 흐름
//   AnalyzeAsync(req) → taskID 즉시 반환 → goroutine 에서 runAnalyze →
//   cancel 신호 있으면 "analyze:cancelled", 에러면 "analyze:error",
//   정상이면 "analyze:done" + AnalysisResult 페이로드 emit.
//
// Diff 입력 정규화
//   loadStacks 가 입력 형식 별로 다른 parser 를 호출해 모두 stack map 으로
//   통일. Jennifer 는 트리 입력이라 stacksFromFlameNode 로 leaf 별 exclusive
//   sample 을 수집해 collapsed map 으로 환산.
//
// 트리키한 부분
//   • Cancel 은 channel 이미 close 된 경우에도 idempotent (select default).
//   • PickPprofOutput / PickProfileFile 은 Wails dialog 호출. application.
//     Get() 이 nil 이면 (테스트 등) 에러 반환.
//   • ExportPprof 는 outputPath 가 비어있으면 dialog 로 사용자에게 직접 묻고,
//     dialog 취소 시 "export cancelled" 에러로 응답.
//   • optionsFromRequest 가 ElapsedSec<0 인 경우 nil 로 만들어 주의.
// ─────────────────────────────────────────────────────────────────────

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
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
// ProgressLogPath is the on-disk progress log path that the analyzer is
// already writing to — surfaced right away so the renderer can show it
// even when the OS kills the process before the result event lands.
type AnalyzeAsyncResponse struct {
	TaskID          string `json:"taskId"`
	ProgressLogPath string `json:"progressLogPath,omitempty"`
}

// AnalyzeDoneEvent / AnalyzeErrorEvent / AnalyzeCancelledEvent are emitted
// over Wails Event so the renderer doesn't have to poll.
type AnalyzeDoneEvent struct {
	TaskID string                  `json:"taskId"`
	Result profiler.AnalysisResult `json:"result"`
}

type AnalyzeErrorEvent struct {
	TaskID          string `json:"taskId"`
	Message         string `json:"message"`
	ProgressLogPath string `json:"progressLogPath,omitempty"`
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
	// triggers analyzer defaults (100k unique stacks, depth 512, 4 GB
	// soft RSS ceiling).
	MaxUniqueStacks    int `json:"maxUniqueStacks,omitempty"`
	MaxStackDepth      int `json:"maxStackDepth,omitempty"`
	MaxRSSMB           int `json:"maxRssMb,omitempty"`
	MaxFlamegraphNodes int `json:"maxFlamegraphNodes,omitempty"`
	// ProgressLogDir overrides where the analyzer writes its
	// streaming progress log. Empty → next to the executable
	// (archscope-logs/), with cwd / temp-dir fallbacks.
	ProgressLogDir string `json:"progressLogDir,omitempty"`
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

// [한글] Analyze — 동기 분석. 호출 즉시 결과 반환. 결과가 작은 경우 또는
// 호출자가 progress 가 필요 없는 경우에 사용. 큰 입력은 AnalyzeAsync 추천.
func (s *ProfilerService) Analyze(req AnalyzeRequest) (profiler.AnalysisResult, error) {
	options, err := s.optionsFromRequest(req)
	if err != nil {
		return profiler.AnalysisResult{}, err
	}
	return s.runAnalyze(req, options)
}

// AnalyzeAsync starts a non-blocking analysis. Returns a taskId immediately;
// the renderer listens for `analyze:done` / `analyze:error` /
// `analyze:cancelled` events keyed by that taskId. The progress log is
// opened synchronously so its path is present in the response, giving
// the renderer something to show even if the OS kills the process
// before the result event arrives.
func (s *ProfilerService) AnalyzeAsync(req AnalyzeRequest) (AnalyzeAsyncResponse, error) {
	options, err := s.optionsFromRequest(req)
	if err != nil {
		return AnalyzeAsyncResponse{}, err
	}
	// Pre-open the progress log so we can return the path
	// immediately. The analyzer reuses this log instead of opening
	// its own on the auto-attach code path.
	if options.ProgressLog == nil {
		if pl, plErr := profiler.OpenProgressLog(options.ProgressLogDir, req.Path); plErr == nil {
			options.ProgressLog = pl
		}
	}
	taskID := newTaskID("analyze")
	cancelCh := globalTasks.add(taskID)
	logPath := ""
	if options.ProgressLog != nil {
		logPath = options.ProgressLog.Path()
	}

	go func() {
		defer globalTasks.remove(taskID)
		if options.ProgressLog != nil {
			defer options.ProgressLog.Close()
		}
		result, runErr := s.runAnalyze(req, options)

		select {
		case <-cancelCh:
			emitEvent("analyze:cancelled", AnalyzeCancelledEvent{TaskID: taskID})
		default:
			if runErr != nil {
				emitEvent("analyze:error", AnalyzeErrorEvent{
					TaskID:          taskID,
					Message:         runErr.Error(),
					ProgressLogPath: logPath,
				})
				return
			}
			emitEvent("analyze:done", AnalyzeDoneEvent{TaskID: taskID, Result: result})
		}
	}()

	return AnalyzeAsyncResponse{TaskID: taskID, ProgressLogPath: logPath}, nil
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

// [한글] Diff — baseline/target 두 파일을 비교 분석.
// 형식별 parser → stack map 통일 → AnalyzeProfilerDiff 호출.
// Normalize=true 면 양쪽을 자기 합계로 정규화(권장 default).
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

// [한글] Drilldown — 분석 + drilldown 필터 시퀀스 적용. 결과 stages 반환.
// 보통 분석은 한 번 캐시해두는 게 효율적이지만, 단순화를 위해 매 호출마다 재분석.
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
		MaxRSSMB:           req.MaxRSSMB,
		MaxFlamegraphNodes: req.MaxFlamegraphNodes,
		TimelineCategories: req.TimelineCategories,
		ProgressLogDir:     req.ProgressLogDir,
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
//
// [한글] WriteDebugLog — debug log 강제 ON 으로 분석 실행 후 JSON 파일 기록.
// 분석 자체가 실패해도 exception 을 debug log 에 추가 후 (가능하면) 기록.
// 진단할 내용이 없으면 outputPath 비운 채 verdict 만 반환.
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

// [한글] ExportPprof — collapsed map 을 pprof.gz 로 export.
// outputPath 비어있으면 SaveFile dialog 로 사용자에게 묻고, 취소 시 에러.
// intervalMs<=0 면 100ms 디폴트.
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
