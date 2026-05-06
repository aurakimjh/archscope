// `otel` group — mirrors typer's otel_app surface.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/otel"
)

func init() {
	group := &cobra.Command{
		Use:   "otel",
		Short: "OpenTelemetry input analysis commands.",
		Long:  "Analyze OpenTelemetry JSONL log exports. Mirrors the typer otel group.",
	}

	var (
		in   string
		topN int
		out  string
	)

	analyze := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze line-delimited OpenTelemetry JSON logs.",
		Long:  "Parse line-delimited OpenTelemetry JSON logs and emit an AnalysisResult JSON envelope.",
		Example: `  archscope-engine otel analyze \
    --in examples/otel/sample-otel-logs.jsonl \
    --out result.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := otel.Analyze(in, otel.Options{TopN: topN})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	analyze.Flags().StringVar(&in, "in", "", "path to OTel JSONL logs (required)")
	analyze.Flags().IntVar(&topN, "top-n", 0, "top-N rows (0 = analyzer default)")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
