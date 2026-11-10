package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/httpcapture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/acceptance"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/halog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
)

func init() {
	group := &cobra.Command{Use: "http-capture", Short: "HTTP capture evidence analysis commands."}
	var in, format, out string
	var topN, transactionRows, maxEntries, maxStringBytes, maxBodyBytes, maxDepth, maxFields, maxDecompressionRatio int
	var maxBytes int64
	var customRedactionPatterns []string
	analyze := &cobra.Command{Use: "analyze", Short: "Import and analyze a redacted HAR or native HALOG file.", RunE: func(cmd *cobra.Command, args []string) error {
		if in == "" {
			return fmt.Errorf("--in is required")
		}
		result, err := httpcapture.Analyze(in, httpcapture.Options{
			Format: format, TopN: topN, TransactionRows: transactionRows,
			MaxEntries: maxEntries, MaxBytes: maxBytes,
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
	analyze.Flags().StringVar(&format, "format", "auto", "input format: auto, har, halog")
	analyze.Flags().IntVar(&topN, "top-n", 0, "maximum rows per aggregate table (endpoints, hosts, slowest, errors)")
	analyze.Flags().IntVar(&transactionRows, "transaction-rows", 0, "maximum rows in the full transaction list (0 = 20000)")
	analyze.Flags().IntVar(&maxEntries, "max-entries", 0, "maximum HAR entries to import (0 = 100000)")
	analyze.Flags().Int64Var(&maxBytes, "max-bytes", 0, "maximum input/decompressed bytes (0 = 256 MiB)")
	analyze.Flags().IntVar(&maxStringBytes, "max-string-bytes", 0, "maximum JSON string bytes (0 = 16 MiB)")
	analyze.Flags().IntVar(&maxBodyBytes, "max-body-bytes", 0, "maximum body bytes per entry (0 = 10 MiB)")
	analyze.Flags().IntVar(&maxDepth, "max-depth", 0, "maximum JSON nesting depth (0 = 256)")
	analyze.Flags().IntVar(&maxFields, "max-fields", 0, "maximum JSON field count (0 = 2000000)")
	analyze.Flags().IntVar(&maxDecompressionRatio, "max-decompression-ratio", 0, "maximum gzip expansion ratio (0 = 1000)")
	analyze.Flags().StringSliceVar(&customRedactionPatterns, "redact-pattern", nil, "additional bounded RE2 redaction pattern (repeatable)")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
	var sessionPath, evidenceOut string
	evidence := &cobra.Command{
		Use:   "acceptance-evidence",
		Short: "Read bounded T-581 evidence from a stopped live-capture session.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionPath == "" {
				return fmt.Errorf("--session-path is required")
			}
			result, err := acceptance.Build(sessionPath)
			if err != nil {
				return err
			}
			return writePrivateJSONAny(result, evidenceOut)
		},
	}
	evidence.Flags().StringVar(&sessionPath, "session-path", "", "path to a stopped live-capture session")
	evidence.Flags().StringVar(&evidenceOut, "out", "-", "output path; `-` for stdout")
	var nativePath, xmlPath, probeOut string
	probe := &cobra.Command{Use: "halog-probe", Short: "Fingerprint unverified legacy logs; never claim codec compatibility.", RunE: func(cmd *cobra.Command, args []string) error {
		if nativePath == "" {
			return fmt.Errorf("--in is required")
		}
		if xmlPath != "" {
			pair, err := halog.ProbePair(nativePath, xmlPath)
			if err != nil {
				return err
			}
			return writePrivateJSONAny(pair, probeOut)
		}
		value, err := halog.Probe(nativePath)
		if err != nil {
			return err
		}
		return writePrivateJSONAny(value, probeOut)
	}}
	probe.Flags().StringVar(&nativePath, "in", "", "legacy file path")
	probe.Flags().StringVar(&xmlPath, "reference-xml", "", "optional full XML saved from the same reference capture")
	probe.Flags().StringVar(&probeOut, "out", "-", "metadata-only evidence destination")
	var convertIn, convertOut, convertFormat string
	convert := &cobra.Command{Use: "convert", Short: "Convert redacted HTTP sessions, HAR or native HALOG.", RunE: func(cmd *cobra.Command, args []string) error {
		if convertIn == "" || convertOut == "" {
			return fmt.Errorf("--in and --out are required")
		}
		doc, _, err := sessionfile.Read(convertIn)
		if err != nil {
			return err
		}
		return sessionfile.Write(context.Background(), convertOut, doc, convertFormat)
	}}
	convert.Flags().StringVar(&convertIn, "in", "", "source session, HAR or native HALOG")
	convert.Flags().StringVar(&convertOut, "out", "", "destination path")
	convert.Flags().StringVar(&convertFormat, "format", "session", "session, har, native, xml, text or other supported export format")
	group.AddCommand(analyze, evidence, probe, convert)
	rootCmd.AddCommand(group)
}
