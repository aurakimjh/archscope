package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
)

func init() {
	group := &cobra.Command{
		Use:   "profiler",
		Short: "Profiler analysis commands.",
		Long:  "Analyze collapsed stacks, FlameGraph SVG/HTML, and Jennifer APM CSV inputs.",
	}

	group.AddCommand(newProfilerAnalyzeCollapsedCommand())
	group.AddCommand(newProfilerAnalyzeSVGCommand())
	group.AddCommand(newProfilerAnalyzeHTMLCommand())
	group.AddCommand(newProfilerAnalyzeJenniferCommand())
	group.AddCommand(newProfilerDrilldownCommand())
	group.AddCommand(newProfilerBreakdownCommand())

	rootCmd.AddCommand(group)
}

type profilerFlags struct {
	in                 string
	collapsed          string
	jenniferCSV        string
	out                string
	intervalMS         float64
	elapsedSec         float64
	topN               int
	profileKind        string
	timelineBaseMethod string
	filterPatterns     []string
	filterType         string
	matchMode          string
	viewMode           string
	caseSensitive      bool
}

func newProfilerAnalyzeCollapsedCommand() *cobra.Command {
	var f profilerFlags
	cmd := &cobra.Command{
		Use:   "analyze-collapsed",
		Short: "Analyze an async-profiler collapsed stack file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := profiler.AnalyzeCollapsedFile(f.in, profilerOptions(f))
			if err != nil {
				return err
			}
			return writeJSONAny(result, f.out)
		},
	}
	addProfilerCommonFlags(cmd, &f)
	return cmd
}

func newProfilerAnalyzeSVGCommand() *cobra.Command {
	var f profilerFlags
	cmd := &cobra.Command{
		Use:   "analyze-flamegraph-svg",
		Short: "Analyze a FlameGraph SVG file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := profiler.AnalyzeFlamegraphSVGFile(f.in, profilerOptions(f))
			if err != nil {
				return err
			}
			return writeJSONAny(result, f.out)
		},
	}
	addProfilerCommonFlags(cmd, &f)
	return cmd
}

func newProfilerAnalyzeHTMLCommand() *cobra.Command {
	var f profilerFlags
	cmd := &cobra.Command{
		Use:   "analyze-flamegraph-html",
		Short: "Analyze an HTML-wrapped flamegraph.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := profiler.AnalyzeFlamegraphHTMLFile(f.in, profilerOptions(f))
			if err != nil {
				return err
			}
			return writeJSONAny(result, f.out)
		},
	}
	addProfilerCommonFlags(cmd, &f)
	return cmd
}

func newProfilerAnalyzeJenniferCommand() *cobra.Command {
	var f profilerFlags
	cmd := &cobra.Command{
		Use:   "analyze-jennifer-csv",
		Short: "Analyze a Jennifer APM flamegraph CSV file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := profiler.AnalyzeJenniferFile(f.in, profilerOptions(f))
			if err != nil {
				return err
			}
			return writeJSONAny(result, f.out)
		},
	}
	addProfilerCommonFlags(cmd, &f)
	return cmd
}

func newProfilerDrilldownCommand() *cobra.Command {
	var f profilerFlags
	cmd := &cobra.Command{
		Use:   "drilldown",
		Short: "Apply one or more profiler drill-down filters.",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, opts, err := analyzeProfilerInput(f)
			if err != nil {
				return err
			}
			stages := profiler.BuildDrilldownStages(result.Charts.Flamegraph, profilerFilters(f), opts.IntervalMS, opts.ElapsedSec, opts.TopN)
			return writeJSONAny(stages, f.out)
		},
	}
	addProfilerInputChoiceFlags(cmd, &f)
	addProfilerCommonOptionFlags(cmd, &f)
	addProfilerFilterFlags(cmd, &f)
	return cmd
}

func newProfilerBreakdownCommand() *cobra.Command {
	var f profilerFlags
	cmd := &cobra.Command{
		Use:   "breakdown",
		Short: "Analyze profiler input and emit execution breakdown sections.",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, _, err := analyzeProfilerInput(f)
			if err != nil {
				return err
			}
			return writeJSONAny(result, f.out)
		},
	}
	addProfilerInputChoiceFlags(cmd, &f)
	addProfilerCommonOptionFlags(cmd, &f)
	addProfilerFilterFlags(cmd, &f)
	return cmd
}

func addProfilerCommonFlags(cmd *cobra.Command, f *profilerFlags) {
	cmd.Flags().StringVar(&f.in, "in", "", "input path (required)")
	addProfilerCommonOptionFlags(cmd, f)
}

func addProfilerInputChoiceFlags(cmd *cobra.Command, f *profilerFlags) {
	cmd.Flags().StringVar(&f.collapsed, "wall", "", "collapsed stack input")
	cmd.Flags().StringVar(&f.jenniferCSV, "jennifer-csv", "", "Jennifer CSV input")
}

func addProfilerCommonOptionFlags(cmd *cobra.Command, f *profilerFlags) {
	cmd.Flags().StringVar(&f.out, "out", "-", "output path; `-` for stdout")
	cmd.Flags().Float64Var(&f.intervalMS, "interval-ms", 100, "sample interval in milliseconds")
	cmd.Flags().Float64Var(&f.intervalMS, "wall-interval-ms", 100, "collapsed wall interval in milliseconds")
	cmd.Flags().Float64Var(&f.elapsedSec, "elapsed-sec", -1, "elapsed seconds; negative means unset")
	cmd.Flags().IntVar(&f.topN, "top-n", 20, "top-N rows to emit")
	cmd.Flags().StringVar(&f.profileKind, "profile-kind", "wall", "profile mode: wall, cpu, or lock")
	cmd.Flags().StringVar(&f.timelineBaseMethod, "timeline-base-method", "", "optional base method for timeline analysis")
}

func addProfilerFilterFlags(cmd *cobra.Command, f *profilerFlags) {
	cmd.Flags().StringArrayVar(&f.filterPatterns, "filter", nil, "drill-down filter pattern; repeatable")
	cmd.Flags().StringVar(&f.filterType, "filter-type", "include_text", "include_text|exclude_text|regex_include|regex_exclude")
	cmd.Flags().StringVar(&f.matchMode, "match-mode", "anywhere", "anywhere|ordered|subtree")
	cmd.Flags().StringVar(&f.viewMode, "view-mode", "preserve_full_path", "preserve_full_path|reroot_at_match")
	cmd.Flags().BoolVar(&f.caseSensitive, "case-sensitive", false, "enable case-sensitive filter matching")
}

func analyzeProfilerInput(f profilerFlags) (profiler.AnalysisResult, profiler.Options, error) {
	if (f.collapsed == "") == (f.jenniferCSV == "") {
		return profiler.AnalysisResult{}, profiler.Options{}, fmt.Errorf("exactly one of --wall or --jennifer-csv is required")
	}
	opts := profilerOptions(f)
	if f.jenniferCSV != "" {
		result, err := profiler.AnalyzeJenniferFile(f.jenniferCSV, opts)
		return result, opts, err
	}
	result, err := profiler.AnalyzeCollapsedFile(f.collapsed, opts)
	return result, opts, err
}

func profilerOptions(f profilerFlags) profiler.Options {
	opts := profiler.Options{
		IntervalMS:         f.intervalMS,
		TopN:               f.topN,
		ProfileKind:        f.profileKind,
		TimelineBaseMethod: f.timelineBaseMethod,
	}
	if opts.IntervalMS <= 0 {
		opts.IntervalMS = 100
	}
	if opts.TopN <= 0 {
		opts.TopN = 20
	}
	if opts.ProfileKind == "" {
		opts.ProfileKind = "wall"
	}
	if f.elapsedSec >= 0 {
		elapsed := f.elapsedSec
		opts.ElapsedSec = &elapsed
	}
	return opts
}

func profilerFilters(f profilerFlags) []profiler.DrilldownFilter {
	filters := make([]profiler.DrilldownFilter, 0, len(f.filterPatterns))
	for _, pattern := range f.filterPatterns {
		filters = append(filters, profiler.DrilldownFilter{
			Pattern:       pattern,
			FilterType:    f.filterType,
			MatchMode:     f.matchMode,
			ViewMode:      f.viewMode,
			CaseSensitive: f.caseSensitive,
		})
	}
	return filters
}
