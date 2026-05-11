// ─────────────────────────────────────────────────────────────────────
// [한글] types — profiler 패키지 공통 타입 정의 + AnalysisResult 컨트랙트.
//
// 책임/목적
//   profiler 분석 파이프라인 전반에서 공유되는 입출력 타입을 한 곳에 모음.
//   AnalysisResult 가 외부와의 표준 컨트랙트(JSON shape) 이며 engine-native
//   의 다른 분석기와 같은 envelope (Type / SourceFiles / Summary / Series /
//   Tables / Charts / Metadata) 을 따른다.
//
// 핵심 타입
//   - Options              : 분석 입력 옵션 (interval, topN, profile kind,
//                            timeline base method, debug log)
//   - AnalysisResult       : 외부 표준 결과 envelope
//   - Summary              : 총 sample / interval / 추정 시간
//   - Series / Tables      : top stacks / breakdown / timeline rows
//   - Charts               : flame graph + drilldown stages
//   - Metadata             : parser / schema_version / diagnostics /
//                            timeline_scope (+ profiler_diff 전용 필드들)
//   - ParserDiagnostics    : 파싱 통계 + 경고/오류 sample
//   - DiagnosticSample     : 단일 진단 row
//   - FlameNode            : flame tree 의 노드 (UI 직접 렌더 가능 형태)
//   - DrilldownStage       : 드릴다운 한 단계 결과
//   - TimelineRow / Scope  : timeline 표 + scope metadata
//   - ExecutionBreakdownRow: execution breakdown 표 row
//   - leafPath             : 내부 helper (iterLeafPaths 결과)
//
// 트리키한 부분
//   • Metadata.DiffSummary / DiffTables 는 profiler_diff 전용 — 일반
//     profiler 분석에서는 omitempty 로 빠진다 (공통 envelope 유지).
//   • FlameNode.Metadata 는 any 타입. profiler_diff 가 {a,b,delta} 부착,
//     일반 profiler 는 nil. 이로써 UI 코드 경로가 단일.
//   • JSON 태그 이름은 Python 측과 byte-level 동등. 변경 시 frontend 가 깨짐.
// ─────────────────────────────────────────────────────────────────────

package profiler

// [한글] Options — 분석 진입 함수에 전달되는 옵션 묶음.
// IntervalMS<=0 / TopN<=0 / ProfileKind="" 면 normalizeOptions 에서 default 로
// 채워짐. ElapsedSec 가 nil 이면 elapsed_ratio 컬럼이 nil 로 남음.
type Options struct {
	IntervalMS         float64
	ElapsedSec         *float64
	TopN               int
	ProfileKind        string
	TimelineBaseMethod string
	DebugLog           *DebugLog
	DebugLogDir        string
	// MaxUniqueStacks caps the number of distinct stacks the
	// collapsed/HTML/SVG parsers retain in memory. Stacks beyond
	// the cap (lowest-sample first) are dropped and tracked under
	// ParserDiagnostics.DroppedStacks. 0 / negative means unlimited.
	// Sensible defaults are applied in normalizeOptions when zero.
	MaxUniqueStacks int
	// MaxStackDepth caps the number of frames per stack. Frames
	// beyond the cap are truncated and tracked under
	// ParserDiagnostics.OverDepthRecords. 0 / negative means unlimited.
	MaxStackDepth int
	// TimelineCategories carries user-supplied additional method
	// patterns per timeline segment. Keys are segment IDs (e.g.
	// "EXTERNAL_CALL", "SQL_EXECUTION", "DB_FETCH"); values are case-insensitive
	// substrings matched against frame text.
	TimelineCategories map[string][]string
	// ProgressLog, when non-nil, receives phase / progress / panic
	// lines during the parse + analyze pipeline. The renderer surfaces
	// the log path back to the user so a 400 MB wall analysis that
	// dies still leaves a tailable trail. Owned by the caller —
	// nothing inside the analyzer closes it.
	ProgressLog *ProgressLog
	// ProgressLogDir is consulted by AnalyzeCollapsedFile when
	// ProgressLog is nil; the analyzer auto-opens a log under this
	// directory so users get coverage without explicit wiring. Empty
	// → fall back to the executable's directory, then cwd, then
	// <os.TempDir>/archscope-logs.
	ProgressLogDir string
	// MaxRSSMB is the soft memory ceiling. The analyzer samples
	// runtime.MemStats during tree build; once Alloc exceeds the
	// limit it aborts with an error rather than letting the OS
	// SIGKILL the process. 0 / negative falls back to defaultMaxRSSMB.
	MaxRSSMB int
	// MaxFlamegraphNodes caps the size of the FlameNode tree shipped
	// to the renderer. Larger trees get pruned per level (top-K
	// children by samples) with a synthetic "…other" sibling holding
	// the dropped weight. 0 / negative falls back to default; -1
	// disables pruning entirely (use only on small inputs).
	MaxFlamegraphNodes int
}

// [한글] AnalysisResult — 외부 표준 결과 envelope.
// engine-native 의 다른 분석기와 동일한 6개 필드 (Type / SourceFiles /
// CreatedAt / Summary / Series / Tables / Charts / Metadata) 로 통일.
type AnalysisResult struct {
	Type        string   `json:"type"`
	SourceFiles []string `json:"source_files"`
	CreatedAt   string   `json:"created_at"`
	Summary     Summary  `json:"summary"`
	Series      Series   `json:"series"`
	Tables      Tables   `json:"tables"`
	Charts      Charts   `json:"charts"`
	Metadata    Metadata `json:"metadata"`
}

type Summary struct {
	ProfileKind      string   `json:"profile_kind"`
	TotalSamples     int      `json:"total_samples"`
	IntervalMS       float64  `json:"interval_ms"`
	EstimatedSeconds float64  `json:"estimated_seconds"`
	ElapsedSeconds   *float64 `json:"elapsed_seconds"`
}

type Series struct {
	TopStacks          []TopStackSeriesRow     `json:"top_stacks"`
	ComponentBreakdown []ComponentBreakdownRow `json:"component_breakdown"`
	ExecutionBreakdown []ExecutionBreakdownRow `json:"execution_breakdown"`
	TimelineAnalysis   []TimelineRow           `json:"timeline_analysis"`
	Threads            []ThreadRow             `json:"threads"`
}

type Tables struct {
	TopStacks        []TopStackTableRow `json:"top_stacks"`
	TopChildFrames   []TopChildFrameRow `json:"top_child_frames"`
	TimelineAnalysis []TimelineRow      `json:"timeline_analysis"`
}

type Charts struct {
	Flamegraph      FlameNode        `json:"flamegraph"`
	DrilldownStages []DrilldownStage `json:"drilldown_stages"`
}

type Metadata struct {
	Parser        string            `json:"parser"`
	SchemaVersion string            `json:"schema_version"`
	Diagnostics   ParserDiagnostics `json:"diagnostics"`
	TimelineScope TimelineScope     `json:"timeline_scope"`
	// ProgressLogPath, when non-empty, points to the on-disk
	// streaming log written during parse + analyze. Surfaced to the
	// renderer so the user has a tailable artifact when a 400 MB
	// wall analysis crashes.
	ProgressLogPath string `json:"progress_log_path,omitempty"`
	// Profiler-diff–specific payload. Empty for ordinary profiler results.
	DiffSummary map[string]any `json:"diff_summary,omitempty"`
	DiffTables  map[string]any `json:"diff_tables,omitempty"`
}

type ParserDiagnostics struct {
	SourceFile      *string            `json:"source_file"`
	Format          string             `json:"format"`
	TotalLines      int                `json:"total_lines"`
	ParsedRecords   int                `json:"parsed_records"`
	SkippedLines    int                `json:"skipped_lines"`
	SkippedByReason map[string]int     `json:"skipped_by_reason"`
	Samples         []DiagnosticSample `json:"samples"`
	WarningCount    int                `json:"warning_count"`
	ErrorCount      int                `json:"error_count"`
	Warnings        []DiagnosticSample `json:"warnings"`
	Errors          []DiagnosticSample `json:"errors"`
	// Memory-guard counters (T-WALL70). Populated when caps in
	// Options trigger; rendered in the parser-report tab so the user
	// can see exactly what the analyzer dropped to stay within budget.
	DroppedStacks      int    `json:"dropped_stacks,omitempty"`
	DroppedStackReason string `json:"dropped_stack_reason,omitempty"`
	OverDepthRecords   int    `json:"over_depth_records,omitempty"`
	MaxObservedDepth   int    `json:"max_observed_depth,omitempty"`
	BytesRead          int64  `json:"bytes_read,omitempty"`
}

type DiagnosticSample struct {
	LineNumber int    `json:"line_number"`
	Reason     string `json:"reason"`
	Message    string `json:"message"`
	RawPreview string `json:"raw_preview"`
}

type TopStackSeriesRow struct {
	Stack            string   `json:"stack"`
	Samples          int      `json:"samples"`
	EstimatedSeconds float64  `json:"estimated_seconds"`
	SampleRatio      float64  `json:"sample_ratio"`
	ElapsedRatio     *float64 `json:"elapsed_ratio"`
}

type TopStackTableRow struct {
	Stack            string   `json:"stack"`
	Samples          int      `json:"samples"`
	EstimatedSeconds float64  `json:"estimated_seconds"`
	SampleRatio      float64  `json:"sample_ratio"`
	ElapsedRatio     *float64 `json:"elapsed_ratio"`
	Frames           []string `json:"frames"`
}

type ComponentBreakdownRow struct {
	Component string `json:"component"`
	Samples   int    `json:"samples"`
}

type ExecutionBreakdownRow struct {
	Category         string    `json:"category"`
	ExecutiveLabel   string    `json:"executive_label"`
	PrimaryCategory  string    `json:"primary_category"`
	WaitReason       *string   `json:"wait_reason"`
	Samples          int       `json:"samples"`
	EstimatedSeconds float64   `json:"estimated_seconds"`
	TotalRatio       float64   `json:"total_ratio"`
	ParentStageRatio float64   `json:"parent_stage_ratio"`
	ElapsedRatio     *float64  `json:"elapsed_ratio"`
	TopMethods       []TopItem `json:"top_methods"`
	TopStacks        []TopItem `json:"top_stacks"`
}

type TopItem struct {
	Name    string `json:"name"`
	Samples int    `json:"samples"`
}

type TimelineChainRow struct {
	Chain   string   `json:"chain"`
	Samples int      `json:"samples"`
	Frames  []string `json:"frames"`
}

type TimelineRow struct {
	Index            int                `json:"index"`
	Segment          string             `json:"segment"`
	Label            string             `json:"label"`
	Samples          int                `json:"samples"`
	EstimatedSeconds float64            `json:"estimated_seconds"`
	StageRatio       float64            `json:"stage_ratio"`
	TotalRatio       float64            `json:"total_ratio"`
	ElapsedRatio     *float64           `json:"elapsed_ratio"`
	TopMethods       []TopItem          `json:"top_methods"`
	MethodChains     []TimelineChainRow `json:"method_chains"`
	TopStacks        []TopItem          `json:"top_stacks"`
}

type TimelineScope struct {
	Mode             string            `json:"mode"`
	BaseMethod       *string           `json:"base_method"`
	MatchMode        string            `json:"match_mode"`
	ViewMode         string            `json:"view_mode"`
	BaseSamples      int               `json:"base_samples"`
	TotalSamples     int               `json:"total_samples"`
	BaseRatioOfTotal *float64          `json:"base_ratio_of_total"`
	Warnings         []TimelineWarning `json:"warnings"`
}

type TimelineWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// [한글] FlameNode — flame tree 노드. UI 가 그대로 렌더 가능한 형태.
// Children 은 sample DESC 정렬되어 직렬화. Metadata 는 profiler_diff 등
// 부가 정보 부착 슬롯 (일반 분석에서는 nil).
type FlameNode struct {
	ID       string      `json:"id"`
	ParentID *string     `json:"parentId"`
	Name     string      `json:"name"`
	Samples  int         `json:"samples"`
	Ratio    float64     `json:"ratio"`
	Category *string     `json:"category"`
	Color    *string     `json:"color"`
	Children []FlameNode `json:"children"`
	// Path is dropped from JSON serialization. It's only consumed
	// internally by buildTimeline / iterLeafPaths during analysis;
	// the renderer reconstructs paths from the parent → child Name
	// chain, so shipping the slice over IPC is pure overhead. On
	// 1M+ node trees this single change drops the result-IPC
	// payload by ~1 GB.
	Path     []string `json:"-"`
	Metadata any      `json:"metadata,omitempty"`
}

type DrilldownStage struct {
	Index          int                `json:"index"`
	Label          string             `json:"label"`
	Breadcrumb     []string           `json:"breadcrumb"`
	Filter         any                `json:"filter"`
	Metrics        map[string]any     `json:"metrics"`
	Flamegraph     FlameNode          `json:"flamegraph"`
	TopStacks      []TopItem          `json:"top_stacks"`
	TopChildFrames []TopChildFrameRow `json:"top_child_frames"`
	Diagnostics    any                `json:"diagnostics"`
}

type TopChildFrameRow struct {
	Frame   string  `json:"frame"`
	Samples int     `json:"samples"`
	Ratio   float64 `json:"ratio"`
}

type ThreadRow struct {
	Name    string  `json:"name"`
	Samples int     `json:"samples"`
	Ratio   float64 `json:"ratio"`
}

type leafPath struct {
	Path    []string
	Samples int
}
