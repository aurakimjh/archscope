package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/stitching"
)

func init() {
	group := &cobra.Command{
		Use:   "stitch",
		Short: "Cross-source evidence stitching commands.",
		Long:  "Join AnalysisResult JSON files by trace, span, request, tenant, container, host, and process correlation keys.",
	}

	var inputs []string
	var out string
	var topN int
	var timeWindowSeconds int
	analyze := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze multiple result JSON files and emit stitched evidence.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(inputs) == 0 {
				return fmt.Errorf("--in is required at least once")
			}
			result, err := stitching.AnalyzeFiles(inputs, stitching.Options{TopN: topN, TimeWindowSeconds: timeWindowSeconds})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	analyze.Flags().StringArrayVar(&inputs, "in", nil, "input AnalysisResult JSON path; repeatable")
	analyze.Flags().IntVar(&topN, "top-n", 100, "maximum rows per stitched table")
	analyze.Flags().IntVar(&timeWindowSeconds, "time-window-seconds", 60, "timestamp window for service-alias stitching")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
