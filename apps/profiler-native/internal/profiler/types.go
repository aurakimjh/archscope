package profiler

type Options struct {
	IntervalMS         float64
	ElapsedSec         *float64
	TopN               int
	ProfileKind        string
	TimelineBaseMethod string
	DebugLog           *DebugLog
	DebugLogDir        string
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
