package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/databaselog"
)

func init() {
	group := &cobra.Command{Use: "database-log", Short: "Database slow-query and engine-log analysis commands."}
	var in, out, format string
	var topN int
	analyze := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze database slow-query, engine-log, profiler, slowlog, xevent, or EXPLAIN evidence.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := databaselog.Analyze(in, databaselog.Options{Format: format, TopN: topN})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	analyze.Flags().StringVar(&in, "in", "", "path to database evidence file")
	analyze.Flags().StringVar(&format, "format", "auto", "auto|postgres-text|postgres-csv|mysql-slow|mongodb-json|redis-slowlog|sqlserver-xevent-json|postgres-explain-json|mysql-explain-json")
	analyze.Flags().IntVar(&topN, "top-n", databaselog.DefaultTopN, "maximum rows per table")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
