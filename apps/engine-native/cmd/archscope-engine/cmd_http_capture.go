package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/httpcapture"
)

func init() {
	group := &cobra.Command{Use: "http-capture", Short: "HTTP capture evidence analysis commands."}
	var in, format, out string
	var topN, maxEntries, maxStringBytes, maxBodyBytes, maxDepth, maxFields, maxDecompressionRatio int
	var maxBytes int64
	var customRedactionPatterns []string
	analyze := &cobra.Command{Use: "analyze", Short: "Import and analyze a redacted HAR file.", RunE: func(cmd *cobra.Command, args []string) error {
		if in == "" {
			return fmt.Errorf("--in is required")
		}
		result, err := httpcapture.Analyze(in, httpcapture.Options{
			Format: format, TopN: topN, MaxEntries: maxEntries, MaxBytes: maxBytes,
			MaxStringBytes: maxStringBytes, MaxBodyBytes: maxBodyBytes, MaxDepth: maxDepth,
			MaxFields: maxFields, MaxDecompressionRatio: maxDecompressionRatio,
			CustomRedactionPatterns: customRedactionPatterns,
		})
		if err != nil {
			return err
		}
		return writeJSONResult(result, out)
	}}
	analyze.Flags().StringVar(&in, "in", "", "path to HAR file (required)")
	analyze.Flags().StringVar(&format, "format", "auto", "input format: auto, har")
	analyze.Flags().IntVar(&topN, "top-n", 0, "maximum rows per table")
	analyze.Flags().IntVar(&maxEntries, "max-entries", 0, "maximum HAR entries to import (0 = 100000)")
	analyze.Flags().Int64Var(&maxBytes, "max-bytes", 0, "maximum input/decompressed bytes (0 = 256 MiB)")
	analyze.Flags().IntVar(&maxStringBytes, "max-string-bytes", 0, "maximum JSON string bytes (0 = 16 MiB)")
	analyze.Flags().IntVar(&maxBodyBytes, "max-body-bytes", 0, "maximum body bytes per entry (0 = 10 MiB)")
	analyze.Flags().IntVar(&maxDepth, "max-depth", 0, "maximum JSON nesting depth (0 = 256)")
	analyze.Flags().IntVar(&maxFields, "max-fields", 0, "maximum JSON field count (0 = 2000000)")
	analyze.Flags().IntVar(&maxDecompressionRatio, "max-decompression-ratio", 0, "maximum gzip expansion ratio (0 = 1000)")
	analyze.Flags().StringSliceVar(&customRedactionPatterns, "redact-pattern", nil, "additional bounded RE2 redaction pattern (repeatable)")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
