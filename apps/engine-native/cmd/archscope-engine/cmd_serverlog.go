package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/serverlog"
)

func init() {
	group := &cobra.Command{
		Use:   "server-log",
		Short: "Application and web-server log analysis commands.",
		Long:  "Analyze application-server and web-server error logs into the server_log AnalysisResult contract.",
	}

	var (
		in       string
		format   string
		maxLines int
		topN     int
		out      string
	)

	analyze := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a server log and write an AnalysisResult JSON file.",
		Example: `  archscope-engine server-log analyze \
    --in examples/server-logs/sample-tomcat.log \
    --format auto \
    --out result.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := serverlog.Analyze(in, format, serverlog.Options{
				MaxLines: maxLines,
				TopN:     topN,
			})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	analyze.Flags().StringVar(&in, "in", "", "path to server log (required)")
	analyze.Flags().StringVar(&format, "format", "auto", "log format selector")
	analyze.Flags().IntVar(&maxLines, "max-lines", 0, "stop after N lines (0 = unlimited)")
	analyze.Flags().IntVar(&topN, "top-n", serverlog.DefaultTopN, "maximum rows per summary table")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
