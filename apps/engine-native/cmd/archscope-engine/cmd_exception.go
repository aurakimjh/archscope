// `exception` group — mirrors typer's exception_app surface.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/exception"
)

func init() {
	group := &cobra.Command{
		Use:   "exception",
		Short: "Java exception stack analysis commands.",
		Long:  "Analyze Java exception stack traces. Mirrors the typer exception group.",
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
		Short: "Analyze Java exception stack traces.",
		Long:  "Parse a Java exception/stack file and emit an AnalysisResult JSON envelope.",
		Example: `  archscope-engine exception analyze \
    --in examples/exceptions/sample-java-exception.txt \
    --out result.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := exception.Analyze(in, exception.Options{
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
	analyze.Flags().StringVar(&in, "in", "", "path to exception/stack file (required)")
	analyze.Flags().IntVar(&topN, "top-n", 0, "top-N rows (0 = analyzer default)")
	analyze.Flags().IntVar(&maxLines, "max-lines", 0, "stop after N lines (0 = unlimited)")
	analyze.Flags().BoolVar(&strict, "strict", false, "surface parser skips as fatal errors")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
