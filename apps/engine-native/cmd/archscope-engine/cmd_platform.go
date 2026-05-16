package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/platform"
)

func init() {
	group := &cobra.Command{Use: "platform", Short: "Kubernetes, container, and cloud audit evidence import commands."}
	var in, out, format string
	var topN int
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import Kubernetes events/pods, kubelet/runtime logs, or cloud audit JSON.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := platform.Analyze(in, platform.Options{Format: format, TopN: topN})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	importCmd.Flags().StringVar(&in, "in", "", "path to platform evidence file")
	importCmd.Flags().StringVar(&format, "format", "auto", "auto|kubectl-events-json|describe-pod-json|kubelet-log|container-runtime-log|aws-cloudtrail-json|gcp-audit-json|azure-activity-json")
	importCmd.Flags().IntVar(&topN, "top-n", 50, "maximum rows per table")
	importCmd.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
	group.AddCommand(importCmd)
	rootCmd.AddCommand(group)
}
