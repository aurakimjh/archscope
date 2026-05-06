// `gc-log` group — mirrors typer's gc_log_app surface.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gclog"
)

func init() {
	group := &cobra.Command{
		Use:   "gc-log",
		Short: "GC log analysis commands.",
		Long:  "Analyze HotSpot unified GC logs. Mirrors the typer gc-log group.",
	}

	var (
		in       string
		topN     int
		maxLines int
		strict   bool
		out      string
	)

	analyze := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a HotSpot unified GC log.",
		Long:  "Parse a HotSpot unified GC log and emit an AnalysisResult JSON envelope.",
		Example: `  archscope-engine gc-log analyze \
    --in examples/gc-logs/sample-hotspot-gc.log \
    --out result.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := gclog.Analyze(in, gclog.Options{
				TopN:     topN,
				MaxLines: maxLines,
				Strict:   strict,
			})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	analyze.Flags().StringVar(&in, "in", "", "path to GC log (required)")
	analyze.Flags().IntVar(&topN, "top-n", 0, "top-N rows in tables.events (0 = analyzer default)")
	analyze.Flags().IntVar(&maxLines, "max-lines", 0, "stop after N lines (0 = unlimited)")
	analyze.Flags().BoolVar(&strict, "strict", false, "surface parser skips as fatal errors")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
