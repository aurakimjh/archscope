package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/traceimport"
)

func init() {
	group := &cobra.Command{
		Use:   "trace",
		Short: "External trace import analysis commands.",
		Long:  "Import portable trace exports such as OTLP JSON files and Zipkin v2 JSON into an AnalysisResult envelope.",
	}

	var (
		in     string
		format string
		topN   int
		out    string
	)

	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import and analyze an external trace export file.",
		Example: `  archscope-engine trace import \
    --in examples/traces/sample-otlp-traces.jsonl \
    --format otlp-json \
    --out result.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := traceimport.Analyze(in, traceimport.Options{
				Format: format,
				TopN:   topN,
			})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	importCmd.Flags().StringVar(&in, "in", "", "path to trace export file (required)")
	importCmd.Flags().StringVar(&format, "format", "auto", "input format: auto, otlp-json, zipkin-v2-json, elastic-apm-search-json, elastic-apm-source-ndjson")
	importCmd.Flags().IntVar(&topN, "top-n", 0, "top-N rows (0 = analyzer default)")
	importCmd.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(importCmd)
	rootCmd.AddCommand(group)
}
