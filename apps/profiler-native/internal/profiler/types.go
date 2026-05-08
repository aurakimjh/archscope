package profiler

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
	// "EXTERNAL_CALL", "SQL_EXECUTION"); values are case-insensitive
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
}

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

type FlameNode struct {
	ID       string      `json:"id"`
	ParentID *string     `json:"parentId"`
	Name     string      `json:"name"`
	Samples  int         `json:"samples"`
	Ratio    float64     `json:"ratio"`
	Category *string     `json:"category"`
	Color    *string     `json:"color"`
	Children []FlameNode `json:"children"`
	Path     []string    `json:"path"`
	Metadata any         `json:"metadata,omitempty"`
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
