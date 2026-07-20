package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/httpcapture"
)

func init() {
	group := &cobra.Command{Use: "http-capture", Short: "HTTP capture evidence analysis commands."}
	var in, format, out string
	var topN, maxEntries int
	analyze := &cobra.Command{Use: "analyze", Short: "Import and analyze a redacted HAR file.", RunE: func(cmd *cobra.Command, args []string) error {
		if in == "" {
			return fmt.Errorf("--in is required")
		}
		result, err := httpcapture.Analyze(in, httpcapture.Options{Format: format, TopN: topN, MaxEntries: maxEntries})
		if err != nil {
			return err
		}
		return writeJSONResult(result, out)
	}}
	analyze.Flags().StringVar(&in, "in", "", "path to HAR file (required)")
	analyze.Flags().StringVar(&format, "format", "auto", "input format: auto, har")
	analyze.Flags().IntVar(&topN, "top-n", 0, "maximum rows per table")
	analyze.Flags().IntVar(&maxEntries, "max-entries", 0, "maximum HAR entries to import (0 = 100000)")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
