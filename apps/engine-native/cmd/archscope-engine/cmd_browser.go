package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/lighthouse"
)

func init() {
	group := &cobra.Command{
		Use:   "browser",
		Short: "Browser performance evidence import commands.",
	}
	var in, out, format string
	var topN int
	var maxBytes int64
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import a Lighthouse report into the common AnalysisResult contract.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			if format != "lighthouse-json" && format != "auto" {
				return fmt.Errorf("unsupported browser evidence format %q", format)
			}
			result, err := lighthouse.Analyze(in, lighthouse.Options{TopN: topN, MaxBytes: maxBytes})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	importCmd.Flags().StringVar(&in, "in", "", "path to browser evidence file")
	importCmd.Flags().StringVar(&format, "format", "auto", "auto|lighthouse-json")
	importCmd.Flags().IntVar(&topN, "top-n", 50, "maximum rows per bounded table")
	importCmd.Flags().Int64Var(&maxBytes, "max-bytes", 64<<20, "maximum input bytes")
	importCmd.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
	group.AddCommand(importCmd)
	rootCmd.AddCommand(group)
}
