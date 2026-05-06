// `access-log` group — mirrors typer's access_log_app surface.
//
// Leaves
//   archscope-engine access-log analyze --in <path> [--format nginx]
//                                       [--max-lines N]
//                                       [--start-time RFC3339]
//                                       [--end-time RFC3339]
//                                       [--out <path>]
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/accesslog"
)

func init() {
	group := &cobra.Command{
		Use:   "access-log",
		Short: "Access log analysis commands.",
		Long:  "Analyze HTTP access logs (nginx, IIS, Apache). Mirrors the typer access-log group.",
	}

	var (
		in        string
		format    string
		maxLines  int
		startTime string
		endTime   string
		out       string
	)

	analyze := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze an access log and write an AnalysisResult JSON file.",
		Long: `Parse a HTTP access log (nginx-combined by default) and emit a
models.AnalysisResult envelope as JSON.`,
		Example: `  archscope-engine access-log analyze \
    --in examples/access-logs/sample-nginx-access.log \
    --format nginx \
    --out result.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			opts := accesslog.Options{MaxLines: maxLines}
			start, err := parseTimeFlag("start-time", startTime)
			if err != nil {
				return err
			}
			opts.StartTime = start
			end, err := parseTimeFlag("end-time", endTime)
			if err != nil {
				return err
			}
			opts.EndTime = end

			result, err := accesslog.Analyze(in, format, opts)
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	analyze.Flags().StringVar(&in, "in", "", "path to access log (required)")
	analyze.Flags().StringVar(&format, "format", "nginx", "log format label")
	analyze.Flags().IntVar(&maxLines, "max-lines", 0, "stop after N lines (0 = unlimited)")
	analyze.Flags().StringVar(&startTime, "start-time", "", "RFC3339 lower bound (inclusive)")
	analyze.Flags().StringVar(&endTime, "end-time", "", "RFC3339 upper bound (inclusive)")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
