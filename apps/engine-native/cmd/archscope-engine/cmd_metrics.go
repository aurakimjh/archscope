package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/observability"
)

func init() {
	metricsGroup := &cobra.Command{Use: "metrics", Short: "Offline metrics snapshot import commands."}
	var metricsIn, metricsOut string
	var metricsTopN, metricsMaxLines int
	importMetrics := &cobra.Command{
		Use:   "import",
		Short: "Import Prometheus/OpenMetrics text evidence.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if metricsIn == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := metrics.Analyze(metricsIn, metrics.Options{TopN: metricsTopN, MaxLines: metricsMaxLines})
			if err != nil {
				return err
			}
			return writeJSONResult(result, metricsOut)
		},
	}
	importMetrics.Flags().StringVar(&metricsIn, "in", "", "path to Prometheus/OpenMetrics text file")
	importMetrics.Flags().StringVar(&metricsOut, "out", "-", "output path; `-` for stdout")
	importMetrics.Flags().IntVar(&metricsTopN, "top-n", metrics.DefaultTopN, "maximum rows per table")
	importMetrics.Flags().IntVar(&metricsMaxLines, "max-lines", 0, "stop after N lines (0 = unlimited)")
	metricsGroup.AddCommand(importMetrics)
	rootCmd.AddCommand(metricsGroup)

	obsGroup := &cobra.Command{Use: "observability", Short: "LGTM and Grafana export import commands."}
	var obsIn, obsOut, obsFormat string
	var obsTopN int
	importObs := &cobra.Command{
		Use:   "import",
		Short: "Import Loki, Tempo, or Grafana dashboard JSON exports.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if obsIn == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := observability.Analyze(obsIn, observability.Options{Format: obsFormat, TopN: obsTopN})
			if err != nil {
				return err
			}
			return writeJSONResult(result, obsOut)
		},
	}
	importObs.Flags().StringVar(&obsIn, "in", "", "path to Loki/Tempo/Grafana JSON file")
	importObs.Flags().StringVar(&obsFormat, "format", "auto", "auto|loki-json|tempo-json|grafana-dashboard-json")
	importObs.Flags().StringVar(&obsOut, "out", "-", "output path; `-` for stdout")
	importObs.Flags().IntVar(&obsTopN, "top-n", 50, "maximum rows per table")
	obsGroup.AddCommand(importObs)
	rootCmd.AddCommand(obsGroup)
}
