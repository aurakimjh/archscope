package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
)

func init() {
	group := &cobra.Command{
		Use:   "profile",
		Short: "Unified runtime profile evidence import commands.",
		Long:  "Import pprof, py-spy/rbspy raw, async-profiler HTML/collapsed, speedscope, StackProf, PHP, perf, Swift, Pyroscope, and Parca profile evidence.",
	}

	var in, out, format, profileKind string
	var topN int
	var intervalMS float64
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import runtime profile evidence into the common AnalysisResult contract.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := profile.Analyze(in, profile.Options{
				Format:      format,
				TopN:        topN,
				IntervalMS:  intervalMS,
				ProfileKind: profileKind,
			})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	importCmd.Flags().StringVar(&in, "in", "", "path to profile evidence file")
	importCmd.Flags().StringVar(&format, "format", "auto", "auto|pprof-gz|async-profiler-collapsed|async-profiler-html|pyspy-raw|rbspy-raw|speedscope-json|dotnet-speedscope-json|perf-collapsed|stackprof-json|php-excimer-json|php-tideways-json|xdebug-cachegrind|swift-backtrace|pyroscope-json|parca-json|v8-cpuprofile|chrome-trace-json")
	importCmd.Flags().IntVar(&topN, "top-n", 50, "maximum rows per table")
	importCmd.Flags().Float64Var(&intervalMS, "interval-ms", 100, "sample interval in milliseconds")
	importCmd.Flags().StringVar(&profileKind, "profile-kind", "", "profile kind override: wall, cpu, lock, memory, or samples")
	importCmd.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(importCmd)
	rootCmd.AddCommand(group)
}
