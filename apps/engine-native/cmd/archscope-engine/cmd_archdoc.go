package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/archdoc"
)

func init() {
	group := &cobra.Command{
		Use:   "architecture-docs",
		Short: "Evidence-backed architecture documentation commands.",
		Long:  "Generate arc42 and ADR draft rows from existing AnalysisResult JSON evidence.",
	}

	var inputs []string
	var out string
	var topN int
	draft := &cobra.Command{
		Use:   "draft",
		Short: "Generate architecture documentation draft inputs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(inputs) == 0 {
				return fmt.Errorf("--in is required at least once")
			}
			result, err := archdoc.AnalyzeFiles(inputs, archdoc.Options{TopN: topN})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	draft.Flags().StringArrayVar(&inputs, "in", nil, "input AnalysisResult JSON path; repeatable")
	draft.Flags().IntVar(&topN, "top-n", 100, "maximum rows per architecture documentation table")
	draft.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
	group.AddCommand(draft)
	rootCmd.AddCommand(group)
}
